from __future__ import annotations

import hashlib
import io
import json
import stat
import threading
from collections.abc import Iterator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, cast

import pytest
from scripts.artifacts import nutanix

FIXTURES = Path("tests/fixtures/nutanix")


class FixtureServer(ThreadingHTTPServer):
    routes: dict[str, tuple[int, str, bytes, dict[str, str]]]
    methods: list[str]


class FixtureHandler(BaseHTTPRequestHandler):
    server: FixtureServer

    def do_GET(self) -> None:
        self.server.methods.append("GET")
        route = self.server.routes.get(self.path)
        if route is None:
            self.send_error(404)
            return
        status, content_type, body, headers = route
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        for name, value in headers.items():
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(body)

    def do_HEAD(self) -> None:
        self.server.methods.append("HEAD")
        self.send_error(405)

    def log_message(self, format: str, *args: object) -> None:
        del format, args


def _fixture(name: str) -> bytes:
    return (FIXTURES / name).read_bytes()


def _json_fixture(name: str) -> Any:
    return json.loads(_fixture(name))


@contextmanager
def fixture_server(
    *,
    overrides: dict[str, tuple[int, str, bytes, dict[str, str]]] | None = None,
) -> Iterator[tuple[str, FixtureServer]]:
    server = FixtureServer(("127.0.0.1", 0), FixtureHandler)
    root = f"http://127.0.0.1:{server.server_port}/api/v1/"
    versions = cast(dict[str, Any], _json_fixture("versions.json"))

    def encoded(value: Any) -> bytes:
        return json.dumps(value).replace("__BASE__/", root).encode()

    routes: dict[str, tuple[int, str, bytes, dict[str, str]]] = {
        "/api/v1/namespaces/": (
            200,
            "application/json; charset=utf-8",
            _fixture("registry.json"),
            {},
        ),
        "/api/v1/namespaces/alpha/versions/": (
            200,
            "application/vnd.nutanix+json",
            encoded(versions["alpha"]),
            {},
        ),
        "/api/v1/namespaces/storage/versions/": (
            200,
            "application/json",
            encoded(versions["storage"]),
            {},
        ),
        "/api/v1/namespaces/alpha/versions/v4.1/yaml": (
            200,
            "application/yaml",
            _fixture("openapi.yaml"),
            {},
        ),
        "/api/v1/namespaces/alpha/versions/v4.1/postman-collection": (
            200,
            "application/json",
            _fixture("postman.json"),
            {},
        ),
        "/api/v1/namespaces/alpha/versions/v4.1/locale/en_US/error": (
            200,
            "application/json",
            _fixture("errors.json"),
            {},
        ),
        "/api/v1/namespaces/storage/versions/v4.0.a3/yaml": (
            200,
            "text/plain; charset=utf-8",
            _fixture("openapi.yaml"),
            {},
        ),
        "/api/v1/namespaces/storage/versions/v4.0.a3/locale/en_US/error": (
            200,
            "application/json",
            _fixture("errors.json"),
            {},
        ),
    }
    routes.update(overrides or {})
    server.routes = routes
    server.methods = []
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield root, server
    finally:
        server.shutdown()
        thread.join()
        server.server_close()


def test_discover_selects_all_namespaces_and_prefers_newest_ga(tmp_path: Path) -> None:
    with fixture_server() as (base_url, server):
        manifest = tmp_path / "manifest.json"
        cache = tmp_path / "cache"
        discovered = nutanix.discover(
            base_url=base_url,
            manifest_path=manifest,
            cache_path=cache,
        )

    assert [(entry["name"], entry["version"], entry["stability"]) for entry in discovered] == [
        ("alpha", "v4.1", "ga"),
        ("storage", "v4.0.a3", "preview"),
    ]
    assert not manifest.exists()
    assert not cache.exists()
    assert server.methods and set(server.methods) == {"GET"}


def test_update_locks_shape_size_digest_and_uses_atomic_replace(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    replacements: list[tuple[Path, Path]] = []
    synchronized_types: list[bool] = []
    real_replace = nutanix.os.replace
    real_fsync = nutanix.os.fsync

    def replace(source: str | Path, destination: str | Path) -> None:
        replacements.append((Path(source), Path(destination)))
        real_replace(source, destination)

    def fsync(descriptor: int) -> None:
        synchronized_types.append(stat.S_ISDIR(nutanix.os.fstat(descriptor).st_mode))
        real_fsync(descriptor)

    monkeypatch.setattr(nutanix.os, "replace", replace)
    monkeypatch.setattr(nutanix.os, "fsync", fsync)
    with fixture_server() as (base_url, server):
        manifest_path = tmp_path / "specs" / "manifest.json"
        cache_path = tmp_path / "cache"
        manifest = nutanix.update(
            base_url=base_url,
            manifest_path=manifest_path,
            cache_path=cache_path,
        )

    assert manifest["schema_version"] == 1
    assert manifest["source"] == base_url
    assert len(manifest["namespaces"]) == 2
    assert manifest_path.read_text().endswith("\n")
    assert json.loads(manifest_path.read_text()) == manifest
    assert replacements
    assert all(source.parent == destination.parent for source, destination in replacements)
    assert False in synchronized_types
    assert True in synchronized_types
    assert not list(tmp_path.rglob("*.tmp"))
    assert set(server.methods) == {"GET"}

    alpha = manifest["namespaces"][0]
    openapi = alpha["artifacts"]["openapi"]
    body = _fixture("openapi.yaml")
    assert openapi["bytes"] == len(body)
    assert openapi["sha256"] == hashlib.sha256(body).hexdigest()
    assert openapi["media_type"] == "application/yaml"
    assert (cache_path / openapi["path"]).read_bytes() == body


def test_verify_rechecks_live_selection_and_rejects_digest_mismatch(tmp_path: Path) -> None:
    with fixture_server() as (base_url, _):
        manifest_path = tmp_path / "manifest.json"
        cache_path = tmp_path / "cache"
        manifest = nutanix.update(
            base_url=base_url,
            manifest_path=manifest_path,
            cache_path=cache_path,
        )
        assert (
            nutanix.verify(
                base_url=base_url,
                manifest_path=manifest_path,
                cache_path=cache_path,
            )
            == 2
        )

        manifest["namespaces"][0]["artifacts"]["openapi"]["sha256"] = "0" * 64
        manifest_path.write_text(json.dumps(manifest))
        with pytest.raises(nutanix.ArtifactError, match="digest mismatch"):
            nutanix.verify(
                base_url=base_url,
                manifest_path=manifest_path,
                cache_path=cache_path,
            )


@pytest.mark.parametrize(
    ("kind", "media_type", "body"),
    [
        ("openapi", "application/x-yaml", _fixture("openapi.yaml")),
        ("openapi", "text/yaml", _fixture("openapi.yaml")),
        ("postman", "application/problem+json", _fixture("postman.json")),
        ("postman", "text/plain; charset=utf-8", _fixture("postman.json")),
        ("errors", "application/json; charset=utf-8", _fixture("errors.json")),
    ],
)
def test_artifact_validation_accepts_published_media_types(
    kind: str, media_type: str, body: bytes
) -> None:
    assert nutanix.validate_artifact(kind, media_type, body)


@pytest.mark.parametrize(
    ("kind", "media_type", "body", "message"),
    [
        ("openapi", "text/html", b"<html></html>", "media type"),
        ("openapi", "application/yaml", b"swagger: '2.0'\n", "OpenAPI 3"),
        ("postman", "application/json", b"[]", "Postman"),
        ("postman", "application/json", b'{"item": "bad"}', "Postman"),
        ("errors", "application/json", b"{}", "error reference"),
        ("errors", "application/json", b"{", "JSON"),
    ],
)
def test_artifact_validation_rejects_media_type_or_shape(
    kind: str, media_type: str, body: bytes, message: str
) -> None:
    with pytest.raises(nutanix.ArtifactError, match=message):
        nutanix.validate_artifact(kind, media_type, body)


@pytest.mark.parametrize(
    ("namespace", "version"),
    [
        ("../escape", "v4.0"),
        ("alpha/child", "v4.0"),
        ("alpha", "../../v4.0"),
        ("alpha", "v4.0%2fescape"),
    ],
)
def test_cache_path_rejects_unsafe_namespace_or_version(
    tmp_path: Path, namespace: str, version: str
) -> None:
    with pytest.raises(nutanix.ArtifactError, match="unsafe"):
        nutanix.cache_file(tmp_path, namespace, version, "openapi")


def test_external_redirect_is_rejected_before_follow(tmp_path: Path) -> None:
    redirect = (
        302,
        "text/plain",
        b"",
        {"Location": "https://example.com/api/v1/namespaces/"},
    )
    with fixture_server(overrides={"/api/v1/namespaces/": redirect}) as (base_url, _):
        with pytest.raises(nutanix.ArtifactError, match="outside allowed API prefix"):
            nutanix.discover(
                base_url=base_url,
                manifest_path=tmp_path / "manifest.json",
                cache_path=tmp_path / "cache",
            )


def test_remote_url_diagnostic_does_not_echo_untrusted_url() -> None:
    secret = "do-not-log-this-token"
    url = f"https://developers.nutanix.com/api/v1/namespaces/?token={secret}"

    with pytest.raises(nutanix.ArtifactError) as caught:
        nutanix._validate_url(url, nutanix.DEFAULT_BASE_URL)

    assert secret not in str(caught.value)
    assert url not in str(caught.value)


@pytest.mark.parametrize("invalid", [False, 0, [], {}])
def test_postman_link_rejects_present_invalid_values(invalid: object) -> None:
    entry = {
        "link": "https://developers.nutanix.com/api/v1/namespaces/vmm/yaml",
        "postmanCollectionLink": invalid,
    }

    with pytest.raises(nutanix.ArtifactError, match="invalid Postman URL"):
        nutanix._artifact_urls(
            entry,
            namespace="vmm",
            version="v4.2",
            base_url=nutanix.DEFAULT_BASE_URL,
        )


def test_registry_and_version_diagnostics_are_actionable_without_body(
    tmp_path: Path,
) -> None:
    with fixture_server(
        overrides={
            "/api/v1/namespaces/": (200, "application/json", b"{", {}),
        }
    ) as (base_url, _):
        with pytest.raises(nutanix.ArtifactError) as caught:
            nutanix.discover(
                base_url=base_url,
                manifest_path=tmp_path / "manifest.json",
                cache_path=tmp_path / "cache",
            )

    message = str(caught.value)
    assert "registry" in message
    assert "JSON" in message
    assert "{" not in message


def test_version_array_rejects_non_object_entries(tmp_path: Path) -> None:
    body = json.dumps({"namespace": "alpha", "versions": ["v4.1"]}).encode()
    override = {
        "/api/v1/namespaces/alpha/versions/": (
            200,
            "application/json",
            body,
            {},
        )
    }
    with fixture_server(overrides=override) as (base_url, _):
        with pytest.raises(nutanix.ArtifactError, match="version entry must be an object"):
            nutanix.discover(
                base_url=base_url,
                manifest_path=tmp_path / "manifest.json",
                cache_path=tmp_path / "cache",
            )


def test_artifact_diagnostic_identifies_namespace_version_and_kind(tmp_path: Path) -> None:
    override = {
        "/api/v1/namespaces/alpha/versions/v4.1/postman-collection": (
            200,
            "text/html",
            b"<html>not a collection</html>",
            {},
        )
    }
    with fixture_server(overrides=override) as (base_url, _):
        with pytest.raises(
            nutanix.ArtifactError,
            match=r"alpha v4\.1 postman: .*media type",
        ):
            nutanix.update(
                base_url=base_url,
                manifest_path=tmp_path / "manifest.json",
                cache_path=tmp_path / "cache",
            )


def test_bounded_read_rejects_oversized_body(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(nutanix, "MAX_RESPONSE_BYTES", 8)
    with fixture_server() as (base_url, _):
        with pytest.raises(nutanix.ArtifactError, match="exceeds"):
            nutanix.discover(
                base_url=base_url,
                manifest_path=tmp_path / "manifest.json",
                cache_path=tmp_path / "cache",
            )


def test_count_prints_decimal_namespace_count(tmp_path: Path) -> None:
    manifest = {
        "schema_version": 1,
        "source": "https://developers.nutanix.com/api/v1/",
        "namespaces": [{}, {}, {}],
    }
    path = tmp_path / "manifest.json"
    path.write_text(json.dumps(manifest))
    stdout = io.StringIO()

    assert nutanix.main(("count", "--manifest", str(path)), stdout=stdout) == 0
    assert stdout.getvalue() == "3\n"
