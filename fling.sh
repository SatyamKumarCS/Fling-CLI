#!/usr/bin/env bash
# ==============================================================================
# Fling CLI - Direct Shell Launcher
# Runs Fling directly from the terminal, auto-compiling if needed.
# ==============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="${SCRIPT_DIR}/fling"

# Check if binary exists or if source is newer than binary
if [ ! -f "$BIN_PATH" ] || [ -n "$(find "${SCRIPT_DIR}/cmd" "${SCRIPT_DIR}/internal" -type f -newer "$BIN_PATH" 2>/dev/null | head -n 1)" ]; then
    echo "⚡ Building Fling binary..." >&2
    (cd "$SCRIPT_DIR" && go build -ldflags="-s -w" -o "$BIN_PATH" ./cmd/fling)
fi

# Execute Fling passing all arguments
exec "$BIN_PATH" "$@"
