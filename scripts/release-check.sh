#!/bin/sh
set -eu

version=${1:?release version is required}

top_level=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [ "$top_level" != "$PWD" ]; then
    echo "release check must run from the root of the standalone repository" >&2
    exit 1
fi

case "$version" in
    *[!0-9.]*|.*|*.|*..*)
        echo "version must be numeric MAJOR.MINOR.PATCH without a v prefix" >&2
        exit 1
        ;;
esac

parts=$(printf '%s' "$version" | awk -F. '{print NF}')
if [ "$parts" -ne 3 ]; then
    echo "version must contain exactly three numeric components" >&2
    exit 1
fi

protocol=$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' mgs/record.go)
if [ "$protocol" != "$version" ]; then
    echo "mgs.Version is $protocol, expected $version" >&2
    exit 1
fi

grep -F "version: $version" CITATION.cff >/dev/null
grep -F "## [$version]" CHANGELOG.md >/dev/null

if [ -n "$(git status --porcelain)" ]; then
    echo "release check requires a clean worktree" >&2
    exit 1
fi

make check
make race
make dist
echo "release v$version is ready for tagging"
