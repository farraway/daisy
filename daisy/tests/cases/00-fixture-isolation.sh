# Acceptance gate for the harness itself: DAISY_HOME must resolve inside the
# fixture, never a real home under the real DAISY_ROOT, and daisy healthcheck
# must pass against the fixture alone.

fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "daisy healthcheck exits 0 inside the fixture"

resolved_home=$(cd "$FIXTURE_WORKSPACE" && source "$DAISY_ROOT/daisy/scripts/common.sh" && resolve_home >/dev/null 2>&1 && echo "$DAISY_HOME")
assert_eq "$resolved_home" "$FIXTURE_ROOT/home/testhome" "DAISY_HOME resolves inside the fixture root"

case "$resolved_home" in
    "$REAL_DAISY_ROOT"/home/*)
        echo "    ASSERT FAILED: resolved home leaked to a real home under $REAL_DAISY_ROOT/home"
        ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
        ;;
esac
