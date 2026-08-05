"""Host launcher for the containerized development environment."""

from __future__ import annotations

import argparse
import os
import stat
import sys
from collections.abc import Callable, Mapping, Sequence
from contextlib import AbstractContextManager
from pathlib import Path
from typing import BinaryIO, cast

from scripts.automation.podman_api import (
    ExecResult,
    PodmanAPI,
    PodmanAPIError,
    PodmanUnavailableError,
    connect,
)
from scripts.automation.process import CommandError, CommandResult, run
from scripts.automation.project import Project, ProjectError, discover_project

SERVICE = "dev"
COMPOSE_FILE = Path("deployments/compose/compose.dev.yml")
BEADS_DIR = "/workspace/.beads"
BEADS_CONFIG = Path(".beads/config.yaml")
BEADS_NOT_INITIALIZED = (
    "Beads is not initialized in this worktree; run './dev beads init --skip-agents' first"
)
COMPOSE_UP_TIMEOUT_SECONDS = 600.0
COMPOSE_DOWN_TIMEOUT_SECONDS = 120.0
HEALTH_TIMEOUT_SECONDS = 180.0
STATUS_TIMEOUT_SECONDS = 15.0
GH_AUTH_TIMEOUT_SECONDS = 15.0
GIT_METADATA_TIMEOUT_SECONDS = 15.0
TASK_COMMAND_TIMEOUT_SECONDS = 3600.0
BEADS_COMMAND_TIMEOUT_SECONDS = 300.0
REMOTE_SETUP_TIMEOUT_SECONDS = 60.0
REMOTE_BEADS_TIMEOUT_SECONDS = 600.0
REMOTE_BEADS_GIT_ENV_PREFIX = (
    "/usr/bin/env",
    "-u",
    "GIT_COMMON_DIR",
    "-u",
    "GIT_DIR",
    "-u",
    "GIT_WORK_TREE",
)
BEADS_INFORMATIONAL_ARGUMENTS = frozenset({"--help", "-h", "--version"})
REDACTED = b"[REDACTED]"


Runner = Callable[..., CommandResult]
Connector = Callable[[str, str], AbstractContextManager[PodmanAPI]]
ProjectFactory = Callable[[], Project]
Execvpe = Callable[[str, list[str], Mapping[str, str]], object]


class LauncherError(RuntimeError):
    """LauncherError reports an invalid or unavailable launcher operation."""


class CredentialError(LauncherError):
    """CredentialError reports unavailable remote Beads authentication."""


def build_parser() -> argparse.ArgumentParser:
    """Build the stable command-line interface."""
    parser = argparse.ArgumentParser(prog="dev")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("up", help="build and start the development container")
    commands.add_parser("down", help="stop the development container")
    commands.add_parser("status", help="show the development container status")
    commands.add_parser("shell", help="open an interactive development shell")
    task = commands.add_parser("task", help="run a Task target in the container")
    task.add_argument("arguments", nargs=argparse.REMAINDER)
    beads = commands.add_parser("beads", help="run bd in the container")
    beads.add_argument("arguments", nargs=argparse.REMAINDER)
    return parser


def _without_github_tokens(env: Mapping[str, str]) -> dict[str, str]:
    sanitized = dict(env)
    sanitized.pop("GH_TOKEN", None)
    sanitized.pop("GITHUB_TOKEN", None)
    return sanitized


def _write(stream: BinaryIO, value: bytes) -> None:
    if value:
        stream.write(value)


def _known_github_tokens(env: Mapping[str, str]) -> tuple[bytes, ...]:
    values: set[bytes] = set()
    for name in ("GH_TOKEN", "GITHUB_TOKEN"):
        raw = env.get(name, "")
        stripped = raw.strip()
        if stripped:
            values.add(raw.encode())
            values.add(stripped.encode())
    return tuple(sorted(values, key=len, reverse=True))


def _redact(value: bytes, secrets: Sequence[bytes]) -> bytes:
    redacted = value
    for secret in secrets:
        redacted = redacted.replace(secret, REDACTED)
    return redacted


def _enabled_boolean_flag(arguments: Sequence[str], name: str) -> bool:
    for argument in arguments:
        if argument == name:
            return True
        prefix = f"{name}="
        if argument.startswith(prefix):
            return argument.removeprefix(prefix).lower() in {"1", "t", "true"}
    return False


def _emit_command(
    result: CommandResult,
    stdout: BinaryIO,
    stderr: BinaryIO,
    secrets: Sequence[bytes],
) -> None:
    _write(stdout, _redact(result.stdout.encode(), secrets))
    _write(stderr, _redact(result.stderr.encode(), secrets))


def _emit_exec(
    result: ExecResult,
    stdout: BinaryIO,
    stderr: BinaryIO,
    *,
    secrets: Sequence[bytes] = (),
) -> None:
    _write(stdout, _redact(result.stdout, secrets))
    _write(stderr, _redact(result.stderr, secrets))


class Launcher:
    """Execute development operations through fixed, typed boundaries."""

    def __init__(
        self,
        project: Project,
        *,
        runner: Runner,
        connector: Connector,
        env: Mapping[str, str],
        stdout: BinaryIO,
        stderr: BinaryIO,
        execvpe: Execvpe,
    ) -> None:
        self.project = project
        self.runner = runner
        self.connector = connector
        self.env = env
        self.host_env = _without_github_tokens(env)
        self.secrets = _known_github_tokens(env)
        self.stdout = stdout
        self.stderr = stderr
        self.execvpe = execvpe

    def up(self) -> int:
        """Build, start, and verify the exact Compose service."""
        result = self.runner(
            (*self._compose_arguments(), "up", "--detach", "--build"),
            cwd=self.project.root,
            env=self._compose_environment(),
            timeout=COMPOSE_UP_TIMEOUT_SECONDS,
        )
        _emit_command(result, self.stdout, self.stderr, self.secrets)
        with self.connector(self.project.name, SERVICE) as api:
            api.wait_until_healthy(timeout=HEALTH_TIMEOUT_SECONDS)
        return 0

    def down(self) -> int:
        """Stop the exact Compose project without an API dependency."""
        result = self.runner(
            (*self._compose_arguments(), "down"),
            cwd=self.project.root,
            env=self._compose_environment(),
            timeout=COMPOSE_DOWN_TIMEOUT_SECONDS,
        )
        _emit_command(result, self.stdout, self.stderr, self.secrets)
        return 0

    def status(self) -> int:
        """Report status through the API, with the sole bounded CLI fallback."""
        try:
            with self.connector(self.project.name, SERVICE) as api:
                status = api.status()
                _write(self.stdout, f"{status.name} {status.state}\n".encode())
                return 0
        except PodmanUnavailableError:
            result = self.runner(
                (
                    "podman",
                    "ps",
                    "--filter",
                    f"label=io.podman.compose.project={self.project.name}",
                    "--filter",
                    f"label=io.podman.compose.service={SERVICE}",
                    "--format",
                    "{{.Names}} {{.Status}}",
                ),
                env=self.host_env,
                timeout=STATUS_TIMEOUT_SECONDS,
            )
            _emit_command(result, self.stdout, self.stderr, self.secrets)
            return 0

    def shell(self) -> int:
        """Resolve readiness via the API, then attach with Podman's CLI."""
        with self.connector(self.project.name, SERVICE) as api:
            container = api.wait_until_healthy(timeout=HEALTH_TIMEOUT_SECONDS)
            name = container.name
        arguments = [
            "podman",
            "exec",
            "--interactive",
            "--tty",
            name,
            "/bin/bash",
        ]
        self.execvpe("podman", arguments, _without_github_tokens(self.env))
        return 0

    def task(self, arguments: Sequence[str]) -> int:
        """Run an exact Task argument vector through the Podman API."""
        return self._exec(
            ("task", *self._required_arguments("task", arguments)),
            environment={},
            timeout=TASK_COMMAND_TIMEOUT_SECONDS,
        )

    def beads(self, arguments: Sequence[str]) -> int:
        """Run bd, injecting a short-lived token only for remote Beads operations."""
        normalized = self._required_arguments("beads", arguments)
        if normalized[0] != "init":
            self._require_local_beads()
        is_bootstrap = normalized[0] == "bootstrap"
        informational_bootstrap = is_bootstrap and any(
            _enabled_boolean_flag(normalized[1:], argument)
            for argument in BEADS_INFORMATIONAL_ARGUMENTS
        )
        remote_operation = (is_bootstrap and not informational_bootstrap) or tuple(
            normalized[:2]
        ) in {
            ("dolt", "push"),
            ("dolt", "pull"),
        }
        if not remote_operation:
            return self._exec(
                ("bd", *normalized),
                environment={"BEADS_DIR": BEADS_DIR},
                timeout=BEADS_COMMAND_TIMEOUT_SECONDS,
            )

        normalize_after_bootstrap = (
            is_bootstrap
            and not _enabled_boolean_flag(normalized[1:], "--dry-run")
            and self._beads_config_has_final_lf()
        )
        token = self._github_token()
        token_environment = {"GH_TOKEN": token}
        beads_environment = {"BEADS_DIR": BEADS_DIR, **token_environment}
        secrets = (*self.secrets, token.encode())
        with self.connector(self.project.name, SERVICE) as api:
            api.wait_until_healthy(timeout=HEALTH_TIMEOUT_SECONDS)
            setup = api.exec(
                ("gh", "auth", "setup-git"),
                environment=token_environment,
                timeout=REMOTE_SETUP_TIMEOUT_SECONDS,
            )
            _emit_exec(setup, self.stdout, self.stderr, secrets=secrets)
            if setup.exit_code != 0:
                return setup.exit_code
            result = api.exec(
                (*REMOTE_BEADS_GIT_ENV_PREFIX, "bd", *normalized),
                environment=beads_environment,
                timeout=REMOTE_BEADS_TIMEOUT_SECONDS,
            )
            _emit_exec(result, self.stdout, self.stderr, secrets=secrets)
            if result.exit_code != 0 or not normalize_after_bootstrap:
                return result.exit_code

            normalization = api.exec(
                ("python", "-m", "scripts.automation.beads_config"),
                environment={"BEADS_DIR": BEADS_DIR},
                timeout=BEADS_COMMAND_TIMEOUT_SECONDS,
            )
        _emit_exec(normalization, self.stdout, self.stderr, secrets=secrets)
        return normalization.exit_code

    def _beads_config_has_final_lf(self) -> bool:
        marker = self.project.root / BEADS_CONFIG
        if not hasattr(os, "O_NOFOLLOW"):
            raise LauncherError("this platform cannot inspect the Beads config safely")
        try:
            descriptor = os.open(marker, os.O_RDONLY | os.O_NOFOLLOW)
        except OSError as error:
            raise LauncherError("cannot inspect the Beads config safely") from error

        try:
            metadata = os.fstat(descriptor)
            if not stat.S_ISREG(metadata.st_mode):
                raise LauncherError(BEADS_NOT_INITIALIZED)
            if metadata.st_size == 0:
                return False
            os.lseek(descriptor, -1, os.SEEK_END)
            return os.read(descriptor, 1) == b"\n"
        except OSError as error:
            raise LauncherError("cannot inspect the Beads config safely") from error
        finally:
            os.close(descriptor)

    def _require_local_beads(self) -> None:
        beads_dir = self.project.root / BEADS_CONFIG.parent
        marker = self.project.root / BEADS_CONFIG
        try:
            is_directory = stat.S_ISDIR(beads_dir.lstat().st_mode)
        except OSError:
            is_directory = False
        if not is_directory:
            raise LauncherError(BEADS_NOT_INITIALIZED)

        try:
            is_regular_config = stat.S_ISREG(marker.lstat().st_mode)
        except OSError:
            is_regular_config = False
        if not is_regular_config:
            raise LauncherError(BEADS_NOT_INITIALIZED)

    def _exec(
        self,
        arguments: Sequence[str],
        *,
        environment: Mapping[str, str],
        timeout: float,
    ) -> int:
        with self.connector(self.project.name, SERVICE) as api:
            api.wait_until_healthy(timeout=HEALTH_TIMEOUT_SECONDS)
            result = api.exec(arguments, environment=environment, timeout=timeout)
        _emit_exec(result, self.stdout, self.stderr, secrets=self.secrets)
        return result.exit_code

    def _required_arguments(self, command: str, arguments: Sequence[str]) -> list[str]:
        normalized = list(arguments)
        if not normalized:
            raise LauncherError(f"{command} requires at least one argument")
        return normalized

    def _compose_arguments(self) -> tuple[str, ...]:
        return (
            "podman-compose",
            "-p",
            self.project.name,
            "-f",
            str(self.project.root / COMPOSE_FILE),
        )

    def _compose_environment(self) -> dict[str, str]:
        version = (self.project.root / "VERSION").read_text().strip()
        if not version:
            raise LauncherError("VERSION is empty")
        revision = self.runner(
            ("git", "-C", str(self.project.root), "rev-parse", "HEAD"),
            env=self.host_env,
            timeout=GIT_METADATA_TIMEOUT_SECONDS,
        ).stdout.strip()
        created = self.runner(
            (
                "git",
                "-C",
                str(self.project.root),
                "show",
                "-s",
                "--format=%cI",
                "HEAD",
            ),
            env=self.host_env,
            timeout=GIT_METADATA_TIMEOUT_SECONDS,
        ).stdout.strip()
        if not revision or not created:
            raise LauncherError("Git build metadata is incomplete")

        compose_env = _without_github_tokens(self.env)
        compose_env.update(
            {
                "NUTANIX_DEV_VERSION": version,
                "NUTANIX_DEV_REVISION": revision,
                "NUTANIX_DEV_CREATED": created,
                "NUTANIX_GIT_COMMON_DIR": str(self.project.git_common_dir),
                "NUTANIX_GIT_DIR_RELATIVE": str(self.project.git_dir_relative),
            }
        )
        return compose_env

    def _github_token(self) -> str:
        for name in ("GH_TOKEN", "GITHUB_TOKEN"):
            value = self.env.get(name, "").strip()
            if value:
                return value

        failed = False
        try:
            result = self.runner(
                ("gh", "auth", "token"),
                env=self.host_env,
                timeout=GH_AUTH_TIMEOUT_SECONDS,
            )
        except CommandError:
            failed = True
            result = None
        if failed or result is None:
            raise CredentialError("GitHub authentication token is unavailable")
        token = result.stdout.strip()
        if not token:
            raise CredentialError("GitHub authentication token is unavailable")
        return token


def main(
    argv: Sequence[str] | None = None,
    *,
    project_factory: ProjectFactory | None = None,
    runner: Runner = run,
    connector: Connector = connect,
    env: Mapping[str, str] | None = None,
    stdout: BinaryIO | None = None,
    stderr: BinaryIO | None = None,
    execvpe: Execvpe = os.execvpe,
) -> int:
    """Parse and run a launcher command, preserving typed failure results."""
    raw_arguments = tuple(sys.argv[1:] if argv is None else argv)
    if raw_arguments and raw_arguments[0] in {"task", "beads"}:
        arguments = argparse.Namespace(command=raw_arguments[0], arguments=list(raw_arguments[1:]))
    else:
        arguments = build_parser().parse_args(raw_arguments)
    output = stdout or cast(BinaryIO, sys.stdout.buffer)
    errors = stderr or cast(BinaryIO, sys.stderr.buffer)
    selected_env = os.environ if env is None else env
    secrets = _known_github_tokens(selected_env)
    try:
        project = (
            discover_project(runner=runner, env=selected_env)
            if project_factory is None
            else project_factory()
        )
        launcher = Launcher(
            project,
            runner=runner,
            connector=connector,
            env=selected_env,
            stdout=output,
            stderr=errors,
            execvpe=execvpe,
        )
        if arguments.command == "up":
            return launcher.up()
        if arguments.command == "down":
            return launcher.down()
        if arguments.command == "status":
            return launcher.status()
        if arguments.command == "shell":
            return launcher.shell()
        if arguments.command == "task":
            return launcher.task(arguments.arguments)
        if arguments.command == "beads":
            return launcher.beads(arguments.arguments)
        raise LauncherError(f"unknown command: {arguments.command}")
    except CommandError as error:
        _write(output, _redact(error.stdout.encode(), secrets))
        _write(errors, _redact(error.stderr.encode(), secrets))
        return error.returncode or 1
    except (LauncherError, PodmanAPIError, ProjectError, OSError) as error:
        _write(errors, f"error: {error}\n".encode())
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
