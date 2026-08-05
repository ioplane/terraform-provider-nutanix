from __future__ import annotations

import copy
import io
import json
import random
from collections.abc import Sequence
from pathlib import Path

import pytest
from scripts.generate import roadmap

FIXTURE = Path("tests/fixtures/tracker/valid.json")


def load_fixture() -> dict[str, list[dict[str, object]]]:
    return json.loads(FIXTURE.read_text())


def write_projection(root: Path, state: dict[str, list[dict[str, object]]]) -> None:
    path = root / ".beads" / "issues.jsonl"
    path.parent.mkdir(parents=True)
    records: list[str] = []
    for issue in state["issues"]:
        record = copy.deepcopy(issue)
        record.pop("parent", None)
        record["_type"] = "issue"
        records.append(json.dumps(record, sort_keys=True))
    path.write_text("\n".join(records) + "\n")


def test_roadmap_is_stable_and_sorts_phase_numbers_numerically() -> None:
    state = load_fixture()
    expected = roadmap.render(state["issues"], state["in_progress"])
    random.Random(42).shuffle(state["issues"])

    actual = roadmap.render(state["issues"], state["in_progress"])

    assert actual == expected
    assert actual.index("| M9 |") < actual.index("| M10 |")


def test_roadmap_contains_generated_warning_phase_table_and_current_task() -> None:
    state = load_fixture()

    document = roadmap.render(state["issues"], state["in_progress"])

    assert "Generated from canonical Beads/Dolt state" in document
    assert "| Phase | Outcome | Status | Blocked by |" in document
    assert "| M0 | Foundation | open | - |" in document
    assert "## Current critical task" in document
    assert "`ntnx-m0.3` — Canonical Beads graph and generated roadmap" in document
    assert document.endswith("\n")


def test_roadmap_rejects_markdown_as_authored_tracker_state() -> None:
    with pytest.raises(TypeError, match="structured issue JSON"):
        roadmap.render("# M0 is complete", [])


def test_roadmap_requires_one_current_task() -> None:
    state = load_fixture()

    with pytest.raises(ValueError, match="exactly one current critical task"):
        roadmap.render(state["issues"], [])


class FixtureRunner:
    def __init__(self, state: dict[str, list[dict[str, object]]]) -> None:
        self.state = state

    def __call__(self, arguments: Sequence[str]) -> str:
        command = tuple(arguments)
        if command[2] == "ready":
            key = "ready"
        elif command[2:4] == ("dep", "cycles"):
            key = "cycles"
        elif "--status=in_progress" in command:
            key = "in_progress"
        else:
            key = "issues"
        return json.dumps(self.state[key])


def test_generate_writes_only_valid_structured_state(tmp_path: Path) -> None:
    destination = tmp_path / "roadmap.md"
    state = load_fixture()

    roadmap.generate(destination, runner=FixtureRunner(state))

    assert destination.read_text() == roadmap.render(state["issues"], state["in_progress"])


def test_generate_does_not_replace_projection_when_tracker_is_invalid(tmp_path: Path) -> None:
    destination = tmp_path / "roadmap.md"
    destination.write_text("preserved\n")
    state = load_fixture()
    state["ready"] = []

    with pytest.raises(roadmap.RoadmapError, match="tracker state is invalid"):
        roadmap.generate(destination, runner=FixtureRunner(state))

    assert destination.read_text() == "preserved\n"


def test_check_reports_projection_drift_without_rewriting(tmp_path: Path) -> None:
    destination = tmp_path / "roadmap.md"
    destination.write_text("authored status\n")

    assert not roadmap.check(destination, runner=FixtureRunner(load_fixture()))
    assert destination.read_text() == "authored status\n"


def test_check_clean_checkout_uses_tracked_projection(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    state = load_fixture()
    write_projection(tmp_path, state)
    destination = tmp_path / "docs" / "roadmap.md"
    destination.parent.mkdir()
    destination.write_text(roadmap.render(state["issues"], state["in_progress"]))
    monkeypatch.chdir(tmp_path)

    assert roadmap.check(Path("docs/roadmap.md"))


def test_main_generates_and_checks_projection(tmp_path: Path) -> None:
    destination = tmp_path / "roadmap.md"
    stdout = io.StringIO()
    stderr = io.StringIO()
    runner = FixtureRunner(load_fixture())

    generated = roadmap.main(
        ["--output", str(destination)], runner=runner, stdout=stdout, stderr=stderr
    )
    checked = roadmap.main(
        ["--check", str(destination)], runner=runner, stdout=stdout, stderr=stderr
    )

    assert generated == 0
    assert checked == 0
    assert stdout.getvalue() == f"roadmap: wrote {destination}\nroadmap: current {destination}\n"
    assert stderr.getvalue() == ""


def test_main_fails_check_on_drift_without_rewriting(tmp_path: Path) -> None:
    destination = tmp_path / "roadmap.md"
    destination.write_text("manual state\n")
    stderr = io.StringIO()

    exit_code = roadmap.main(
        ["--check", str(destination)],
        runner=FixtureRunner(load_fixture()),
        stdout=io.StringIO(),
        stderr=stderr,
    )

    assert exit_code == 1
    assert stderr.getvalue() == f"roadmap: generated projection differs: {destination}\n"
    assert destination.read_text() == "manual state\n"
