#!/usr/bin/env bash
# Invocation: run as `daisy test` — do not execute this file directly.
# Run the hermetic daisy/scripts test suite.
# Usage: test.sh [name-substring]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    if [ ! -x "$DAISY_ROOT/daisy/tests/run.sh" ]; then
        echo "Error: test runner not found at daisy/tests/run.sh" >&2
        exit 1
    fi
    exit 0
fi

require_root || exit 1

exec "$DAISY_ROOT/daisy/tests/run.sh" "$@"
