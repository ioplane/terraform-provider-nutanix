"""Canonicalize the Beads project config after pinned bootstrap serialization."""

from __future__ import annotations

import os
import stat
import sys
from collections.abc import Mapping
from pathlib import Path
from typing import TextIO

CONFIG_NAME = "config.yaml"


class ConfigNormalizationError(RuntimeError):
    """ConfigNormalizationError reports an unsafe or invalid Beads config."""


def normalize(path: Path) -> None:
    """Append exactly one final LF to a nonempty, regular, non-symlink file."""
    flags = os.O_RDWR | os.O_APPEND
    if not hasattr(os, "O_NOFOLLOW"):
        raise ConfigNormalizationError("this platform cannot reject config symlinks")
    flags |= os.O_NOFOLLOW

    try:
        descriptor = os.open(path, flags)
    except OSError as error:
        raise ConfigNormalizationError(f"cannot open Beads config safely: {error}") from error

    try:
        metadata = os.fstat(descriptor)
        if not stat.S_ISREG(metadata.st_mode):
            raise ConfigNormalizationError("Beads config must be a regular file")
        if metadata.st_size == 0:
            raise ConfigNormalizationError("Beads config must not be empty")

        os.lseek(descriptor, -1, os.SEEK_END)
        if os.read(descriptor, 1) != b"\n":
            os.write(descriptor, b"\n")
            os.fsync(descriptor)
    except OSError as error:
        raise ConfigNormalizationError(f"cannot normalize Beads config: {error}") from error
    finally:
        os.close(descriptor)


def main(
    *,
    env: Mapping[str, str] | None = None,
    stderr: TextIO | None = None,
) -> int:
    """Normalize the config selected by the explicit absolute BEADS_DIR."""
    selected_env = os.environ if env is None else env
    selected_stderr = sys.stderr if stderr is None else stderr
    raw_beads_dir = selected_env.get("BEADS_DIR", "")
    beads_dir = Path(raw_beads_dir)
    if not raw_beads_dir or not beads_dir.is_absolute():
        print("beads-config: BEADS_DIR must be an absolute path", file=selected_stderr)
        return 1

    try:
        normalize(beads_dir / CONFIG_NAME)
    except ConfigNormalizationError as error:
        print(f"beads-config: {error}", file=selected_stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
