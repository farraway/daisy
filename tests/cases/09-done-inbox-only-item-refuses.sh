# Test Plan #9: Inbox-only item ("workout") — refuses: not in todo.txt.
# today.md untouched.

seed_basic
today_before=$(fixture_read_today)

fixture_run_cli done "Workout"

assert_eq "$RUN_EXIT" "1" "an Inbox-only item is refused, not guessed at"
assert_contains "$RUN_STDERR" "No task found matching: Workout" "the refusal says no task in todo.txt matches"
assert_eq "$(fixture_read_today)" "$today_before" "today.md untouched — Inbox items are checked off manually, not via daisy done"
