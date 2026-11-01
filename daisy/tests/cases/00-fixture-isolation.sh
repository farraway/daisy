# Acceptance gate for the harness itself: DAISY_HOME must resolve to the
# fixture's registered home — outside the fixture root, never a real home —
# and daisy healthcheck must pass against the fixture alone.

fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "daisy healthcheck exits 0 inside the fixture"

resolved_home=$(cd "$FIXTURE_WORKSPACE" && source "$DAISY_ROOT/daisy/scripts/common.sh" && resolve_home >/dev/null 2>&1 && echo "$DAISY_HOME")
assert_eq "$resolved_home" "$FIXTURE_HOME" "DAISY_HOME resolves to the registered fixture home"

case "$resolved_home" in
    "$REAL_DAISY_ROOT"/*)
        echo "    ASSERT FAILED: resolved home leaked under the real DAISY_ROOT: $resolved_home"
        ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
        ;;
    "$FIXTURE_ROOT"/*)
        echo "    ASSERT FAILED: the fixture home lives under the fixture root — nothing may derive a home from DAISY_ROOT"
        ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
        ;;
esac
