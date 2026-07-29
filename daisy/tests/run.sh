#!/usr/bin/env bash
# Invocation: run as `daisy test` — do not execute this file directly.
# Hermetic test runner for daisy/scripts. Each case in daisy/tests/cases/
# gets its own fixture (lib/fixture.sh) — a throwaway DAISY_ROOT and
# workspace — so a failing case can never touch a real home.
# Usage: run.sh [name-substring]

set -u

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$TESTS_DIR/lib/fixture.sh"
source "$TESTS_DIR/lib/assert.sh"

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
