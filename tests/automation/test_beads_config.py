from __future__ import annotations

import io
import stat
from pathlib import Path

import pytest
from scripts.automation import beads_config


def test_normalize_appends_one_newline_and_preserves_mode(tmp_path: Path) -> None:
    config = tmp_path / "config.yaml"
    config.write_bytes(b"sync.remote: example")
    config.chmod(0o600)

    beads_config.normalize(config)
    beads_config.normalize(config)

    assert config.read_bytes() == b"sync.remote: example\n"
    assert stat.S_IMODE(config.stat().st_mode) == 0o600


@pytest.mark.parametrize("kind", ["missing", "empty", "directory", "symlink"])
def test_normalize_rejects_unsafe_config(tmp_path: Path, kind: str) -> None:
    config = tmp_path / "config.yaml"
    if kind == "empty":
        config.touch()
    elif kind == "directory":
        config.mkdir()
    elif kind == "symlink":
        target = tmp_path / "target.yaml"
        target.write_text("sync.remote: example\n")
        config.symlink_to(target)

    with pytest.raises(beads_config.ConfigNormalizationError):
        beads_config.normalize(config)


def test_main_uses_absolute_beads_dir_and_reports_errors(tmp_path: Path) -> None:
    config = tmp_path / "config.yaml"
    config.write_bytes(b"sync.remote: example")
    stderr = io.StringIO()

    assert beads_config.main(env={"BEADS_DIR": str(tmp_path)}, stderr=stderr) == 0
    assert config.read_bytes().endswith(b"\n")
    assert stderr.getvalue() == ""

    assert beads_config.main(env={"BEADS_DIR": "relative"}, stderr=stderr) == 1
    assert "BEADS_DIR must be an absolute path" in stderr.getvalue()
