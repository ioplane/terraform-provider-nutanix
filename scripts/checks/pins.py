"""Validate immutable development, dependency, and artifact pins."""

from __future__ import annotations

import hashlib
import json
import re
import shlex
import sys
from pathlib import Path
from typing import Any, TextIO, cast

import yaml

CONTAINERFILE = Path("deployments/containers/Containerfile.dev")
TOOL_ASSET_LOCK = Path("deployments/containers/tool-assets.lock")
COMPOSE_FILE = Path("deployments/compose/compose.dev.yml")
MANIFEST_FILE = Path("specs/nutanix/manifest.json")
CI_FILE = Path(".github/workflows/ci.yml")
RELEASE_FILE = Path(".github/workflows/release.yml")
RELEASE_PLEASE_CONFIG_FILE = Path("release-please-config.json")
RELEASE_PLEASE_MANIFEST_FILE = Path(".release-please-manifest.json")
DEPENDABOT_FILE = Path(".github/dependabot.yml")
CODEOWNERS_FILE = Path(".github/CODEOWNERS")
PULL_REQUEST_TEMPLATE_FILE = Path(".github/pull_request_template.md")
EXPECTED_BASE_DIGEST = "sha256:4ee9ffa999b4583ce281939cdff828763083610292f252279a0cee77473bd9a7"
EXPECTED_CONTAINERFILE_SHA256 = "0a0b9c9e5f6a60f901ff4341b7d074339bbbb1d0a121de482548464950ddf095"
EXPECTED_TOOL_ARGUMENTS = {
    "TASK_VERSION": "3.52.0",
    "BEADS_VERSION": "1.1.2",
    "TERRAFORM_VERSION": "1.15.8",
    "GOLANGCI_LINT_VERSION": "2.12.2",
    "GORELEASER_VERSION": "2.17.1",
    "SYFT_VERSION": "1.50.0",
    "TFPLUGINDOCS_VERSION": "0.25.0",
    "GOVULNCHECK_VERSION": "1.6.0",
    "GOPLS_VERSION": "0.23.0",
    "GH_VERSION": "2.97.0",
    "UV_VERSION": "0.12.1",
    "HADOLINT_VERSION": "2.15.1",
    "DEBIAN_SNAPSHOT": "20260713T000000Z",
}
EXPECTED_OCI_LABELS = {
    "org.opencontainers.image.title": "Terraform Provider Nutanix Development Toolbox",
    "org.opencontainers.image.description": (
        "Reproducible development toolbox for the Terraform Provider for Nutanix"
    ),
    "org.opencontainers.image.source": "https://github.com/ioplane/terraform-provider-nutanix",
    "org.opencontainers.image.licenses": "Apache-2.0",
    "org.opencontainers.image.version": "${OCI_VERSION}",
    "org.opencontainers.image.revision": "${OCI_REVISION}",
    "org.opencontainers.image.created": "${OCI_CREATED}",
    "org.opencontainers.image.base.name": "docker.io/library/golang:1.26-trixie",
    "org.opencontainers.image.base.digest": EXPECTED_BASE_DIGEST,
}
EXPECTED_TOOL_ASSETS = {
    ("task", "3.52.0", "amd64"): (
        "task_linux_amd64.tar.gz",
        "02c679ffae53dca791804847d78b31731615894e292948397c971c87ac9e95bd",
    ),
    ("task", "3.52.0", "arm64"): (
        "task_linux_arm64.tar.gz",
        "7e0044108830cec0534577b289564e3b7c83e6df276feb631a1edc63d04e4ebe",
    ),
    ("beads", "1.1.2", "amd64"): (
        "beads_1.1.2_linux_amd64.tar.gz",
        "a72d71ed374955dc9f83a0f90b54bd7b6a0016709dd1676ae2e368651ed401c2",
    ),
    ("beads", "1.1.2", "arm64"): (
        "beads_1.1.2_linux_arm64.tar.gz",
        "a134015faf4be0a43f8681a8d602eaf0b7c255c957f09d3c933257c8c92fdd10",
    ),
    ("terraform", "1.15.8", "amd64"): (
        "terraform_1.15.8_linux_amd64.zip",
        "d25ce7b6902013ad905db3d2eab0be4cd905887fe88b81a6171b8d5503c31f3d",
    ),
    ("terraform", "1.15.8", "arm64"): (
        "terraform_1.15.8_linux_arm64.zip",
        "8891e9dcedc9e3b8950bc6af9d4d8af1f4cfade3062f53b9dc403a89f6ce8c9c",
    ),
    ("golangci-lint", "2.12.2", "amd64"): (
        "golangci-lint-2.12.2-linux-amd64.tar.gz",
        "8df580d2670fed8fa984aac0507099af8df275e665215f5c7a2ae3943893a553",
    ),
    ("golangci-lint", "2.12.2", "arm64"): (
        "golangci-lint-2.12.2-linux-arm64.tar.gz",
        "44cd40a8c76c86755375adfeea52cfd3533cb43d7bd647771e0ae065e166df3a",
    ),
    ("goreleaser", "2.17.1", "amd64"): (
        "goreleaser_Linux_x86_64.tar.gz",
        "a99bbc7ae0d8d897b07c4c497a9b62f222558804715ef219d1af05a7e417bc80",
    ),
    ("goreleaser", "2.17.1", "arm64"): (
        "goreleaser_Linux_arm64.tar.gz",
        "702f03769ac8bcb0e47839c82243cc614ae995633599a98c63062e13ea85f829",
    ),
    ("syft", "1.50.0", "amd64"): (
        "syft_1.50.0_linux_amd64.tar.gz",
        "bf7b29ff57f06da30918266a0e1c2885a8f99784798d1bdb1628886aa015d788",
    ),
    ("syft", "1.50.0", "arm64"): (
        "syft_1.50.0_linux_arm64.tar.gz",
        "887c57cbcc2d0e8c5c110a4571a3fc7150058b24d74f993ee4663516e5c8ce86",
    ),
    ("tfplugindocs", "0.25.0", "amd64"): (
        "tfplugindocs_0.25.0_linux_amd64.zip",
        "912bd663e2deafc9ebf54e932bd2adf91bf6b7fcf545d4d9a82dc9597255854c",
    ),
    ("tfplugindocs", "0.25.0", "arm64"): (
        "tfplugindocs_0.25.0_linux_arm64.zip",
        "de1317d79f2783d68a7a090cbe06dea61dbab20c6376c52eadee42e9e8c7c169",
    ),
    ("gh", "2.97.0", "amd64"): (
        "gh_2.97.0_linux_amd64.tar.gz",
        "a2c9b8497e1f85b1ad0dfcb78b5a622e098801b8e461e459e88e1ee12f018112",
    ),
    ("gh", "2.97.0", "arm64"): (
        "gh_2.97.0_linux_arm64.tar.gz",
        "73ea440ecad9c9e284429997ee6f93577bc6f7bc6fba357ef62c53ad8fb641a5",
    ),
    ("uv", "0.12.1", "amd64"): (
        "uv-x86_64-unknown-linux-gnu.tar.gz",
        "90b2f223fb69d19db49e117da601f64978593417988530aa733d456141b4bcbb",
    ),
    ("uv", "0.12.1", "arm64"): (
        "uv-aarch64-unknown-linux-gnu.tar.gz",
        "769d373e146692c639b5fbaae33b331c297a32e03d30448772051902df52bbf4",
    ),
    ("hadolint", "2.15.1", "amd64"): (
        "hadolint-linux-x86_64",
        "c7187db94eeeeca956519a6af171adc31453941a1e777961f6e680f697c8c507",
    ),
    ("hadolint", "2.15.1", "arm64"): (
        "hadolint-linux-arm64",
        "f6198ef8090f404dbb771abfee086eb8c48ac177f30da7fd3510aca35b344b5d",
    ),
}
EXPECTED_DOWNLOAD_URLS = {
    "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/${task_asset}",
    "https://github.com/gastownhall/beads/releases/download/v${BEADS_VERSION}/${beads_asset}",
    "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/${terraform_asset}",
    "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/${golangci_lint_asset}",
    "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/${goreleaser_asset}",
    "https://github.com/anchore/syft/releases/download/v${SYFT_VERSION}/${syft_asset}",
    "https://github.com/hashicorp/terraform-plugin-docs/releases/download/v${TFPLUGINDOCS_VERSION}/${tfplugindocs_asset}",
    "https://github.com/cli/cli/releases/download/v${GH_VERSION}/${gh_asset}",
    "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/${uv_asset}",
    "https://github.com/hadolint/hadolint/releases/download/v${HADOLINT_VERSION}/${hadolint_asset}",
}
EXPECTED_VERIFY_CALLS = {
    'verify_locked task "${TASK_VERSION}" "${arch}" "${task_asset}"',
    'verify_locked beads "${BEADS_VERSION}" "${arch}" "${beads_asset}"',
    'verify_locked terraform "${TERRAFORM_VERSION}" "${arch}" "${terraform_asset}"',
    'verify_locked golangci-lint "${GOLANGCI_LINT_VERSION}" "${arch}" "${golangci_lint_asset}"',
    'verify_locked goreleaser "${GORELEASER_VERSION}" "${arch}" "${goreleaser_asset}"',
    'verify_locked syft "${SYFT_VERSION}" "${arch}" "${syft_asset}"',
    'verify_locked tfplugindocs "${TFPLUGINDOCS_VERSION}" "${arch}" "${tfplugindocs_asset}"',
    'verify_locked gh "${GH_VERSION}" "${arch}" "${gh_asset}"',
    'verify_locked uv "${UV_VERSION}" "${arch}" "${uv_asset}"',
    'verify_locked hadolint "${HADOLINT_VERSION}" "${arch}" "${hadolint_asset}"',
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
EXPECTED_ACTIONS = {
    "actions/checkout": ("3d3c42e5aac5ba805825da76410c181273ba90b1", "v7.0.1"),
    "astral-sh/setup-uv": ("c771a70e6277c0a99b617c7a806ffedaca235ff9", "v9.0.0"),
    "googleapis/release-please-action": (
        "45996ed1f6d02564a971a2fa1b5860e934307cf7",
        "v5.0.0",
    ),
}
EXPECTED_CI_ACTIONS = {"actions/checkout", "astral-sh/setup-uv"}
EXPECTED_PODMAN_SERVICE_SCRIPT = """\
set -euo pipefail
socket_dir="${RUNNER_TEMP}/podman-api"
socket="${socket_dir}/podman.sock"
service_log="${RUNNER_TEMP}/podman-service.log"
mkdir -m 0700 "${socket_dir}"
podman system service --time=0 "unix://${socket}" >"${service_log}" 2>&1 &
service_pid=$!
echo "PODMAN_SERVICE_PID=${service_pid}" >> "${GITHUB_ENV}"
echo "CONTAINER_HOST=unix://${socket}" >> "${GITHUB_ENV}"
for _ in {1..100}; do
  if test -S "${socket}"; then
    exit 0
  fi
  if ! kill -0 "${service_pid}" 2>/dev/null; then
    cat "${service_log}"
    exit 1
  fi
  sleep 0.1
done
cat "${service_log}"
exit 1
"""
EXPECTED_CI_STEPS: list[dict[str, object]] = [
    {
        "name": "Check out repository",
        "uses": "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
        "with": {"persist-credentials": "false"},
    },
    {
        "name": "Install pinned uv launcher",
        "uses": "astral-sh/setup-uv@c771a70e6277c0a99b617c7a806ffedaca235ff9",
        "with": {"version": "0.12.1", "enable-cache": "false"},
    },
    {"name": "Verify host Podman", "run": "podman version"},
    {
        "name": "Start private Podman API service",
        "shell": "bash",
        "run": EXPECTED_PODMAN_SERVICE_SCRIPT,
    },
    {"name": "Build development container", "run": "./dev up"},
    {"name": "Run complete foundation gate", "run": "./dev task all"},
    {
        "name": "Report container status on failure",
        "if": "failure()",
        "continue-on-error": "true",
        "run": "./dev status",
    },
]
EXPECTED_DEPENDABOT_DIRECTORIES = {
    "docker": "/deployments/containers",
    "github-actions": "/",
    "gomod": "/",
    "uv": "/",
}
REQUIRED_DEPENDABOT_ECOSYSTEMS = set(EXPECTED_DEPENDABOT_DIRECTORIES)

_ARGUMENT = re.compile(r"^ARG ([A-Z][A-Z0-9_]*)=(\S+)$", re.MULTILINE)
_FROM = re.compile(r"^FROM\s+(\S+)", re.MULTILINE)
_DOWNLOAD = re.compile(r'\bdownload\s+"([^"\n]+)"')
_ACTION = re.compile(r"^\s*-?\s*uses:\s*([^\s#]+)", re.MULTILINE)
_ACTION_SHA = re.compile(r"^[^@]+@[0-9a-f]{40}$")
_SEMVER = re.compile(r"^(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$")
_CI_MARKER = re.compile(r"^\s*#\s*tool-version:\s*([A-Z][A-Z0-9_]*)=(\S+)\s*$", re.MULTILINE)
_HOST_DEVELOPMENT_TOOL = re.compile(
    r"(?<![A-Za-z0-9_.-])(?:/[A-Za-z0-9_.-]+)*/?"
    r"(?:go|gofmt|terraform|task|pytest|python|python3|ruff|ty|uv|uvx|"
    r"golangci-lint|govulncheck|gopls|goreleaser|tfplugindocs)(?=\s|$)"
)
_RISKY_COMPOSE_KEYS = frozenset({"cap_add", "devices", "security_opt", "sysctls", "volumes_from"})
_ALLOWED_COMPOSE_SERVICE_KEYS = frozenset(
    {
        "build",
        "command",
        "environment",
        "healthcheck",
        "init",
        "volumes",
        "working_dir",
    }
)
_EXPECTED_COMPOSE_TOP_LEVEL_KEYS = frozenset({"services", "volumes"})
_EXPECTED_COMPOSE_SERVICES = frozenset({"dev"})
_EXPECTED_COMPOSE_NAMED_VOLUMES = frozenset({"go-mod-cache", "go-build-cache", "uv-cache"})
_EXPECTED_COMPOSE_DEV_SERVICE: dict[str, object] = {
    "build": {
        "context": "../..",
        "dockerfile": "deployments/containers/Containerfile.dev",
        "args": {
            "OCI_VERSION": "${NUTANIX_DEV_VERSION:-0.0.0-dev}",
            "OCI_REVISION": ("${NUTANIX_DEV_REVISION:-0000000000000000000000000000000000000000}"),
            "OCI_CREATED": "${NUTANIX_DEV_CREATED:-1970-01-01T00:00:00Z}",
        },
    },
    "init": True,
    "working_dir": "/workspace",
    "command": ["sleep", "infinity"],
    "environment": {
        "BEADS_DIR": "/workspace/.beads",
        "GIT_COMMON_DIR": "/git-metadata/.git",
        "GIT_DIR": "/git-metadata/.git/${NUTANIX_GIT_DIR_RELATIVE}",
        "GIT_WORK_TREE": "/workspace",
    },
    "volumes": [
        "../..:/workspace:z",
        "${NUTANIX_GIT_COMMON_DIR:?required}:/git-metadata/.git:z",
        "go-mod-cache:/go/pkg/mod",
        "go-build-cache:/root/.cache/go-build",
        "uv-cache:/opt/uv-cache",
    ],
    "healthcheck": {
        "test": [
            "CMD-SHELL",
            (
                "test -f /run/.containerenv && command -v go >/dev/null && "
                "command -v terraform >/dev/null && command -v task >/dev/null && "
                "command -v bd >/dev/null && command -v gopls >/dev/null && "
                "command -v goreleaser >/dev/null && command -v syft >/dev/null && "
                "command -v uv >/dev/null"
            ),
        ],
        "interval": "10s",
        "timeout": "5s",
        "retries": 12,
        "start_period": "5s",
    },
}
_ALLOWED_COMPOSE_VOLUME_SOURCES = frozenset(
    {
        "../..",
        "${NUTANIX_GIT_COMMON_DIR:?required}",
        *_EXPECTED_COMPOSE_NAMED_VOLUMES,
    }
)


def _container_instructions(text: str) -> tuple[str, ...]:
    logical: list[str] = []
    pending = ""
    for raw_line in text.splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        pending = f"{pending} {stripped}".strip()
        if pending.endswith("\\"):
            pending = pending[:-1].rstrip()
            continue
        logical.append(pending)
        pending = ""
    if pending:
        logical.append(pending)
    return tuple(logical)


def _oci_labels(text: str) -> dict[str, str]:
    entries: list[str] = []
    for instruction in _container_instructions(text):
        keyword, separator, value = instruction.partition(" ")
        if separator and keyword.casefold() == "label":
            entries.extend(shlex.split(value))
    labels: dict[str, str] = {}
    for entry in entries:
        name, separator, value = entry.partition("=")
        if separator:
            labels[name] = value
    return labels


def _tool_lock_diagnostics(root: Path) -> list[str]:
    path = root / TOOL_ASSET_LOCK
    if not path.is_file():
        return [f"pin file missing: {TOOL_ASSET_LOCK.as_posix()}"]
    observed: dict[tuple[str, str, str], tuple[str, str]] = {}
    malformed = False
    for raw_line in path.read_text().splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        if len(fields) != 5 or not re.fullmatch(r"[0-9a-f]{64}", fields[4]):
            malformed = True
            continue
        tool, version, architecture, asset, digest = fields
        key = (tool, version, architecture)
        if key in observed:
            malformed = True
        observed[key] = (asset, digest)
    diagnostics: list[str] = []
    if malformed:
        diagnostics.append("tool asset lock is malformed")
    if observed != EXPECTED_TOOL_ASSETS:
        diagnostics.append("tool asset lock differs")
    return diagnostics


def _container_diagnostics(root: Path) -> tuple[list[str], dict[str, str]]:
    path = root / CONTAINERFILE
    if not path.is_file():
        return [f"pin file missing: {CONTAINERFILE.as_posix()}"], {}
    text = path.read_text()
    diagnostics: list[str] = []
    if hashlib.sha256(path.read_bytes()).hexdigest() != EXPECTED_CONTAINERFILE_SHA256:
        diagnostics.append("development Containerfile digest differs")
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
    if _oci_labels(text) != EXPECTED_OCI_LABELS:
        diagnostics.append("development OCI label set differs")
    if set(_DOWNLOAD.findall(text)) != EXPECTED_DOWNLOAD_URLS:
        for tool in sorted({key[0] for key in EXPECTED_TOOL_ASSETS}):
            diagnostics.append(f"tool download origin differs: {tool}")
    for call in EXPECTED_VERIFY_CALLS:
        if call not in text:
            diagnostics.append(
                f"repository-owned tool asset verification is missing: {call.split()[1]}"
            )
    if "COPY deployments/containers/tool-assets.lock /tmp/tool-assets.lock" not in text:
        diagnostics.append("tool asset lock is not copied into the development image")
    if "sha256sum --check" not in text or "/tmp/tool-assets.lock" not in text:
        diagnostics.append("repository-owned tool asset verifier differs")
    diagnostics.extend(_tool_lock_diagnostics(root))
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
    if set(document) != _EXPECTED_COMPOSE_TOP_LEVEL_KEYS:
        diagnostics.append("Compose top-level key set differs")
    services = document.get("services", {})
    if not isinstance(services, dict):
        diagnostics.append("Compose services must be an object")
        return diagnostics
    if set(services) != _EXPECTED_COMPOSE_SERVICES:
        diagnostics.append("Compose service set differs")
    if services.get("dev") != _EXPECTED_COMPOSE_DEV_SERVICE:
        diagnostics.append("Compose dev service model differs")
    named_volumes = document.get("volumes")
    if not isinstance(named_volumes, dict):
        diagnostics.append("Compose named volumes must be an object")
    else:
        if set(named_volumes) != _EXPECTED_COMPOSE_NAMED_VOLUMES:
            diagnostics.append("Compose named volume set differs")
        for name, definition in named_volumes.items():
            if definition != {}:
                diagnostics.append(f"Compose named volume must be internal and empty: {name}")
    for service in services.values():
        if not isinstance(service, dict):
            continue
        for key in sorted(
            key
            for key in service
            if isinstance(key, str) and key not in _ALLOWED_COMPOSE_SERVICE_KEYS
        ):
            diagnostics.append(f"Compose service key is not allowed: {key}")
        if any(not isinstance(key, str) for key in service):
            diagnostics.append("Compose service keys must be strings")
        if service.get("privileged") is True:
            diagnostics.append("Compose privileged execution is forbidden")
        if service.get("pid") == "host":
            diagnostics.append("Compose host pid namespace is forbidden")
        if service.get("network_mode") == "host":
            diagnostics.append("Compose host network namespace is forbidden")
        for namespace in ("cgroupns_mode", "ipc", "userns_mode", "uts"):
            if service.get(namespace) == "host":
                diagnostics.append(f"Compose host {namespace} namespace is forbidden")
        for key in sorted(_RISKY_COMPOSE_KEYS & service.keys()):
            diagnostics.append(f"Compose risky service key is forbidden: {key}")
        volumes = service.get("volumes", [])
        if not isinstance(volumes, list):
            diagnostics.append("Compose service volumes must be an array")
            continue
        for volume in volumes:
            if _is_socket_mount(volume):
                diagnostics.append("Compose must not mount a Podman or Docker socket")
            source = _volume_source(volume)
            if source is None:
                diagnostics.append("Compose volume source must be explicit")
                continue
            if source == "/run" or source.startswith(("/run/", "/var/run/")):
                diagnostics.append("Compose runtime socket directory mount is forbidden")
            if source not in _ALLOWED_COMPOSE_VOLUME_SOURCES:
                diagnostics.append(f"Compose volume source is not allowed: {source}")
    return diagnostics


def _volume_source(volume: object) -> str | None:
    if isinstance(volume, str):
        expansion_depth = 0
        index = 0
        while index < len(volume):
            if volume.startswith("${", index):
                expansion_depth += 1
                index += 2
                continue
            if volume[index] == "}" and expansion_depth:
                expansion_depth -= 1
            elif volume[index] == ":" and expansion_depth == 0:
                return volume[:index]
            index += 1
        return None
    if isinstance(volume, dict):
        source = volume.get("source")
        return source if isinstance(source, str) else None
    return None


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
    workflow_paths = sorted((*workflow_root.glob("*.yml"), *workflow_root.glob("*.yaml")))
    if [path.relative_to(root) for path in workflow_paths] != [CI_FILE, RELEASE_FILE]:
        diagnostics.append("GitHub workflow file set differs")
    for path in workflow_paths:
        relative = path.relative_to(root).as_posix()
        text = path.read_text()
        for reference in _ACTION.findall(text):
            if reference.startswith("./"):
                diagnostics.append(f"{relative}: local action reference is forbidden: {reference}")
                continue
            if not _ACTION_SHA.fullmatch(reference):
                diagnostics.append(f"{relative}: floating action reference {reference}")
        for name, version in _CI_MARKER.findall(text):
            if arguments.get(name) != version:
                diagnostics.append(f"CI tool marker differs: {name}")
    return diagnostics


def _ci_policy_diagnostics(root: Path) -> list[str]:
    path = root / CI_FILE
    if not path.is_file():
        return [f"GitHub policy file missing: {CI_FILE.as_posix()}"]
    text = path.read_text()
    try:
        document = yaml.load(text, Loader=yaml.BaseLoader)
    except yaml.YAMLError:
        return ["CI workflow is invalid YAML"]
    if not isinstance(document, dict):
        return ["CI workflow must be an object"]

    diagnostics: list[str] = []
    if set(document) != {"name", "on", "permissions", "concurrency", "jobs"}:
        diagnostics.append("CI workflow key set differs")
    if "defaults" in document:
        diagnostics.append("CI workflow execution defaults are forbidden")
    if "env" in document:
        diagnostics.append("CI workflow environment overrides are forbidden")
    triggers = document.get("on")
    if not isinstance(triggers, dict) or set(triggers) != {
        "push",
        "pull_request",
        "workflow_dispatch",
    }:
        diagnostics.append("CI triggers must include push, pull_request, and workflow_dispatch")
    elif not isinstance(triggers.get("push"), dict) or triggers["push"].get("branches") != [
        "dev",
        "main",
    ]:
        diagnostics.append("CI push trigger must select dev and main")
    if document.get("permissions") != {"contents": "read"}:
        diagnostics.append("CI permissions must be exactly contents read")
    concurrency = document.get("concurrency")
    if not isinstance(concurrency, dict) or concurrency.get("cancel-in-progress") != "true":
        diagnostics.append("CI concurrency cancellation differs")
    elif concurrency.get("group") != "${{ github.workflow }}-${{ github.ref }}":
        diagnostics.append("CI concurrency group differs")

    jobs = document.get("jobs")
    if isinstance(jobs, dict) and set(jobs) != {"foundation"}:
        diagnostics.append("CI job set differs")
    foundation = jobs.get("foundation") if isinstance(jobs, dict) else None
    if not isinstance(foundation, dict):
        diagnostics.append("CI foundation job is missing")
        return diagnostics
    if foundation.get("name") != "Foundation":
        diagnostics.append("CI foundation check name differs")
    if foundation.get("runs-on") != "ubuntu-24.04":
        diagnostics.append("CI foundation runner must be ubuntu-24.04")
    if set(foundation) != {"name", "runs-on", "timeout-minutes", "steps"}:
        diagnostics.append("CI foundation job model differs")
    if foundation.get("timeout-minutes") != "90":
        diagnostics.append("CI foundation timeout differs")
    if "permissions" in foundation:
        diagnostics.append("CI job-level permissions are forbidden")
    if "if" in foundation:
        diagnostics.append("CI foundation job must be unconditional")
    if "continue-on-error" in foundation:
        diagnostics.append("CI foundation job must fail the workflow")
    if "defaults" in foundation:
        diagnostics.append("CI foundation job execution defaults are forbidden")
    if "env" in foundation:
        diagnostics.append("CI foundation job environment overrides are forbidden")

    steps = foundation.get("steps")
    if not isinstance(steps, list) or not all(isinstance(step, dict) for step in steps):
        diagnostics.append("CI foundation steps must be an array of objects")
        return diagnostics
    if steps != EXPECTED_CI_STEPS:
        diagnostics.append("CI foundation ordered step model differs")

    observed_actions: dict[str, list[str]] = {}
    for step in steps:
        reference = step.get("uses")
        if not isinstance(reference, str) or reference.startswith("./"):
            continue
        action, separator, _revision = reference.partition("@")
        if not separator:
            continue
        observed_actions.setdefault(action, []).append(reference)
    if set(observed_actions) != EXPECTED_CI_ACTIONS:
        diagnostics.append("CI action set differs")
    for action in sorted(EXPECTED_CI_ACTIONS):
        sha256, release = EXPECTED_ACTIONS[action]
        expected_reference = f"{action}@{sha256}"
        if observed_actions.get(action) != [expected_reference]:
            diagnostics.append(f"CI action pin differs: {action}")
        annotation = re.compile(
            rf"^\s*uses:\s*{re.escape(expected_reference)}\s+#\s*{re.escape(release)}\s*$",
            re.MULTILINE,
        )
        if annotation.search(text) is None:
            diagnostics.append(f"CI action release annotation differs: {action}")

    checkout = next(
        (
            step
            for step in steps
            if isinstance(step.get("uses"), str)
            and cast(str, step["uses"]).startswith("actions/checkout@")
        ),
        None,
    )
    checkout_with = checkout.get("with") if isinstance(checkout, dict) else None
    if not isinstance(checkout_with, dict) or checkout_with.get("persist-credentials") != "false":
        diagnostics.append("CI checkout must disable persisted credentials")

    setup_uv = next(
        (
            step
            for step in steps
            if isinstance(step.get("uses"), str)
            and cast(str, step["uses"]).startswith("astral-sh/setup-uv@")
        ),
        None,
    )
    setup_uv_with = setup_uv.get("with") if isinstance(setup_uv, dict) else None
    if not isinstance(setup_uv_with, dict) or str(setup_uv_with.get("version")) != "0.12.1":
        diagnostics.append("CI setup-uv version differs")
    if f"# tool-version: UV_VERSION={EXPECTED_TOOL_ARGUMENTS['UV_VERSION']}" not in text:
        diagnostics.append("CI UV tool marker is missing")

    run_steps = [cast(str, step["run"]) for step in steps if isinstance(step.get("run"), str)]
    run_text = "\n".join(run_steps)
    service_steps = [
        step for step in steps if step.get("name") == "Start private Podman API service"
    ]
    if (
        len(service_steps) != 1
        or not isinstance(service_steps[0].get("run"), str)
        or cast(str, service_steps[0]["run"]).strip() != EXPECTED_PODMAN_SERVICE_SCRIPT.strip()
    ):
        diagnostics.append("CI Podman service step differs")
    if (
        "${RUNNER_TEMP}" not in run_text
        or "mkdir -m 0700" not in run_text
        or "podman system service --time=0" not in run_text
    ):
        diagnostics.append("CI must start a private Podman API service")
    if "CONTAINER_HOST=unix://" not in run_text or "${GITHUB_ENV}" not in run_text:
        diagnostics.append("CI must export CONTAINER_HOST")
    if not any(command.strip() == "podman version" for command in run_steps):
        diagnostics.append("CI must verify host Podman")
    if not any(command.strip() == "./dev up" for command in run_steps):
        diagnostics.append("CI must run ./dev up")
    if not any(command.strip() == "./dev task all" for command in run_steps):
        diagnostics.append("CI must run ./dev task all")
    complete_gate_steps = [
        step
        for step in steps
        if isinstance(step.get("run"), str) and cast(str, step["run"]).strip() == "./dev task all"
    ]
    if len(complete_gate_steps) == 1:
        complete_gate = complete_gate_steps[0]
        if "if" in complete_gate:
            diagnostics.append("CI complete foundation gate must be unconditional")
        if "continue-on-error" in complete_gate:
            diagnostics.append("CI complete foundation gate must fail the job")
        if {"env", "shell", "working-directory"} & complete_gate.keys():
            diagnostics.append("CI complete foundation gate execution overrides are forbidden")
    if not any(
        step.get("if") == "failure()"
        and isinstance(step.get("run"), str)
        and cast(str, step["run"]).strip() == "./dev status"
        for step in steps
    ):
        diagnostics.append("CI must run ./dev status on failure")

    approved_commands = {"podman version", "./dev up", "./dev task all", "./dev status"}
    for step in steps:
        command = step.get("run")
        if not isinstance(command, str):
            continue
        if (
            command.strip() not in approved_commands
            and step.get("name") != "Start private Podman API service"
        ):
            diagnostics.append(
                f"CI host command step is not approved: {step.get('name', '<unnamed>')}"
            )
        for line in command.splitlines():
            stripped = line.strip()
            if stripped.startswith("./dev "):
                continue
            if _HOST_DEVELOPMENT_TOOL.search(stripped):
                diagnostics.append(f"CI host development command is forbidden: {stripped}")
    return diagnostics


def _release_policy_diagnostics(root: Path) -> list[str]:
    workflow_path = root / RELEASE_FILE
    if not workflow_path.is_file():
        return [f"GitHub policy file missing: {RELEASE_FILE.as_posix()}"]
    text = workflow_path.read_text()
    try:
        document = yaml.load(text, Loader=yaml.BaseLoader)
    except yaml.YAMLError:
        return ["Release workflow is invalid YAML"]
    if not isinstance(document, dict):
        return ["Release workflow must be an object"]

    diagnostics: list[str] = []
    if set(document) != {"name", "on", "permissions", "concurrency", "jobs"}:
        diagnostics.append("Release workflow key set differs")
    triggers = document.get("on")
    if not isinstance(triggers, dict) or set(triggers) != {"push"}:
        diagnostics.append("Release trigger must be push only")
    elif not isinstance(triggers.get("push"), dict) or triggers["push"].get("branches") != ["main"]:
        diagnostics.append("Release push trigger must select main")
    expected_permissions = {
        "actions": "write",
        "contents": "write",
        "issues": "write",
        "pull-requests": "write",
    }
    if document.get("permissions") != expected_permissions:
        diagnostics.append("Release permissions differ")
    concurrency = document.get("concurrency")
    if not isinstance(concurrency, dict) or concurrency != {
        "group": "release-${{ github.ref }}",
        "cancel-in-progress": "false",
    }:
        diagnostics.append("Release concurrency differs")

    jobs = document.get("jobs")
    if not isinstance(jobs, dict) or set(jobs) != {"release"}:
        diagnostics.append("Release job set differs")
        return diagnostics
    release_job = jobs.get("release")
    if not isinstance(release_job, dict):
        diagnostics.append("Release job is missing")
        return diagnostics
    if set(release_job) != {"name", "runs-on", "timeout-minutes", "steps"}:
        diagnostics.append("Release job model differs")
    if release_job.get("name") != "Release":
        diagnostics.append("Release job name differs")
    if release_job.get("runs-on") != "ubuntu-24.04":
        diagnostics.append("Release runner must be ubuntu-24.04")
    if release_job.get("timeout-minutes") != "90":
        diagnostics.append("Release timeout differs")

    steps = release_job.get("steps")
    if not isinstance(steps, list) or not all(isinstance(step, dict) for step in steps):
        diagnostics.append("Release steps must be an array of objects")
        return diagnostics
    by_name = {cast(str, step["name"]): step for step in steps if isinstance(step.get("name"), str)}
    required_names = {
        "Create or update release pull request",
        "Dispatch Foundation gate for release pull request",
        "Check out release tag",
        "Install pinned uv launcher",
        "Verify host Podman",
        "Start private Podman API service",
        "Build release toolbox",
        "Build release artifacts in Podman",
        "Upload release artifacts",
        "Report container status on failure",
    }
    if set(by_name) != required_names or len(steps) != len(required_names):
        diagnostics.append("Release ordered step set differs")

    observed_actions: dict[str, list[str]] = {}
    for step in steps:
        reference = step.get("uses")
        if not isinstance(reference, str):
            continue
        action, separator, _revision = reference.partition("@")
        if separator:
            observed_actions.setdefault(action, []).append(reference)
    if set(observed_actions) != set(EXPECTED_ACTIONS):
        diagnostics.append("Release action set differs")
    for action, (sha256, release) in EXPECTED_ACTIONS.items():
        expected_reference = f"{action}@{sha256}"
        if observed_actions.get(action) != [expected_reference]:
            diagnostics.append(f"Release action pin differs: {action}")
        annotation = re.compile(
            rf"^\s*uses:\s*{re.escape(expected_reference)}\s+#\s*{re.escape(release)}\s*$",
            re.MULTILINE,
        )
        if annotation.search(text) is None:
            diagnostics.append(f"Release action release annotation differs: {action}")

    release_step = by_name.get("Create or update release pull request", {})
    if release_step.get("id") != "release" or release_step.get("with") != {
        "config-file": "release-please-config.json",
        "manifest-file": ".release-please-manifest.json",
    }:
        diagnostics.append("Release Please invocation differs")
    checkout = by_name.get("Check out release tag", {})
    if checkout.get("with") != {
        "fetch-depth": "0",
        "persist-credentials": "false",
        "ref": "${{ steps.release.outputs.tag_name }}",
    }:
        diagnostics.append("Release checkout policy differs")
    setup_uv = by_name.get("Install pinned uv launcher", {})
    if setup_uv.get("with") != {"version": "0.12.1", "enable-cache": "false"}:
        diagnostics.append("Release setup-uv policy differs")
    if f"# tool-version: UV_VERSION={EXPECTED_TOOL_ARGUMENTS['UV_VERSION']}" not in text:
        diagnostics.append("Release UV tool marker is missing")

    dispatch = by_name.get("Dispatch Foundation gate for release pull request", {})
    if dispatch.get("if") != "steps.release.outputs.prs_created == 'true'":
        diagnostics.append("Release PR dispatch condition differs")
    dispatch_run = dispatch.get("run")
    if (
        not isinstance(dispatch_run, str)
        or "gh workflow run ci.yml --ref" not in dispatch_run
        or '--repo "${GITHUB_REPOSITORY}"' not in dispatch_run
    ):
        diagnostics.append("Release PR must dispatch the Foundation workflow")
    build = by_name.get("Build release artifacts in Podman", {})
    if build.get("run") != "./dev task release:build" or "env" in build:
        diagnostics.append("Release artifact build boundary differs")
    upload = by_name.get("Upload release artifacts", {})
    upload_run = upload.get("run")
    if (
        not isinstance(upload_run, str)
        or "gh release upload" not in upload_run
        or 'test "${#artifacts[@]}" -eq 5' not in upload_run
    ):
        diagnostics.append("Release artifact upload policy differs")
    for step in steps:
        if step.get("name") in {
            "Dispatch Foundation gate for release pull request",
            "Create or update release pull request",
        }:
            continue
        if step.get("if") not in {
            "steps.release.outputs.release_created == 'true'",
            "failure() && steps.release.outputs.release_created == 'true'",
        }:
            diagnostics.append(f"Release tag gate differs: {step.get('name', '<unnamed>')}")

    config_path = root / RELEASE_PLEASE_CONFIG_FILE
    manifest_path = root / RELEASE_PLEASE_MANIFEST_FILE
    try:
        config = json.loads(config_path.read_text())
    except (OSError, json.JSONDecodeError):
        diagnostics.append("Release Please configuration is invalid")
        config = None
    try:
        manifest = json.loads(manifest_path.read_text())
    except (OSError, json.JSONDecodeError):
        diagnostics.append("Release Please manifest is invalid")
        manifest = None
    manifest_version = manifest.get(".") if isinstance(manifest, dict) else None
    if (
        not isinstance(manifest, dict)
        or set(manifest) != {"."}
        or not isinstance(manifest_version, str)
        or _SEMVER.fullmatch(manifest_version) is None
    ):
        diagnostics.append("Release Please manifest differs")
    elif manifest_version != "0.0.0":
        changelog_path = root / "CHANGELOG.md"
        try:
            changelog = changelog_path.read_text()
        except OSError:
            diagnostics.append("Release Please changelog is unavailable")
        else:
            heading = re.compile(rf"^## {re.escape(manifest_version)}(?: \(|$)", re.MULTILINE)
            if heading.search(changelog) is None:
                diagnostics.append("Release Please manifest and changelog differ")
    if not isinstance(config, dict) or set(config) != {"$schema", "packages"}:
        diagnostics.append("Release Please configuration shape differs")
    else:
        packages = config.get("packages")
        package = packages.get(".") if isinstance(packages, dict) else None
        if not isinstance(package, dict):
            diagnostics.append("Release Please root package is missing")
        elif (
            package.get("release-type") != "go"
            or package.get("package-name") != "terraform-provider-nutanix"
            or package.get("changelog-path") != "CHANGELOG.md"
            or package.get("include-component-in-tag") is not False
            or package.get("bump-minor-pre-major") is not True
        ):
            diagnostics.append("Release Please root package policy differs")
    return diagnostics


def _dependabot_diagnostics(root: Path) -> list[str]:
    path = root / DEPENDABOT_FILE
    if not path.is_file():
        return [f"GitHub policy file missing: {DEPENDABOT_FILE.as_posix()}"]
    try:
        document = yaml.safe_load(path.read_text())
    except yaml.YAMLError:
        return ["Dependabot configuration is invalid YAML"]
    if not isinstance(document, dict) or document.get("version") != 2:
        return ["Dependabot configuration version differs"]
    updates = document.get("updates")
    if not isinstance(updates, list):
        return ["Dependabot updates must be an array"]

    entries: dict[str, dict[str, Any]] = {}
    duplicate = False
    for update in updates:
        if not isinstance(update, dict) or not isinstance(update.get("package-ecosystem"), str):
            continue
        ecosystem = cast(str, update["package-ecosystem"])
        if ecosystem in entries:
            duplicate = True
        entries[ecosystem] = cast(dict[str, Any], update)

    diagnostics: list[str] = []
    if set(entries) != REQUIRED_DEPENDABOT_ECOSYSTEMS or duplicate:
        diagnostics.append("Dependabot ecosystem set differs")
    for ecosystem in sorted(set(entries) & REQUIRED_DEPENDABOT_ECOSYSTEMS):
        entry = entries[ecosystem]
        if entry.get("directory") != EXPECTED_DEPENDABOT_DIRECTORIES[ecosystem]:
            diagnostics.append(f"Dependabot directory differs: {ecosystem}")
        schedule = entry.get("schedule")
        if not isinstance(schedule, dict) or schedule.get("interval") != "weekly":
            diagnostics.append(f"Dependabot schedule differs: {ecosystem}")
    return diagnostics


def _github_policy_diagnostics(root: Path, arguments: dict[str, str]) -> list[str]:
    diagnostics = _ci_policy_diagnostics(root)
    diagnostics.extend(_release_policy_diagnostics(root))
    diagnostics.extend(_dependabot_diagnostics(root))
    for relative in (CODEOWNERS_FILE, PULL_REQUEST_TEMPLATE_FILE):
        path = root / relative
        if not path.is_file():
            diagnostics.append(f"GitHub policy file missing: {relative.as_posix()}")
        elif not path.read_text().strip():
            diagnostics.append(f"GitHub policy file is empty: {relative.as_posix()}")
    diagnostics.extend(_workflow_diagnostics(root, arguments))
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
    diagnostics.extend(_github_policy_diagnostics(root, arguments))
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
        "pins: ok "
        f"({len(EXPECTED_TOOL_ARGUMENTS)} tools, {len(EXPECTED_ACTIONS)} actions, "
        f"{len(REQUIRED_NAMESPACES)} namespaces)",
        file=stdout,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
