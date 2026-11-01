# Test Plan #3: pattern matches 2 active tasks — candidates listed, non-zero
# exit, all files untouched.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Shared token Alpha +daisy
(B) 2026-07-27 Shared token Beta +daisy
EOF
seed_today_basic

todo_before=$(fixture_read_todo)
today_before=$(fixture_read_today)

fixture_run done "Shared token"

assert_eq "$RUN_EXIT" "1" "multi-match refuses with non-zero exit"
assert_eq "$(fixture_read_todo)" "$todo_before" "todo.txt untouched on multi-match"
assert_eq "$(fixture_read_today)" "$today_before" "today.md untouched on multi-match"
assert_contains "$RUN_STDOUT$RUN_STDERR" "Shared token Alpha" "candidate 1 listed"
assert_contains "$RUN_STDOUT$RUN_STDERR" "Shared token Beta" "candidate 2 listed"
