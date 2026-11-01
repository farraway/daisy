# Test Plan #2: unique match, last task in a 25-line list — the original
# corruption scenario (sed's 0,/pattern/ range ticked everything above it).

{
    echo "### 2026-07-28 Tuesday"
    echo
    echo "#### Agenda"
    echo "- 0900 Plan Day"
    echo
    echo "#### Tasks"
    echo
    echo "**Now:**"
    echo "- [ ] Unrelated now task"
    echo
    echo "**Next:**"
    for i in $(seq 1 24); do echo "- [ ] Filler task $i +daisy"; done
    echo "- [ ] Delta target task +daisy"
    echo
    echo
    echo "**Inbox:**"
    echo "- [ ] Review weekly resolutions"
    echo "- [ ] Workout"
    echo
    echo
    echo "**GitHub PRs:**"
    echo
    echo
    echo "#### Log"
    echo
    echo "- 0900 New day started"
    echo
    echo "#### Retrospective"
    echo
    echo "* **Successes:**"
    echo "* **Misses:**"
    echo "* **What would a Sage do next:**"
} > "$(fixture_home_dir)/journal/today.md"

fixture_write_todo <<'EOF'
(B) 2026-07-27 Delta target task +daisy
EOF

before=$(fixture_read_today)

fixture_run done "Delta target"

after=$(fixture_read_today | strip_log_section)
expected=$(printf '%s\n' "$before" | strip_log_section | sed 's/^- \[ \] Delta target task +daisy$/- [x] Delta target task +daisy/')

assert_eq "$RUN_EXIT" "0" "done exits 0 on a unique match"
assert_text_eq "$after" "$expected" "only the last (25th) checkbox flips — none of the 24 fillers above it change"
