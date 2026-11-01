# Test Plan #10: completed-line format — `x YYYY-MM-DD YYYY-MM-DD
# Description`, priority stripped, tags intact, last line of todo.txt.

seed_basic

fixture_run done "Delta target"

assert_eq "$RUN_EXIT" "0" "resolves and completes"
today=$(date +%Y-%m-%d)
last_line=$(fixture_read_todo | tail -1)
assert_eq "$last_line" "x $today 2026-07-27 Delta target task +daisy" "completed line has no priority, has both dates and the +tag, and sits last"
