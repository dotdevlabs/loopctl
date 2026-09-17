#!/usr/bin/env bash
# sync_spec.sh — Download the authoritative OpenAPI spec from dotdevlabs/loopcontrol.
# With --check: exit non-zero if the local copy differs from the live source.
# Requires GITHUB_TOKEN environment variable; silently exits 0 when not set.
set -euo pipefail

DEST="internal/schema/testdata/api_spec.yaml"
URL="https://api.github.com/repos/dotdevlabs/loopcontrol/contents/docs/api_spec.yaml"
COMMITS_URL="https://api.github.com/repos/dotdevlabs/loopcontrol/commits?path=docs/api_spec.yaml&per_page=1"
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

# Fetch the commit SHA of the last change to docs/api_spec.yaml.
LIVE_SHA=$(curl -fsSL \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "$COMMITS_URL" | grep '"sha"' | head -1 | awk -F'"' '{print $4}')

if [ "$CHECK_ONLY" = "--check" ]; then
    if ! diff -q "$DEST" "$TMPFILE" > /dev/null 2>&1; then
        echo "ERROR: ${DEST} is out of sync with the published spec." >&2
        echo "Run: GITHUB_TOKEN=<token> ./scripts/sync_spec.sh" >&2
        exit 1
    fi
    STORED_SHA="${DEST%.yaml}.sha"
    if [ -f "$STORED_SHA" ] && [ "$(cat "$STORED_SHA" | tr -d '[:space:]')" != "$LIVE_SHA" ]; then
        echo "ERROR: ${STORED_SHA} is out of sync with the live SHA." >&2
        exit 1
    fi
    echo "Spec is in sync."
else
    cp "$TMPFILE" "$DEST"
    echo "Updated ${DEST}"
    SHA_DEST="${DEST%.yaml}.sha"
    printf '%s\n' "$LIVE_SHA" > "$SHA_DEST"
    echo "Updated ${SHA_DEST} (SHA: ${LIVE_SHA})"
fi
