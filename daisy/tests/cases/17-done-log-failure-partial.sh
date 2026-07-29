# Test Plan #17: `daisy log` fails (today.md has no `#### Log`) — todo.txt/
# today.md still committed; partial state reported explicitly; exits
# non-zero; sub-call error text preserved.
#
# today.md missing `#### Log` also fails the master healthcheck that done.sh
# runs up front — so to reach the actually-interesting failure (edits happen,
# *then* journaling fails), this simulates the real-world path where the
# master healthcheck was already cached as passed earlier in the same shell
# session (DAISY_HEALTHCHECK_PASSED=1) before today.md lost its Log section.

fixture_write_todo <<'EOF'
(B) 2026-07-27 Ship widget without log section +daisy
EOF
fixture_write_today <<'EOF'
### 2026-07-28 Tuesday

#### Agenda
- 0900 Plan Day

#### Tasks

**Now:**
- [ ] Ship widget without log section +daisy


**Inbox:**
- [ ] Review weekly resolutions


**GitHub PRs:**

#### Retrospective

* **Successes:**
* **Misses:**
* **What would a Sage do next:**
EOF

commits_before=$(fixture_commit_count)

export DAISY_HEALTHCHECK_PASSED=1
fixture_run done "Ship widget without log"
unset DAISY_HEALTHCHECK_PASSED

assert_eq "$RUN_EXIT" "1" "a journaling failure must not be reported as success"
today=$(date +%Y-%m-%d)
assert_contains "$(fixture_read_todo)" "x $today 2026-07-27 Ship widget without log section +daisy" "todo.txt edit still lands despite the journal failure"
assert_eq "$(fixture_commit_count)" "$((commits_before + 1))" "the edits are still committed"
assert_not_contains "$RUN_STDOUT" "✅ Done" "no success line when journaling failed"
assert_contains "$RUN_STDOUT$RUN_STDERR" "Log" "the journaling failure is surfaced, not swallowed"
