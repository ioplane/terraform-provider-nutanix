"""Generate Framework reference docs offline and reject tracked drift."""

from __future__ import annotations

import json
import os
import re
import sys
import tempfile
from collections.abc import Callable, Mapping, Sequence
from pathlib import Path
from typing import TextIO

from scripts.automation.process import CommandError, run
from scripts.package import provider

GENERATED_DOCS = (
    "data-sources/categories_v2.md",
    "data-sources/clusters_v2.md",
    "data-sources/images_v2.md",
    "data-sources/license_keys_v2.md",
    "data-sources/licenses_v2.md",
    "data-sources/operations_v2.md",
    "data-sources/roles_v2.md",
    "data-sources/subnet_v2.md",
    "index.md",
    "resources/category.md",
    "resources/image_placement_policy.md",
    "resources/storage_container.md",
    "resources/subnet.md",
)
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


def render_generated_docs(
    *,
    root: Path,
    temporary: Path,
    rendered: Path,
    source_date_epoch: int,
    schema_generator: SchemaGenerator = provider.generate_offline_schema,
    generator: Generator = _run_generator,
) -> None:
    """Render the complete offline Framework reference into one temporary tree."""
    schema = schema_generator(
        root=root,
        temporary=temporary / "schema-work",
        version=provider.DEFAULT_VERSION,
        source_date_epoch=source_date_epoch,
    )
    schema_path = temporary / "provider-schema.json"
    schema_path.write_text(tfplugindocs_schema(schema))
    provider_dir = root / "cmd" / "terraform-provider-nutanix"
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
        root,
        _environment(temporary),
    )
    # tfplugindocs emits two empty template paragraphs before a resource schema
    # when no examples directory is populated for that resource. Keep the
    # generated page deterministic and compatible with the repository Markdown
    # lint contract without changing the upstream generator globally.
    category_page = rendered / "resources" / "category.md"
    if category_page.is_file():
        category_page.write_text(
            category_page.read_text().replace("\n\n\n\n<!-- schema", "\n\n<!-- schema")
        )
    subnet_page = rendered / "resources" / "subnet.md"
    if subnet_page.is_file():
        subnet_page.write_text(
            subnet_page.read_text().replace("\n\n\n\n<!-- schema", "\n\n<!-- schema")
        )
    storage_container_page = rendered / "resources" / "storage_container.md"
    if storage_container_page.is_file():
        storage_container_page.write_text(
            storage_container_page.read_text().replace("\n\n\n\n<!-- schema", "\n\n<!-- schema")
        )
    placement_policy_page = rendered / "resources" / "image_placement_policy.md"
    if placement_policy_page.is_file():
        placement = placement_policy_page.read_text()
        placement = placement.replace("\n\n\n\n<!-- schema", "\n\n<!-- schema")
        # tfplugindocs emits raw anchors for nested attributes. They are
        # redundant because the generated headings remain stable and violate
        # the repository Markdown lint contract (MD033/MD022/MD012).
        placement = re.sub(
            r"#nestedatt--([a-z0-9_]+)",
            r"#nested-schema-for-\1",
            placement,
        )
        placement = re.sub(r'^<a id="nestedatt--[^"]+"></a>\n', "", placement, flags=re.MULTILINE)
        placement = re.sub(r"\n{3,}(?=### Nested Schema)", "\n\n", placement)
        placement_policy_page.write_text(placement)


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
            rendered = temporary / "rendered"
            render_generated_docs(
                root=selected_root,
                temporary=temporary,
                rendered=rendered,
                source_date_epoch=epoch,
                schema_generator=schema_generator,
                generator=generator,
            )
            diagnostics = validate_rendered(selected_root, rendered)
    except (DocsCheckError, provider.PackageError, OSError, ValueError) as error:
        print(f"docs: {error}", file=stderr)
        return 1
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"docs: {diagnostic}", file=stderr)
        return 1
    print(f"docs: ok ({len(GENERATED_DOCS)} generated files)", file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
