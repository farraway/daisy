#!/usr/bin/env bash
# Invocation: run as `daisy test` — do not execute this file directly.
# Hermetic acceptance suite for the `daisy` CLI. Each case in tests/cases/
# gets its own fixture (lib/fixture.sh) — a throwaway DAISY_ROOT and
# workspace — so a failing case can never touch a real home.
# Usage: run.sh [name-substring]

set -u

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$TESTS_DIR/lib/fixture.sh"
source "$TESTS_DIR/lib/assert.sh"

# The cases drive `daisy` on PATH, so it must be this root's binary: a stale
# ~/bin/daisy left pointing at another checkout, or at daisy.sh, would run
# that code against these cases and say nothing. `daisy test` builds the
# binary and puts it first on PATH before running this.
DAISY_ON_PATH=$(readlink -f "$(command -v daisy)" 2>/dev/null) || DAISY_ON_PATH=""
if [ "$DAISY_ON_PATH" != "$REAL_DAISY_ROOT/bin/daisy" ]; then
    echo "Error: 'daisy' on PATH is ${DAISY_ON_PATH:-not found}, not $REAL_DAISY_ROOT/bin/daisy" >&2
    echo "  Run 'daisy test', which builds the binary and puts it on PATH for the suite." >&2
    exit 1
fi

FILTER="${1:-}"

PASS=0
FAIL=0
FAILED_NAMES=()

for case_file in "$TESTS_DIR"/cases/*.sh; do
    [ -f "$case_file" ] || continue
    name=$(basename "$case_file" .sh)
    if [ -n "$FILTER" ] && [[ "$name" != *"$FILTER"* ]]; then
        continue
    fi

    fixture_setup
    ASSERT_FAILURES=0
    # shellcheck disable=SC1090
    source "$case_file"
    fixture_teardown

    if [ "$ASSERT_FAILURES" -eq 0 ]; then
        echo "PASS  $name"
        PASS=$((PASS + 1))
    else
        echo "FAIL  $name  ($ASSERT_FAILURES assertion(s))"
        FAIL=$((FAIL + 1))
        FAILED_NAMES+=("$name")
    fi
done

echo ""
echo "$PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
    echo "Failed: ${FAILED_NAMES[*]}"
    exit 1
fi
exit 0
