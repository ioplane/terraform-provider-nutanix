from __future__ import annotations

from pathlib import Path

import yaml


def test_canonical_gate_checks_formatting_without_rewriting_sources() -> None:
    document = yaml.safe_load(Path("Taskfile.yml").read_text())
    commands = document["tasks"]["all"]["cmds"]

    assert {"task": "python:format:check"} in commands
    assert {"task": "go:format:check"} in commands
    assert {"task": "python:format"} not in commands
    assert {"task": "go:format"} not in commands


def test_canonical_gate_includes_document_and_oci_validation() -> None:
    document = yaml.safe_load(Path("Taskfile.yml").read_text())
    tasks = document["tasks"]
    commands = tasks["all"]["cmds"]

    assert {"task": "docs:check"} in commands
    assert {"task": "oci:check"} in commands

    docs_commands = "\n".join(str(command) for command in tasks["docs:check"]["cmds"])
    assert "rumdl check" in docs_commands
    assert "yamllint" in docs_commands
    assert "scripts.checks.docs_links" in docs_commands

    oci_commands = "\n".join(str(command) for command in tasks["oci:check"]["cmds"])
    assert "hadolint" in oci_commands
    assert "podman-compose" in oci_commands
    assert "config" in oci_commands
