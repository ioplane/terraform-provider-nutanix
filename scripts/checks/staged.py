"""Reject staged Nutanix vendor artifact bodies."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path
from typing import TextIO

CACHE_PREFIX = ".cache/nutanix/artifacts/"
TIMEOUT_SECONDS = 15.0


class StagedCheckError(RuntimeError):
    """StagedCheckError reports an unavailable Git index inspection."""


def _git_environment(repository: Path) -> dict[str, str]:
    environment = dict(os.environ)
    configured_worktree = environment.get("GIT_WORK_TREE")
    preserve_explicit = False
    if configured_worktree:
        try:
            preserve_explicit = Path(configured_worktree).resolve() == repository.resolve()
        except OSError:
            preserve_explicit = False
    if not preserve_explicit:
        for name in ("GIT_COMMON_DIR", "GIT_DIR", "GIT_WORK_TREE"):
            environment.pop(name, None)
    return environment


def check(repository: Path) -> list[str]:
    """Return sorted staged paths located under the vendor artifact cache."""
    try:
        result = subprocess.run(
            ("git", "diff", "--cached", "--name-only", "-z"),
            cwd=repository,
            env=_git_environment(repository),
            check=False,
            capture_output=True,
            timeout=TIMEOUT_SECONDS,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise StagedCheckError("cannot inspect staged paths") from error
    if result.returncode != 0:
        raise StagedCheckError("git diff --cached failed")
    paths = [value.decode("utf-8") for value in result.stdout.split(b"\0") if value]
    return sorted(path for path in paths if path.startswith(CACHE_PREFIX))


def main(
    *,
    repository: Path | None = None,
    stdout: TextIO | None = None,
    stderr: TextIO | None = None,
) -> int:
    """Run the staged vendor-cache gate."""
    selected_repository = Path.cwd() if repository is None else repository
    selected_stdout = sys.stdout if stdout is None else stdout
    selected_stderr = sys.stderr if stderr is None else stderr
    try:
        rejected = check(selected_repository)
    except StagedCheckError as error:
        print(f"staged: {error}", file=selected_stderr)
        return 1
    if rejected:
        for path in rejected:
            print(f"staged: vendor artifact body is staged: {path}", file=selected_stderr)
        return 1
    print("staged: vendor artifact cache is absent from the index", file=selected_stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
