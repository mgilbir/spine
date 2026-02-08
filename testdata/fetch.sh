#!/usr/bin/env bash
#
# Download external test fixtures listed in testdata/external.txt.
# Usage: bash testdata/fetch.sh [--force]
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG="$SCRIPT_DIR/external.txt"

FORCE=false
if [[ "${1:-}" == "--force" ]]; then
    FORCE=true
fi

if [[ ! -f "$CONFIG" ]]; then
    echo "ERROR: config file not found: $CONFIG" >&2
    exit 1
fi

ok=0
skipped=0
failed=0
total=0

while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip comments and blank lines
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue

    # Split on tab: DEST_PATH<tab>URL
    dest="${line%%	*}"
    url="${line#*	}"

    # Trim whitespace
    dest="$(echo "$dest" | xargs)"
    url="$(echo "$url" | xargs)"

    if [[ -z "$dest" || -z "$url" || "$dest" == "$url" ]]; then
        echo "WARN: skipping malformed line: $line" >&2
        continue
    fi

    total=$((total + 1))
    full_path="$REPO_ROOT/$dest"

    if [[ -f "$full_path" && "$FORCE" == false ]]; then
        skipped=$((skipped + 1))
        echo "SKIP: $dest (already exists)"
        continue
    fi

    mkdir -p "$(dirname "$full_path")"
    echo -n "GET:  $dest ... "
    if curl -fSL --retry 2 --retry-delay 3 -o "$full_path" "$url" 2>/dev/null; then
        ok=$((ok + 1))
        echo "OK"
    else
        failed=$((failed + 1))
        rm -f "$full_path"
        echo "FAILED"
    fi
done < "$CONFIG"

echo ""
echo "Done: $ok downloaded, $skipped skipped, $failed failed (of $total entries)"

if [[ $failed -gt 0 ]]; then
    exit 1
fi
