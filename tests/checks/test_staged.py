from __future__ import annotations

import os
import subprocess
from pathlib import Path

from scripts.checks import staged


def git(repository: Path, *arguments: str) -> None:
    environment = dict(os.environ)
    for name in ("GIT_COMMON_DIR", "GIT_DIR", "GIT_WORK_TREE"):
        environment.pop(name, None)
    subprocess.run(
        ("git", *arguments),
        cwd=repository,
        env=environment,
        check=True,
        capture_output=True,
        text=True,
    )


def test_staged_checker_accepts_source_and_rejects_forced_vendor_cache(
    tmp_path: Path,
) -> None:
    repository = tmp_path / "repository"
    repository.mkdir()
    git(repository, "init")
    git(repository, "config", "user.email", "test@example.com")
    git(repository, "config", "user.name", "Test")
    (repository / ".gitignore").write_text(".cache/\n")
    manifest = repository / "specs" / "nutanix" / "manifest.json"
    manifest.parent.mkdir(parents=True)
    manifest.write_text("{}\n")
    source = repository / "source.py"
    source.write_text("value = 1\n")
    git(repository, "add", ".gitignore", "specs/nutanix/manifest.json", "source.py")

    assert staged.check(repository) == []

    vendor = repository / ".cache" / "nutanix" / "artifacts" / "x" / "v4.0"
    vendor.mkdir(parents=True)
    (vendor / "openapi.yaml").write_text("openapi: 3.1.0\n")
    git(repository, "add", "-f", ".cache/nutanix/artifacts/x/v4.0/openapi.yaml")

    assert staged.check(repository) == [".cache/nutanix/artifacts/x/v4.0/openapi.yaml"]
