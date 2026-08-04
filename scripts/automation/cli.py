"""Host launcher for the containerized development environment."""

from __future__ import annotations

import argparse
import os
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
COMPOSE_UP_TIMEOUT_SECONDS = 600.0
COMPOSE_DOWN_TIMEOUT_SECONDS = 120.0
HEALTH_TIMEOUT_SECONDS = 180.0
STATUS_TIMEOUT_SECONDS = 15.0
GH_AUTH_TIMEOUT_SECONDS = 15.0
GIT_METADATA_TIMEOUT_SECONDS = 15.0
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


def _emit_command(result: CommandResult, stdout: BinaryIO, stderr: BinaryIO) -> None:
    _write(stdout, result.stdout.encode())
    _write(stderr, result.stderr.encode())


def _emit_exec(
    result: ExecResult,
    stdout: BinaryIO,
    stderr: BinaryIO,
    *,
    token: str | None = None,
) -> None:
    output = result.stdout
    errors = result.stderr
    if token is not None:
        secret = token.encode()
        output = output.replace(secret, REDACTED)
        errors = errors.replace(secret, REDACTED)
    _write(stdout, output)
    _write(stderr, errors)


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
        _emit_command(result, self.stdout, self.stderr)
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
        _emit_command(result, self.stdout, self.stderr)
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
                timeout=STATUS_TIMEOUT_SECONDS,
            )
            _emit_command(result, self.stdout, self.stderr)
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
        return self._exec(("task", *self._required_arguments("task", arguments)))

    def beads(self, arguments: Sequence[str]) -> int:
        """Run bd, injecting a short-lived token only for remote Dolt operations."""
        normalized = self._required_arguments("beads", arguments)
        if tuple(normalized[:2]) not in {("dolt", "push"), ("dolt", "pull")}:
            return self._exec(("bd", *normalized))

        token = self._github_token()
        environment = {"GH_TOKEN": token}
        with self.connector(self.project.name, SERVICE) as api:
            api.wait_until_healthy(timeout=HEALTH_TIMEOUT_SECONDS)
            setup = api.exec(("gh", "auth", "setup-git"), environment=environment)
            _emit_exec(setup, self.stdout, self.stderr, token=token)
            if setup.exit_code != 0:
                return setup.exit_code
            result = api.exec(("bd", *normalized), environment=environment)
        _emit_exec(result, self.stdout, self.stderr, token=token)
        return result.exit_code

    def _exec(self, arguments: Sequence[str]) -> int:
        with self.connector(self.project.name, SERVICE) as api:
            api.wait_until_healthy(timeout=HEALTH_TIMEOUT_SECONDS)
            result = api.exec(arguments, environment={})
        _emit_exec(result, self.stdout, self.stderr)
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
            result = self.runner(("gh", "auth", "token"), timeout=GH_AUTH_TIMEOUT_SECONDS)
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
    project_factory: ProjectFactory = discover_project,
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
    try:
        launcher = Launcher(
            project_factory(),
            runner=runner,
            connector=connector,
            env=os.environ if env is None else env,
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
        _write(output, error.stdout.encode())
        _write(errors, error.stderr.encode())
        return error.returncode or 1
    except (LauncherError, PodmanAPIError, ProjectError, OSError) as error:
        _write(errors, f"error: {error}\n".encode())
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
