# Test Plan #4: pattern matches nothing — "No task found matching:",
# non-zero exit, files untouched.

seed_basic
todo_before=$(fixture_read_todo)
today_before=$(fixture_read_today)

fixture_run done "Nonexistent Zeta Task"

assert_eq "$RUN_EXIT" "1" "no-match refuses with non-zero exit"
assert_contains "$RUN_STDOUT$RUN_STDERR" "No task found matching:" "refusal message present"
assert_eq "$(fixture_read_todo)" "$todo_before" "todo.txt untouched"
assert_eq "$(fixture_read_today)" "$today_before" "today.md untouched"
