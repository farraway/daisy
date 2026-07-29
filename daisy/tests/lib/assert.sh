#!/usr/bin/env bash
# Invocation: sourced by daisy/tests/run.sh — not executed directly.
# Minimal assertion helpers. A failed assertion prints a diagnostic and
# increments ASSERT_FAILURES rather than aborting, so one test case reports
# every violated expectation in a single run.

assert_eq() {
    local actual="$1" expected="$2" msg="$3"
    if [ "$actual" != "$expected" ]; then
        echo "    ASSERT FAILED: $msg"
        echo "      expected: $expected"
        echo "      actual:   $actual"
        ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
    fi
}

assert_contains() {
    local haystack="$1" needle="$2" msg="$3"
    case "$haystack" in
        *"$needle"*) ;;
        *)
            echo "    ASSERT FAILED: $msg"
            echo "      expected to contain: $needle"
            echo "      actual: $haystack"
            ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
            ;;
    esac
}

assert_not_contains() {
    local haystack="$1" needle="$2" msg="$3"
    case "$haystack" in
        *"$needle"*)
            echo "    ASSERT FAILED: $msg"
            echo "      expected NOT to contain: $needle"
            echo "      actual: $haystack"
            ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
            ;;
        *) ;;
    esac
}

assert_text_eq() {
    local actual="$1" expected="$2" msg="$3"
    if [ "$actual" != "$expected" ]; then
        echo "    ASSERT FAILED: $msg"
        diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") | sed 's/^/      /'
        ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
    fi
}
