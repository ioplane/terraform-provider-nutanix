from __future__ import annotations

import importlib
import importlib.util
import io
import stat
from collections.abc import Iterator, Mapping, Sequence
from contextlib import contextmanager
from pathlib import Path
from types import ModuleType, SimpleNamespace
from typing import Any, cast

import pytest
import yaml
from scripts.automation.podman_api import ExecResult, PodmanSocketError
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
    host_env = {"PATH": "/bin", "GH_TOKEN": "must-not-reach-compose"}

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

    exit_code, stdout, stderr, _ = _main(
        cli,
        (command, *arguments),
        project=_project(tmp_path),
        runner=runner,
        api=api,
        env={"GH_TOKEN": "host-secret"},
    )

    assert api.wait_calls == [{"timeout": cli.HEALTH_TIMEOUT_SECONDS}]
    assert api.exec_calls == [(expected, {"environment": {}})]
    assert exit_code == 0
    assert stdout == b"exec stdout\n"
    assert stderr == b"exec stderr\n"


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
    assert api.exec_calls == [
        (("gh", "auth", "setup-git"), {"environment": injected}),
        (("bd", "dolt", "push", "--force-with-lease"), {"environment": injected}),
    ]
    assert exit_code == 7
    assert secret not in stdout + stderr
    assert b"[REDACTED]" in stdout + stderr
    assert all(expected_token not in argument for call, _ in api.exec_calls for argument in call)
    assert sorted(project.root.iterdir()) == before
    assert ("gh", "auth", "token") not in [call for call, _ in runner.calls]


def test_remote_beads_falls_back_to_bounded_host_gh_token(tmp_path: Path) -> None:
    cli = _cli()
    runner = FakeRunner()
    api = FakeAPI()

    exit_code, _, _, _ = _main(
        cli,
        ("beads", "dolt", "pull"),
        project=_project(tmp_path),
        runner=runner,
        api=api,
    )

    gh_call = next(call for call in runner.calls if call[0] == ("gh", "auth", "token"))
    assert gh_call[1]["timeout"] == cli.GH_AUTH_TIMEOUT_SECONDS
    assert api.exec_calls[0][1] == {"environment": {"GH_TOKEN": "token-from-gh"}}
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
    exit_code, stdout, stderr, _ = _main(
        cli,
        ("beads", "dolt", "pull"),
        project=_project(tmp_path),
        runner=FailingGHRunner(),
        api=api,
    )

    assert exit_code == 1
    assert b"host-secret" not in stdout + stderr
    assert b"authentication token is unavailable" in stderr
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
            {"timeout": cli.STATUS_TIMEOUT_SECONDS},
        )
    ]


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


def test_compose_mounts_only_selected_git_metadata_without_podman_socket() -> None:
    compose = yaml.safe_load(Path("deployments/compose/compose.dev.yml").read_text())
    service = compose["services"]["dev"]
    assert "${NUTANIX_GIT_COMMON_DIR:?required}:/git-common:z" in service["volumes"]
    assert service["environment"] == {
        "GIT_COMMON_DIR": "/git-common",
        "GIT_DIR": "/git-common/${NUTANIX_GIT_DIR_RELATIVE}",
        "GIT_WORK_TREE": "/workspace",
    }
    serialized = Path("deployments/compose/compose.dev.yml").read_text()
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
