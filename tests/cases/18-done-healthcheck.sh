# Test Plan #18: `daisy done --healthcheck` — exits 0 with files present, 1
# without.

fixture_run_cli done --healthcheck
assert_eq "$RUN_EXIT" "0" "healthcheck passes with today.md and todo.txt present"
assert_eq "$RUN_STDERR" "" "a passing healthcheck says nothing"

rm -f "$(fixture_home_dir)/journal/today.md" "$(fixture_home_dir)/tasks/todo.txt"

fixture_run_cli done --healthcheck
assert_eq "$RUN_EXIT" "1" "healthcheck fails once today.md/todo.txt are gone"
assert_contains "$RUN_STDERR" "today.md not found" "the failure names the missing file"
