"""Stable Compose project identity for Git worktrees."""

from __future__ import annotations

import hashlib
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from scripts.automation.process import CommandResult, run

PROJECT_PREFIX = "nutanix-provider"
PROJECT_HASH_LENGTH = 12

Runner = Callable[..., CommandResult]


@dataclass(frozen=True, slots=True)
class Project:
    """Resolved repository paths and worktree-specific Compose identity."""

    root: Path
    git_common_dir: Path
    name: str


def project_name(git_common_dir: Path, worktree: Path) -> str:
    """Return a stable DNS-safe name unique to a repository worktree."""
    common = git_common_dir.expanduser().resolve()
    root = worktree.expanduser().resolve()
    identity = f"{common}\0{root}".encode()
    suffix = hashlib.sha256(identity).hexdigest()[:PROJECT_HASH_LENGTH]
    return f"{PROJECT_PREFIX}-{suffix}"


def discover_project(start: Path | None = None, *, runner: Runner = run) -> Project:
    """Resolve the worktree root, Git common directory, and Compose name."""
    starting_path = (start or Path.cwd()).expanduser().resolve()
    root_arguments = (
        "git",
        "-C",
        str(starting_path),
        "rev-parse",
        "--path-format=absolute",
        "--show-toplevel",
    )
    root = Path(runner(root_arguments).stdout.strip()).expanduser().resolve()
    common_arguments = (
        "git",
        "-C",
        str(root),
        "rev-parse",
        "--path-format=absolute",
        "--git-common-dir",
    )
    git_common_dir = Path(runner(common_arguments).stdout.strip()).expanduser().resolve()
    return Project(
        root=root, git_common_dir=git_common_dir, name=project_name(git_common_dir, root)
    )
