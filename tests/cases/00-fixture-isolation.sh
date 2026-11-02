# Acceptance gate for the harness itself: the home must resolve to the
# fixture's registered home — outside the fixture root, never a real home —
# and daisy healthcheck must pass against the fixture alone.

fixture_run_cli healthcheck
assert_eq "$RUN_EXIT" "0" "daisy healthcheck exits 0 inside the fixture"

# `daisy files` prints the resolved real path of every home file; the home
# is the one todo.txt resolves into.
fixture_run_cli files
assert_eq "$RUN_EXIT" "0" "daisy files exits 0 inside the fixture"
resolved_home=$(printf '%s\n' "$RUN_STDOUT" | awk '$1 == "todo.txt" { sub(/\/tasks\/todo\.txt$/, "", $2); print $2 }')
assert_eq "$resolved_home" "$(readlink -f "$FIXTURE_HOME")" "the home resolves to the registered fixture home"

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

# fixture_run_cli reaches the binary's cli: `--help` as the sole argument is
# answered from the command table (exit 0, the usage) and never taken for a
# task pattern.
fixture_run_cli done --help
assert_eq "$RUN_EXIT" "0" "fixture_run_cli goes through the daisy dispatcher"
assert_contains "$RUN_STDOUT" "Mark a task" "and the command's help was printed"
