"""Bounded subprocess execution without a shell."""

from __future__ import annotations

import os
import signal
import subprocess
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path

DEFAULT_TIMEOUT_SECONDS = 30.0
DEFAULT_TERMINATE_GRACE_SECONDS = 2.0


@dataclass(frozen=True, slots=True)
class CommandResult:
    """Captured result of a completed command."""

    arguments: tuple[str, ...]
    returncode: int
    stdout: str
    stderr: str


class CommandError(RuntimeError):
    """Raised when a command exits unsuccessfully."""

    def __init__(self, result: CommandResult) -> None:
        self.result = result
        super().__init__(f"command exited with status {result.returncode}")

    @property
    def arguments(self) -> tuple[str, ...]:
        """Return the exact argument vector passed to the command."""
        return self.result.arguments

    @property
    def returncode(self) -> int:
        """Return the command exit status."""
        return self.result.returncode

    @property
    def stdout(self) -> str:
        """Return captured standard output."""
        return self.result.stdout

    @property
    def stderr(self) -> str:
        """Return captured standard error."""
        return self.result.stderr


class CommandTimeoutError(CommandError):
    """Raised after a timed-out command and its process group are terminated."""

    def __init__(self, result: CommandResult, timeout: float) -> None:
        self.timeout = timeout
        super().__init__(result)
        self.args = (f"command timed out after {timeout:g} seconds",)


def _normalize_arguments(arguments: Sequence[str | os.PathLike[str]]) -> tuple[str, ...]:
    if isinstance(arguments, (str, bytes)):
        raise TypeError("command arguments must be a sequence, not a string")
    normalized = tuple(os.fspath(argument) for argument in arguments)
    if not normalized:
        raise ValueError("command arguments must not be empty")
    return normalized


def _send_process_group_signal(process_group: int, sent_signal: signal.Signals) -> None:
    try:
        os.killpg(process_group, sent_signal)
    except ProcessLookupError:
        return


def run(
    arguments: Sequence[str | os.PathLike[str]],
    *,
    cwd: Path | None = None,
    env: Mapping[str, str] | None = None,
    timeout: float = DEFAULT_TIMEOUT_SECONDS,
    terminate_grace: float = DEFAULT_TERMINATE_GRACE_SECONDS,
) -> CommandResult:
    """Run an argument vector with captured output and a bounded lifetime."""
    if timeout <= 0:
        raise ValueError("command timeout must be positive")
    if terminate_grace <= 0:
        raise ValueError("termination grace period must be positive")

    normalized = _normalize_arguments(arguments)
    process = subprocess.Popen(
        normalized,
        cwd=cwd,
        env=dict(env) if env is not None else None,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        shell=False,
        start_new_session=True,
    )

    try:
        stdout, stderr = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        _send_process_group_signal(process.pid, signal.SIGTERM)
        try:
            stdout, stderr = process.communicate(timeout=terminate_grace)
        except subprocess.TimeoutExpired:
            _send_process_group_signal(process.pid, signal.SIGKILL)
            stdout, stderr = process.communicate()

        result = CommandResult(normalized, process.returncode, stdout, stderr)
        raise CommandTimeoutError(result, timeout) from None

    result = CommandResult(normalized, process.returncode, stdout, stderr)
    if result.returncode != 0:
        raise CommandError(result)
    return result
