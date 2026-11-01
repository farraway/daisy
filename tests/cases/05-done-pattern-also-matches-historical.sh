# Test Plan #5: pattern also matches a historical `x` line — that line
# survives verbatim; only the active task completes.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Ship widget +daisy
x 2026-07-01 2026-06-01 Ship widget old release +daisy
EOF
seed_today_basic

fixture_run done "Ship widget"

assert_eq "$RUN_EXIT" "0" "resolves the sole active match"
assert_contains "$(fixture_read_todo)" "x 2026-07-01 2026-06-01 Ship widget old release +daisy" "historical x line survives verbatim"
assert_contains "$(fixture_read_todo)" "2026-07-27 Ship widget +daisy" "active task completed"
