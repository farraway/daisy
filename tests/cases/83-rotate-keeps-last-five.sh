#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# `daisy rotate`: journal.md keeps its last five day-blocks; each older one
# goes to the last-14, month-to-date and year-to-date windows, each named by
# its first block's date, and a month window closes into journal-YYYYMM.md
# when a block of the next month arrives. No commit. Five blocks or fewer:
# nothing written, nothing printed. Content before the first heading is
# refused with nothing written.

jdir="$(fixture_home_dir)/journal"
blocks() { local d; for d in "$@"; do printf '### %s Day\n\nWork on %s\n\n---\n\n' "$d" "$d"; done; }
blocks 2026-07-30 2026-07-31 2026-08-01 2026-08-02 2026-08-03 > "$jdir/journal.md"
before=$(fixture_commit_count)

fixture_run_cli rotate
assert_eq "$RUN_EXIT" "0" "five blocks: exit 0"
assert_eq "$RUN_STDOUT" "" "five blocks: nothing printed"
assert_eq "$(ls "$jdir" | grep -c 'journal-')" "0" "five blocks: no window written"

blocks 2026-08-04 2026-08-05 2026-08-06 >> "$jdir/journal.md"
fixture_run_cli rotate
assert_eq "$RUN_EXIT" "0" "eight blocks: exit 0"
assert_eq "$RUN_STDOUT" "✅ Rotated 3 day-block(s); journal.md retains its last 5" "reports three moved"
assert_eq "$(grep -c '^### ' "$jdir/journal.md")" "5" "journal.md keeps five"
assert_eq "$(grep '^### ' "$jdir/journal.md" | head -1)" "### 2026-08-02 Day" "the newest five"
assert_contains "$(cat "$jdir/journal-202607.md")" "Work on 2026-07-30" "July closed into its month file"
assert_contains "$(cat "$jdir/journal-202607.md")" "2 entries, 2026-07-30 to 2026-07-31." "with its summary"
assert_eq "$(ls "$jdir" | grep -- '-mtd.md\|-ytd.md\|-last14.md' | tr '\n' ' ')" "journal-20260730-last14.md journal-20260730-ytd.md journal-20260801-mtd.md " "each rolling window named by its first block"
assert_eq "$(grep -c '^### ' "$jdir/journal-20260730-last14.md")" "3" "every moved block in the last-14 window"
assert_eq "$(grep -c '^### ' "$jdir/journal-20260730-ytd.md")" "3" "and in the year window"
assert_eq "$(grep -c '^### ' "$jdir/journal-20260801-mtd.md")" "1" "August's month window holds August only"
assert_eq "$(fixture_commit_count)" "$before" "rotate commits nothing"

printf 'stray note\n\n' | cat - "$jdir/journal.md" > "$jdir/journal.tmp" && mv "$jdir/journal.tmp" "$jdir/journal.md"
blocks 2026-08-07 >> "$jdir/journal.md"
journal_before=$(cat "$jdir/journal.md")
fixture_run_cli rotate
assert_eq "$RUN_EXIT" "1" "content before the first heading: exit 1"
assert_contains "$RUN_STDERR" "non-blank content before its first '### YYYY-MM-DD' heading" "says why"
assert_eq "$(cat "$jdir/journal.md")" "$journal_before" "and writes nothing"
