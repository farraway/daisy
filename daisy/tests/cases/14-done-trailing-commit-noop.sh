# Test Plan #14: trailing `daisy commit` after `daisy log` — no-ops, exits
# 0, produces no second commit; done still exits 0 under set -e.

seed_basic
commits_before=$(fixture_commit_count)

fixture_run done "Delta target"

assert_eq "$RUN_EXIT" "0" "done exits 0 even though its own trailing commit call finds nothing staged"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "no second commit from done's own trailing commit --home call"
