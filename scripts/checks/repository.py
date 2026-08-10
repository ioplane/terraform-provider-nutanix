"""Validate repository structure and tracked-file safety boundaries."""

from __future__ import annotations

import re
import sys
from collections.abc import Callable, Sequence
from pathlib import Path, PurePosixPath
from typing import TextIO

from scripts.automation.process import CommandError, run

REQUIRED_FILES = (
    ".hadolint.yaml",
    ".release-please-manifest.json",
    ".yamllint.yml",
    ".goreleaser.yml",
    "CHANGELOG.md",
    "CODE_OF_CONDUCT.md",
    "CONTRIBUTING.md",
    "LICENSE",
    "README.md",
    "SECURITY.md",
    "Taskfile.yml",
    "dev",
    "docs/architecture.md",
    "docs/contract.md",
    "docs/index.md",
    "docs/release-process.md",
    "docs/roadmap.md",
    "docs/standards/dependencies.md",
    "docs/standards/go-1.26.md",
    "docs/standards/naming.md",
    "docs/standards/nutanix-artifacts.md",
    "docs/standards/testing.md",
    "deployments/containers/tool-assets.lock",
    "go.mod",
    "go.sum",
    "release-please-config.json",
)
FORBIDDEN_PUBLIC_FILES = {".beads/issues.jsonl", "CLAUDE.md", "CODEX.md"}
FORBIDDEN_PUBLIC_DIRECTORIES = {PurePosixPath("docs/adr"), PurePosixPath("docs/superpowers")}

Runner = Callable[[Sequence[str], Path], str]

_NUTANIX_MODULE = re.compile(
    r"(?im)^\s*(?:require\s+)?github\.com/(?:nutanix|nutanix-cloud-native)/\S+"
)


class RepositoryCheckError(RuntimeError):
    """RepositoryCheckError reports an unavailable Git index inspection."""


def _forbidden(path_text: str) -> bool:
    path = PurePosixPath(path_text)
    parts = path.parts
    name = path.name
    return (
        path_text in FORBIDDEN_PUBLIC_FILES
        or any(
            directory == path or directory in path.parents
            for directory in FORBIDDEN_PUBLIC_DIRECTORIES
        )
        or ".cache" in parts
        or ".terraform" in parts
        or name == ".env"
        or name.startswith(".env.")
        or name.endswith((".tfstate", ".tfplan", ".pem", ".key"))
        or ".tfstate." in name
        or name in {"tfplan", "crash.log"}
    )


def validate(root: Path, tracked_paths: Sequence[str]) -> list[str]:
    """Return deterministic repository contract diagnostics."""
    diagnostics: list[str] = []
    for relative in REQUIRED_FILES:
        if not (root / relative).is_file():
            diagnostics.append(f"required file missing: {relative}")

    for relative in sorted(set(tracked_paths)):
        if _forbidden(relative):
            diagnostics.append(f"tracked forbidden path: {relative}")

    module = root / "go.mod"
    if module.is_file() and _NUTANIX_MODULE.search(module.read_text()):
        diagnostics.append("go.mod contains a Nutanix SDK dependency")
    return sorted(set(diagnostics))


def _git_ls_files(arguments: Sequence[str], root: Path) -> str:
    try:
        return run(arguments, cwd=root, timeout=15.0).stdout
    except (CommandError, OSError) as error:
        raise RepositoryCheckError("cannot inspect tracked files") from error


def main(
    *,
    root: Path | None = None,
    runner: Runner = _git_ls_files,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Run the repository contract gate."""
    selected_root = Path.cwd() if root is None else root
    try:
        payload = runner(("git", "ls-files", "-z"), selected_root)
    except RepositoryCheckError as error:
        print(f"repository: {error}", file=stderr)
        return 1
    tracked = [item for item in payload.split("\0") if item]
    diagnostics = validate(selected_root, tracked)
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"repository: {diagnostic}", file=stderr)
        return 1
    print(f"repository: ok ({len(tracked)} tracked files)", file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
