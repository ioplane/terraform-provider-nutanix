"""Build, package, and offline-smoke the Terraform provider deterministically."""

from __future__ import annotations

import argparse
import hashlib
import hmac
import io
import json
import os
import re
import stat
import sys
import tempfile
import zipfile
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import TextIO, cast

from scripts.automation.process import CommandError, run

PROVIDER_ADDRESS = "registry.terraform.io/ioplane/nutanix"
PROVIDER_NAME = "terraform-provider-nutanix"
DEFAULT_VERSION = "0.0.0-dev"
MAX_ARCHIVE_ENTRY_BYTES = 512 * 1024 * 1024

_CORE_VERSION = r"(?:0|[1-9][0-9]*)"
_PRERELEASE_IDENTIFIER = r"(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)"
_VERSION = re.compile(
    rf"^{_CORE_VERSION}\.{_CORE_VERSION}\.{_CORE_VERSION}"
    rf"(?:-{_PRERELEASE_IDENTIFIER}(?:\.{_PRERELEASE_IDENTIFIER})*)?$"
)
_PLATFORM = re.compile(r"^[a-z0-9]+$")
_CHECKSUM = re.compile(r"^([0-9a-f]{64})  ([A-Za-z0-9_.-]+\.zip)\n$")


class PackageError(RuntimeError):
    """PackageError reports unsafe input or a failed deterministic package gate."""


@dataclass(frozen=True, slots=True)
class PackageResult:
    """PackageResult identifies one archive and its checksum evidence."""

    archive: Path
    checksums: Path
    sha256: str


def _validate_identity(version: str, goos: str, goarch: str) -> None:
    if not _VERSION.fullmatch(version):
        raise PackageError("unsafe provider version")
    if not _PLATFORM.fullmatch(goos) or not _PLATFORM.fullmatch(goarch):
        raise PackageError("unsafe provider platform")


def _regular_file(path: Path, label: str) -> None:
    try:
        mode = path.lstat().st_mode
    except OSError as error:
        raise PackageError(f"{label} is unavailable") from error
    if not stat.S_ISREG(mode):
        raise PackageError(f"{label} must be a regular file")


def _zip_timestamp(source_date_epoch: int) -> tuple[int, int, int, int, int, int]:
    try:
        timestamp = datetime.fromtimestamp(source_date_epoch, UTC)
    except (OverflowError, OSError, ValueError) as error:
        raise PackageError("unsafe SOURCE_DATE_EPOCH") from error
    if not 1980 <= timestamp.year <= 2107:
        raise PackageError("unsafe SOURCE_DATE_EPOCH")
    return (
        timestamp.year,
        timestamp.month,
        timestamp.day,
        timestamp.hour,
        timestamp.minute,
        timestamp.second - (timestamp.second % 2),
    )


def _zip_info(
    name: str, mode: int, timestamp: tuple[int, int, int, int, int, int]
) -> zipfile.ZipInfo:
    information = zipfile.ZipInfo(name, date_time=timestamp)
    information.create_system = 3
    information.compress_type = zipfile.ZIP_DEFLATED
    information.external_attr = (stat.S_IFREG | mode) << 16
    return information


def _atomic_write(path: Path, body: bytes, mode: int = 0o644) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.",
        suffix=".tmp",
        dir=path.parent,
    )
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(body)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary, mode)
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def create_package(
    *,
    binary: Path,
    license_path: Path,
    output_dir: Path,
    version: str,
    goos: str,
    goarch: str,
    source_date_epoch: int,
) -> PackageResult:
    """Create one deterministic provider ZIP and SHA256SUMS file."""
    _validate_identity(version, goos, goarch)
    _regular_file(binary, "provider binary")
    _regular_file(license_path, "license")
    timestamp = _zip_timestamp(source_date_epoch)
    provider_entry = f"{PROVIDER_NAME}_v{version}"
    entries = (
        ("LICENSE", license_path.read_bytes(), 0o644),
        (provider_entry, binary.read_bytes(), 0o755),
    )

    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name, body, mode in entries:
            archive.writestr(_zip_info(name, mode, timestamp), body, compresslevel=9)

    archive_name = f"{PROVIDER_NAME}_{version}_{goos}_{goarch}.zip"
    archive_path = output_dir / archive_name
    archive_body = buffer.getvalue()
    digest = hashlib.sha256(archive_body).hexdigest()
    _atomic_write(archive_path, archive_body)
    checksums = output_dir / "SHA256SUMS"
    _atomic_write(checksums, f"{digest}  {archive_name}\n".encode())
    return PackageResult(archive=archive_path, checksums=checksums, sha256=digest)


def verify_checksum(checksums: Path, archive: Path) -> str:
    """Verify that SHA256SUMS selects the exact archive by basename and digest."""
    _regular_file(checksums, "checksum file")
    _regular_file(archive, "provider archive")
    match = _CHECKSUM.fullmatch(checksums.read_text())
    if match is None or match.group(2) != archive.name:
        raise PackageError("checksum file does not select the provider archive")
    observed = hashlib.sha256(archive.read_bytes()).hexdigest()
    if not hmac.compare_digest(match.group(1), observed):
        raise PackageError("provider archive checksum mismatch")
    return observed


def extract_provider(*, archive: Path, destination: Path, version: str) -> Path:
    """Extract only the checksum-selected provider binary into a mirror directory."""
    _validate_identity(version, "linux", "amd64")
    _regular_file(archive, "provider archive")
    expected_binary = f"{PROVIDER_NAME}_v{version}"
    with zipfile.ZipFile(archive) as package:
        entries = package.infolist()
        if [entry.filename for entry in entries] != ["LICENSE", expected_binary]:
            raise PackageError("unexpected archive entries")
        binary_info = entries[1]
        if binary_info.file_size > MAX_ARCHIVE_ENTRY_BYTES:
            raise PackageError("provider archive entry is too large")
        body = package.read(binary_info)
        if len(body) != binary_info.file_size:
            raise PackageError("provider archive entry size mismatch")
    destination.mkdir(parents=True, exist_ok=True)
    if destination.is_symlink():
        raise PackageError("provider mirror destination must not be a symlink")
    selected = destination / expected_binary
    _atomic_write(selected, body, mode=0o755)
    return selected


def _command(label: str, arguments: Sequence[str], *, cwd: Path, env: Mapping[str, str]) -> str:
    try:
        return run(arguments, cwd=cwd, env=env, timeout=300.0).stdout
    except (CommandError, OSError) as error:
        raise PackageError(f"{label} command failed") from error


def _build_environment() -> dict[str, str]:
    path = os.environ.get("PATH")
    if not path:
        raise PackageError("PATH is unavailable")
    return {
        "PATH": path,
        "CGO_ENABLED": "0",
        "GOCACHE": "/root/.cache/go-build",
        "GOMODCACHE": "/go/pkg/mod",
        "GOPATH": "/go",
        "GOTOOLCHAIN": "local",
    }


def _build(root: Path, output: Path, version: str) -> None:
    environment = _build_environment()
    _command(
        "provider build",
        (
            "go",
            "build",
            "-buildvcs=false",
            "-trimpath",
            "-ldflags",
            f"-s -w -X main.version={version}",
            "-o",
            str(output),
            "./cmd/terraform-provider-nutanix",
        ),
        cwd=root,
        env=environment,
    )
    _regular_file(output, "built provider binary")


def _platform(root: Path) -> tuple[str, str]:
    output = _command(
        "go environment",
        ("go", "env", "GOOS", "GOARCH"),
        cwd=root,
        env=_build_environment(),
    ).splitlines()
    if len(output) != 2:
        raise PackageError("go environment output is invalid")
    goos, goarch = output
    _validate_identity(DEFAULT_VERSION, goos, goarch)
    return goos, goarch


def _terraform_environment(root: Path, temporary: Path, cli_config: Path) -> dict[str, str]:
    path = os.environ.get("PATH")
    if not path:
        raise PackageError("PATH is unavailable")
    home = temporary / "home"
    data = temporary / "terraform-data"
    home.mkdir()
    data.mkdir()
    return {
        "PATH": path,
        "HOME": str(home),
        "CHECKPOINT_DISABLE": "1",
        "TF_CLI_CONFIG_FILE": str(cli_config),
        "TF_DATA_DIR": str(data),
        "TF_IN_AUTOMATION": "1",
        "TF_INPUT": "0",
    }


def _write_offline_configuration(
    *, temporary: Path, mirror: Path, configuration: Path, version: str
) -> Path:
    cli_config = temporary / "terraform.tfrc"
    cli_config.write_text(
        "provider_installation {\n"
        "  filesystem_mirror {\n"
        f"    path = {json.dumps(str(mirror))}\n"
        f"    include = [{json.dumps(PROVIDER_ADDRESS)}]\n"
        "  }\n"
        "}\n"
    )
    configuration.mkdir()
    (configuration / "main.tf").write_text(
        "terraform {\n"
        "  required_providers {\n"
        "    nutanix = {\n"
        '      source = "ioplane/nutanix"\n'
        f'      version = "={version}"\n'
        "    }\n"
        "  }\n"
        "}\n\n"
        'provider "nutanix" {}\n'
    )
    return cli_config


def _verify_schema(payload: str) -> None:
    try:
        document = json.loads(payload)
    except json.JSONDecodeError as error:
        raise PackageError("Terraform schema output is invalid JSON") from error
    if not isinstance(document, dict) or not isinstance(document.get("provider_schemas"), dict):
        raise PackageError("Terraform schema output shape is invalid")
    schemas = cast(dict[str, object], document["provider_schemas"])
    if set(schemas) != {PROVIDER_ADDRESS}:
        raise PackageError("Terraform schema provider address differs")


def _offline_schema_from_package(
    *,
    root: Path,
    temporary: Path,
    package: PackageResult,
    version: str,
    goos: str,
    goarch: str,
) -> str:
    temporary.mkdir(parents=True)
    verify_checksum(package.checksums, package.archive)
    mirror_platform = temporary / "mirror" / PROVIDER_ADDRESS / version / f"{goos}_{goarch}"
    extract_provider(archive=package.archive, destination=mirror_platform, version=version)
    configuration = temporary / "configuration"
    cli_config = _write_offline_configuration(
        temporary=temporary,
        mirror=temporary / "mirror",
        configuration=configuration,
        version=version,
    )
    environment = _terraform_environment(root, temporary, cli_config)
    _command(
        "Terraform init",
        ("terraform", "init", "-backend=false", "-input=false", "-no-color"),
        cwd=configuration,
        env=environment,
    )
    schema = _command(
        "Terraform schema",
        ("terraform", "providers", "schema", "-json"),
        cwd=configuration,
        env=environment,
    )
    _verify_schema(schema)
    return schema


def generate_offline_schema(
    *, root: Path, temporary: Path, version: str, source_date_epoch: int
) -> str:
    """Build and load one checksum-selected provider entirely from a filesystem mirror."""
    _validate_identity(version, "linux", "amd64")
    goos, goarch = _platform(root)
    binary = temporary / "build" / PROVIDER_NAME
    binary.parent.mkdir(parents=True)
    _build(root, binary, version)
    package = create_package(
        binary=binary,
        license_path=root / "LICENSE",
        output_dir=temporary / "package",
        version=version,
        goos=goos,
        goarch=goarch,
        source_date_epoch=source_date_epoch,
    )
    return _offline_schema_from_package(
        root=root,
        temporary=temporary / "offline",
        package=package,
        version=version,
        goos=goos,
        goarch=goarch,
    )


def run_package_test(*, root: Path, version: str, source_date_epoch: int) -> str:
    """Build twice, compare packages, and load the checksum-selected binary offline."""
    _validate_identity(version, "linux", "amd64")
    goos, goarch = _platform(root)
    with tempfile.TemporaryDirectory(prefix="nutanix-package-") as raw_temporary:
        temporary = Path(raw_temporary)
        first_binary = temporary / "first" / PROVIDER_NAME
        second_binary = temporary / "second" / PROVIDER_NAME
        first_binary.parent.mkdir()
        second_binary.parent.mkdir()
        _build(root, first_binary, version)
        _build(root, second_binary, version)
        first = create_package(
            binary=first_binary,
            license_path=root / "LICENSE",
            output_dir=temporary / "package-first",
            version=version,
            goos=goos,
            goarch=goarch,
            source_date_epoch=source_date_epoch,
        )
        second = create_package(
            binary=second_binary,
            license_path=root / "LICENSE",
            output_dir=temporary / "package-second",
            version=version,
            goos=goos,
            goarch=goarch,
            source_date_epoch=source_date_epoch,
        )
        if first.archive.read_bytes() != second.archive.read_bytes():
            raise PackageError("repeated provider packages differ")
        schema = _offline_schema_from_package(
            root=root,
            temporary=temporary / "offline",
            package=first,
            version=version,
            goos=goos,
            goarch=goarch,
        )
        _verify_schema(schema)
        return f"package: ok ({first.sha256}, protocol 6 schema)"


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="provider-package")
    parser.add_argument("command", choices=("test",))
    parser.add_argument("--version", default=DEFAULT_VERSION)
    return parser


def main(
    arguments: Sequence[str] | None = None,
    *,
    root: Path | None = None,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Run the deterministic package command."""
    parsed = _parser().parse_args(arguments)
    selected_root = Path.cwd() if root is None else root
    raw_epoch = os.environ.get("SOURCE_DATE_EPOCH", "")
    try:
        epoch = int(raw_epoch)
        result = run_package_test(
            root=selected_root,
            version=parsed.version,
            source_date_epoch=epoch,
        )
    except (PackageError, OSError, ValueError) as error:
        print(f"package: {error}", file=stderr)
        return 1
    print(result, file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
