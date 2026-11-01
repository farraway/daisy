# Test Plan #12: today.md structure — no blank line added or removed
# anywhere. A new (non-blank) Log entry line is expected from `daisy log`;
# what must not change is the blank-line skeleton around it.

seed_basic
before_blanks=$(fixture_read_today | grep -c '^$')
before_total=$(fixture_read_today | wc -l)

fixture_run done "Delta target"

after_blanks=$(fixture_read_today | grep -c '^$')
after_total=$(fixture_read_today | wc -l)

assert_eq "$RUN_EXIT" "0" "resolves and completes"
assert_eq "$after_blanks" "$before_blanks" "blank-line count unchanged — no blank line inserted or removed"
assert_eq "$after_total" "$((before_total + 1))" "exactly one new (non-blank) line — the log entry — added"
