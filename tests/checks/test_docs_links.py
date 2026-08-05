from __future__ import annotations

from pathlib import Path

from scripts.checks import docs_links


def test_accepts_existing_relative_files_and_github_heading_fragments(tmp_path: Path) -> None:
    guide = tmp_path / "docs" / "guide.md"
    guide.parent.mkdir()
    guide.write_text("# Guide\n\nSee [contract](contract.md#public-contract).\n")
    contract = guide.parent / "contract.md"
    contract.write_text("# Public contract\n")

    assert docs_links.validate(tmp_path, ("docs/guide.md", "docs/contract.md")) == []


def test_rejects_missing_targets_fragments_and_repository_escape(tmp_path: Path) -> None:
    guide = tmp_path / "docs" / "guide.md"
    guide.parent.mkdir()
    guide.write_text(
        "# Guide\n\n"
        "[missing](missing.md)\n\n"
        "[fragment](contract.md#missing-fragment)\n\n"
        "[escape](../../outside.md)\n"
    )
    (guide.parent / "contract.md").write_text("# Public contract\n")

    diagnostics = docs_links.validate(tmp_path, ("docs/guide.md", "docs/contract.md"))

    assert "docs/guide.md: missing local link target: missing.md" in diagnostics
    assert "docs/guide.md: missing local link fragment: contract.md#missing-fragment" in diagnostics
    assert "docs/guide.md: local link escapes repository: ../../outside.md" in diagnostics


def test_validates_reference_style_links(tmp_path: Path) -> None:
    readme = tmp_path / "README.md"
    readme.write_text("# Readme\n\nSee [guide][guide].\n\n[guide]: docs/guide.md\n")
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "guide.md").write_text("# Guide\n")

    assert docs_links.validate(tmp_path, ("README.md", "docs/guide.md")) == []
