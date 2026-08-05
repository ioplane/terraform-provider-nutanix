from __future__ import annotations

import json
from pathlib import Path

from scripts.checks import pins


def valid_pin_repository(root: Path) -> None:
    container = root / pins.CONTAINERFILE
    container.parent.mkdir(parents=True)
    arguments = "\n".join(
        f"ARG {name}={version}" for name, version in pins.EXPECTED_TOOL_ARGUMENTS.items()
    )
    container.write_text(
        f"FROM docker.io/library/golang:1.26-trixie@{pins.EXPECTED_BASE_DIGEST}\n{arguments}\n"
    )

    compose = root / pins.COMPOSE_FILE
    compose.parent.mkdir(parents=True)
    compose.write_text(
        "services:\n"
        "  dev:\n"
        "    build:\n"
        "      context: ../..\n"
        "      dockerfile: deployments/containers/Containerfile.dev\n"
        "    volumes:\n"
        "      - ../..:/workspace:z\n"
    )

    manifest = root / pins.MANIFEST_FILE
    manifest.parent.mkdir(parents=True)
    manifest.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "namespaces": [
                    {
                        "name": name,
                        "artifacts": {kind: {} for kind in ("openapi", "postman", "errors")},
                    }
                    for name in pins.REQUIRED_NAMESPACES
                ],
            }
        )
    )


def test_valid_pins_have_no_diagnostics(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)

    assert pins.validate(tmp_path) == []


def test_rejects_base_or_tool_pin_drift(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    path = tmp_path / pins.CONTAINERFILE
    content = path.read_text()
    content = content.replace(pins.EXPECTED_BASE_DIGEST, "sha256:" + "0" * 64)
    content = content.replace("ARG TERRAFORM_VERSION=1.15.8", "ARG TERRAFORM_VERSION=latest")
    path.write_text(content)

    diagnostics = pins.validate(tmp_path)

    assert "development base image digest differs" in diagnostics
    assert "tool pin differs: TERRAFORM_VERSION" in diagnostics


def test_rejects_compose_version_and_socket_mount(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    compose = tmp_path / pins.COMPOSE_FILE
    compose.write_text(
        "version: '3.9'\n"
        "services:\n"
        "  dev:\n"
        "    volumes:\n"
        "      - /run/user/1000/podman/podman.sock:/run/podman/podman.sock\n"
    )

    diagnostics = pins.validate(tmp_path)

    assert "Compose top-level version is forbidden" in diagnostics
    assert "Compose must not mount a Podman or Docker socket" in diagnostics


def test_rejects_long_syntax_socket_mount(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    compose = tmp_path / pins.COMPOSE_FILE
    compose.write_text(
        "services:\n"
        "  dev:\n"
        "    volumes:\n"
        "      - type: bind\n"
        "        source: /run/user/1000/podman/podman.sock\n"
        "        target: /run/podman/podman.sock\n"
    )

    assert "Compose must not mount a Podman or Docker socket" in pins.validate(tmp_path)


def test_rejects_floating_action_and_ci_marker_drift(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    workflow.parent.mkdir(parents=True)
    workflow.write_text(
        "# tool-version: TERRAFORM_VERSION=0.0.0\nsteps:\n  - uses: actions/checkout@v4\n"
    )

    diagnostics = pins.validate(tmp_path)

    assert ".github/workflows/ci.yml: floating action reference actions/checkout@v4" in diagnostics
    assert "CI tool marker differs: TERRAFORM_VERSION" in diagnostics


def test_requires_complete_locked_namespace_and_artifact_set(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    manifest = tmp_path / pins.MANIFEST_FILE
    document = json.loads(manifest.read_text())
    document["namespaces"] = document["namespaces"][:-1]
    document["namespaces"][0]["artifacts"].pop("postman")
    manifest.write_text(json.dumps(document))

    diagnostics = pins.validate(tmp_path)

    assert "Nutanix manifest namespace set differs" in diagnostics
    assert "Nutanix manifest artifact set differs: aiops" in diagnostics
