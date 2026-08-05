from __future__ import annotations

import importlib
import importlib.util
import io
import os
import stat
import subprocess
from collections.abc import Iterator, Mapping, Sequence
from contextlib import contextmanager
from pathlib import Path
from types import ModuleType, SimpleNamespace
from typing import Any, cast

import pytest
import yaml
from scripts.automation.podman_api import (
    AmbiguousContainerError,
    ContainerNotFoundError,
    ExecResult,
    PodmanAPIError,
    PodmanConfigurationError,
    PodmanSocketError,
)
from scripts.automation.process import CommandError, CommandResult


class FakeRunner:
    def __init__(self) -> None:
        self.calls: list[tuple[tuple[str, ...], dict[str, object]]] = []
        self.gh_token = "token-from-gh"
        self.result = CommandResult((), 0, "runner stdout\n", "runner stderr\n")

    def __call__(self, arguments: Sequence[str], **kwargs: object) -> CommandResult:
        normalized = tuple(arguments)
        self.calls.append((normalized, kwargs))
        if normalized[-2:] == ("rev-parse", "HEAD"):
            return CommandResult(normalized, 0, "a" * 40 + "\n", "")
        if normalized[-4:] == ("show", "-s", "--format=%cI", "HEAD"):
            return CommandResult(normalized, 0, "2026-08-05T00:00:00+00:00\n", "")
        if normalized == ("gh", "auth", "token"):
            return CommandResult(normalized, 0, self.gh_token + "\n", "")
        return CommandResult(
            normalized,
            self.result.returncode,
            self.result.stdout,
            self.result.stderr,
        )


class FakeAPI:
    def __init__(self) -> None:
        self.container = SimpleNamespace(name="project-a_dev_1", status="running")
        self.wait_calls: list[dict[str, object]] = []
        self.find_calls = 0
        self.status_error: PodmanAPIError | None = None
        self.exec_calls: list[tuple[tuple[str, ...], dict[str, object]]] = []
        self.results: list[ExecResult] = []

    def wait_until_healthy(self, **kwargs: object) -> object:
        self.wait_calls.append(kwargs)
        return self.container

    def find_container(self) -> object:
        self.find_calls += 1
        return self.container

    def status(self) -> object:
        self.find_calls += 1
        if self.status_error is not None:
            raise self.status_error
        return SimpleNamespace(name=self.container.name, state=self.container.status)

    def exec(self, arguments: Sequence[str], **kwargs: object) -> ExecResult:
        self.exec_calls.append((tuple(arguments), kwargs))
        if self.results:
            return self.results.pop(0)
        return ExecResult(0, b"exec stdout\n", b"exec stderr\n")


def _cli() -> ModuleType:
    spec = importlib.util.find_spec("scripts.automation.cli")
    assert spec is not None, "scripts.automation.cli is not implemented"
    return importlib.import_module("scripts.automation.cli")


def _project(tmp_path: Path) -> SimpleNamespace:
    root = tmp_path / "checkout"
    root.mkdir()
    (root / "VERSION").write_text("0.0.0-dev\n")
    common = tmp_path / "repository.git"
    return SimpleNamespace(
        root=root,
        git_common_dir=common,
        git_dir=common / "worktrees" / "feature",
        git_dir_relative=Path("worktrees/feature"),
        name="project-a",
    )


def _write_beads_marker(project: object) -> Path:
    root = cast(Path, getattr(project, "root"))
    marker = root / ".beads" / "config.yaml"
    marker.parent.mkdir()
    marker.write_text("backend: dolt\n")
    return marker


def _connector(api: FakeAPI, calls: list[tuple[str, str]]) -> Any:
    @contextmanager
    def connect(project: str, service: str) -> Iterator[FakeAPI]:
        calls.append((project, service))
        yield api

    return connect


def _main(
    cli: ModuleType,
    arguments: Sequence[str],
    *,
    project: object,
    runner: FakeRunner,
    api: FakeAPI,
    env: Mapping[str, str] | None = None,
    connector: object | None = None,
    execvpe: object | None = None,
) -> tuple[int, bytes, bytes, list[tuple[str, str]]]:
    stdout = io.BytesIO()
    stderr = io.BytesIO()
    connector_calls: list[tuple[str, str]] = []
    selected_connector = connector or _connector(api, connector_calls)
    kwargs: dict[str, object] = {
        "project_factory": lambda: project,
        "runner": runner,
        "connector": selected_connector,
        "env": dict(env or {}),
        "stdout": stdout,
        "stderr": stderr,
    }
    if execvpe is not None:
        kwargs["execvpe"] = execvpe
    exit_code = cli.main(tuple(arguments), **kwargs)
    return exit_code, stdout.getvalue(), stderr.getvalue(), connector_calls


def test_dev_wrapper_uses_exact_uv_bootstrap_and_preserves_arguments() -> None:
    path = Path("dev")
    assert path.exists(), "dev wrapper is not implemented"
    assert path.stat().st_mode & stat.S_IXUSR
    assert path.read_text().splitlines() == [
        "#!/bin/sh",
        "set -eu",
        'exec uvx --from uv==0.12.1 uv run --frozen python -m scripts.automation.cli "$@"',
    ]


def test_beads_config_uses_repository_text_canonicalization() -> None:
    config = Path(".beads/config.yaml").read_bytes()

    assert config.endswith(
        b'sync.remote: "git+https://github.com/ioplane/terraform-provider-nutanix.git"\n'
    )


@pytest.mark.parametrize("command", ["up", "down", "status", "shell", "task", "beads"])
def test_parser_exposes_exact_commands(command: str) -> None:
    cli = _cli()
    arguments = [command]
    if command in {"task", "beads"}:
        arguments.append("example")
    parsed = cli.build_parser().parse_args(arguments)
    assert parsed.command == command


def test_up_uses_fixed_compose_command_metadata_and_waits_healthy(tmp_path: Path) -> None:
    cli = _cli()
    project = _project(tmp_path)
    runner = FakeRunner()
    api = FakeAPI()
    host_env = {
        "PATH": "/bin",
        "GH_TOKEN": "must-not-reach-compose",
        "GITHUB_TOKEN": "must-not-reach-either",
    }

    exit_code, stdout, stderr, connector_calls = _main(
        cli, ("up",), project=project, runner=runner, api=api, env=host_env
    )

    compose_call = runner.calls[-1]
    assert compose_call[0] == (
        "podman-compose",
        "-p",
        "project-a",
        "-f",
        str(project.root / "deployments/compose/compose.dev.yml"),
        "up",
        "--detach",
        "--build",
        "--force-recreate",
    )
    compose_env_value = compose_call[1]["env"]
    assert isinstance(compose_env_value, dict)
    compose_env = cast(dict[str, str], compose_env_value)
    assert compose_env["NUTANIX_DEV_VERSION"] == "0.0.0-dev"
    assert compose_env["NUTANIX_DEV_REVISION"] == "a" * 40
    assert compose_env["NUTANIX_DEV_CREATED"] == "2026-08-05T00:00:00+00:00"
    assert compose_env["NUTANIX_GIT_COMMON_DIR"] == str(project.git_common_dir)
    assert compose_env["NUTANIX_GIT_DIR_RELATIVE"] == "worktrees/feature"
    assert "GH_TOKEN" not in compose_env
    assert "GITHUB_TOKEN" not in compose_env
    assert all("env" in kwargs for _, kwargs in runner.calls)
    assert all(
        "GH_TOKEN" not in cast(dict[str, str], kwargs["env"])
        and "GITHUB_TOKEN" not in cast(dict[str, str], kwargs["env"])
        for _, kwargs in runner.calls
    )
    assert compose_call[1]["timeout"] == cli.COMPOSE_UP_TIMEOUT_SECONDS
    assert connector_calls == [("project-a", "dev")]
    assert api.wait_calls == [{"timeout": cli.HEALTH_TIMEOUT_SECONDS}]
    assert exit_code == 0
    assert stdout == b"runner stdout\n"
    assert stderr == b"runner stderr\n"


def test_down_uses_exact_bounded_compose_command_without_api(tmp_path: Path) -> None:
    cli = _cli()
    project = _project(tmp_path)
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, _, _, connector_calls = _main(
        cli, ("down",), project=project, runner=runner, api=api
    )

    assert runner.calls[-1][0][-1] == "down"
    assert runner.calls[-1][1]["timeout"] == cli.COMPOSE_DOWN_TIMEOUT_SECONDS
    assert connector_calls == []
    assert exit_code == 0


@pytest.mark.parametrize(
    ("command", "arguments", "expected"),
    [
        (
            "task",
            ("python:test", "--", "tests/automation"),
            ("task", "python:test", "--", "tests/automation"),
        ),
        ("beads", ("--version",), ("bd", "--version")),
    ],
)
def test_noninteractive_commands_wait_and_forward_exact_arguments_without_token(
    tmp_path: Path,
    command: str,
    arguments: tuple[str, ...],
    expected: tuple[str, ...],
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()
    project = _project(tmp_path)
    if command == "beads":
        _write_beads_marker(project)

    exit_code, stdout, stderr, _ = _main(
        cli,
        (command, *arguments),
        project=project,
        runner=runner,
        api=api,
        env={"GH_TOKEN": "host-secret"},
    )

    assert api.wait_calls == [{"timeout": cli.HEALTH_TIMEOUT_SECONDS}]
    expected_timeout = (
        cli.TASK_COMMAND_TIMEOUT_SECONDS if command == "task" else cli.BEADS_COMMAND_TIMEOUT_SECONDS
    )
    expected_environment = {} if command == "task" else {"BEADS_DIR": "/workspace/.beads"}
    assert api.exec_calls == [
        (expected, {"environment": expected_environment, "timeout": expected_timeout})
    ]
    assert exit_code == 0
    assert stdout == b"exec stdout\n"
    assert stderr == b"exec stderr\n"


@pytest.mark.parametrize(
    "arguments",
    [
        ("where",),
        ("--quiet", "init", "--skip-agents"),
    ],
)
def test_launcher_beads_without_marker_rejects_ordinary_command_before_operational_boundaries(
    tmp_path: Path, arguments: tuple[str, ...]
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, stdout, stderr, connector_calls = _main(
        cli,
        ("beads", *arguments),
        project=_project(tmp_path),
        runner=runner,
        api=api,
    )

    assert exit_code == 1
    assert stdout == b""
    assert stderr == (
        b"error: Beads is not initialized in this worktree; "
        b"run './dev beads init --skip-agents' first\n"
    )
    assert connector_calls == []
    assert api.wait_calls == []
    assert api.exec_calls == []
    assert runner.calls == []


def test_launcher_remote_beads_without_marker_rejects_before_credentials_or_connector(
    tmp_path: Path,
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, stdout, stderr, connector_calls = _main(
        cli,
        ("beads", "dolt", "pull"),
        project=_project(tmp_path),
        runner=runner,
        api=api,
    )

    assert exit_code == 1
    assert stdout == b""
    assert b"Beads is not initialized in this worktree" in stderr
    assert connector_calls == []
    assert api.wait_calls == []
    assert api.exec_calls == []
    assert runner.calls == []


def test_launcher_bootstrap_without_marker_rejects_before_credentials_or_connector(
    tmp_path: Path,
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, stdout, stderr, connector_calls = _main(
        cli,
        ("beads", "bootstrap", "--non-interactive"),
        project=_project(tmp_path),
        runner=runner,
        api=api,
    )

    assert exit_code == 1
    assert stdout == b""
    assert b"Beads is not initialized in this worktree" in stderr
    assert connector_calls == []
    assert api.wait_calls == []
    assert api.exec_calls == []
    assert runner.calls == []


def test_beads_init_without_marker_uses_exact_worktree_environment(tmp_path: Path) -> None:
    cli = _cli()
    api = FakeAPI()

    exit_code, _, _, connector_calls = _main(
        cli,
        ("beads", "init", "--skip-agents"),
        project=_project(tmp_path),
        runner=FakeRunner(),
        api=api,
    )

    assert exit_code == 0
    assert connector_calls == [("project-a", "dev")]
    assert api.exec_calls == [
        (
            ("bd", "init", "--skip-agents"),
            {
                "environment": {"BEADS_DIR": "/workspace/.beads"},
                "timeout": cli.BEADS_COMMAND_TIMEOUT_SECONDS,
            },
        )
    ]


def test_beads_with_regular_marker_preserves_exact_worktree_environment(tmp_path: Path) -> None:
    cli = _cli()
    project = _project(tmp_path)
    _write_beads_marker(project)
    api = FakeAPI()

    exit_code, _, _, connector_calls = _main(
        cli,
        ("beads", "where"),
        project=project,
        runner=FakeRunner(),
        api=api,
    )

    assert exit_code == 0
    assert connector_calls == [("project-a", "dev")]
    assert api.exec_calls == [
        (
            ("bd", "where"),
            {
                "environment": {"BEADS_DIR": "/workspace/.beads"},
                "timeout": cli.BEADS_COMMAND_TIMEOUT_SECONDS,
            },
        )
    ]


@pytest.mark.parametrize("marker_kind", ["directory", "symlink"])
def test_beads_rejects_non_regular_config_marker(tmp_path: Path, marker_kind: str) -> None:
    cli = _cli()
    project = _project(tmp_path)
    marker = project.root / ".beads" / "config.yaml"
    marker.parent.mkdir()
    if marker_kind == "directory":
        marker.mkdir()
    else:
        external = tmp_path / "external-config.yaml"
        external.write_text("backend: dolt\n")
        marker.symlink_to(external)
    api = FakeAPI()

    exit_code, _, _, connector_calls = _main(
        cli,
        ("beads", "where"),
        project=project,
        runner=FakeRunner(),
        api=api,
    )

    assert exit_code == 1
    assert connector_calls == []
    assert api.exec_calls == []


def test_symlinked_beads_parent_rejects_remote_before_token_connector_or_api(
    tmp_path: Path,
) -> None:
    cli = _cli()
    project = _project(tmp_path)
    external = tmp_path / "external-fallback"
    (external / "config.yaml").parent.mkdir(parents=True)
    (external / "config.yaml").write_text("backend: dolt\n")
    (project.root / ".beads").symlink_to(external, target_is_directory=True)
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, stdout, stderr, connector_calls = _main(
        cli,
        ("beads", "dolt", "pull"),
        project=project,
        runner=runner,
        api=api,
        env={"GH_TOKEN": "primary", "GITHUB_TOKEN": "secondary"},
    )

    assert exit_code == 1
    assert stdout == b""
    assert b"Beads is not initialized in this worktree" in stderr
    assert runner.calls == []
    assert connector_calls == []
    assert api.wait_calls == []
    assert api.exec_calls == []


def test_main_discovers_project_with_three_scrubbed_bounded_git_calls_before_guard(
    tmp_path: Path,
) -> None:
    cli = _cli()
    project = _project(tmp_path)

    class DiscoveryRunner:
        def __init__(self) -> None:
            self.calls: list[tuple[tuple[str, ...], dict[str, object]]] = []

        def __call__(self, arguments: Sequence[str], **kwargs: object) -> CommandResult:
            normalized = tuple(arguments)
            self.calls.append((normalized, kwargs))
            if "--show-toplevel" in normalized:
                output = project.root
            elif "--git-common-dir" in normalized:
                output = project.git_common_dir
            else:
                output = project.git_dir
            return CommandResult(normalized, 0, f"{output}\n", "")

    runner = DiscoveryRunner()
    api = FakeAPI()
    connector_calls: list[tuple[str, str]] = []
    stdout = io.BytesIO()
    stderr = io.BytesIO()
    host_env = {
        "PATH": "/bin",
        "GH_TOKEN": "primary",
        "GITHUB_TOKEN": "secondary",
    }

    exit_code = cli.main(
        ("beads", "where"),
        runner=runner,
        connector=_connector(api, connector_calls),
        env=host_env,
        stdout=stdout,
        stderr=stderr,
    )

    root = project.root.resolve()
    expected_calls = [
        (
            "git",
            "-C",
            str(Path.cwd().resolve()),
            "rev-parse",
            "--path-format=absolute",
            "--show-toplevel",
        ),
        (
            "git",
            "-C",
            str(root),
            "rev-parse",
            "--path-format=absolute",
            "--git-common-dir",
        ),
        (
            "git",
            "-C",
            str(root),
            "rev-parse",
            "--path-format=absolute",
            "--git-dir",
        ),
    ]
    assert [arguments for arguments, _ in runner.calls] == expected_calls
    assert [kwargs for _, kwargs in runner.calls] == [
        {"env": {"PATH": "/bin"}, "timeout": 15.0},
        {"env": {"PATH": "/bin"}, "timeout": 15.0},
        {"env": {"PATH": "/bin"}, "timeout": 15.0},
    ]
    assert all(arguments[0] == "git" for arguments, _ in runner.calls)
    assert exit_code == 1
    assert stdout.getvalue() == b""
    assert b"Beads is not initialized in this worktree" in stderr.getvalue()
    assert connector_calls == []
    assert api.wait_calls == []
    assert api.exec_calls == []


@pytest.mark.parametrize(
    ("env", "expected_token"),
    [
        ({"GH_TOKEN": "gh-primary", "GITHUB_TOKEN": "github-secondary"}, "gh-primary"),
        ({"GH_TOKEN": " ", "GITHUB_TOKEN": "github-secondary"}, "github-secondary"),
    ],
)
def test_remote_beads_uses_precedence_per_exec_and_redacts_streams(
    tmp_path: Path, env: dict[str, str], expected_token: str
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()
    secret = expected_token.encode()
    api.results = [
        ExecResult(0, b"setup " + secret + b"\n", b""),
        ExecResult(7, b"push " + secret + b"\n", b"error " + secret + b"\n"),
    ]
    project = _project(tmp_path)
    _write_beads_marker(project)
    before = sorted(project.root.iterdir())

    exit_code, stdout, stderr, _ = _main(
        cli,
        ("beads", "dolt", "push", "--force-with-lease"),
        project=project,
        runner=runner,
        api=api,
        env=env,
    )

    injected = {"GH_TOKEN": expected_token}
    beads_environment = {
        "BEADS_DIR": "/workspace/.beads",
        "GH_TOKEN": expected_token,
    }
    assert api.exec_calls == [
        (
            ("gh", "auth", "setup-git"),
            {
                "environment": injected,
                "timeout": cli.REMOTE_SETUP_TIMEOUT_SECONDS,
            },
        ),
        (
            (
                "/usr/bin/env",
                "-u",
                "GIT_COMMON_DIR",
                "-u",
                "GIT_DIR",
                "-u",
                "GIT_WORK_TREE",
                "bd",
                "dolt",
                "push",
                "--force-with-lease",
            ),
            {
                "environment": beads_environment,
                "timeout": cli.REMOTE_BEADS_TIMEOUT_SECONDS,
            },
        ),
    ]
    assert exit_code == 7
    assert secret not in stdout + stderr
    assert b"[REDACTED]" in stdout + stderr
    assert all(expected_token not in argument for call, _ in api.exec_calls for argument in call)
    assert sorted(project.root.iterdir()) == before
    assert ("gh", "auth", "token") not in [call for call, _ in runner.calls]


def test_remote_beads_bootstrap_uses_sanitized_git_environment(tmp_path: Path) -> None:
    cli = _cli()
    api = FakeAPI()
    project = _project(tmp_path)
    _write_beads_marker(project)

    exit_code, _, _, _ = _main(
        cli,
        ("beads", "bootstrap", "--non-interactive"),
        project=project,
        runner=FakeRunner(),
        api=api,
        env={"GH_TOKEN": "bootstrap-token"},
    )

    assert api.exec_calls == [
        (
            ("gh", "auth", "setup-git"),
            {
                "environment": {"GH_TOKEN": "bootstrap-token"},
                "timeout": cli.REMOTE_SETUP_TIMEOUT_SECONDS,
            },
        ),
        (
            (
                "/usr/bin/env",
                "-u",
                "GIT_COMMON_DIR",
                "-u",
                "GIT_DIR",
                "-u",
                "GIT_WORK_TREE",
                "bd",
                "bootstrap",
                "--non-interactive",
            ),
            {
                "environment": {
                    "BEADS_DIR": "/workspace/.beads",
                    "GH_TOKEN": "bootstrap-token",
                },
                "timeout": cli.REMOTE_BEADS_TIMEOUT_SECONDS,
            },
        ),
        (
            ("python", "-m", "scripts.automation.beads_config"),
            {
                "environment": {"BEADS_DIR": "/workspace/.beads"},
                "timeout": cli.BEADS_COMMAND_TIMEOUT_SECONDS,
            },
        ),
    ]
    assert exit_code == 0


@pytest.mark.parametrize(
    ("results", "expected_exit", "expected_exec_count"),
    [
        ([ExecResult(0, b"", b""), ExecResult(9, b"", b"bootstrap failed")], 9, 2),
        (
            [
                ExecResult(0, b"", b""),
                ExecResult(0, b"", b""),
                ExecResult(8, b"", b"normalization failed"),
            ],
            8,
            3,
        ),
    ],
)
def test_bootstrap_propagates_remote_or_normalizer_failure(
    tmp_path: Path,
    results: list[ExecResult],
    expected_exit: int,
    expected_exec_count: int,
) -> None:
    cli = _cli()
    api = FakeAPI()
    api.results = results
    project = _project(tmp_path)
    _write_beads_marker(project)

    exit_code, _, stderr, _ = _main(
        cli,
        ("beads", "bootstrap", "--non-interactive"),
        project=project,
        runner=FakeRunner(),
        api=api,
        env={"GH_TOKEN": "bootstrap-token"},
    )

    assert exit_code == expected_exit
    assert len(api.exec_calls) == expected_exec_count
    assert b"failed" in stderr


@pytest.mark.parametrize("argument", ["--dry-run", "--dry-run=true", "--dry-run=t", "--dry-run=1"])
def test_bootstrap_dry_run_never_invokes_normalizer(tmp_path: Path, argument: str) -> None:
    cli = _cli()
    api = FakeAPI()
    project = _project(tmp_path)
    _write_beads_marker(project)

    exit_code, _, _, _ = _main(
        cli,
        ("beads", "bootstrap", argument),
        project=project,
        runner=FakeRunner(),
        api=api,
        env={"GH_TOKEN": "bootstrap-token"},
    )

    assert exit_code == 0
    assert len(api.exec_calls) == 2
    assert api.exec_calls[-1][0][-2:] == ("bootstrap", argument)


@pytest.mark.parametrize(
    "argument",
    ["--help", "--help=true", "-h", "-h=true", "--version", "--version=true"],
)
def test_bootstrap_information_never_resolves_credentials_or_normalizes(
    tmp_path: Path, argument: str
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()
    project = _project(tmp_path)
    _write_beads_marker(project)

    exit_code, _, _, _ = _main(
        cli,
        ("beads", "bootstrap", argument),
        project=project,
        runner=runner,
        api=api,
    )

    assert exit_code == 0
    assert api.exec_calls == [
        (
            ("bd", "bootstrap", argument),
            {
                "environment": {"BEADS_DIR": "/workspace/.beads"},
                "timeout": cli.BEADS_COMMAND_TIMEOUT_SECONDS,
            },
        )
    ]
    assert runner.calls == []


def test_bootstrap_preserves_preexisting_noncanonical_config(tmp_path: Path) -> None:
    cli = _cli()
    api = FakeAPI()
    project = _project(tmp_path)
    marker = _write_beads_marker(project)
    marker.write_bytes(b"backend: dolt")

    exit_code, _, _, _ = _main(
        cli,
        ("beads", "bootstrap", "--non-interactive"),
        project=project,
        runner=FakeRunner(),
        api=api,
        env={"GH_TOKEN": "bootstrap-token"},
    )

    assert exit_code == 0
    assert len(api.exec_calls) == 2


def test_remote_beads_falls_back_to_bounded_host_gh_token(tmp_path: Path) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()

    project = _project(tmp_path)
    _write_beads_marker(project)
    exit_code, _, _, _ = _main(
        cli,
        ("beads", "dolt", "pull"),
        project=project,
        runner=runner,
        api=api,
    )

    gh_call = next(call for call in runner.calls if call[0] == ("gh", "auth", "token"))
    assert gh_call[1]["timeout"] == cli.GH_AUTH_TIMEOUT_SECONDS
    assert gh_call[1]["env"] == {}
    assert api.exec_calls[0][1] == {
        "environment": {"GH_TOKEN": "token-from-gh"},
        "timeout": cli.REMOTE_SETUP_TIMEOUT_SECONDS,
    }
    assert api.exec_calls[1][1] == {
        "environment": {
            "BEADS_DIR": "/workspace/.beads",
            "GH_TOKEN": "token-from-gh",
        },
        "timeout": cli.REMOTE_BEADS_TIMEOUT_SECONDS,
    }
    assert api.exec_calls[1][0] == (
        "/usr/bin/env",
        "-u",
        "GIT_COMMON_DIR",
        "-u",
        "GIT_DIR",
        "-u",
        "GIT_WORK_TREE",
        "bd",
        "dolt",
        "pull",
    )
    assert exit_code == 0


def test_remote_beads_reports_missing_credentials_without_leaking_gh_output(
    tmp_path: Path,
) -> None:
    cli = _cli()

    class FailingGHRunner(FakeRunner):
        def __call__(self, arguments: Sequence[str], **kwargs: object) -> CommandResult:
            if tuple(arguments) == ("gh", "auth", "token"):
                result = CommandResult(tuple(arguments), 1, "host-secret\n", "denied\n")
                raise CommandError(result)
            return super().__call__(arguments, **kwargs)

    api = FakeAPI()
    project = _project(tmp_path)
    _write_beads_marker(project)
    exit_code, stdout, stderr, _ = _main(
        cli,
        ("beads", "dolt", "pull"),
        project=project,
        runner=FailingGHRunner(),
        api=api,
    )

    assert exit_code == 1
    assert b"host-secret" not in stdout + stderr
    assert b"authentication token is unavailable" in stderr
    assert api.exec_calls == []


def test_real_beads_dot_git_common_selects_only_sibling_fallback(tmp_path: Path) -> None:
    cli = _cli()
    metadata_root = tmp_path / "git-metadata"
    common = metadata_root / ".git"
    primary = tmp_path / "primary"
    linked = tmp_path / "linked"
    isolated_env = dict(os.environ)
    for name in ("BEADS_DIR", "GIT_COMMON_DIR", "GIT_DIR", "GIT_WORK_TREE"):
        isolated_env.pop(name, None)
    isolated_env.update(
        {
            "BD_NON_INTERACTIVE": "1",
            "GIT_CONFIG_GLOBAL": "/dev/null",
            "GIT_CONFIG_NOSYSTEM": "1",
            "XDG_CONFIG_HOME": str(tmp_path / "xdg-config"),
            "XDG_DATA_HOME": str(tmp_path / "xdg-data"),
            "XDG_STATE_HOME": str(tmp_path / "xdg-state"),
        }
    )

    def invoke(
        *arguments: str,
        env: Mapping[str, str] = isolated_env,
        cwd: Path | None = None,
    ) -> subprocess.CompletedProcess[str]:
        result = subprocess.run(
            arguments,
            check=False,
            capture_output=True,
            cwd=cwd,
            env=env,
            text=True,
            timeout=60,
        )
        assert result.returncode == 0, (
            f"command failed: {arguments!r}\nstdout: {result.stdout}\nstderr: {result.stderr}"
        )
        return result

    metadata_root.mkdir()
    invoke("git", "init", "--separate-git-dir", str(common), str(primary))
    invoke("git", "-C", str(primary), "config", "user.name", "Boundary Test")
    invoke("git", "-C", str(primary), "config", "user.email", "boundary@example.invalid")
    invoke("git", "-C", str(primary), "commit", "--allow-empty", "-m", "initial")
    invoke("git", "-C", str(primary), "worktree", "add", "-b", "boundary", str(linked))

    resolved_common = Path(
        invoke("git", "-C", str(linked), "rev-parse", "--git-common-dir").stdout.strip()
    ).resolve()
    assert resolved_common == common.resolve()
    assert resolved_common.name == ".git"

    host_fallback = common / ".beads"
    host_init_env = {**isolated_env, "BEADS_DIR": str(host_fallback)}
    invoke(
        "bd",
        "init",
        "--skip-agents",
        "--skip-hooks",
        "--non-interactive",
        "--prefix",
        "host",
        env=host_init_env,
        cwd=linked,
    )
    sibling_fallback = metadata_root / ".beads"
    sibling_init_env = {**isolated_env, "BEADS_DIR": str(sibling_fallback)}
    invoke(
        "bd",
        "init",
        "--skip-agents",
        "--skip-hooks",
        "--non-interactive",
        "--prefix",
        "container",
        env=sibling_init_env,
        cwd=linked,
    )
    local_marker = linked / ".beads" / "config.yaml"
    assert (host_fallback / "config.yaml").is_file()
    assert (sibling_fallback / "config.yaml").is_file()
    assert not local_marker.exists()

    raw_env = {**isolated_env, "BEADS_DIR": str(linked / ".beads")}
    raw_where = invoke("bd", "where", env=raw_env, cwd=linked)
    selected = Path(raw_where.stdout.splitlines()[0]).resolve()
    assert selected == sibling_fallback.resolve()
    assert selected != host_fallback.resolve()

    project = SimpleNamespace(
        root=linked,
        git_common_dir=common,
        git_dir=common / "worktrees" / "linked",
        git_dir_relative=Path("worktrees/linked"),
        name="boundary-project",
    )
    api = FakeAPI()
    exit_code, stdout, stderr, connector_calls = _main(
        cli,
        ("beads", "where"),
        project=project,
        runner=FakeRunner(),
        api=api,
    )

    assert exit_code == 1
    assert stdout == b""
    assert b"Beads is not initialized in this worktree" in stderr
    assert connector_calls == []
    assert api.exec_calls == []


def test_status_uses_api_and_only_status_has_bounded_ps_fallback(tmp_path: Path) -> None:
    cli = _cli()
    project = _project(tmp_path)
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, stdout, _, _ = _main(cli, ("status",), project=project, runner=runner, api=api)
    assert exit_code == 0
    assert api.find_calls == 1
    assert b"project-a_dev_1 running" in stdout
    assert not runner.calls

    @contextmanager
    def unavailable(_: str, __: str) -> Iterator[FakeAPI]:
        raise PodmanSocketError((Path("/missing.sock"),))
        yield api

    runner = FakeRunner()
    exit_code, _, _, _ = _main(
        cli,
        ("status",),
        project=project,
        runner=runner,
        api=api,
        connector=unavailable,
    )
    assert exit_code == 0
    assert runner.calls == [
        (
            (
                "podman",
                "ps",
                "--filter",
                "label=io.podman.compose.project=project-a",
                "--filter",
                "label=io.podman.compose.service=dev",
                "--format",
                "{{.Names}} {{.Status}}",
            ),
            {"env": {}, "timeout": cli.STATUS_TIMEOUT_SECONDS},
        )
    ]


def test_status_uses_bounded_ps_fallback_for_api_unavailability(tmp_path: Path) -> None:
    cli = _cli()
    unavailable_type = getattr(cli, "PodmanUnavailableError", None)
    assert isinstance(unavailable_type, type)
    unavailable_error = cast(type[Exception], unavailable_type)
    api = FakeAPI()

    @contextmanager
    def unavailable(_: str, __: str) -> Iterator[FakeAPI]:
        raise unavailable_error("Podman service unavailable")
        yield api

    runner = FakeRunner()
    exit_code, _, _, _ = _main(
        cli,
        ("status",),
        project=_project(tmp_path),
        runner=runner,
        api=api,
        connector=unavailable,
    )

    assert exit_code == 0
    assert runner.calls[0][0][:2] == ("podman", "ps")
    assert runner.calls[0][1] == {
        "env": {},
        "timeout": cli.STATUS_TIMEOUT_SECONDS,
    }


@pytest.mark.parametrize("error_type", [ContainerNotFoundError, AmbiguousContainerError])
def test_status_semantic_api_error_does_not_use_cli_fallback(
    tmp_path: Path, error_type: type[PodmanAPIError]
) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()
    api.status_error = error_type("semantic status failure")

    exit_code, _, stderr, _ = _main(
        cli,
        ("status",),
        project=_project(tmp_path),
        runner=runner,
        api=api,
    )

    assert exit_code == 1
    assert runner.calls == []
    assert stderr == b"error: semantic status failure\n"


def test_task_never_uses_status_cli_fallback(tmp_path: Path) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()

    @contextmanager
    def unavailable(_: str, __: str) -> Iterator[FakeAPI]:
        raise PodmanSocketError((Path("/missing.sock"),))
        yield api

    exit_code, _, _, _ = _main(
        cli,
        ("task", "versions"),
        project=_project(tmp_path),
        runner=runner,
        api=api,
        connector=unavailable,
    )
    assert exit_code == 1
    assert not any(call[:2] == ("podman", "ps") for call, _ in runner.calls)


def test_shell_resolves_healthy_exact_name_then_execs_interactive_podman(
    tmp_path: Path,
) -> None:
    cli = _cli()
    api = FakeAPI()
    calls: list[tuple[str, list[str], dict[str, str]]] = []

    def execvpe(file: str, arguments: Sequence[str], env: Mapping[str, str]) -> None:
        calls.append((file, list(arguments), dict(env)))

    exit_code, _, _, _ = _main(
        cli,
        ("shell",),
        project=_project(tmp_path),
        runner=FakeRunner(),
        api=api,
        env={"PATH": "/bin", "GH_TOKEN": "must-not-leak"},
        execvpe=execvpe,
    )

    assert api.wait_calls == [{"timeout": cli.HEALTH_TIMEOUT_SECONDS}]
    assert api.exec_calls == []
    assert calls == [
        (
            "podman",
            [
                "podman",
                "exec",
                "--interactive",
                "--tty",
                "project-a_dev_1",
                "/bin/bash",
            ],
            {"PATH": "/bin"},
        )
    ]
    assert exit_code == 0


def test_compose_mounts_common_as_dot_git_without_nested_mounts_or_privileges() -> None:
    compose = yaml.safe_load(Path("deployments/compose/compose.dev.yml").read_text())
    service = compose["services"]["dev"]
    common_mounts = [
        volume
        for volume in service["volumes"]
        if volume.startswith("${NUTANIX_GIT_COMMON_DIR:?required}:")
    ]
    assert common_mounts == ["${NUTANIX_GIT_COMMON_DIR:?required}:/git-metadata/.git:z"]
    assert "tmpfs" not in service
    assert service["environment"] == {
        "BEADS_DIR": "/workspace/.beads",
        "GIT_COMMON_DIR": "/git-metadata/.git",
        "GIT_DIR": "/git-metadata/.git/${NUTANIX_GIT_DIR_RELATIVE}",
        "GIT_WORK_TREE": "/workspace",
    }
    serialized = Path("deployments/compose/compose.dev.yml").read_text()
    assert "/git-common" not in serialized
    assert "podman.sock" not in serialized
    assert service.get("privileged", False) is False


def test_exec_nonzero_exit_preserves_streams_and_exit_code(tmp_path: Path) -> None:
    cli = _cli()
    api = FakeAPI()
    api.results = [ExecResult(23, b"kept stdout\n", b"kept stderr\n")]

    exit_code, stdout, stderr, _ = _main(
        cli,
        ("task", "failing"),
        project=_project(tmp_path),
        runner=FakeRunner(),
        api=api,
    )

    assert exit_code == 23
    assert stdout == b"kept stdout\n"
    assert stderr == b"kept stderr\n"


def test_ordinary_command_results_redact_all_known_tokens(tmp_path: Path) -> None:
    cli = _cli()
    runner = FakeRunner()
    runner.result = CommandResult(
        (),
        0,
        "stdout primary-sentinel secondary-sentinel\n",
        "stderr secondary-sentinel primary-sentinel\n",
    )

    exit_code, stdout, stderr, _ = _main(
        cli,
        ("down",),
        project=_project(tmp_path),
        runner=runner,
        api=FakeAPI(),
        env={
            "PATH": "/bin",
            "GH_TOKEN": "primary-sentinel",
            "GITHUB_TOKEN": "secondary-sentinel",
        },
    )

    assert exit_code == 0
    assert b"primary-sentinel" not in stdout + stderr
    assert b"secondary-sentinel" not in stdout + stderr
    assert (stdout + stderr).count(b"[REDACTED]") == 4


def test_command_errors_redact_all_known_tokens() -> None:
    cli = _cli()
    stdout = io.BytesIO()
    stderr = io.BytesIO()

    def failed_project() -> object:
        raise CommandError(
            CommandResult(
                ("git", "rev-parse"),
                19,
                "primary-sentinel\n",
                "secondary-sentinel\n",
            )
        )

    exit_code = cli.main(
        ("status",),
        project_factory=failed_project,
        env={
            "GH_TOKEN": "primary-sentinel",
            "GITHUB_TOKEN": "secondary-sentinel",
        },
        stdout=stdout,
        stderr=stderr,
    )

    assert exit_code == 19
    assert b"primary-sentinel" not in stdout.getvalue() + stderr.getvalue()
    assert b"secondary-sentinel" not in stdout.getvalue() + stderr.getvalue()
    assert stdout.getvalue() == b"[REDACTED]\n"
    assert stderr.getvalue() == b"[REDACTED]\n"


def test_status_configuration_error_does_not_fallback_or_traceback(tmp_path: Path) -> None:
    cli = _cli()
    api = FakeAPI()

    @contextmanager
    def invalid(_: str, __: str) -> Iterator[FakeAPI]:
        raise PodmanConfigurationError("Podman client configuration is invalid")
        yield api

    runner = FakeRunner()
    exit_code, stdout, stderr, _ = _main(
        cli,
        ("status",),
        project=_project(tmp_path),
        runner=runner,
        api=api,
        connector=invalid,
    )

    assert exit_code == 1
    assert stdout == b""
    assert stderr == b"error: Podman client configuration is invalid\n"
    assert runner.calls == []


def test_project_discovery_error_is_reported_without_traceback() -> None:
    cli = _cli()
    stdout = io.BytesIO()
    stderr = io.BytesIO()

    def unavailable_project() -> object:
        raise cli.ProjectError("unsafe Git layout")

    exit_code = cli.main(
        ("status",),
        project_factory=unavailable_project,
        stdout=stdout,
        stderr=stderr,
    )

    assert exit_code == 1
    assert stdout.getvalue() == b""
    assert stderr.getvalue() == b"error: unsafe Git layout\n"
