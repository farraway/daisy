# Every Daisy data mutation commits in the home's repository and never in the
# root's. With the home as its own repository, `done`, `log` and `new-day`
# each produce exactly one commit there, and the root repository's history
# does not move. (new-day runs the health check and `daisy commit` through
# PATH, so this also proves the fixture is honoured end to end.)

seed_basic
root_before=$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)
home_before=$(fixture_commit_count)

fixture_run done "Delta target"
assert_eq "$RUN_EXIT" "0" "done succeeds"
assert_eq "$(fixture_commit_count)" "$((home_before + 1))" "done: one commit in the home repository"

fixture_run log "A log line"
assert_eq "$RUN_EXIT" "0" "log succeeds"
assert_eq "$(fixture_commit_count)" "$((home_before + 2))" "log: one commit in the home repository"

fixture_run new-day
assert_eq "$RUN_EXIT" "0" "new-day succeeds"
assert_eq "$(fixture_commit_count)" "$((home_before + 3))" "new-day: one commit in the home repository"

assert_eq "$(git -C "$FIXTURE_ROOT" rev-list --count HEAD)" "$root_before" \
    "the root repository has no new commits"
assert_eq "$(git -C "$FIXTURE_ROOT" status --porcelain)" "" \
    "and nothing pending in it"
