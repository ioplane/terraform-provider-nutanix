from __future__ import annotations

import copy
import io
import json
from collections.abc import Sequence
from pathlib import Path

import pytest
from scripts.checks import tracker

FIXTURE = Path("tests/fixtures/tracker/valid.json")


def load_fixture() -> dict[str, list[dict[str, object]]]:
    return json.loads(FIXTURE.read_text())


def write_projection(root: Path, state: dict[str, list[dict[str, object]]]) -> Path:
    path = root / ".beads" / "issues.jsonl"
    path.parent.mkdir(parents=True)
    records: list[str] = []
    for issue in state["issues"]:
        record = copy.deepcopy(issue)
        record.pop("parent", None)
        record["_type"] = "issue"
        records.append(json.dumps(record, sort_keys=True))
    path.write_text("\n".join(records) + "\n")
    return path


class FixtureRunner:
    def __init__(self, state: dict[str, list[dict[str, object]]]) -> None:
        self.state = state
        self.calls: list[tuple[str, ...]] = []

    def __call__(self, arguments: Sequence[str]) -> str:
        command = tuple(arguments)
        self.calls.append(command)
        responses = {
            ("bd", "--readonly", "list", "--all", "--limit", "0", "--json"): "issues",
            ("bd", "--readonly", "ready", "--limit", "0", "--json"): "ready",
            (
                "bd",
                "--readonly",
                "list",
                "--status=in_progress",
                "--limit",
                "0",
                "--json",
            ): "in_progress",
            ("bd", "--readonly", "dep", "cycles", "--json"): "cycles",
        }
        return json.dumps(self.state[responses[command]])


def validation_messages(state: dict[str, list[dict[str, object]]]) -> list[str]:
    return tracker.validate(tracker.load_state(FixtureRunner(state)))


def test_load_state_uses_readonly_json_commands() -> None:
    runner = FixtureRunner(load_fixture())

    state = tracker.load_state(runner)

    assert len(state.issues) == 19
    assert runner.calls == [
        ("bd", "--readonly", "list", "--all", "--limit", "0", "--json"),
        ("bd", "--readonly", "ready", "--limit", "0", "--json"),
        (
            "bd",
            "--readonly",
            "list",
            "--status=in_progress",
            "--limit",
            "0",
            "--json",
        ),
        ("bd", "--readonly", "dep", "cycles", "--json"),
    ]


def test_valid_graph_has_no_diagnostics() -> None:
    assert validation_messages(load_fixture()) == []


def test_requires_exactly_one_current_critical_task() -> None:
    state = load_fixture()
    duplicate = copy.deepcopy(state["issues"][-1])
    duplicate["id"] = "ntnx-extra"
    duplicate["status"] = "in_progress"
    state["issues"].append(duplicate)
    state["in_progress"].append(duplicate)

    assert "exactly one critical-path task must be in_progress" in validation_messages(state)


def test_rejects_reported_or_structural_dependency_cycles() -> None:
    reported = load_fixture()
    reported["cycles"] = [{"cycle": ["ntnx-m1", "ntnx-m0"]}]
    assert "bd reported dependency cycles" in validation_messages(reported)

    structural = load_fixture()
    structural["issues"][0]["dependencies"] = [{"depends_on_id": "ntnx-m10", "type": "blocks"}]
    assert "blocking dependency graph contains a cycle" in validation_messages(structural)


def test_requires_complete_sequential_macro_phase_set() -> None:
    state = load_fixture()
    state["issues"] = [issue for issue in state["issues"] if issue["id"] != "ntnx-m7"]

    messages = validation_messages(state)

    assert "required macro epic missing: ntnx-m7" in messages
    assert "ntnx-m8 must depend only on previous phase ntnx-m7" in messages


def test_ready_output_must_match_computed_open_unblocked_front() -> None:
    state = load_fixture()
    state["ready"] = [state["issues"][1]]

    assert "bd ready mismatch: expected ['ntnx-m0'], received ['ntnx-m1']" in validation_messages(
        state
    )


@pytest.mark.parametrize(
    ("field", "message"),
    [
        ("description", "ntnx-m0.4 missing required field: description"),
        ("acceptance_criteria", "ntnx-m0.4 missing required field: acceptance_criteria"),
        ("labels", "ntnx-m0.4 missing required field: labels"),
        ("design", "ntnx-m0.4 missing required field: design"),
    ],
)
def test_requires_issue_contract_fields(field: str, message: str) -> None:
    state = load_fixture()
    issue = next(item for item in state["issues"] if item["id"] == "ntnx-m0.4")
    issue[field] = [] if field == "labels" else ""

    assert message in validation_messages(state)


def test_requires_exact_m0_children_and_sequential_blockers() -> None:
    state = load_fixture()
    state["issues"] = [issue for issue in state["issues"] if issue["id"] != "ntnx-m0.6"]
    task = next(item for item in state["issues"] if item["id"] == "ntnx-m0.5")
    task["dependencies"] = [
        {"depends_on_id": "ntnx-m0", "type": "parent-child"},
        {"depends_on_id": "ntnx-m0.3", "type": "blocks"},
    ]

    messages = validation_messages(state)

    assert "required M0 child missing: ntnx-m0.6" in messages
    assert "ntnx-m0.5 must depend only on previous task ntnx-m0.4" in messages


def test_requires_closed_prefix_current_front_and_open_tail() -> None:
    state = load_fixture()
    future = next(item for item in state["issues"] if item["id"] == "ntnx-m0.5")
    future["status"] = "closed"

    assert (
        "M0 child statuses must be all closed for a closed M0 or a closed prefix, "
        "one in_progress task, then open tasks" in validation_messages(state)
    )


def test_allows_completed_m0_with_next_phase_task_in_progress() -> None:
    state = load_fixture()
    for issue in state["issues"]:
        if issue["id"] == "ntnx-m0" or str(issue["id"]).startswith("ntnx-m0."):
            issue["status"] = "closed"
            if str(issue["id"]).startswith("ntnx-m0."):
                issue["notes"] = "Verified evidence."
    next_task = copy.deepcopy(state["issues"][-1])
    next_task.update(
        {
            "id": "ntnx-m1.1",
            "title": "M1 kernel contract and ARC approval",
            "status": "in_progress",
            "priority": 0,
            "issue_type": "task",
            "parent": "ntnx-m1",
            "labels": ["critical-path", "m1", "task-1"],
            "dependencies": [{"depends_on_id": "ntnx-m1", "type": "parent-child"}],
        }
    )
    state["issues"].append(next_task)
    state["in_progress"] = [next_task]
    state["ready"] = [next(issue for issue in state["issues"] if issue["id"] == "ntnx-m1")]

    assert validation_messages(state) == []


def test_closed_m0_child_requires_attached_evidence() -> None:
    state = load_fixture()
    task = next(item for item in state["issues"] if item["id"] == "ntnx-m0.2")
    task["notes"] = ""

    assert "ntnx-m0.2 is closed without attached evidence notes" in validation_messages(state)


def test_rejects_invalid_json_from_bd() -> None:
    def runner(arguments: Sequence[str]) -> str:
        del arguments
        return "not-json"

    with pytest.raises(tracker.TrackerInputError, match="invalid JSON from bd"):
        tracker.load_state(runner)


def test_clean_checkout_validates_tracked_projection_without_database(tmp_path: Path) -> None:
    write_projection(tmp_path, load_fixture())

    def runner(arguments: Sequence[str]) -> str:
        raise AssertionError(f"bd must not run in clean checkout: {arguments!r}")

    stdout = io.StringIO()
    stderr = io.StringIO()

    exit_code = tracker.main(root=tmp_path, runner=runner, stdout=stdout, stderr=stderr)

    assert exit_code == 0
    assert stdout.getvalue() == "tracker: ok (19 issues, current ntnx-m0.3)\n"
    assert stderr.getvalue() == ""


def test_local_database_rejects_tracked_projection_drift(tmp_path: Path) -> None:
    projection = load_fixture()
    write_projection(tmp_path, projection)
    (tmp_path / ".beads" / "embeddeddolt" / "ntnx" / ".dolt").mkdir(parents=True)
    live = copy.deepcopy(projection)
    live["issues"][0]["title"] = "drifted live title"
    stdout = io.StringIO()
    stderr = io.StringIO()

    exit_code = tracker.main(
        root=tmp_path,
        runner=FixtureRunner(live),
        stdout=stdout,
        stderr=stderr,
    )

    assert exit_code == 1
    assert stdout.getvalue() == ""
    assert stderr.getvalue() == "tracker: tracked projection differs from live Beads state\n"


def test_main_reports_valid_issue_count(tmp_path: Path) -> None:
    state = load_fixture()
    write_projection(tmp_path, state)
    (tmp_path / ".beads" / "embeddeddolt" / "ntnx" / ".dolt").mkdir(parents=True)
    stdout = io.StringIO()
    stderr = io.StringIO()

    exit_code = tracker.main(
        root=tmp_path,
        runner=FixtureRunner(state),
        stdout=stdout,
        stderr=stderr,
    )

    assert exit_code == 0
    assert stdout.getvalue() == "tracker: ok (19 issues, current ntnx-m0.3)\n"
    assert stderr.getvalue() == ""


def test_main_fails_closed_with_deterministic_diagnostics(tmp_path: Path) -> None:
    state = load_fixture()
    write_projection(tmp_path, state)
    (tmp_path / ".beads" / "embeddeddolt" / "ntnx" / ".dolt").mkdir(parents=True)
    state["ready"] = []
    stdout = io.StringIO()
    stderr = io.StringIO()

    exit_code = tracker.main(
        root=tmp_path,
        runner=FixtureRunner(state),
        stdout=stdout,
        stderr=stderr,
    )

    assert exit_code == 1
    assert stdout.getvalue() == ""
    assert stderr.getvalue() == "tracker: bd ready mismatch: expected ['ntnx-m0'], received []\n"
