from __future__ import annotations

import re
from pathlib import Path

import pytest
from scripts.automation.process import CommandResult
from scripts.automation.project import ProjectError, discover_project, project_name


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
    git_dir = common_dir / "worktrees" / "feature"
    calls: list[tuple[tuple[str, ...], dict[str, object]]] = []

    def fake_run(arguments: tuple[str, ...], **kwargs: object) -> CommandResult:
        calls.append((arguments, kwargs))
        if "--show-toplevel" in arguments:
            output = root
        elif "--git-common-dir" in arguments:
            output = common_dir
        else:
            output = git_dir
        return CommandResult(arguments=arguments, returncode=0, stdout=f"{output}\n", stderr="")

    project = discover_project(
        start,
        runner=fake_run,
        env={"PATH": "/bin", "GH_TOKEN": "primary", "GITHUB_TOKEN": "secondary"},
    )

    assert project.root == root.resolve()
    assert project.git_common_dir == common_dir.resolve()
    assert project.git_dir == git_dir.resolve()
    assert project.git_dir_relative == Path("worktrees/feature")
    assert project.name == project_name(common_dir, root)
    assert [arguments for arguments, _ in calls] == [
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
        (
            "git",
            "-C",
            str(root.resolve()),
            "rev-parse",
            "--path-format=absolute",
            "--git-dir",
        ),
    ]
    assert [kwargs for _, kwargs in calls] == [
        {"env": {"PATH": "/bin"}, "timeout": 15.0},
        {"env": {"PATH": "/bin"}, "timeout": 15.0},
        {"env": {"PATH": "/bin"}, "timeout": 15.0},
    ]


def test_discover_project_rejects_git_dir_outside_common_directory(tmp_path: Path) -> None:
    root = tmp_path / "checkout"
    common_dir = tmp_path / "repository.git"
    outside_git_dir = tmp_path / "other.git"
    outputs = iter((root, common_dir, outside_git_dir))

    def fake_run(arguments: tuple[str, ...], **_: object) -> CommandResult:
        output = next(outputs)
        return CommandResult(arguments=arguments, returncode=0, stdout=f"{output}\n", stderr="")

    with pytest.raises(ProjectError):
        discover_project(root, runner=fake_run)
