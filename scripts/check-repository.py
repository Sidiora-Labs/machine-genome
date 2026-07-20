#!/usr/bin/env python3
"""Validate repository policy files, JSON documents, and local Markdown links."""

from __future__ import annotations

import json
import pathlib
import re
import sys
import urllib.parse

ROOT = pathlib.Path(__file__).resolve().parent.parent
REQUIRED = (
    "README.md",
    "LICENSE",
    "NOTICE",
    "SECURITY.md",
    "CONTRIBUTING.md",
    "GOVERNANCE.md",
    "CODE_OF_CONDUCT.md",
    "CHANGELOG.md",
    "SUPPORT.md",
    "CITATION.cff",
    "api/openapi.json",
    "docs/README.md",
    "docs/architecture.md",
    "docs/threat-model.md",
    "docs/conformance.md",
    "docs/versioning.md",
)
LINK = re.compile(r"!?\[[^\]]*]\(([^)]+)\)")


def markdown_files() -> list[pathlib.Path]:
    roots = ("README.md", "CONTRIBUTING.md", "SECURITY.md", "GOVERNANCE.md",
             "CODE_OF_CONDUCT.md", "SUPPORT.md", "CHANGELOG.md")
    files = [ROOT / item for item in roots]
    for directory in ("docs", "spec", "records", "deploy"):
        files.extend(sorted((ROOT / directory).rglob("*.md")))
    return files


def local_target(source: pathlib.Path, raw: str) -> pathlib.Path | None:
    value = raw.strip().split(maxsplit=1)[0].strip("<>")
    if value.startswith(("#", "http://", "https://", "mailto:", "data:")):
        return None
    path = urllib.parse.unquote(value.split("#", 1)[0])
    if not path:
        return None
    return (source.parent / path).resolve()


def main() -> int:
    failures: list[str] = []
    for relative in REQUIRED:
        if not (ROOT / relative).is_file():
            failures.append(f"missing required file: {relative}")

    for path in sorted(ROOT.rglob("*.json")):
        if any(part in {".git", "dist"} for part in path.parts):
            continue
        try:
            json.loads(path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, json.JSONDecodeError) as error:
            failures.append(f"{path.relative_to(ROOT)}: invalid JSON: {error}")

    for source in markdown_files():
        if not source.exists():
            continue
        text = source.read_text(encoding="utf-8")
        for match in LINK.finditer(text):
            target = local_target(source, match.group(1))
            if target is not None and not target.exists():
                failures.append(
                    f"{source.relative_to(ROOT)}: broken link {match.group(1)!r}"
                )

    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1
    print("repository policy, JSON, and local documentation links verified")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
