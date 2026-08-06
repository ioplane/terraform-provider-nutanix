"""Stable Compose project identity for Git worktrees."""

from __future__ import annotations

import hashlib
import os
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path

from scripts.automation.process import CommandResult, run

PROJECT_PREFIX = "nutanix-provider"
PROJECT_HASH_LENGTH = 12
GIT_DISCOVERY_TIMEOUT_SECONDS = 15.0

Runner = Callable[..., CommandResult]


def _host_git_environment(env: Mapping[str, str]) -> dict[str, str]:
    sanitized = dict(env)
    sanitized.pop("GH_TOKEN", None)
    sanitized.pop("GITHUB_TOKEN", None)
    return sanitized


@dataclass(frozen=True, slots=True)
class Project:
    """Resolved repository paths and worktree-specific Compose identity."""

    root: Path
    git_common_dir: Path
    git_dir: Path
    git_dir_relative: Path
    name: str


class ProjectError(RuntimeError):
    """ProjectError reports an unsafe or inconsistent Git worktree layout."""


def project_name(git_common_dir: Path, worktree: Path) -> str:
    """Return a stable DNS-safe name unique to a repository worktree."""
    common = git_common_dir.expanduser().resolve()
    root = worktree.expanduser().resolve()
    identity = f"{common}\0{root}".encode()
    suffix = hashlib.sha256(identity).hexdigest()[:PROJECT_HASH_LENGTH]
    return f"{PROJECT_PREFIX}-{suffix}"


def discover_project(
    start: Path | None = None,
    *,
    runner: Runner = run,
    env: Mapping[str, str] = os.environ,
) -> Project:
    """Resolve worktree paths and a stable Compose project identity."""
    starting_path = (start or Path.cwd()).expanduser().resolve()
    git_env = _host_git_environment(env)
    root_arguments = (
        "git",
        "-C",
        str(starting_path),
        "rev-parse",
        "--path-format=absolute",
        "--show-toplevel",
    )
    root = (
        Path(
            runner(
                root_arguments,
                env=git_env,
                timeout=GIT_DISCOVERY_TIMEOUT_SECONDS,
            ).stdout.strip()
        )
        .expanduser()
        .resolve()
    )
    common_arguments = (
        "git",
        "-C",
        str(root),
        "rev-parse",
        "--path-format=absolute",
        "--git-common-dir",
    )
    git_common_dir = (
        Path(
            runner(
                common_arguments,
                env=git_env,
                timeout=GIT_DISCOVERY_TIMEOUT_SECONDS,
            ).stdout.strip()
        )
        .expanduser()
        .resolve()
    )
    git_dir_arguments = (
        "git",
        "-C",
        str(root),
        "rev-parse",
        "--path-format=absolute",
        "--git-dir",
    )
    git_dir = (
        Path(
            runner(
                git_dir_arguments,
                env=git_env,
                timeout=GIT_DISCOVERY_TIMEOUT_SECONDS,
            ).stdout.strip()
        )
        .expanduser()
        .resolve()
    )
    try:
        git_dir_relative = git_dir.relative_to(git_common_dir)
    except ValueError:
        raise ProjectError(
            f"Git directory {git_dir} is outside common directory {git_common_dir}"
        ) from None
    return Project(
        root=root,
        git_common_dir=git_common_dir,
        git_dir=git_dir,
        git_dir_relative=git_dir_relative,
        name=project_name(git_common_dir, root),
    )
