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

    service_script = "".join(
        f"          {line}\n" for line in pins.EXPECTED_PODMAN_SERVICE_SCRIPT.splitlines()
    )
    workflow = root / ".github" / "workflows" / "ci.yml"
    workflow.parent.mkdir(parents=True)
    workflow.write_text(
        "name: CI\n"
        "on:\n"
        "  push:\n"
        "    branches: [main]\n"
        "  pull_request:\n"
        "permissions:\n"
        "  contents: read\n"
        "concurrency:\n"
        "  group: ${{ github.workflow }}-${{ github.ref }}\n"
        "  cancel-in-progress: true\n"
        "jobs:\n"
        "  foundation:\n"
        "    name: Foundation\n"
        "    runs-on: ubuntu-24.04\n"
        "    steps:\n"
        "      - name: Check out repository\n"
        "        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1\n"
        "        with:\n"
        "          persist-credentials: false\n"
        "      # tool-version: UV_VERSION=0.12.1\n"
        "      - name: Install pinned uv launcher\n"
        "        uses: astral-sh/setup-uv@c771a70e6277c0a99b617c7a806ffedaca235ff9 # v9.0.0\n"
        "        with:\n"
        "          version: 0.12.1\n"
        "      - run: podman version\n"
        "      - name: Start private Podman API service\n"
        "        run: |\n" + service_script + "      - run: ./dev up\n"
        "      - run: ./dev task all\n"
        "      - if: failure()\n"
        "        run: ./dev status\n"
    )

    dependabot = root / ".github" / "dependabot.yml"
    dependabot.write_text(
        "version: 2\n"
        "updates:\n"
        + "".join(
            "  - package-ecosystem: "
            f'"{ecosystem}"\n'
            f'    directory: "{directory}"\n'
            "    schedule:\n"
            '      interval: "weekly"\n'
            for ecosystem, directory in pins.EXPECTED_DEPENDABOT_DIRECTORIES.items()
        )
    )
    (root / ".github" / "CODEOWNERS").write_text("* @dantte-lp\n")
    (root / ".github" / "pull_request_template.md").write_text("# Pull request\n")


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
    workflow.parent.mkdir(parents=True, exist_ok=True)
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


def test_requires_github_policy_files(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    for relative in (
        ".github/workflows/ci.yml",
        ".github/dependabot.yml",
        ".github/CODEOWNERS",
        ".github/pull_request_template.md",
    ):
        (tmp_path / relative).unlink()

    diagnostics = pins.validate(tmp_path)

    for relative in (
        ".github/workflows/ci.yml",
        ".github/dependabot.yml",
        ".github/CODEOWNERS",
        ".github/pull_request_template.md",
    ):
        assert f"GitHub policy file missing: {relative}" in diagnostics


def test_rejects_incomplete_or_host_native_ci_policy(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    workflow.write_text(
        "name: CI\n"
        "on: [push]\n"
        "permissions:\n"
        "  contents: write\n"
        "jobs:\n"
        "  foundation:\n"
        "    runs-on: ubuntu-latest\n"
        "    steps:\n"
        "      - uses: actions/checkout@v7\n"
        "      - run: go test ./...\n"
    )

    diagnostics = pins.validate(tmp_path)

    assert "CI permissions must be exactly contents read" in diagnostics
    assert "CI concurrency cancellation differs" in diagnostics
    assert "CI foundation runner must be ubuntu-24.04" in diagnostics
    assert "CI must start a private Podman API service" in diagnostics
    assert "CI must export CONTAINER_HOST" in diagnostics
    assert "CI must run ./dev up" in diagnostics
    assert "CI must run ./dev task all" in diagnostics
    assert "CI must run ./dev status on failure" in diagnostics
    assert "CI host development command is forbidden: go test ./..." in diagnostics
    assert "CI action pin differs: actions/checkout" in diagnostics
    assert "CI action pin differs: astral-sh/setup-uv" in diagnostics


def test_requires_all_dependabot_ecosystems_at_repository_root(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    dependabot = tmp_path / ".github" / "dependabot.yml"
    dependabot.write_text(
        "version: 2\n"
        "updates:\n"
        '  - package-ecosystem: "gomod"\n'
        '    directory: "/src"\n'
        "    schedule:\n"
        '      interval: "daily"\n'
    )

    diagnostics = pins.validate(tmp_path)

    assert "Dependabot ecosystem set differs" in diagnostics
    assert "Dependabot directory differs: gomod" in diagnostics
    assert "Dependabot schedule differs: gomod" in diagnostics


def test_ci_policy_cannot_be_spoofed_by_run_block(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    workflow.write_text(
        "name: CI\n"
        "permissions:\n"
        "  contents: read\n"
        "concurrency:\n"
        "  group: ${{ github.workflow }}-${{ github.ref }}\n"
        "  cancel-in-progress: true\n"
        "jobs:\n"
        "  foundation:\n"
        "    name: Foundation\n"
        "    runs-on: ubuntu-24.04\n"
        "    steps:\n"
        "      - name: Check out repository\n"
        "        uses: actions/checkout@0000000000000000000000000000000000000000 # v7.0.1\n"
        "        with:\n"
        "          persist-credentials: false\n"
        "      # tool-version: UV_VERSION=0.12.1\n"
        "      - name: Install pinned uv launcher\n"
        "        uses: astral-sh/setup-uv@1111111111111111111111111111111111111111 # v9.0.0\n"
        "        with:\n"
        "          version: 0.12.1\n"
        "      - run: podman version\n"
        "      - name: Start private Podman API service\n"
        "        run: |\n"
        "          push:\n"
        "          pull_request:\n"
        "          uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1\n"
        "          uses: astral-sh/setup-uv@c771a70e6277c0a99b617c7a806ffedaca235ff9 # v9.0.0\n"
        "          env go test ./...\n"
        '          socket_dir="${RUNNER_TEMP}/podman-api"\n'
        '          mkdir -m 0700 "${socket_dir}"\n'
        '          socket="${socket_dir}/podman.sock"\n'
        '          podman system service --time=0 "unix://${socket}" &\n'
        '          echo "CONTAINER_HOST=unix://${socket}" >> "${GITHUB_ENV}"\n'
        "      - run: ./dev up\n"
        "      - run: ./dev task all\n"
        "      - if: failure()\n"
        "        run: ./dev status\n"
    )

    diagnostics = pins.validate(tmp_path)

    assert "CI triggers must include push and pull_request" in diagnostics
    assert "CI action pin differs: actions/checkout" in diagnostics
    assert "CI action pin differs: astral-sh/setup-uv" in diagnostics
    assert "CI Podman service step differs" in diagnostics
    assert "CI host development command is forbidden: env go test ./..." in diagnostics
