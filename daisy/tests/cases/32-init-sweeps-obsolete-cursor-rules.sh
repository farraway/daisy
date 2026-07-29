# daisy init must remove pre-.mdc-rename Cursor rule files (daisy.md,
# daisy-logging.md) — daisy-init.sh used to only ever write the current
# DAISY_CURSOR_RULE_FILES names and never clean up names that left that list
# (defect 6c, deferred item 5 from plan 260714).

mkdir -p "$FIXTURE_WORKSPACE/.cursor/rules"
echo "stale pre-rename rule" > "$FIXTURE_WORKSPACE/.cursor/rules/daisy.md"
echo "stale pre-rename rule" > "$FIXTURE_WORKSPACE/.cursor/rules/daisy-logging.md"

fixture_run_cli init testhome
assert_eq "$RUN_EXIT" "0" "daisy init exits 0"

if [ -f "$FIXTURE_WORKSPACE/.cursor/rules/daisy.md" ]; then
    echo "    ASSERT FAILED: obsolete daisy.md survived daisy init"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi
if [ -f "$FIXTURE_WORKSPACE/.cursor/rules/daisy-logging.md" ]; then
    echo "    ASSERT FAILED: obsolete daisy-logging.md survived daisy init"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi

# Current .mdc rules must still be installed.
if [ ! -f "$FIXTURE_WORKSPACE/.cursor/rules/daisy.mdc" ]; then
    echo "    ASSERT FAILED: current daisy.mdc rule was not installed"
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
fi
