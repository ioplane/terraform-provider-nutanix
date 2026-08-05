"""Discover, lock, and verify official Nutanix Developer Portal artifacts."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
from collections.abc import Mapping, Sequence
from http.client import HTTPMessage
from pathlib import Path
from typing import IO, Any, TextIO, cast
from urllib.error import HTTPError, URLError
from urllib.parse import unquote, urlsplit
from urllib.request import (
    HTTPRedirectHandler,
    OpenerDirector,
    Request,
    build_opener,
)

import yaml

DEFAULT_BASE_URL = "https://developers.nutanix.com/api/v1/"
DEFAULT_MANIFEST_PATH = Path("specs/nutanix/manifest.json")
DEFAULT_CACHE_PATH = Path(".cache/nutanix/artifacts")
HTTP_TIMEOUT_SECONDS = 60.0
MAX_RESPONSE_BYTES = 64 * 1024 * 1024
USER_AGENT = "ioplane-terraform-provider-nutanix-artifact-lock/0.0.0-dev"

_NAMESPACE = re.compile(r"^[a-z][a-z0-9]*$")
_GA_VERSION = re.compile(r"^v(\d+)\.(\d+)$")
_PREVIEW_VERSION = re.compile(r"^v(\d+)\.(\d+)\.(a|b|rc)(\d+)$")
_SAFE_VERSION = re.compile(r"^v\d+\.\d+(?:\.(?:a|b|rc)\d+)?$")
_TRUE_JSON_MEDIA = re.compile(r"^application/(?:json|[a-z0-9!#$&^_.+-]+\+json)$")
_OPENAPI_MEDIA = {
    "application/yaml",
    "application/x-yaml",
    "text/plain",
    "text/x-yaml",
    "text/yaml",
}
_ARTIFACT_FILENAMES = {
    "openapi": "openapi.yaml",
    "postman": "postman.json",
    "errors": "errors.json",
}


class ArtifactError(RuntimeError):
    """ArtifactError reports an invalid source, response, artifact, or lock."""


def _media_type(value: str) -> str:
    return value.partition(";")[0].strip().lower()


def _normalize_base_url(base_url: str) -> str:
    if not base_url.endswith("/"):
        raise ArtifactError("base URL must end with '/'")
    parsed = urlsplit(base_url)
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ArtifactError("base URL contains forbidden URL components")
    official = base_url == DEFAULT_BASE_URL
    loopback = parsed.hostname in {"127.0.0.1", "::1", "localhost"}
    if official:
        if parsed.scheme != "https" or parsed.netloc != "developers.nutanix.com":
            raise ArtifactError("official base URL must use developers.nutanix.com over HTTPS")
    elif parsed.scheme != "http" or not loopback:
        raise ArtifactError("test base URL must use loopback HTTP")
    if unquote(parsed.path) != parsed.path or not parsed.path.startswith("/api/v1/"):
        raise ArtifactError("base URL has an unsafe API path")
    return base_url


def _validate_url(url: str, base_url: str) -> str:
    base = urlsplit(_normalize_base_url(base_url))
    parsed = urlsplit(url)
    if parsed.scheme != base.scheme or parsed.netloc != base.netloc:
        raise ArtifactError("URL is outside allowed API prefix")
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ArtifactError("URL contains forbidden components")
    if unquote(parsed.path) != parsed.path or not parsed.path.startswith(base.path):
        raise ArtifactError("URL has an unsafe path")
    suffix = parsed.path[len(base.path) :]
    if any(segment in {".", ".."} for segment in suffix.split("/")):
        raise ArtifactError("URL has an unsafe path")
    return url


class _PrefixRedirectHandler(HTTPRedirectHandler):
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url

    def redirect_request(
        self,
        req: Request,
        fp: IO[bytes],
        code: int,
        msg: str,
        headers: HTTPMessage,
        newurl: str,
    ) -> Request | None:
        _validate_url(newurl, self.base_url)
        return super().redirect_request(
            req,
            fp,
            code,
            msg,
            headers,
            newurl,
        )


class _Client:
    def __init__(self, base_url: str, opener: OpenerDirector | None = None) -> None:
        self.base_url = _normalize_base_url(base_url)
        self.opener = opener or build_opener(_PrefixRedirectHandler(self.base_url))

    def get(self, url: str, *, label: str) -> tuple[str, bytes]:
        selected_url = _validate_url(url, self.base_url)
        request = Request(
            selected_url,
            headers={"Accept": "*/*", "User-Agent": USER_AGENT},
            method="GET",
        )
        try:
            with self.opener.open(request, timeout=HTTP_TIMEOUT_SECONDS) as response:
                final_url = cast(str, response.geturl())
                _validate_url(final_url, self.base_url)
                raw_length = response.headers.get("Content-Length")
                if raw_length:
                    try:
                        length = int(raw_length)
                    except ValueError as error:
                        raise ArtifactError(f"{label}: invalid Content-Length") from error
                    if length > MAX_RESPONSE_BYTES:
                        raise ArtifactError(f"{label}: response exceeds {MAX_RESPONSE_BYTES} bytes")
                body = response.read(MAX_RESPONSE_BYTES + 1)
                if len(body) > MAX_RESPONSE_BYTES:
                    raise ArtifactError(f"{label}: response exceeds {MAX_RESPONSE_BYTES} bytes")
                return _media_type(response.headers.get("Content-Type", "")), body
        except ArtifactError:
            raise
        except HTTPError as error:
            raise ArtifactError(f"{label}: HTTP {error.code}") from error
        except URLError as error:
            reason = type(error.reason).__name__
            raise ArtifactError(f"{label}: request failed ({reason})") from error
        except OSError as error:
            raise ArtifactError(f"{label}: request failed ({type(error).__name__})") from error

    def json(self, url: str, *, label: str) -> Any:
        media_type, body = self.get(url, label=label)
        if not _TRUE_JSON_MEDIA.fullmatch(media_type):
            raise ArtifactError(f"{label}: rejected media type {media_type or '<missing>'}")
        try:
            return json.loads(body)
        except (UnicodeDecodeError, json.JSONDecodeError) as error:
            raise ArtifactError(f"{label}: invalid JSON response") from error


def _version_key(version: str) -> tuple[int, int, int, int] | None:
    if match := _GA_VERSION.fullmatch(version):
        return int(match.group(1)), int(match.group(2)), 3, 0
    if match := _PREVIEW_VERSION.fullmatch(version):
        phase = {"a": 0, "b": 1, "rc": 2}[match.group(3)]
        return int(match.group(1)), int(match.group(2)), phase, int(match.group(4))
    return None


def _select_version(entries: list[Any], namespace: str) -> tuple[Mapping[str, Any], str]:
    ga: list[tuple[tuple[int, int, int, int], Mapping[str, Any]]] = []
    preview: list[tuple[tuple[int, int, int, int], Mapping[str, Any]]] = []
    for entry in entries:
        if not isinstance(entry, Mapping):
            raise ArtifactError(f"versions {namespace}: version entry must be an object")
        version = entry.get("version")
        if not isinstance(version, str):
            raise ArtifactError(f"versions {namespace}: version must be a string")
        key = _version_key(version)
        if key is None:
            continue
        if _GA_VERSION.fullmatch(version):
            ga.append((key, entry))
        else:
            preview.append((key, entry))
    if ga:
        return max(ga, key=lambda item: item[0])[1], "ga"
    if preview:
        return max(preview, key=lambda item: item[0])[1], "preview"
    raise ArtifactError(f"versions {namespace}: no supported GA or preview version")


def _artifact_urls(
    entry: Mapping[str, Any],
    *,
    namespace: str,
    version: str,
    base_url: str,
) -> dict[str, dict[str, str]]:
    openapi = entry.get("link")
    if not isinstance(openapi, str) or not openapi:
        raise ArtifactError(f"versions {namespace}: selected {version} has no OpenAPI URL")
    artifacts = {"openapi": {"url": _validate_url(openapi, base_url)}}

    postman = entry.get("postmanCollectionLink")
    if postman is not None and postman != "":
        if not isinstance(postman, str):
            raise ArtifactError(f"versions {namespace}: invalid Postman URL")
        artifacts["postman"] = {"url": _validate_url(postman, base_url)}

    error_links = entry.get("errDocLinks", [])
    if not isinstance(error_links, list):
        raise ArtifactError(f"versions {namespace}: errDocLinks must be an array")
    english: list[str] = []
    for item in error_links:
        if not isinstance(item, dict):
            raise ArtifactError(f"versions {namespace}: invalid error reference entry")
        if item.get("locale") == "en_US":
            link = item.get("link")
            if not isinstance(link, str) or not link:
                raise ArtifactError(f"versions {namespace}: invalid English error URL")
            english.append(link)
    if len(english) > 1:
        raise ArtifactError(f"versions {namespace}: duplicate English error URLs")
    if english:
        artifacts["errors"] = {"url": _validate_url(english[0], base_url)}
    return artifacts


def _discover(client: _Client) -> list[dict[str, Any]]:
    registry_url = f"{client.base_url}namespaces/"
    registry = client.json(registry_url, label="registry")
    if not isinstance(registry, dict) or not isinstance(registry.get("namespaces"), list):
        raise ArtifactError("registry: expected an object with a namespaces array")
    namespaces = cast(list[Any], registry["namespaces"])
    if not namespaces:
        raise ArtifactError("registry: namespaces array is empty")

    selected: list[dict[str, Any]] = []
    seen: set[str] = set()
    for raw_namespace in namespaces:
        if not isinstance(raw_namespace, dict):
            raise ArtifactError("registry: namespace entry must be an object")
        name = raw_namespace.get("name")
        if not isinstance(name, str) or not _NAMESPACE.fullmatch(name):
            raise ArtifactError("registry: unsafe namespace name")
        if name in seen:
            raise ArtifactError(f"registry: duplicate namespace {name}")
        seen.add(name)

        versions_url = f"{client.base_url}namespaces/{name}/versions/"
        document = client.json(versions_url, label=f"versions {name}")
        if not isinstance(document, dict) or document.get("namespace") != name:
            raise ArtifactError(f"versions {name}: namespace mismatch")
        entries = document.get("versions")
        if not isinstance(entries, list) or not entries:
            raise ArtifactError(f"versions {name}: expected a nonempty versions array")
        entry, stability = _select_version(entries, name)
        version = cast(str, entry["version"])
        selected.append(
            {
                "name": name,
                "version": version,
                "stability": stability,
                "artifacts": _artifact_urls(
                    entry,
                    namespace=name,
                    version=version,
                    base_url=client.base_url,
                ),
            }
        )
    return sorted(selected, key=lambda item: cast(str, item["name"]))


def discover(
    *,
    base_url: str,
    manifest_path: Path,
    cache_path: Path,
    opener: OpenerDirector | None = None,
) -> list[dict[str, Any]]:
    """Discover and select every live namespace without writing files."""
    del manifest_path, cache_path
    return _discover(_Client(base_url, opener))


def cache_file(root: Path, namespace: str, version: str, kind: str) -> Path:
    """Return a safe cache path for one selected artifact."""
    if not _NAMESPACE.fullmatch(namespace) or not _SAFE_VERSION.fullmatch(version):
        raise ArtifactError("unsafe namespace or version cache path")
    try:
        filename = _ARTIFACT_FILENAMES[kind]
    except KeyError as error:
        raise ArtifactError(f"unsafe artifact kind: {kind}") from error
    return root / namespace / version / filename


def validate_artifact(kind: str, media_type: str, body: bytes) -> str:
    """Validate media type and minimum shape, returning normalized media type."""
    normalized = _media_type(media_type)
    if kind == "openapi":
        if normalized not in _OPENAPI_MEDIA:
            raise ArtifactError(f"OpenAPI rejected media type {normalized or '<missing>'}")
        try:
            document = yaml.safe_load(body)
        except (UnicodeDecodeError, yaml.YAMLError) as error:
            raise ArtifactError("OpenAPI invalid YAML") from error
        if not isinstance(document, dict):
            raise ArtifactError("OpenAPI document must be an object")
        version = document.get("openapi")
        if not isinstance(version, str) or not version.startswith(("3.0", "3.1")):
            raise ArtifactError("OpenAPI 3.0 or 3.1 metadata is required")
        return normalized

    json_media = _TRUE_JSON_MEDIA.fullmatch(normalized) is not None
    if kind == "postman" and not (json_media or normalized == "text/plain"):
        raise ArtifactError(f"{kind} rejected media type {normalized or '<missing>'}")
    if kind == "errors" and not json_media:
        raise ArtifactError(f"{kind} rejected media type {normalized or '<missing>'}")
    try:
        document = json.loads(body)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ArtifactError(f"{kind} invalid JSON") from error
    if kind == "postman":
        if not isinstance(document, dict) or not isinstance(document.get("item"), list):
            raise ArtifactError("Postman document must be an object with an item array")
    elif kind == "errors":
        if not isinstance(document, list):
            raise ArtifactError("error reference must be a JSON array")
    else:
        raise ArtifactError(f"unsafe artifact kind: {kind}")
    return normalized


def _atomic_write(path: Path, body: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.",
        suffix=".tmp",
        dir=path.parent,
    )
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(body)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def _locked_artifact(
    *,
    client: _Client,
    cache_path: Path,
    namespace: str,
    version: str,
    kind: str,
    url: str,
) -> dict[str, Any]:
    media_type, body = client.get(url, label=f"{namespace} {version} {kind}")
    try:
        observed = validate_artifact(kind, media_type, body)
    except ArtifactError as error:
        raise ArtifactError(f"{namespace} {version} {kind}: {error}") from error
    destination = cache_file(cache_path, namespace, version, kind)
    _atomic_write(destination, body)
    return {
        "url": url,
        "media_type": observed,
        "bytes": len(body),
        "sha256": hashlib.sha256(body).hexdigest(),
        "path": destination.relative_to(cache_path).as_posix(),
    }


def update(
    *,
    base_url: str,
    manifest_path: Path,
    cache_path: Path,
    opener: OpenerDirector | None = None,
) -> dict[str, Any]:
    """Download selected artifacts and atomically write the proposed lock."""
    client = _Client(base_url, opener)
    selected = _discover(client)
    locked: list[dict[str, Any]] = []
    for entry in selected:
        namespace = cast(str, entry["name"])
        version = cast(str, entry["version"])
        artifacts: dict[str, Any] = {}
        for kind, metadata in cast(dict[str, dict[str, str]], entry["artifacts"]).items():
            artifacts[kind] = _locked_artifact(
                client=client,
                cache_path=cache_path,
                namespace=namespace,
                version=version,
                kind=kind,
                url=metadata["url"],
            )
        locked.append(
            {
                "name": namespace,
                "version": version,
                "stability": entry["stability"],
                "artifacts": artifacts,
            }
        )
    manifest = {
        "schema_version": 1,
        "source": client.base_url,
        "namespaces": locked,
    }
    encoded = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode()
    _atomic_write(manifest_path, encoded)
    return manifest


def _load_manifest(path: Path) -> dict[str, Any]:
    try:
        body = path.read_bytes()
    except OSError as error:
        raise ArtifactError(f"manifest: cannot read {path}") from error
    if len(body) > MAX_RESPONSE_BYTES:
        raise ArtifactError("manifest: file exceeds size limit")
    try:
        document = json.loads(body)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ArtifactError("manifest: invalid JSON") from error
    if (
        not isinstance(document, dict)
        or document.get("schema_version") != 1
        or not isinstance(document.get("namespaces"), list)
    ):
        raise ArtifactError("manifest: invalid lock shape")
    return cast(dict[str, Any], document)


def _live_signature(entry: Mapping[str, Any]) -> tuple[str, str, str, tuple[tuple[str, str], ...]]:
    artifacts = entry.get("artifacts")
    if not isinstance(artifacts, dict):
        raise ArtifactError("manifest: namespace artifacts must be an object")
    urls: list[tuple[str, str]] = []
    for kind, metadata in artifacts.items():
        if not isinstance(kind, str) or not isinstance(metadata, dict):
            raise ArtifactError("manifest: invalid artifact entry")
        url = metadata.get("url")
        if not isinstance(url, str):
            raise ArtifactError("manifest: artifact URL must be a string")
        urls.append((kind, url))
    name = entry.get("name")
    version = entry.get("version")
    stability = entry.get("stability")
    if not all(isinstance(value, str) for value in (name, version, stability)):
        raise ArtifactError("manifest: namespace identity must be strings")
    return cast(str, name), cast(str, version), cast(str, stability), tuple(sorted(urls))


def verify(
    *,
    base_url: str,
    manifest_path: Path,
    cache_path: Path,
    opener: OpenerDirector | None = None,
) -> int:
    """Verify the live registry selection and every locked artifact."""
    del cache_path
    client = _Client(base_url, opener)
    manifest = _load_manifest(manifest_path)
    if manifest.get("source") != client.base_url:
        raise ArtifactError("manifest: source does not match requested base URL")
    locked = cast(list[Mapping[str, Any]], manifest["namespaces"])
    live = _discover(client)
    if [_live_signature(item) for item in locked] != [_live_signature(item) for item in live]:
        raise ArtifactError("manifest: live namespace selection mismatch")

    for entry in locked:
        namespace = cast(str, entry["name"])
        version = cast(str, entry["version"])
        artifacts = cast(dict[str, Mapping[str, Any]], entry["artifacts"])
        for kind, metadata in artifacts.items():
            url = cast(str, metadata["url"])
            _validate_url(url, client.base_url)
            media_type, body = client.get(url, label=f"{namespace} {version} {kind}")
            observed = validate_artifact(kind, media_type, body)
            if metadata.get("media_type") != observed:
                raise ArtifactError(f"{namespace} {version} {kind}: media type mismatch")
            if metadata.get("bytes") != len(body):
                raise ArtifactError(f"{namespace} {version} {kind}: byte count mismatch")
            digest = hashlib.sha256(body).hexdigest()
            if metadata.get("sha256") != digest:
                raise ArtifactError(f"{namespace} {version} {kind}: digest mismatch")
    return len(locked)


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="nutanix-artifacts")
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("discover", "update", "verify", "count"):
        command = commands.add_parser(name)
        command.add_argument(
            "--manifest",
            type=Path,
            default=DEFAULT_MANIFEST_PATH,
        )
        if name != "count":
            command.add_argument(
                "--cache",
                type=Path,
                default=DEFAULT_CACHE_PATH,
            )
    return parser


def main(
    arguments: Sequence[str] | None = None,
    *,
    stdout: TextIO | None = None,
    stderr: TextIO | None = None,
) -> int:
    """Run the fixed production CLI against the official Developer Portal."""
    selected_stdout = sys.stdout if stdout is None else stdout
    selected_stderr = sys.stderr if stderr is None else stderr
    parsed = _parser().parse_args(arguments)
    try:
        if parsed.command == "count":
            manifest = _load_manifest(parsed.manifest)
            print(len(manifest["namespaces"]), file=selected_stdout)
            return 0
        if parsed.command == "discover":
            selection = discover(
                base_url=DEFAULT_BASE_URL,
                manifest_path=parsed.manifest,
                cache_path=parsed.cache,
            )
            print(json.dumps(selection, indent=2, sort_keys=True), file=selected_stdout)
            return 0
        if parsed.command == "update":
            manifest = update(
                base_url=DEFAULT_BASE_URL,
                manifest_path=parsed.manifest,
                cache_path=parsed.cache,
            )
            print(
                f"artifacts: locked {len(manifest['namespaces'])} namespaces",
                file=selected_stdout,
            )
            return 0
        count = verify(
            base_url=DEFAULT_BASE_URL,
            manifest_path=parsed.manifest,
            cache_path=parsed.cache,
        )
        print(f"artifacts: verified {count} namespaces", file=selected_stdout)
        return 0
    except (ArtifactError, OSError) as error:
        print(f"artifacts: {error}", file=selected_stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
