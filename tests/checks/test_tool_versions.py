from __future__ import annotations

import io
from collections.abc import Sequence

import pytest
from scripts.checks import tool_versions


def valid_output() -> str:
    return "".join(
        f"{name}={version}\n" for name, version in tool_versions.EXPECTED_VERSIONS.items()
    )


def test_accepts_exact_machine_readable_versions() -> None:
    assert tool_versions.validate_output(valid_output()) == []


@pytest.mark.parametrize(
    ("payload", "message"),
    [
        ("go version go1.26.5 linux/amd64\n", "malformed versions line 1"),
        ("go=1.26.5\ngo=1.26.5\n", "duplicate versions key: go"),
        ("go=next\n", "invalid semantic version for go: next"),
    ],
)
def test_rejects_malformed_duplicate_or_non_semver_output(payload: str, message: str) -> None:
    assert message in tool_versions.validate_output(payload)


def test_rejects_missing_unexpected_and_drifted_versions() -> None:
    output = valid_output().replace("terraform=1.15.8\n", "terraform=0.0.0\n")
    output = output.replace("gh=2.97.0\n", "unexpected=1.0.0\n")

    diagnostics = tool_versions.validate_output(output)

    assert "tool version differs: terraform" in diagnostics
    assert "missing tool version: gh" in diagnostics
    assert "unexpected tool version: unexpected" in diagnostics


def test_main_runs_task_versions_and_reports_count() -> None:
    stdout = io.StringIO()
    stderr = io.StringIO()
    calls: list[tuple[str, ...]] = []

    def runner(arguments: Sequence[str]) -> str:
        calls.append(tuple(arguments))
        return valid_output()

    assert tool_versions.main(runner=runner, stdout=stdout, stderr=stderr) == 0
    assert calls == [("task", "versions")]
    assert stdout.getvalue() == f"tools: ok ({len(tool_versions.EXPECTED_VERSIONS)} versions)\n"
    assert stderr.getvalue() == ""
