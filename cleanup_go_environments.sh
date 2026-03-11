#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Syncing Go workspace ==="
cd "$ROOT_DIR"
go work sync

echo "=== Tidying Go modules ==="
for mod in $(find "$ROOT_DIR" -name go.mod -not -path '*/vendor/*'); do
    dir=$(dirname "$mod")
    rel=$(realpath --relative-to="$ROOT_DIR" "$dir" 2>/dev/null || python3 -c "import os.path; print(os.path.relpath('$dir', '$ROOT_DIR'))")
    echo "  -> $rel"
    (cd "$dir" && go mod tidy)
done

echo "=== Clearing Go build & test cache ==="
go clean -cache -testcache

echo "=== Done ==="
