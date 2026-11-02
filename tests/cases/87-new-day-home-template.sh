#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# `daisy new-day` and `new-week` take their template from the home's
# templates/ before the root's: a home template differing from the root's by
# one line gives a today.md carrying the home's line, not the root's. With
# neither, the refusal and the --healthcheck hook name both places looked.

seed_basic
home="$(fixture_home_dir)"
mkdir -p "$home/templates"
sed 's/^- \[ \] Check calendar for upcoming events$/- [ ] Check calendar for upcoming events (Fixture calendar)/' \
    "$FIXTURE_ROOT/templates/journal-day.md" > "$home/templates/journal-day.md"
assert_eq "$(diff "$FIXTURE_ROOT/templates/journal-day.md" "$home/templates/journal-day.md" | grep -c '^[<>]')" "2" "the home's template differs by one line"

fixture_run_cli new-day
assert_eq "$RUN_EXIT" "0" "new-day exits 0"
today=$(fixture_read_today)
assert_contains "$today" "- [ ] Check calendar for upcoming events (Fixture calendar)" "today.md carries the home's line"
assert_eq "$(grep -c '^- \[ \] Check calendar' <<<"$today")" "1" "and no other calendar line"

# The refusals, with the run-time health check passed so the hook's own
# message is the one seen whatever stamp an earlier run left.
export DAISY_HEALTHCHECK_PASSED=1
rm "$FIXTURE_ROOT/templates/journal-day.md" "$home/templates/journal-day.md"
fixture_run_cli new-day
assert_eq "$RUN_EXIT" "1" "new-day with neither template exits 1"
assert_contains "$RUN_STDERR" "Error: Template missing: journal-day.md" "names the template"
assert_contains "$RUN_STDERR" "  Looked for: $home/templates/journal-day.md, then $FIXTURE_ROOT/templates/journal-day.md" "and both places looked, the home's first"
fixture_run_cli new-day --healthcheck
assert_eq "$RUN_EXIT" "1" "the health hook fails with neither"
assert_contains "$RUN_STDERR" "  Looked for: $home/templates/journal-day.md, then $FIXTURE_ROOT/templates/journal-day.md" "with the same two paths"
rm "$FIXTURE_ROOT/templates/journal-week.md"
fixture_run_cli new-week
assert_eq "$RUN_EXIT" "1" "new-week with neither template exits 1"
assert_contains "$RUN_STDERR" "  Looked for: $home/templates/journal-week.md, then $FIXTURE_ROOT/templates/journal-week.md" "new-week names both places too"
unset DAISY_HEALTHCHECK_PASSED
