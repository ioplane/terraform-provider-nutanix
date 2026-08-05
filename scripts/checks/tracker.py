"""Validate canonical Beads tracker state from structured JSON output."""

from __future__ import annotations

import json
import sys
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import TextIO, cast

from scripts.automation.process import CommandError, run

DESIGN_PATH = "docs/superpowers/specs/2026-08-04-foundation-design.md"
PLAN_PATH = "docs/superpowers/plans/2026-08-04-foundation.md"
REQUIRED_EPIC_IDS = tuple(f"ntnx-m{index}" for index in range(11))
REQUIRED_M0_CHILD_IDS = tuple(f"ntnx-m0.{index}" for index in range(1, 9))
PROJECTION_PATH = Path(".beads/issues.jsonl")
REQUIRED_FIELDS = (
    "id",
    "title",
    "description",
    "design",
    "acceptance_criteria",
    "status",
    "priority",
    "issue_type",
    "labels",
)

Issue = dict[str, object]
Runner = Callable[[Sequence[str]], str]


class TrackerInputError(RuntimeError):
    """TrackerInputError reports malformed structured output from Beads."""


@dataclass(frozen=True, slots=True)
class TrackerState:
    """Structured snapshots returned by the read-only Beads commands."""

    issues: tuple[Issue, ...]
    ready: tuple[Issue, ...]
    in_progress: tuple[Issue, ...]
    cycles: tuple[Issue, ...]


def _run_bd(arguments: Sequence[str]) -> str:
    try:
        return run(arguments, timeout=30.0).stdout
    except (CommandError, OSError) as error:
        raise TrackerInputError("cannot query local Beads database") from error


def _parse_array(payload: str, source: str) -> tuple[Issue, ...]:
    try:
        value = json.loads(payload)
    except json.JSONDecodeError as error:
        raise TrackerInputError(f"invalid JSON from bd {source}: {error.msg}") from error
    if not isinstance(value, list) or not all(isinstance(item, dict) for item in value):
        raise TrackerInputError(f"bd {source} must return a JSON array of objects")
    return tuple(cast(Issue, item) for item in value)


def load_state(runner: Runner = _run_bd) -> TrackerState:
    """Load the complete tracker views through injected read-only commands."""
    commands = (
        ("issues", ("bd", "--readonly", "list", "--all", "--limit", "0", "--json")),
        ("ready", ("bd", "--readonly", "ready", "--limit", "0", "--json")),
        (
            "in_progress",
            (
                "bd",
                "--readonly",
                "list",
                "--status=in_progress",
                "--limit",
                "0",
                "--json",
            ),
        ),
        ("cycles", ("bd", "--readonly", "dep", "cycles", "--json")),
    )
    snapshots = {name: _parse_array(runner(arguments), name) for name, arguments in commands}
    return TrackerState(
        issues=snapshots["issues"],
        ready=snapshots["ready"],
        in_progress=snapshots["in_progress"],
        cycles=snapshots["cycles"],
    )


def _projection_issue(value: object, line_number: int) -> Issue:
    if not isinstance(value, dict) or value.get("_type") != "issue":
        raise TrackerInputError(f"invalid tracked projection record at line {line_number}")
    issue = cast(Issue, {key: item for key, item in value.items() if key != "_type"})
    # `bd export` embeds comment bodies, while `bd list --json` exposes only
    # `comment_count`. Compare the common issue projection and retain the
    # count so additions or removals still produce live-state drift.
    issue.pop("comments", None)
    dependencies = issue.get("dependencies", [])
    parents: list[str] = []
    if isinstance(dependencies, list):
        for dependency in dependencies:
            if not isinstance(dependency, Mapping) or dependency.get("type") != "parent-child":
                continue
            parent = dependency.get("depends_on_id")
            if isinstance(parent, str):
                parents.append(parent)
    if len(parents) > 1:
        raise TrackerInputError(f"multiple projection parents at line {line_number}")
    if parents:
        existing = issue.get("parent")
        if existing not in (None, parents[0]):
            raise TrackerInputError(f"projection parent differs at line {line_number}")
        issue["parent"] = parents[0]
    return issue


def load_projection(path: Path) -> TrackerState:
    """Load and derive read-only tracker views from the committed Beads export."""
    try:
        payload = path.read_text()
    except OSError as error:
        raise TrackerInputError("tracked Beads projection is unavailable") from error
    issues: list[Issue] = []
    for line_number, line in enumerate(payload.splitlines(), start=1):
        if not line.strip():
            raise TrackerInputError(f"blank tracked projection record at line {line_number}")
        try:
            value = json.loads(line)
        except json.JSONDecodeError as error:
            raise TrackerInputError(
                f"invalid tracked projection JSON at line {line_number}: {error.msg}"
            ) from error
        issues.append(_projection_issue(value, line_number))
    if not issues:
        raise TrackerInputError("tracked Beads projection is empty")

    by_id = {_text(issue, "id"): issue for issue in issues if _text(issue, "id")}
    ready = tuple(by_id[issue_id] for issue_id in _expected_ready(by_id))
    in_progress = tuple(issue for issue in issues if _text(issue, "status") == "in_progress")
    return TrackerState(
        issues=tuple(issues),
        ready=ready,
        in_progress=in_progress,
        cycles=(),
    )


def _database_present(root: Path) -> bool:
    embedded = root / ".beads" / "embeddeddolt"
    return embedded.is_dir() and any(path.is_dir() for path in embedded.glob("*/.dolt"))


def _canonical_issues(issues: Sequence[Issue]) -> tuple[str, ...]:
    return tuple(
        sorted(json.dumps(issue, sort_keys=True, separators=(",", ":")) for issue in issues)
    )


def load_repository_state(root: Path, runner: Runner = _run_bd) -> TrackerState:
    """Use the live database when present, otherwise the committed clean-clone projection."""
    projection = load_projection(root / PROJECTION_PATH)
    if not _database_present(root):
        return projection
    live = load_state(runner)
    if _canonical_issues(live.issues) != _canonical_issues(projection.issues):
        raise TrackerInputError("tracked projection differs from live Beads state")
    return live


def _text(issue: Mapping[str, object], field: str) -> str:
    value = issue.get(field)
    return value if isinstance(value, str) else ""


def _labels(issue: Mapping[str, object]) -> set[str]:
    value = issue.get("labels")
    if not isinstance(value, list):
        return set()
    return {label for label in value if isinstance(label, str)}


def _dependencies(issue: Mapping[str, object], dependency_type: str) -> set[str]:
    value = issue.get("dependencies", [])
    if not isinstance(value, list):
        return set()
    dependencies: set[str] = set()
    for dependency in value:
        if not isinstance(dependency, dict) or dependency.get("type") != dependency_type:
            continue
        dependency_id = dependency.get("depends_on_id")
        if isinstance(dependency_id, str):
            dependencies.add(dependency_id)
    return dependencies


def _blocking_dependencies(issue: Mapping[str, object]) -> set[str]:
    return _dependencies(issue, "blocks")


def _has_blocking_cycle(issues: Mapping[str, Issue]) -> bool:
    visiting: set[str] = set()
    visited: set[str] = set()

    def visit(issue_id: str) -> bool:
        if issue_id in visiting:
            return True
        if issue_id in visited:
            return False
        visiting.add(issue_id)
        for dependency_id in _blocking_dependencies(issues[issue_id]):
            if dependency_id in issues and visit(dependency_id):
                return True
        visiting.remove(issue_id)
        visited.add(issue_id)
        return False

    return any(visit(issue_id) for issue_id in issues if issue_id not in visited)


def _expected_ready(issues: Mapping[str, Issue]) -> list[str]:
    ready: list[str] = []
    for issue_id, issue in issues.items():
        if _text(issue, "status") != "open":
            continue
        blockers = _blocking_dependencies(issue)
        if all(
            blocker in issues and _text(issues[blocker], "status") == "closed"
            for blocker in blockers
        ):
            ready.append(issue_id)
    return sorted(ready)


def _validate_required_fields(issues: Sequence[Issue]) -> list[str]:
    diagnostics: list[str] = []
    for issue in issues:
        issue_id = _text(issue, "id") or "<unknown>"
        for field in REQUIRED_FIELDS:
            value = issue.get(field)
            if value is None or value == "" or value == []:
                diagnostics.append(f"{issue_id} missing required field: {field}")
        combined_references = f"{_text(issue, 'description')} {_text(issue, 'design')}"
        for reference in (DESIGN_PATH, PLAN_PATH):
            if reference not in combined_references:
                diagnostics.append(f"{issue_id} missing required reference: {reference}")
    return diagnostics


def _validate_epics(issues: Mapping[str, Issue]) -> list[str]:
    diagnostics: list[str] = []
    for index, issue_id in enumerate(REQUIRED_EPIC_IDS):
        issue = issues.get(issue_id)
        if issue is None:
            diagnostics.append(f"required macro epic missing: {issue_id}")
            continue
        if _text(issue, "issue_type") != "epic":
            diagnostics.append(f"{issue_id} must have issue_type epic")
        expected = set() if index == 0 else {REQUIRED_EPIC_IDS[index - 1]}
        predecessor_missing = index > 0 and REQUIRED_EPIC_IDS[index - 1] not in issues
        if _blocking_dependencies(issue) != expected or predecessor_missing:
            if index == 0:
                diagnostics.append("ntnx-m0 must not have a phase blocker")
            else:
                diagnostics.append(
                    f"{issue_id} must depend only on previous phase {REQUIRED_EPIC_IDS[index - 1]}"
                )
    return diagnostics


def _validate_m0_children(issues: Mapping[str, Issue]) -> list[str]:
    diagnostics: list[str] = []
    statuses: list[str] = []
    for index, issue_id in enumerate(REQUIRED_M0_CHILD_IDS):
        issue = issues.get(issue_id)
        if issue is None:
            diagnostics.append(f"required M0 child missing: {issue_id}")
            continue
        statuses.append(_text(issue, "status"))
        if issue.get("parent") != "ntnx-m0":
            diagnostics.append(f"{issue_id} must have parent ntnx-m0")
        if _text(issue, "status") == "closed" and not _text(issue, "notes").strip():
            diagnostics.append(f"{issue_id} is closed without attached evidence notes")
        expected = set() if index == 0 else {REQUIRED_M0_CHILD_IDS[index - 1]}
        if _blocking_dependencies(issue) != expected:
            if index == 0:
                diagnostics.append("ntnx-m0.1 must not have a task blocker")
            else:
                diagnostics.append(
                    f"{issue_id} must depend only on previous task {REQUIRED_M0_CHILD_IDS[index - 1]}"
                )

    if len(statuses) == len(REQUIRED_M0_CHILD_IDS):
        m0_closed = _text(issues.get("ntnx-m0", {}), "status") == "closed"
        valid = m0_closed and all(status == "closed" for status in statuses)
        if not valid and not m0_closed:
            in_progress = [
                index for index, status in enumerate(statuses) if status == "in_progress"
            ]
            valid = len(in_progress) == 1
            if valid:
                current = in_progress[0]
                valid = all(status == "closed" for status in statuses[:current]) and all(
                    status in {"open", "blocked"} for status in statuses[current + 1 :]
                )
        if not valid:
            diagnostics.append(
                "M0 child statuses must be all closed for a closed M0 or a closed prefix, "
                "one in_progress task, then open tasks"
            )
    return diagnostics


def validate(state: TrackerState) -> list[str]:
    """Return deterministic diagnostics for every tracker contract violation."""
    diagnostics = _validate_required_fields(state.issues)
    by_id: dict[str, Issue] = {}
    for issue in state.issues:
        issue_id = _text(issue, "id")
        if not issue_id:
            continue
        if issue_id in by_id:
            diagnostics.append(f"duplicate issue id: {issue_id}")
        by_id[issue_id] = issue

    diagnostics.extend(_validate_epics(by_id))
    diagnostics.extend(_validate_m0_children(by_id))

    issue_in_progress = sorted(
        issue_id for issue_id, issue in by_id.items() if _text(issue, "status") == "in_progress"
    )
    reported_in_progress = sorted(_text(issue, "id") for issue in state.in_progress)
    if reported_in_progress != issue_in_progress:
        diagnostics.append(
            "bd in_progress mismatch: "
            f"expected {issue_in_progress!r}, received {reported_in_progress!r}"
        )

    current = [
        issue
        for issue in state.in_progress
        if _text(issue, "issue_type") == "task" and "critical-path" in _labels(issue)
    ]
    if len(current) != 1 or len(state.in_progress) != 1:
        diagnostics.append("exactly one critical-path task must be in_progress")

    if state.cycles:
        diagnostics.append("bd reported dependency cycles")
    if _has_blocking_cycle(by_id):
        diagnostics.append("blocking dependency graph contains a cycle")

    expected_ready = _expected_ready(by_id)
    reported_ready = sorted(_text(issue, "id") for issue in state.ready)
    if reported_ready != expected_ready:
        diagnostics.append(
            f"bd ready mismatch: expected {expected_ready!r}, received {reported_ready!r}"
        )

    return sorted(set(diagnostics))


def main(
    *,
    root: Path | None = None,
    runner: Runner = _run_bd,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Run the tracker contract gate and return a process exit code."""
    selected_root = Path.cwd() if root is None else root
    try:
        state = load_repository_state(selected_root, runner)
    except TrackerInputError as error:
        print(f"tracker: {error}", file=stderr)
        return 1
    diagnostics = validate(state)
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"tracker: {diagnostic}", file=stderr)
        return 1
    current_id = _text(state.in_progress[0], "id")
    print(f"tracker: ok ({len(state.issues)} issues, current {current_id})", file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
