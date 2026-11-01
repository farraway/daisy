#!/usr/bin/env bash
# Invocation: run as `daisy test` — do not execute this file directly.
# Run the hermetic acceptance suite.
# Usage: test.sh [name-substring]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Health check mode
if [ "$1" = "--healthcheck" ]; then
    require_root || exit 1
    if [ ! -x "$DAISY_ROOT/tests/run.sh" ]; then
        echo "Error: test runner not found at tests/run.sh" >&2
        exit 1
    fi
    exit 0
fi

require_root || exit 1

# The cases drive `daisy` on PATH, which is the binary at bin/daisy: build it
# from the current source first, or the suite would test a stale build.
make -C "$DAISY_ROOT" --silent build || {
    echo "Error: could not build bin/daisy in $DAISY_ROOT (see above)" >&2
    exit 1
}
export PATH="$DAISY_ROOT/bin:$PATH"

exec "$DAISY_ROOT/tests/run.sh" "$@"
