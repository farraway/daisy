#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# A Log section whose last entry abuts the next heading (no blank line
# between) still receives the new entry at the end of the Log section, not
# in the first blank-line gap of a later section. The bash `log` scanned
# past the heading for a blank line and misplaced the entry; the Go command
# ends the section at the next heading. Case 12 pins the well-formed shape.
# check_today would fail this file (no blank before Retrospective), so the
# health check is bypassed as case 17 does.

fixture_write_today <<'EOF2'
### 2026-07-28 Tuesday

#### Agenda
- 0900 Plan Day

#### Tasks

**Now:**
- [ ] Alpha task +daisy

#### Log

- 0900 New day started
- 0930 Second entry
#### Retrospective

* **Successes:**
* **Misses:**
* **What would a Sage do next:**
EOF2

export DAISY_HEALTHCHECK_PASSED=1
fixture_run_cli log "Placed at the end of the Log section"
unset DAISY_HEALTHCHECK_PASSED

assert_eq "$RUN_EXIT" "0" "log exits 0"
assert_contains "$RUN_STDOUT" "Logged:" "log reports the entry"

after=$(fixture_read_today)
log_section=$(printf '%s\n' "$after" | awk '/^#### Log/{p=1; next} /^#### /{p=0} p')
assert_contains "$log_section" "Placed at the end of the Log section" "the entry is inside the Log section"
last_entry=$(printf '%s\n' "$log_section" | grep -v '^$' | tail -1)
assert_contains "$last_entry" "Placed at the end of the Log section" "and is its last entry"
retro=$(printf '%s\n' "$after" | awk '/^#### Retrospective/{p=1} p')
assert_not_contains "$retro" "Placed at the end" "the Retrospective section is untouched"
assert_eq "$(printf '%s\n' "$after" | grep -c '^$')" "$(fixture_read_today | grep -c '^$')" "no blank line was added"
