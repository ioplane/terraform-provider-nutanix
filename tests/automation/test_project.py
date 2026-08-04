from __future__ import annotations

import re
from pathlib import Path

from scripts.automation.process import CommandResult
from scripts.automation.project import discover_project, project_name


def test_project_name_is_stable_and_worktree_specific(tmp_path: Path) -> None:
    common_dir = tmp_path / "repository.git"
    first_worktree = tmp_path / "worktrees" / "first"
    second_worktree = tmp_path / "worktrees" / "second"

    first = project_name(common_dir, first_worktree)

    assert first == project_name(common_dir, first_worktree)
    assert first != project_name(common_dir, second_worktree)
    assert re.fullmatch(r"nutanix-provider-[0-9a-f]{12}", first)


def test_discover_project_resolves_root_and_common_directory(tmp_path: Path) -> None:
    start = tmp_path / "nested"
    root = tmp_path / "checkout"
    common_dir = tmp_path / "repository.git"
    calls: list[tuple[str, ...]] = []

    def fake_run(arguments: tuple[str, ...], **_: object) -> CommandResult:
        calls.append(arguments)
        output = root if "--show-toplevel" in arguments else common_dir
        return CommandResult(arguments=arguments, returncode=0, stdout=f"{output}\n", stderr="")

    project = discover_project(start, runner=fake_run)

    assert project.root == root.resolve()
    assert project.git_common_dir == common_dir.resolve()
    assert project.name == project_name(common_dir, root)
    assert calls == [
        (
            "git",
            "-C",
            str(start.resolve()),
            "rev-parse",
            "--path-format=absolute",
            "--show-toplevel",
        ),
        (
            "git",
            "-C",
            str(root.resolve()),
            "rev-parse",
            "--path-format=absolute",
            "--git-common-dir",
        ),
    ]
