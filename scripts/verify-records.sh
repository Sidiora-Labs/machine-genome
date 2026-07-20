#!/bin/sh
set -eu

binary=${1:-bin/mgs}
manifest=records/neo/registry-genesis.json
count=0

jq -e '.format == "mgs-registry-genesis-v1"' "$manifest" >/dev/null

for record in records/neo/*.json; do
    [ "$record" = "$manifest" ] && continue
    "$binary" verify --in "$record" >/dev/null
    gene=$("$binary" gene --in "$record")
    if ! jq -e --arg gene "$gene" 'any(.records[]; .gene == $gene)' "$manifest" >/dev/null; then
        echo "$record: Gene $gene is missing from $manifest" >&2
        exit 1
    fi
    count=$((count + 1))
done

expected=$(jq '.records | length' "$manifest")
if [ "$count" -ne "$expected" ]; then
    echo "verified $count records, manifest declares $expected" >&2
    exit 1
fi

echo "verified $count signed genesis records and manifest Genes"
