from __future__ import annotations

import io
import json
from collections.abc import Mapping, Sequence
from pathlib import Path

from scripts.checks import docs


def test_adapts_verified_ioplane_schema_for_tfplugindocs_lookup() -> None:
    payload = json.dumps(
        {
            "format_version": "1.0",
            "provider_schemas": {
                "registry.terraform.io/ioplane/nutanix": {"provider": {"version": 0}}
            },
        }
    )

    adapted = json.loads(docs.tfplugindocs_schema(payload))

    assert set(adapted["provider_schemas"]) == {"registry.terraform.io/hashicorp/nutanix"}
    assert adapted["provider_schemas"]["registry.terraform.io/hashicorp/nutanix"] == {
        "provider": {"version": 0}
    }


def test_validate_rendered_rejects_file_set_and_content_drift(tmp_path: Path) -> None:
    tracked = tmp_path / "docs" / "index.md"
    tracked.parent.mkdir()
    tracked.write_text("tracked\n")
    rendered = tmp_path / "rendered"
    rendered.mkdir()
    (rendered / "index.md").write_text("generated\n")
    (rendered / "unexpected.md").write_text("unexpected\n")

    assert docs.validate_rendered(tmp_path, rendered) == [
        "generated documentation file set differs",
        "generated documentation differs: docs/index.md",
    ]


def test_main_uses_offline_schema_and_temporary_output(tmp_path: Path) -> None:
    tracked = tmp_path / "docs" / "index.md"
    tracked.parent.mkdir(parents=True)
    tracked.write_text("generated\n")
    provider_dir = tmp_path / "cmd" / "terraform-provider-nutanix"
    provider_dir.mkdir(parents=True)
    calls: list[tuple[str, ...]] = []

    def schema_generator(**arguments: object) -> str:
        assert arguments["root"] == tmp_path
        return (
            '{"provider_schemas": {'
            '"registry.terraform.io/ioplane/nutanix": {"provider": {"version": 0}}}}\n'
        )

    def generator(arguments: Sequence[str], root: Path, environment: Mapping[str, str]) -> None:
        command = tuple(arguments)
        calls.append(command)
        assert root == tmp_path
        assert set(environment) == {"PATH", "HOME"}
        schema_flag = command.index("--providers-schema")
        schema_path = (provider_dir / command[schema_flag + 1]).resolve()
        assert set(json.loads(schema_path.read_text())["provider_schemas"]) == {
            "registry.terraform.io/hashicorp/nutanix"
        }
        rendered_flag = command.index("--rendered-website-dir")
        rendered = (provider_dir / command[rendered_flag + 1]).resolve()
        rendered.mkdir(parents=True)
        (rendered / "index.md").write_text("generated\n")

    stdout = io.StringIO()
    stderr = io.StringIO()
    assert (
        docs.main(
            root=tmp_path,
            schema_generator=schema_generator,
            generator=generator,
            stdout=stdout,
            stderr=stderr,
        )
        == 0
    )
    assert len(calls) == 1
    assert stdout.getvalue() == "docs: ok (1 generated file)\n"
    assert stderr.getvalue() == ""
