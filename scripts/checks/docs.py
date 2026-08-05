"""Generate Framework reference docs offline and reject tracked drift."""

from __future__ import annotations

import json
import os
import sys
import tempfile
from collections.abc import Callable, Mapping, Sequence
from pathlib import Path
from typing import TextIO

from scripts.automation.process import CommandError, run
from scripts.package import provider

GENERATED_DOCS = ("index.md",)
DEFAULT_SOURCE_DATE_EPOCH = 1_700_000_000
TFPLUGINDOCS_PROVIDER_ADDRESS = "registry.terraform.io/hashicorp/nutanix"

SchemaGenerator = Callable[..., str]
Generator = Callable[[Sequence[str], Path, Mapping[str, str]], None]


class DocsCheckError(RuntimeError):
    """DocsCheckError reports a failed offline schema or docs generation command."""


def tfplugindocs_schema(payload: str) -> str:
    """Adapt a verified provider schema to tfplugindocs' fixed default namespace lookup."""
    try:
        document = json.loads(payload)
    except json.JSONDecodeError as error:
        raise DocsCheckError("offline provider schema is invalid JSON") from error
    if not isinstance(document, dict) or not isinstance(document.get("provider_schemas"), dict):
        raise DocsCheckError("offline provider schema shape is invalid")
    schemas = document["provider_schemas"]
    if set(schemas) != {provider.PROVIDER_ADDRESS}:
        raise DocsCheckError("offline provider schema address differs")
    document["provider_schemas"] = {
        TFPLUGINDOCS_PROVIDER_ADDRESS: schemas[provider.PROVIDER_ADDRESS]
    }
    return json.dumps(document, sort_keys=True) + "\n"


def validate_rendered(root: Path, rendered: Path) -> list[str]:
    """Return deterministic generated-doc drift diagnostics."""
    observed = sorted(
        path.relative_to(rendered).as_posix() for path in rendered.rglob("*") if path.is_file()
    )
    diagnostics: list[str] = []
    if observed != list(GENERATED_DOCS):
        diagnostics.append("generated documentation file set differs")
    for relative in sorted(set(observed) & set(GENERATED_DOCS)):
        tracked = root / "docs" / relative
        generated = rendered / relative
        if not tracked.is_file() or tracked.read_bytes() != generated.read_bytes():
            diagnostics.append(f"generated documentation differs: docs/{relative}")
    return diagnostics


def _run_generator(arguments: Sequence[str], root: Path, environment: Mapping[str, str]) -> None:
    try:
        run(arguments, cwd=root, env=environment, timeout=300.0)
    except (CommandError, OSError) as error:
        raise DocsCheckError("tfplugindocs generation failed") from error


def _environment(temporary: Path) -> dict[str, str]:
    path = os.environ.get("PATH")
    if not path:
        raise DocsCheckError("PATH is unavailable")
    home = temporary / "home"
    home.mkdir()
    return {"PATH": path, "HOME": str(home)}


def main(
    *,
    root: Path | None = None,
    schema_generator: SchemaGenerator = provider.generate_offline_schema,
    generator: Generator = _run_generator,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Run offline generated-doc drift validation."""
    selected_root = Path.cwd() if root is None else root
    raw_epoch = os.environ.get("SOURCE_DATE_EPOCH", str(DEFAULT_SOURCE_DATE_EPOCH))
    try:
        epoch = int(raw_epoch)
        with tempfile.TemporaryDirectory(prefix="nutanix-docs-") as raw_temporary:
            temporary = Path(raw_temporary)
            schema = schema_generator(
                root=selected_root,
                temporary=temporary / "schema-work",
                version=provider.DEFAULT_VERSION,
                source_date_epoch=epoch,
            )
            schema_path = temporary / "provider-schema.json"
            schema_path.write_text(tfplugindocs_schema(schema))
            rendered = temporary / "rendered"
            provider_dir = selected_root / "cmd" / "terraform-provider-nutanix"
            relative_temporary = Path(os.path.relpath(temporary, provider_dir)).as_posix()
            generator(
                (
                    "tfplugindocs",
                    "generate",
                    "--provider-dir",
                    str(provider_dir),
                    "--provider-name",
                    "nutanix",
                    "--rendered-provider-name",
                    "Nutanix",
                    "--examples-dir",
                    "../../examples",
                    "--providers-schema",
                    f"{relative_temporary}/provider-schema.json",
                    "--rendered-website-dir",
                    f"{relative_temporary}/rendered",
                    "--website-temp-dir",
                    f"{relative_temporary}/website-work",
                ),
                selected_root,
                _environment(temporary),
            )
            diagnostics = validate_rendered(selected_root, rendered)
    except (DocsCheckError, provider.PackageError, OSError, ValueError) as error:
        print(f"docs: {error}", file=stderr)
        return 1
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"docs: {diagnostic}", file=stderr)
        return 1
    print(f"docs: ok ({len(GENERATED_DOCS)} generated file)", file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
