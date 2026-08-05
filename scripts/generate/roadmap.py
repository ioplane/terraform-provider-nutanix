"""Render the human roadmap projection from structured Beads JSON."""

from __future__ import annotations

import argparse
import re
import sys
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import TextIO, cast

from scripts.checks import tracker

PHASE_ID = re.compile(r"^ntnx-m(\d+)$")


class RoadmapError(RuntimeError):
    """RoadmapError reports invalid canonical input or projection drift."""


def _require_issue_sequence(value: object) -> Sequence[Mapping[str, object]]:
    if isinstance(value, (str, bytes)) or not isinstance(value, Sequence):
        raise TypeError("roadmap input must be structured issue JSON")
    if not all(isinstance(item, Mapping) for item in value):
        raise TypeError("roadmap input must be structured issue JSON")
    return cast(Sequence[Mapping[str, object]], value)


def _text(issue: Mapping[str, object], field: str) -> str:
    value = issue.get(field)
    return value if isinstance(value, str) else ""


def _labels(issue: Mapping[str, object]) -> set[str]:
    value = issue.get("labels", [])
    if not isinstance(value, list):
        return set()
    return {label for label in value if isinstance(label, str)}


def _phase_number(issue: Mapping[str, object]) -> int | None:
    match = PHASE_ID.fullmatch(_text(issue, "id"))
    if match is None or _text(issue, "issue_type") != "epic":
        return None
    return int(match.group(1))


def _blocking_dependencies(issue: Mapping[str, object]) -> list[str]:
    value = issue.get("dependencies", [])
    if not isinstance(value, list):
        return []
    dependencies: list[str] = []
    for dependency in value:
        if not isinstance(dependency, Mapping) or dependency.get("type") != "blocks":
            continue
        dependency_id = dependency.get("depends_on_id")
        if isinstance(dependency_id, str):
            dependencies.append(dependency_id)
    return sorted(dependencies)


def _cell(value: str) -> str:
    return value.replace("|", "\\|").replace("\n", " ")


def _outcome(issue: Mapping[str, object], number: int) -> str:
    title = _text(issue, "title")
    prefix = f"M{number} "
    return title.removeprefix(prefix)


def render(issues: object, in_progress: object) -> str:
    """Render a deterministic Markdown projection without reading Markdown state."""
    issue_sequence = _require_issue_sequence(issues)
    current_sequence = _require_issue_sequence(in_progress)
    current = [
        issue
        for issue in current_sequence
        if _text(issue, "issue_type") == "task" and "critical-path" in _labels(issue)
    ]
    if len(current) != 1 or len(current_sequence) != 1:
        raise ValueError("roadmap requires exactly one current critical task")

    phases = sorted(
        (
            (number, issue)
            for issue in issue_sequence
            if (number := _phase_number(issue)) is not None
        ),
        key=lambda item: item[0],
    )
    lines = [
        "# Delivery roadmap",
        "",
        "> Generated from canonical Beads/Dolt state. Do not edit task status here.",
        "",
        "| Phase | Outcome | Status | Blocked by |",
        "| --- | --- | --- | --- |",
    ]
    for number, issue in phases:
        blockers = _blocking_dependencies(issue)
        blocked_by = ", ".join(f"`{dependency}`" for dependency in blockers) or "-"
        lines.append(
            "| "
            f"M{number} | {_cell(_outcome(issue, number))} | "
            f"{_cell(_text(issue, 'status'))} | {blocked_by} |"
        )

    current_issue = current[0]
    lines.extend(
        [
            "",
            "## Current critical task",
            "",
            f"`{_text(current_issue, 'id')}` — {_text(current_issue, 'title')}",
            "",
        ]
    )
    return "\n".join(lines)


def _document(runner: tracker.Runner | None) -> str:
    state = tracker.load_state() if runner is None else tracker.load_state(runner)
    diagnostics = tracker.validate(state)
    if diagnostics:
        raise RoadmapError("tracker state is invalid: " + "; ".join(diagnostics))
    return render(state.issues, state.in_progress)


def generate(destination: Path, *, runner: tracker.Runner | None = None) -> None:
    """Write the projection only after the structured tracker state validates."""
    document = _document(runner)
    destination.write_text(document)


def check(destination: Path, *, runner: tracker.Runner | None = None) -> bool:
    """Return whether an existing roadmap equals the canonical projection."""
    document = _document(runner)
    try:
        existing = destination.read_text()
    except FileNotFoundError:
        return False
    return existing == document


def main(
    arguments: Sequence[str] | None = None,
    *,
    runner: tracker.Runner | None = None,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Generate or check the repository roadmap projection."""
    parser = argparse.ArgumentParser(prog="roadmap")
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--output", type=Path)
    mode.add_argument("--check", type=Path)
    parsed = parser.parse_args(arguments)
    try:
        if parsed.output is not None:
            generate(parsed.output, runner=runner)
            print(f"roadmap: wrote {parsed.output}", file=stdout)
            return 0
        if check(parsed.check, runner=runner):
            print(f"roadmap: current {parsed.check}", file=stdout)
            return 0
        print(f"roadmap: generated projection differs: {parsed.check}", file=stderr)
        return 1
    except (RoadmapError, tracker.TrackerInputError, OSError) as error:
        print(f"roadmap: {error}", file=stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
