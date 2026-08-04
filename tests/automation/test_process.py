from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import pytest
from scripts.automation import process
from scripts.automation.process import CommandError, CommandTimeoutError, run


def test_run_uses_argument_array_without_shell(monkeypatch: pytest.MonkeyPatch) -> None:
    observed: dict[str, Any] = {}

    class FakePopen:
        pid = 4242
        returncode = 0

        def __init__(self, arguments: Sequence[str], **kwargs: object) -> None:
            observed["arguments"] = arguments
            observed["kwargs"] = kwargs

        def communicate(self, timeout: float) -> tuple[str, str]:
            observed["timeout"] = timeout
            return "stdout", "stderr"

    monkeypatch.setattr(process.subprocess, "Popen", FakePopen)

    result = run(("printf", "%s", "$(touch should-not-run)"), timeout=2.0)

    assert observed["arguments"] == ("printf", "%s", "$(touch should-not-run)")
    assert observed["kwargs"]["shell"] is False
    assert observed["kwargs"]["start_new_session"] is True
    assert observed["kwargs"]["text"] is True
    assert observed["timeout"] == 2.0
    assert result.stdout == "stdout"
    assert result.stderr == "stderr"


def test_timeout_terminates_the_process_group(monkeypatch: pytest.MonkeyPatch) -> None:
    signals: list[tuple[int, signal.Signals]] = []

    class TimeoutPopen:
        pid = 7331
        returncode = -signal.SIGKILL
        attempts = 0

        def __init__(self, arguments: Sequence[str], **_: object) -> None:
            self.arguments = arguments

        def communicate(self, timeout: float | None = None) -> tuple[str, str]:
            self.attempts += 1
            if self.attempts <= 2:
                assert timeout is not None
                raise subprocess.TimeoutExpired(self.arguments, timeout)
            return "partial stdout", "partial stderr"

    def fake_killpg(process_group: int, sent_signal: signal.Signals) -> None:
        signals.append((process_group, sent_signal))

    monkeypatch.setattr(process.subprocess, "Popen", TimeoutPopen)
    monkeypatch.setattr(process.os, "killpg", fake_killpg)

    with pytest.raises(CommandTimeoutError) as raised:
        run(("long-running",), timeout=0.01, terminate_grace=0.02)

    assert signals == [(7331, signal.SIGTERM), (7331, signal.SIGKILL)]
    assert raised.value.stdout == "partial stdout"
    assert raised.value.stderr == "partial stderr"
    assert raised.value.timeout == 0.01


def test_nonzero_exit_preserves_stdout_and_stderr(tmp_path: Path) -> None:
    program = "import sys; print('kept stdout'); print('kept stderr', file=sys.stderr); sys.exit(7)"

    with pytest.raises(CommandError) as raised:
        run((sys.executable, "-c", program), cwd=tmp_path)

    assert raised.value.returncode == 7
    assert raised.value.stdout == "kept stdout\n"
    assert raised.value.stderr == "kept stderr\n"
    assert raised.value.arguments == (sys.executable, "-c", program)


def test_keyboard_interrupt_terminates_real_child_process_group(tmp_path: Path) -> None:
    child_pid_path = tmp_path / "child.pid"
    child_program = (
        "import os, pathlib, time; "
        f"pathlib.Path({str(child_pid_path)!r}).write_text(str(os.getpid())); "
        "time.sleep(30)"
    )
    controller_program = (
        "import sys; "
        "from scripts.automation.process import run; "
        f"run((sys.executable, '-c', {child_program!r}), timeout=30, terminate_grace=0.1)"
    )
    controller = subprocess.Popen((sys.executable, "-c", controller_program), cwd="/workspace")

    deadline = time.monotonic() + 2.0
    while not child_pid_path.exists() and time.monotonic() < deadline:
        time.sleep(0.01)
    assert child_pid_path.exists()
    child_pid = int(child_pid_path.read_text())

    try:
        controller.send_signal(signal.SIGINT)
        assert controller.wait(timeout=2.0) != 0

        deadline = time.monotonic() + 1.0
        while _process_is_running(child_pid) and time.monotonic() < deadline:
            time.sleep(0.01)
        assert not _process_is_running(child_pid)
    finally:
        if controller.poll() is None:
            controller.kill()
            controller.wait(timeout=1.0)
        if _process_is_running(child_pid):
            os.killpg(child_pid, signal.SIGKILL)


def test_sigkill_drain_is_bounded_when_escaped_descendant_holds_pipes() -> None:
    escaped_program = "import time; time.sleep(1.2)"
    parent_program = (
        "import subprocess, sys, time; "
        f"subprocess.Popen((sys.executable, '-c', {escaped_program!r}), start_new_session=True); "
        "print('ready', flush=True); "
        "time.sleep(30)"
    )

    started = time.monotonic()
    with pytest.raises(CommandTimeoutError):
        run(
            (sys.executable, "-c", parent_program),
            timeout=0.05,
            terminate_grace=0.05,
        )
    elapsed = time.monotonic() - started

    assert elapsed < 0.5


def _process_is_running(process_id: int) -> bool:
    status_path = Path(f"/proc/{process_id}/stat")
    try:
        state = status_path.read_text().split()[2]
    except FileNotFoundError:
        return False
    return state != "Z"
