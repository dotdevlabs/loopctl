#!/usr/bin/env bash
# sync_spec.sh — Download the authoritative OpenAPI spec from dotdevlabs/loopcontrol.
# With --check: exit non-zero if the local copy differs from the live source.
# With --ref COMMIT: sync/check the spec and provenance at that commit.
# Requires GITHUB_TOKEN environment variable; silently exits 0 when not set.
set -euo pipefail

DEST="internal/schema/testdata/api_spec.yaml"
URL="https://api.github.com/repos/dotdevlabs/loopcontrol/contents/docs/api_spec.yaml"
COMMITS_URL="https://api.github.com/repos/dotdevlabs/loopcontrol/commits?path=docs/api_spec.yaml&per_page=1"
CHECK_ONLY=""
REF=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --check) CHECK_ONLY="--check"; shift ;;
        --ref) REF="${2:?--ref requires a commit or ref}"; shift 2 ;;
        *) echo "Usage: $0 [--check] [--ref COMMIT]" >&2; exit 2 ;;
    esac
done

TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$TOKEN" ]; then
    echo "GITHUB_TOKEN is not set; skipping spec sync" >&2
    exit 0
fi

TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

# Resolve provenance before downloading so the bytes always match the SHA.
SHA_URL="$COMMITS_URL"
if [ -n "$REF" ]; then
    SHA_URL="https://api.github.com/repos/dotdevlabs/loopcontrol/commits/${REF}"
fi
LIVE_SHA=$(curl -fsSL \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "$SHA_URL" | awk -F'"' '/"sha"/ && !found {print $4; found=1}')
if [ -z "$LIVE_SHA" ]; then
    echo "Could not resolve spec commit" >&2
    exit 1
fi
curl -fsSL \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/vnd.github.raw+json" \
    "${URL}?ref=${LIVE_SHA}" -o "$TMPFILE"

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
