#!/usr/bin/env bash
# PostToolUse hook: run go vet on edited .go files
# Warns when *_types.go is modified

set -euo pipefail

FILE_PATH=$(cat /dev/stdin | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('file_path', ''))
" 2>/dev/null || echo "")

if [ -z "$FILE_PATH" ]; then
  exit 0
fi

# Only process Go files
case "$FILE_PATH" in
  *.go) ;;
  *) exit 0 ;;
esac

# Warn on types file changes
case "$FILE_PATH" in
  *_types.go)
    echo "WARNING: *_types.go modified — remember to run 'make manifests generate' before committing." >&2
    ;;
esac

# Run go vet on the package
PKG_DIR=$(dirname "$FILE_PATH")
cd "$PKG_DIR" 2>/dev/null && go vet ./... 2>&1 || true

exit 0
