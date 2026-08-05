"""Validate repository-local links in tracked Markdown documents."""

from __future__ import annotations

import re
import sys
import unicodedata
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import TextIO
from urllib.parse import unquote, urlsplit

from scripts.automation.process import CommandError, run

Runner = Callable[[Sequence[str], Path], str]

_INLINE_LINK = re.compile(r"!?\[[^\]\n]*\]\(([^)\n]+)\)")
_REFERENCE_DEFINITION = re.compile(r"^\s{0,3}\[[^\]\n]+\]:\s*(\S+)", re.MULTILINE)
_HEADING = re.compile(r"^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$", re.MULTILINE)
_HTML_ANCHOR = re.compile(r"<(?:a|span)\s+[^>]*(?:id|name)=[\"']([^\"']+)[\"'][^>]*>", re.I)
_MARKDOWN_LINK_TEXT = re.compile(r"!?\[([^\]]+)\]\([^)]*\)")
_HTML_TAG = re.compile(r"<[^>]+>")
_CODE_SPAN = re.compile(r"`+([^`]*)`+")
_WHITESPACE = re.compile(r"\s+")
_EXTERNAL_SCHEMES = frozenset({"data", "http", "https", "mailto", "tel"})


class DocsLinkError(RuntimeError):
    """DocsLinkError reports an unavailable tracked-file inventory."""


def _link_target(raw: str) -> str:
    value = raw.strip()
    if value.startswith("<") and ">" in value:
        return value[1 : value.index(">")]
    return value.split(maxsplit=1)[0] if value else ""


def _github_slug(value: str) -> str:
    text = _CODE_SPAN.sub(r"\1", value)
    text = _MARKDOWN_LINK_TEXT.sub(r"\1", text)
    text = _HTML_TAG.sub("", text)
    normalized = unicodedata.normalize("NFKC", text).casefold()
    kept = "".join(
        character
        for character in normalized
        if character in {"-", "_"}
        or character.isspace()
        or unicodedata.category(character)[0] in {"L", "N"}
    )
    return _WHITESPACE.sub("-", kept.strip())


def _anchors(path: Path) -> set[str]:
    text = path.read_text()
    anchors = set(_HTML_ANCHOR.findall(text))
    occurrences: dict[str, int] = {}
    for heading in _HEADING.findall(text):
        base = _github_slug(heading)
        if not base:
            continue
        count = occurrences.get(base, 0)
        occurrences[base] = count + 1
        anchors.add(base if count == 0 else f"{base}-{count}")
    return anchors


def _targets(text: str) -> tuple[str, ...]:
    inline = (_link_target(raw) for raw in _INLINE_LINK.findall(text))
    references = (_link_target(raw) for raw in _REFERENCE_DEFINITION.findall(text))
    return tuple(target for target in (*inline, *references) if target)


def validate(root: Path, tracked_paths: Sequence[str]) -> list[str]:
    """Return deterministic diagnostics for repository-local Markdown links."""
    repository = root.resolve()
    diagnostics: list[str] = []
    anchor_cache: dict[Path, set[str]] = {}
    for relative in sorted(set(tracked_paths)):
        source = repository / relative
        if source.suffix.lower() not in {".md", ".markdown"} or not source.is_file():
            continue
        for target in _targets(source.read_text()):
            parsed = urlsplit(target)
            if parsed.scheme.casefold() in _EXTERNAL_SCHEMES or target.startswith("//"):
                continue
            if parsed.scheme or parsed.netloc:
                diagnostics.append(f"{relative}: unsupported local link target: {target}")
                continue
            decoded_path = unquote(parsed.path)
            if decoded_path.startswith("/"):
                candidate = repository / decoded_path.lstrip("/")
            elif decoded_path:
                candidate = source.parent / decoded_path
            else:
                candidate = source
            resolved = candidate.resolve()
            try:
                resolved.relative_to(repository)
            except ValueError:
                diagnostics.append(f"{relative}: local link escapes repository: {target}")
                continue
            if not resolved.exists():
                diagnostics.append(f"{relative}: missing local link target: {target}")
                continue
            fragment = unquote(parsed.fragment)
            if not fragment or not resolved.is_file() or resolved.suffix.lower() != ".md":
                continue
            anchors = anchor_cache.setdefault(resolved, _anchors(resolved))
            if fragment not in anchors:
                diagnostics.append(f"{relative}: missing local link fragment: {target}")
    return sorted(set(diagnostics))


def _git_ls_markdown(arguments: Sequence[str], root: Path) -> str:
    try:
        return run(arguments, cwd=root, timeout=15.0).stdout
    except (CommandError, OSError) as error:
        raise DocsLinkError("cannot inspect tracked Markdown files") from error


def main(
    *,
    root: Path | None = None,
    runner: Runner = _git_ls_markdown,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    """Validate internal links for every tracked Markdown file."""
    selected_root = Path.cwd() if root is None else root
    try:
        payload = runner(("git", "ls-files", "-z", "--", "*.md", "*.markdown"), selected_root)
    except DocsLinkError as error:
        print(f"docs-links: {error}", file=stderr)
        return 1
    tracked = tuple(path for path in payload.split("\0") if path)
    try:
        diagnostics = validate(selected_root, tracked)
    except (OSError, UnicodeError) as error:
        print(f"docs-links: cannot read Markdown inputs: {error}", file=stderr)
        return 1
    if diagnostics:
        for diagnostic in diagnostics:
            print(f"docs-links: {diagnostic}", file=stderr)
        return 1
    print(f"docs-links: ok ({len(tracked)} tracked Markdown files)", file=stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
