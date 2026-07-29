# Test Plan #8: pattern contains `.` / `[` — treated literally, matching
# only a literal occurrence (not "any character").

fixture_write_todo <<'EOF'
(B) 2026-07-27 Release version 1.0 +daisy
(B) 2026-07-27 Release version 1X0 +daisy
EOF
seed_today_basic

fixture_run done "version 1.0"

assert_eq "$RUN_EXIT" "0" "literal '.' pattern resolves uniquely rather than multi-matching '1X0' as a regex would"
today=$(date +%Y-%m-%d)
assert_contains "$(fixture_read_todo)" "x $today 2026-07-27 Release version 1.0 +daisy" "the literal '1.0' task completed"
assert_contains "$(fixture_read_todo)" "(B) 2026-07-27 Release version 1X0 +daisy" "the unrelated '1X0' task is untouched"
