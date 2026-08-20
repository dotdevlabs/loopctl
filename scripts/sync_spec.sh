#!/usr/bin/env bash
# sync_spec.sh — Download the authoritative OpenAPI spec from dotdevlabs/loopcontrol.
# With --check: exit non-zero if the local copy differs from the live source.
# Requires GITHUB_TOKEN environment variable; silently exits 0 when not set.
set -euo pipefail

DEST="internal/schema/testdata/api_spec.yaml"
URL="https://api.github.com/repos/dotdevlabs/loopcontrol/contents/docs/api_spec.yaml"
CHECK_ONLY="${1:-}"

TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$TOKEN" ]; then
    echo "GITHUB_TOKEN is not set; skipping spec sync" >&2
    exit 0
fi

TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

curl -fsSL \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/vnd.github.raw+json" \
    "$URL" -o "$TMPFILE"

if [ "$CHECK_ONLY" = "--check" ]; then
    if ! diff -q "$DEST" "$TMPFILE" > /dev/null 2>&1; then
        echo "ERROR: ${DEST} is out of sync with the published spec." >&2
        echo "Run: GITHUB_TOKEN=<token> ./scripts/sync_spec.sh" >&2
        exit 1
    fi
    echo "Spec is in sync."
else
    cp "$TMPFILE" "$DEST"
    echo "Updated ${DEST}"
fi
