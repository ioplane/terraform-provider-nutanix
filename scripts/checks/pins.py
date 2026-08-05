"""Validate immutable development, dependency, and artifact pins."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any, TextIO, cast

import yaml

CONTAINERFILE = Path("deployments/containers/Containerfile.dev")
COMPOSE_FILE = Path("deployments/compose/compose.dev.yml")
MANIFEST_FILE = Path("specs/nutanix/manifest.json")
EXPECTED_BASE_DIGEST = "sha256:4ee9ffa999b4583ce281939cdff828763083610292f252279a0cee77473bd9a7"
EXPECTED_TOOL_ARGUMENTS = {
    "TASK_VERSION": "3.52.0",
    "BEADS_VERSION": "1.1.2",
    "TERRAFORM_VERSION": "1.15.8",
    "GOLANGCI_LINT_VERSION": "2.12.2",
    "GORELEASER_VERSION": "2.17.1",
    "TFPLUGINDOCS_VERSION": "0.25.0",
    "GOVULNCHECK_VERSION": "1.6.0",
    "GH_VERSION": "2.97.0",
    "UV_VERSION": "0.12.1",
    "DEBIAN_SNAPSHOT": "20260713T000000Z",
}
REQUIRED_NAMESPACES = (
    "aiops",
    "clustermgmt",
    "datapolicies",
    "dataprotection",
    "files",
    "iam",
    "licensing",
    "lifecycle",
    "microseg",
    "monitoring",
    "multidomain",
    "networking",
    "objects",
    "opsmgmt",
    "prism",
    "security",
    "storage",
    "vmm",
    "volumes",
)
REQUIRED_ARTIFACTS = {"openapi", "postman", "errors"}

_ARGUMENT = re.compile(r"^ARG ([A-Z][A-Z0-9_]*)=(\S+)$", re.MULTILINE)
_FROM = re.compile(r"^FROM\s+(\S+)", re.MULTILINE)
_ACTION = re.compile(r"^\s*-?\s*uses:\s*([^\s#]+)", re.MULTILINE)
_ACTION_SHA = re.compile(r"^[^@]+@[0-9a-f]{40}$")
_CI_MARKER = re.compile(r"^\s*#\s*tool-version:\s*([A-Z][A-Z0-9_]*)=(\S+)\s*$", re.MULTILINE)


def _container_diagnostics(root: Path) -> tuple[list[str], dict[str, str]]:
    path = root / CONTAINERFILE
    if not path.is_file():
        return [f"pin file missing: {CONTAINERFILE.as_posix()}"], {}
    text = path.read_text()
    diagnostics: list[str] = []
    from_entries = _FROM.findall(text)
    expected_base = f"docker.io/library/golang:1.26-trixie@{EXPECTED_BASE_DIGEST}"
    if not from_entries or from_entries[0] != expected_base:
        diagnostics.append("development base image digest differs")
    for image in from_entries:
        if "@sha256:" not in image:
            diagnostics.append(f"floating container image reference: {image}")

    observed = dict(_ARGUMENT.findall(text))
    for name, expected in EXPECTED_TOOL_ARGUMENTS.items():
        if observed.get(name) != expected:
            diagnostics.append(f"tool pin differs: {name}")
    return diagnostics, observed


def _compose_diagnostics(root: Path) -> list[str]:
    path = root / COMPOSE_FILE
    if not path.is_file():
        return [f"pin file missing: {COMPOSE_FILE.as_posix()}"]
    try:
        document = yaml.safe_load(path.read_text())
    except yaml.YAMLError:
        return ["Compose file is invalid YAML"]
    if not isinstance(document, dict):
        return ["Compose file must be an object"]
    diagnostics: list[str] = []
    if "version" in document:
        diagnostics.append("Compose top-level version is forbidden")
    services = document.get("services", {})
    if not isinstance(services, dict):
        diagnostics.append("Compose services must be an object")
        return diagnostics
    for service in services.values():
        if not isinstance(service, dict):
            continue
        volumes = service.get("volumes", [])
        if not isinstance(volumes, list):
            diagnostics.append("Compose service volumes must be an array")
            continue
        if any(_is_socket_mount(volume) for volume in volumes):
            diagnostics.append("Compose must not mount a Podman or Docker socket")
    return diagnostics


def _is_socket_mount(volume: object) -> bool:
    if isinstance(volume, str):
        paths = (volume,)
    elif isinstance(volume, dict):
        paths = tuple(
            value for key in ("source", "target") if isinstance((value := volume.get(key)), str)
        )
    else:
        return False
    return any("podman.sock" in path or "docker.sock" in path for path in paths)


def _workflow_diagnostics(root: Path, arguments: dict[str, str]) -> list[str]:
    diagnostics: list[str] = []
    workflow_root = root / ".github" / "workflows"
    if not workflow_root.is_dir():
        return diagnostics
    for path in sorted((*workflow_root.glob("*.yml"), *workflow_root.glob("*.yaml"))):
        relative = path.relative_to(root).as_posix()
        text = path.read_text()
        for reference in _ACTION.findall(text):
            if reference.startswith("./"):
                continue
            if not _ACTION_SHA.fullmatch(reference):
                diagnostics.append(f"{relative}: floating action reference {reference}")
        for name, version in _CI_MARKER.findall(text):
            if arguments.get(name) != version:
                diagnostics.append(f"CI tool marker differs: {name}")
    return diagnostics


def _manifest_diagnostics(root: Path) -> list[str]:
    path = root / MANIFEST_FILE
    if not path.is_file():
        return [f"pin file missing: {MANIFEST_FILE.as_posix()}"]
    try:
        document = json.loads(path.read_bytes())
    except (UnicodeDecodeError, json.JSONDecodeError):
        return ["Nutanix manifest is invalid JSON"]
    if not isinstance(document, dict) or not isinstance(document.get("namespaces"), list):
        return ["Nutanix manifest shape is invalid"]
    entries = cast(list[Any], document["namespaces"])
    by_name: dict[str, dict[str, Any]] = {}
    for entry in entries:
        if isinstance(entry, dict) and isinstance(entry.get("name"), str):
            by_name[cast(str, entry["name"])] = entry
    diagnostics: list[str] = []
    if set(by_name) != set(REQUIRED_NAMESPACES) or len(entries) != len(REQUIRED_NAMESPACES):
        diagnostics.append("Nutanix manifest namespace set differs")
    for name in sorted(set(by_name) & set(REQUIRED_NAMESPACES)):
        artifacts = by_name[name].get("artifacts")
        if not isinstance(artifacts, dict) or set(artifacts) != REQUIRED_ARTIFACTS:
            diagnostics.append(f"Nutanix manifest artifact set differs: {name}")
    return diagnostics


def validate(root: Path) -> list[str]:
    """Return deterministic pin diagnostics."""
    container, arguments = _container_diagnostics(root)
    diagnostics = container
    diagnostics.extend(_compose_diagnostics(root))
    diagnostics.extend(_workflow_diagnostics(root, arguments))
    diagnostics.extend(_manifest_diagnostics(root))
    return sorted(set(diagnostics))


def main(
    *,
    root: Path | None = None,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Run the immutable pin gate."""
    selected_root = Path.cwd() if root is None else root
    diagnostics = validate(selected_root)
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"pins: {diagnostic}", file=stderr)
        return 1
    print(
        f"pins: ok ({len(EXPECTED_TOOL_ARGUMENTS)} tools, {len(REQUIRED_NAMESPACES)} namespaces)",
        file=stdout,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
