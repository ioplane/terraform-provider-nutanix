"""Generate the tracked offline Terraform Framework reference pages."""

from __future__ import annotations

import argparse
import os
import tempfile
from collections.abc import Sequence
from pathlib import Path

from scripts.checks import docs
from scripts.package import provider


def generate(root: Path, source_date_epoch: int) -> int:
    """Render and replace only the reviewed generated-document file set."""
    with tempfile.TemporaryDirectory(prefix="nutanix-docs-") as raw_temporary:
        temporary = Path(raw_temporary)
        rendered = temporary / "rendered"
        docs.render_generated_docs(
            root=root,
            temporary=temporary,
            rendered=rendered,
            source_date_epoch=source_date_epoch,
        )
        observed = sorted(
            path.relative_to(rendered).as_posix() for path in rendered.rglob("*") if path.is_file()
        )
        if observed != list(docs.GENERATED_DOCS):
            raise docs.DocsCheckError("generated documentation file set differs")
        for relative in docs.GENERATED_DOCS:
            destination = root / "docs" / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            temporary_destination = destination.with_suffix(destination.suffix + ".tmp")
            temporary_destination.write_bytes((rendered / relative).read_bytes())
            os.replace(temporary_destination, destination)
    return len(docs.GENERATED_DOCS)


def main(arguments: Sequence[str] | None = None) -> int:
    """Run the deterministic provider-document generation command."""
    parser = argparse.ArgumentParser(prog="provider-docs")
    parser.add_argument("--root", type=Path, default=Path.cwd())
    parser.add_argument(
        "--source-date-epoch",
        type=int,
        default=docs.DEFAULT_SOURCE_DATE_EPOCH,
    )
    parsed = parser.parse_args(arguments)
    try:
        count = generate(parsed.root.resolve(), parsed.source_date_epoch)
    except (docs.DocsCheckError, provider.PackageError, OSError, ValueError) as error:
        print(f"provider-docs: {error}")
        return 1
    print(f"provider-docs: wrote {count} generated files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
