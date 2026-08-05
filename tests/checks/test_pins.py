from __future__ import annotations

import copy
import json
from pathlib import Path

import yaml
from scripts.checks import pins


def valid_pin_repository(root: Path) -> None:
    container = root / pins.CONTAINERFILE
    container.parent.mkdir(parents=True)
    arguments = "\n".join(
        f"ARG {name}={version}" for name, version in pins.EXPECTED_TOOL_ARGUMENTS.items()
    )
    container.write_text(
        f"FROM docker.io/library/golang:1.26-trixie@{pins.EXPECTED_BASE_DIGEST}\n"
        f"{arguments}\n"
        "ARG OCI_VERSION=0.0.0-dev\n"
        "ARG OCI_REVISION=0000000000000000000000000000000000000000\n"
        "ARG OCI_CREATED=1970-01-01T00:00:00Z\n"
        "LABEL "
        + " \\\n      ".join(
            f'{name}="{value}"' for name, value in pins.EXPECTED_OCI_LABELS.items()
        )
        + "\n"
        "COPY deployments/containers/tool-assets.lock /tmp/tool-assets.lock\n"
        "RUN sha256sum --check /tmp/tool-assets.lock; \\\n"
        + "; \\\n".join(
            f'download "{url}" "${{tmp}}/asset"' for url in sorted(pins.EXPECTED_DOWNLOAD_URLS)
        )
        + "; \\\n"
        + "; \\\n".join(sorted(pins.EXPECTED_VERIFY_CALLS))
        + "\n"
    )
    lock = root / pins.TOOL_ASSET_LOCK
    lock.write_text(
        "# tool version architecture asset sha256\n"
        + "".join(
            f"{tool} {version} {architecture} {asset} {digest}\n"
            for (tool, version, architecture), (asset, digest) in sorted(
                pins.EXPECTED_TOOL_ASSETS.items()
            )
        )
    )

    compose = root / pins.COMPOSE_FILE
    compose.parent.mkdir(parents=True)
    compose.write_text(
        yaml.safe_dump(
            {
                "services": {"dev": pins._EXPECTED_COMPOSE_DEV_SERVICE},
                "volumes": {name: {} for name in pins._EXPECTED_COMPOSE_NAMED_VOLUMES},
            },
            sort_keys=False,
        )
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
        "    timeout-minutes: 90\n"
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
        "          enable-cache: false\n"
        "      - name: Verify host Podman\n"
        "        run: podman version\n"
        "      - name: Start private Podman API service\n"
        "        shell: bash\n"
        "        run: |\n" + service_script + "      - name: Build development container\n"
        "        run: ./dev up\n"
        "      - name: Run complete foundation gate\n"
        "        run: ./dev task all\n"
        "      - name: Report container status on failure\n"
        "        if: failure()\n"
        "        continue-on-error: true\n"
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


def test_accepts_required_environment_expansion_in_volume_source(tmp_path: Path) -> None:
    assert (
        pins._volume_source("${NUTANIX_GIT_COMMON_DIR:?required}:/git-metadata/.git:z")
        == "${NUTANIX_GIT_COMMON_DIR:?required}"
    )


def test_rejects_named_volume_bind_override(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    compose = tmp_path / pins.COMPOSE_FILE
    compose.write_text(
        "services:\n"
        "  dev:\n"
        "    volumes:\n"
        "      - uv-cache:/opt/uv-cache\n"
        "volumes:\n"
        "  go-mod-cache: {}\n"
        "  go-build-cache: {}\n"
        "  uv-cache:\n"
        "    driver_opts:\n"
        "      type: none\n"
        "      o: bind\n"
        "      device: /run/user/1000/podman\n"
    )

    assert "Compose named volume must be internal and empty: uv-cache" in pins._compose_diagnostics(
        tmp_path
    )


def test_rejects_privileged_host_namespaces_and_socket_directory_mount(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    compose = tmp_path / pins.COMPOSE_FILE
    compose.write_text(
        "services:\n"
        "  dev:\n"
        "    privileged: true\n"
        "    pid: host\n"
        "    network_mode: host\n"
        "    volumes:\n"
        "      - /run/podman:/run/podman\n"
    )

    diagnostics = pins.validate(tmp_path)

    assert "Compose privileged execution is forbidden" in diagnostics
    assert "Compose host pid namespace is forbidden" in diagnostics
    assert "Compose host network namespace is forbidden" in diagnostics
    assert "Compose runtime socket directory mount is forbidden" in diagnostics


def test_rejects_extends_quoted_privileged_and_api_socket(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    compose = tmp_path / pins.COMPOSE_FILE
    compose.write_text(
        "services:\n"
        "  dev:\n"
        "    extends:\n"
        "      file: escape.yml\n"
        "      service: base\n"
        '    privileged: "true"\n'
        "    use_api_socket: true\n"
        "volumes:\n"
        "  go-mod-cache: {}\n"
        "  go-build-cache: {}\n"
        "  uv-cache: {}\n"
    )

    diagnostics = pins._compose_diagnostics(tmp_path)

    assert "Compose service key is not allowed: extends" in diagnostics
    assert "Compose service key is not allowed: privileged" in diagnostics
    assert "Compose service key is not allowed: use_api_socket" in diagnostics


def test_rejects_build_ssh_forwarding(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    compose = tmp_path / pins.COMPOSE_FILE
    document = yaml.safe_load(compose.read_text())
    service = copy.deepcopy(document["services"]["dev"])
    service["build"]["ssh"] = ["default"]
    document["services"]["dev"] = service
    compose.write_text(yaml.safe_dump(document, sort_keys=False))

    assert "Compose dev service model differs" in pins._compose_diagnostics(tmp_path)


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


def test_required_ci_gate_cannot_be_conditionally_skipped_or_ignored(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    content = workflow.read_text()
    content = content.replace(
        "      - name: Run complete foundation gate\n        run: ./dev task all\n",
        "      - name: Run complete foundation gate\n"
        "        if: false\n"
        "        continue-on-error: true\n"
        "        run: ./dev task all\n",
    )
    workflow.write_text(content)

    diagnostics = pins.validate(tmp_path)

    assert "CI complete foundation gate must be unconditional" in diagnostics
    assert "CI complete foundation gate must fail the job" in diagnostics


def test_required_ci_gate_rejects_shell_and_environment_overrides(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    content = workflow.read_text()
    content = content.replace(
        "jobs:\n",
        "defaults:\n  run:\n    shell: bash -c 'true' -- {0}\njobs:\n",
    )
    content = content.replace(
        "  foundation:\n",
        "  foundation:\n    defaults:\n      run:\n        shell: bash -c 'true' -- {0}\n",
    )
    content = content.replace(
        "      - name: Run complete foundation gate\n        run: ./dev task all\n",
        "      - name: Run complete foundation gate\n"
        "        shell: bash -c 'true' -- {0}\n"
        "        env:\n"
        "          BASH_ENV: bypass.sh\n"
        "        run: ./dev task all\n",
    )
    workflow.write_text(content)

    diagnostics = pins.validate(tmp_path)

    assert "CI workflow execution defaults are forbidden" in diagnostics
    assert "CI foundation job execution defaults are forbidden" in diagnostics
    assert "CI complete foundation gate execution overrides are forbidden" in diagnostics


def test_ci_rejects_local_actions_and_pre_gate_execution_overrides(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    content = workflow.read_text()
    content = content.replace(
        "      - name: Build development container\n        run: ./dev up\n",
        "      - name: Mutate launcher\n"
        "        uses: ./ci/bypass\n"
        "      - name: Build development container\n"
        "        shell: bash -c 'true' -- {0}\n"
        "        env:\n"
        "          BASH_ENV: bypass.sh\n"
        "        run: ./dev up\n",
    )
    workflow.write_text(content)

    diagnostics = pins.validate(tmp_path)

    assert (
        ".github/workflows/ci.yml: local action reference is forbidden: ./ci/bypass" in diagnostics
    )
    assert "CI foundation ordered step model differs" in diagnostics


def test_ci_rejects_skipped_job_dependency(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    workflow = tmp_path / ".github" / "workflows" / "ci.yml"
    content = workflow.read_text().replace(
        "jobs:\n  foundation:\n",
        "jobs:\n"
        "  prerequisite:\n"
        "    if: false\n"
        "    runs-on: ubuntu-24.04\n"
        "    steps:\n"
        "      - run: true\n"
        "  foundation:\n"
        "    needs: prerequisite\n",
    )
    workflow.write_text(content)

    diagnostics = pins.validate(tmp_path)

    assert "CI job set differs" in diagnostics
    assert "CI foundation job model differs" in diagnostics


def test_ci_rejects_additional_workflow(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    extra = tmp_path / ".github" / "workflows" / "extra.yml"
    extra.write_text(
        "name: Bypass\n"
        "on: pull_request\n"
        "permissions:\n"
        "  contents: write\n"
        "jobs:\n"
        "  bypass:\n"
        "    name: Foundation\n"
        "    runs-on: ubuntu-24.04\n"
        "    steps:\n"
        "      - run: true\n"
    )

    assert "GitHub workflow file set differs" in pins.validate(tmp_path)


def test_requires_oci_labels_official_downloads_and_locked_verification(tmp_path: Path) -> None:
    valid_pin_repository(tmp_path)
    container = tmp_path / pins.CONTAINERFILE
    content = container.read_text()
    content = content.replace(
        "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/${task_asset}",
        "https://attacker.invalid/task",
    )
    content = content.replace(
        'verify_locked task "${TASK_VERSION}" "${arch}" "${task_asset}"', "true"
    )
    content = content.replace("org.opencontainers.image.description=", "invalid.label=")
    container.write_text(content)

    diagnostics = pins.validate(tmp_path)

    assert "development OCI label set differs" in diagnostics
    assert "tool download origin differs: task" in diagnostics
    assert "repository-owned tool asset verification is missing: task" in diagnostics
