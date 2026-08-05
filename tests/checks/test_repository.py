from __future__ import annotations

import io
from collections.abc import Sequence
from pathlib import Path

from scripts.checks import repository


def valid_repository(root: Path) -> list[str]:
    tracked: list[str] = []
    for relative in repository.REQUIRED_FILES:
        path = root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        content = repository.POINTER_FILES.get(relative, "placeholder\n")
        if relative == "go.mod":
            content = "module github.com/ioplane/terraform-provider-nutanix\n\ngo 1.26.0\n"
        path.write_text(content)
        tracked.append(relative)
    return tracked


def test_valid_repository_contract_has_no_diagnostics(tmp_path: Path) -> None:
    tracked = valid_repository(tmp_path)

    assert repository.validate(tmp_path, tracked) == []


def test_requires_files_and_exact_pointer_content(tmp_path: Path) -> None:
    tracked = valid_repository(tmp_path)
    (tmp_path / "CLAUDE.md").write_text("copied instructions\n")
    (tmp_path / "docs/index.md").unlink()

    diagnostics = repository.validate(tmp_path, tracked)

    assert "pointer file differs: CLAUDE.md" in diagnostics
    assert "required file missing: docs/index.md" in diagnostics


def test_rejects_tracked_cache_state_secret_and_nutanix_sdk(tmp_path: Path) -> None:
    tracked = valid_repository(tmp_path)
    tracked.extend(
        [
            ".cache/nutanix/artifacts/vmm/openapi.yaml",
            ".env",
            "terraform.tfstate",
        ]
    )
    (tmp_path / "go.mod").write_text(
        "module github.com/ioplane/terraform-provider-nutanix\n\n"
        "require github.com/nutanix-cloud-native/prism-go-client v0.4.0\n"
    )

    diagnostics = repository.validate(tmp_path, tracked)

    assert "tracked forbidden path: .cache/nutanix/artifacts/vmm/openapi.yaml" in diagnostics
    assert "tracked forbidden path: .env" in diagnostics
    assert "tracked forbidden path: terraform.tfstate" in diagnostics
    assert "go.mod contains a Nutanix SDK dependency" in diagnostics


def test_main_reads_nul_delimited_git_index(tmp_path: Path) -> None:
    tracked = valid_repository(tmp_path)
    stdout = io.StringIO()
    stderr = io.StringIO()
    calls: list[tuple[str, ...]] = []

    def runner(arguments: Sequence[str], repository_root: Path) -> str:
        calls.append(tuple(arguments))
        assert repository_root == tmp_path
        return "\0".join(tracked) + "\0"

    assert repository.main(root=tmp_path, runner=runner, stdout=stdout, stderr=stderr) == 0
    assert calls == [("git", "ls-files", "-z")]
    assert stdout.getvalue() == f"repository: ok ({len(tracked)} tracked files)\n"
    assert stderr.getvalue() == ""
