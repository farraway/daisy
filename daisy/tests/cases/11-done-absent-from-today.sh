# Test Plan #11: task in todo.txt, absent from today.md — todo.txt
# completes; warning; log + commit still fire.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Ghost task not in journal +daisy
EOF
seed_today_basic

commits_before=$(fixture_commit_count)

fixture_run done "Ghost task"

assert_eq "$RUN_EXIT" "0" "todo.txt-only completion still succeeds"
today=$(date +%Y-%m-%d)
assert_contains "$(fixture_read_todo)" "x $today 2026-07-27 Ghost task not in journal +daisy" "todo.txt completed"
assert_contains "$RUN_STDOUT$RUN_STDERR" "today.md" "a warning mentions today.md was not updated"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "log + commit still fire despite the today.md miss"
