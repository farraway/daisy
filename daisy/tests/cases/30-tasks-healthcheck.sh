# Test Plan #30: `tasks.sh --healthcheck` — exits 0 with tasks dir present,
# 1 without.

fixture_run tasks --healthcheck
assert_eq "$RUN_EXIT" "0" "healthcheck passes with tasks/ present"

rm -rf "$(fixture_home_dir)/tasks"

fixture_run tasks --healthcheck
assert_eq "$RUN_EXIT" "1" "healthcheck fails once tasks/ is gone"
