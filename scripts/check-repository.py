#!/usr/bin/env python3
"""Validate repository policy files, JSON documents, and local Markdown links."""

from __future__ import annotations

import json
import pathlib
import re
import sys
import urllib.parse

ROOT = pathlib.Path(__file__).resolve().parent.parent
EXPECTED_MODULE = "github.com/Sidiora-Labs/machine-genome"
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
    "Dockerfile",
    ".goreleaser.yaml",
    "api/openapi.json",
    "docs/README.md",
    "docs/architecture.md",
    "docs/threat-model.md",
    "docs/conformance.md",
    "docs/registry-policy.md",
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


def values(value: object):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from values(child)
    elif isinstance(value, list):
        for child in value:
            yield from values(child)


def resolve_pointer(document: object, pointer: str) -> None:
    current = document
    if not pointer:
        return
    if not pointer.startswith("/"):
        raise ValueError(f"invalid JSON Pointer #{pointer}")
    for encoded in pointer[1:].split("/"):
        token = encoded.replace("~1", "/").replace("~0", "~")
        if isinstance(current, dict):
            current = current[token]
        elif isinstance(current, list):
            current = current[int(token)]
        else:
            raise KeyError(token)


def main() -> int:
    failures: list[str] = []
    for relative in REQUIRED:
        if not (ROOT / relative).is_file():
            failures.append(f"missing required file: {relative}")

    try:
        module_line = (ROOT / "go.mod").read_text(encoding="utf-8").splitlines()[0]
    except (OSError, UnicodeError, IndexError) as error:
        failures.append(f"go.mod: unable to read module path: {error}")
    else:
        if module_line != f"module {EXPECTED_MODULE}":
            failures.append(
                f"go.mod: expected module {EXPECTED_MODULE!r}, found {module_line!r}"
            )

    documents: dict[pathlib.Path, object] = {}
    for path in sorted(ROOT.rglob("*.json")):
        if any(part in {".git", "dist"} for part in path.parts):
            continue
        try:
            documents[path.resolve()] = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, json.JSONDecodeError) as error:
            failures.append(f"{path.relative_to(ROOT)}: invalid JSON: {error}")

    for path, document in documents.items():
        for value in values(document):
            reference = value.get("$ref")
            if not isinstance(reference, str):
                continue
            file_part, separator, fragment = reference.partition("#")
            target_path = path if not file_part else (path.parent / file_part).resolve()
            target = documents.get(target_path)
            if target is None:
                failures.append(
                    f"{path.relative_to(ROOT)}: unresolved JSON reference {reference!r}"
                )
                continue
            if separator:
                try:
                    resolve_pointer(target, fragment)
                except (KeyError, IndexError, ValueError) as error:
                    failures.append(
                        f"{path.relative_to(ROOT)}: invalid JSON reference "
                        f"{reference!r}: {error}"
                    )

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
