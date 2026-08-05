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
