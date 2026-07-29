# Test Plan #6: pattern matches one active + one completed — resolves to
# the active one; completes it.

fixture_write_todo <<'EOF'
x 2026-07-01 2026-06-01 Refactor gizmo old pass +daisy
(B) 2026-07-27 Refactor gizmo +daisy
EOF
seed_today_basic

fixture_run done "Refactor gizmo"

assert_eq "$RUN_EXIT" "0" "resolves the sole active match despite an existing completed line matching too"
today=$(date +%Y-%m-%d)
assert_contains "$(fixture_read_todo)" "x $today 2026-07-27 Refactor gizmo +daisy" "active task completed with today's date"
assert_contains "$(fixture_read_todo)" "x 2026-07-01 2026-06-01 Refactor gizmo old pass +daisy" "pre-existing completed line untouched"
