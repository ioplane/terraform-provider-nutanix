"""Validate machine-readable output from the pinned development toolbox."""

from __future__ import annotations

import re
import sys
from collections.abc import Callable, Sequence
from typing import TextIO

from scripts.automation.process import CommandError, run

EXPECTED_VERSIONS = {
    "go": "1.26.5",
    "terraform": "1.15.8",
    "task": "3.52.0",
    "beads": "1.1.2",
    "podman-py": "5.8.0",
    "podman-compose": "1.6.0",
    "uv": "0.12.1",
    "ruff": "0.16.1",
    "rumdl": "0.2.50",
    "ty": "0.0.66",
    "yamllint": "1.38.0",
    "golangci-lint": "2.12.2",
    "goreleaser": "2.17.1",
    "syft": "1.50.0",
    "tfplugindocs": "0.25.0",
    "govulncheck": "1.6.0",
    "gopls": "0.23.0",
    "gh": "2.97.0",
    "hadolint": "2.15.1",
}

Runner = Callable[[Sequence[str]], str]
_VERSION = re.compile(r"^\d+\.\d+\.\d+$")


class ToolVersionError(RuntimeError):
    """ToolVersionError reports an unavailable versions command."""


def validate_output(payload: str) -> list[str]:
    """Return deterministic diagnostics for a versions payload."""
    diagnostics: list[str] = []
    observed: dict[str, str] = {}
    for line_number, line in enumerate(payload.splitlines(), start=1):
        if line.count("=") != 1:
            diagnostics.append(f"malformed versions line {line_number}")
            continue
        name, version = line.split("=", 1)
        if not name or not version:
            diagnostics.append(f"malformed versions line {line_number}")
            continue
        if name in observed:
            diagnostics.append(f"duplicate versions key: {name}")
            continue
        observed[name] = version
        if not _VERSION.fullmatch(version):
            diagnostics.append(f"invalid semantic version for {name}: {version}")

    for name, expected in EXPECTED_VERSIONS.items():
        if name not in observed:
            diagnostics.append(f"missing tool version: {name}")
        elif observed[name] != expected:
            diagnostics.append(f"tool version differs: {name}")
    for name in observed.keys() - EXPECTED_VERSIONS.keys():
        diagnostics.append(f"unexpected tool version: {name}")
    return sorted(set(diagnostics))


def _run_versions(arguments: Sequence[str]) -> str:
    try:
        return run(arguments, timeout=120.0).stdout
    except (CommandError, OSError) as error:
        raise ToolVersionError("cannot execute task versions") from error


def main(
    *,
    runner: Runner = _run_versions,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Run the toolbox version gate."""
    try:
        payload = runner(("task", "versions"))
    except ToolVersionError as error:
        print(f"tools: {error}", file=stderr)
        return 1
    diagnostics = validate_output(payload)
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"tools: {diagnostic}", file=stderr)
        return 1
    print(f"tools: ok ({len(EXPECTED_VERSIONS)} versions)", file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
