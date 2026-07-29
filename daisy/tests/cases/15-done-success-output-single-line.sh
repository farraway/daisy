# Test Plan #15: success output — one summary line on stdout carrying task +
# commit hash; no "✅ Logged:", no "📝 Committed:", no "No changes to commit".

seed_basic

fixture_run done "Delta target"

assert_eq "$RUN_EXIT" "0" "resolves and completes"
stdout_lines=$(printf '%s\n' "$RUN_STDOUT" | grep -c .)
assert_eq "$stdout_lines" "1" "stdout carries exactly one non-blank line"
assert_contains "$RUN_STDOUT" "Delta target task" "summary line names the completed task"
assert_not_contains "$RUN_STDOUT" "Logged:" "sub-call chatter (log) suppressed on success"
assert_not_contains "$RUN_STDOUT" "Committed:" "sub-call chatter (commit) suppressed on success"
assert_not_contains "$RUN_STDOUT" "No changes to commit" "trailing no-op commit chatter suppressed on success"
