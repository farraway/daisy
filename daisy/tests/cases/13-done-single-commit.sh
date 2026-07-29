# Test Plan #13: commit count for one completion — exactly one commit
# produced, containing todo.txt + today.md together.

seed_basic
commits_before=$(fixture_commit_count)

fixture_run done "Delta target"

assert_eq "$RUN_EXIT" "0" "resolves and completes"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "exactly one commit for the whole completion"

changed_files=$(git -C "$FIXTURE_ROOT" show --stat -1 --name-only)
assert_contains "$changed_files" "tasks/todo.txt" "commit includes todo.txt"
assert_contains "$changed_files" "journal/today.md" "commit includes today.md"
