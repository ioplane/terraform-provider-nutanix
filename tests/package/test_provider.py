from __future__ import annotations

import hashlib
import re
import stat
import zipfile
from pathlib import Path

import pytest
from scripts.package import provider

VERSION = "1.2.3"
SOURCE_DATE_EPOCH = 1_700_000_000


def fake_inputs(root: Path) -> tuple[Path, Path]:
    binary = root / "terraform-provider-nutanix"
    binary.write_bytes(b"provider-binary\n")
    binary.chmod(0o755)
    license_path = root / "LICENSE"
    license_path.write_text("test license\n")
    return binary, license_path


def test_packages_are_byte_identical_with_normalized_entries(tmp_path: Path) -> None:
    binary, license_path = fake_inputs(tmp_path)
    first = provider.create_package(
        binary=binary,
        license_path=license_path,
        output_dir=tmp_path / "first",
        version=VERSION,
        goos="linux",
        goarch="amd64",
        source_date_epoch=SOURCE_DATE_EPOCH,
    )
    second = provider.create_package(
        binary=binary,
        license_path=license_path,
        output_dir=tmp_path / "second",
        version=VERSION,
        goos="linux",
        goarch="amd64",
        source_date_epoch=SOURCE_DATE_EPOCH,
    )

    assert first.archive.read_bytes() == second.archive.read_bytes()
    assert first.checksums.read_bytes() == second.checksums.read_bytes()
    assert first.sha256 == hashlib.sha256(first.archive.read_bytes()).hexdigest()
    assert first.archive.name == "terraform-provider-nutanix_1.2.3_linux_amd64.zip"

    with zipfile.ZipFile(first.archive) as archive:
        entries = archive.infolist()
        assert [entry.filename for entry in entries] == [
            "LICENSE",
            "terraform-provider-nutanix_v1.2.3",
        ]
        assert {entry.date_time for entry in entries} == {(2023, 11, 14, 22, 13, 20)}
        assert [stat.S_IMODE(entry.external_attr >> 16) for entry in entries] == [0o644, 0o755]
        assert all(entry.create_system == 3 for entry in entries)


def test_checksum_is_exact_and_selects_consumer_binary(tmp_path: Path) -> None:
    binary, license_path = fake_inputs(tmp_path)
    result = provider.create_package(
        binary=binary,
        license_path=license_path,
        output_dir=tmp_path / "output",
        version=VERSION,
        goos="linux",
        goarch="amd64",
        source_date_epoch=SOURCE_DATE_EPOCH,
    )

    expected = f"{result.sha256}  {result.archive.name}\n"
    assert result.checksums.name == "SHA256SUMS"
    assert result.checksums.read_text() == expected
    assert re.fullmatch(r"[0-9a-f]{64}  [A-Za-z0-9_.-]+\.zip\n", expected)
    assert provider.verify_checksum(result.checksums, result.archive) == result.sha256

    destination = tmp_path / "mirror"
    selected = provider.extract_provider(
        archive=result.archive,
        destination=destination,
        version=VERSION,
    )
    assert selected.name == "terraform-provider-nutanix_v1.2.3"
    assert selected.read_bytes() == binary.read_bytes()
    assert stat.S_IMODE(selected.stat().st_mode) == 0o755


@pytest.mark.parametrize(
    ("version", "goos", "goarch"),
    [
        ("../1.2.3", "linux", "amd64"),
        ("1.2.3\nforged", "linux", "amd64"),
        ("1.2", "linux", "amd64"),
        ("1.2.3-01", "linux", "amd64"),
        ("1.2.3-a..b", "linux", "amd64"),
        ("1.2.3", "../linux", "amd64"),
        ("1.2.3", "linux", "amd64/escape"),
    ],
)
def test_rejects_version_or_platform_path_injection(
    tmp_path: Path, version: str, goos: str, goarch: str
) -> None:
    binary, license_path = fake_inputs(tmp_path)

    with pytest.raises(provider.PackageError, match="unsafe"):
        provider.create_package(
            binary=binary,
            license_path=license_path,
            output_dir=tmp_path / "output",
            version=version,
            goos=goos,
            goarch=goarch,
            source_date_epoch=SOURCE_DATE_EPOCH,
        )


def test_rejects_archive_path_traversal(tmp_path: Path) -> None:
    malicious = tmp_path / "malicious.zip"
    with zipfile.ZipFile(malicious, "w") as archive:
        archive.writestr("../terraform-provider-nutanix_v1.2.3", b"bad")

    with pytest.raises(provider.PackageError, match="unexpected archive entries"):
        provider.extract_provider(
            archive=malicious,
            destination=tmp_path / "destination",
            version=VERSION,
        )
