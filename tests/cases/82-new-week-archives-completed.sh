#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# `daisy new-week` is new-day with the week template, after moving
# todo.txt's completed lines to the end of done.txt. One commit.

seed_today_basic
seed_done_basic
fixture_write_todo <<'EOF2'
(A) 2026-07-27 Ship the release +daisy
x 2026-07-28 2026-07-02 Finished one
x 2026-07-28 2026-07-03 Finished two +daisy
EOF2
before=$(fixture_commit_count)

fixture_run_cli new-week
assert_eq "$RUN_EXIT" "0" "new-week exits 0"
assert_contains "$RUN_STDOUT" "📦 Archived 2 completed task(s) to done.txt" "reports the completed tasks"
assert_contains "$RUN_STDOUT" "✅ New week started: " "reports the new week"
assert_eq "$(fixture_commit_count)" "$((before + 1))" "one commit in the home"
assert_eq "$(fixture_read_done | tail -2)" "x 2026-07-28 2026-07-02 Finished one
x 2026-07-28 2026-07-03 Finished two +daisy" "the completed lines end done.txt"
assert_eq "$(fixture_read_done | head -1)" "x 2026-07-01 2026-06-01 Ship the old widget +daisy" "done.txt's history is kept"
assert_eq "$(fixture_read_todo)" "(A) 2026-07-27 Ship the release +daisy" "todo.txt keeps only the open task"
today=$(fixture_read_today)
assert_eq "$(head -1 <<<"$today")" "#### Weekly Retrospective" "today.md is the week template"
assert_contains "$today" "#### Resolutions" "with its Resolutions section"
assert_contains "$today" "- [ ] Ship the release +daisy" "and the open task carried"
