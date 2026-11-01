# Test Plan #18: `done.sh --healthcheck` — exits 0 with files present, 1
# without.

fixture_run done --healthcheck
assert_eq "$RUN_EXIT" "0" "healthcheck passes with today.md and todo.txt present"

rm -f "$(fixture_home_dir)/journal/today.md" "$(fixture_home_dir)/tasks/todo.txt"

fixture_run done --healthcheck
assert_eq "$RUN_EXIT" "1" "healthcheck fails once today.md/todo.txt are gone"
