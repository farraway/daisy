# Test Plan #7: pattern contains `/` — resolves cleanly, no sed/awk
# delimiter breakage. The today.md line must contain the same "/" text: the
# old code's bug was in the sed *address* built from today.md's own search,
# not just the todo.txt grep, so the match has to be exercised in both files.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Fix a/b path handling +daisy
EOF
fixture_write_today <<'EOF'
### 2026-07-28 Tuesday

#### Agenda
- 0900 Plan Day

#### Tasks

**Now:**
- [ ] Fix a/b path handling +daisy


**Inbox:**
- [ ] Review weekly resolutions


**GitHub PRs:**


#### Log

- 0900 New day started

#### Retrospective

* **Successes:**
* **Misses:**
* **What would a Sage do next:**
EOF

fixture_run done "a/b path"

assert_eq "$RUN_EXIT" "0" "a pattern containing / resolves without a syntax error"
assert_not_contains "$RUN_STDERR" "sed:" "no sed delimiter error"
assert_not_contains "$RUN_STDERR" "unterminated" "no unterminated expression error"
today=$(date +%Y-%m-%d)
assert_contains "$(fixture_read_todo)" "x $today 2026-07-27 Fix a/b path handling +daisy" "task completed"
assert_contains "$(fixture_read_today)" "- [x] Fix a/b path handling +daisy" "today.md checkbox flipped"
