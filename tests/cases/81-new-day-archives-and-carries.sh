#!/usr/bin/env bash
# Invocation: sourced by tests/run.sh — not executed directly.
# `daisy new-day`: yesterday's today.md is appended to journal.md after a
# `---` separator, the journal rotates (six blocks: the oldest leaves for the
# window files), cancelled tasks leave todo.txt, and the new today.md carries
# the open tasks — (A) to Now, (B) to Next, dated unprioritised to Inbox,
# @github whatever its priority — with completed lines left in todo.txt. One
# commit in the home.

seed_today_basic
cat > "$(fixture_home_dir)/journal/journal.md" <<'EOF2'
### 2026-07-23 Thursday

Old work

---

### 2026-07-24 Friday

---

### 2026-07-25 Saturday

---

### 2026-07-26 Sunday

---

### 2026-07-27 Monday
EOF2
fixture_write_todo <<'EOF2'
(A) 2026-07-27 Ship the release +daisy
(B) 2026-07-27 Write the notes
(C) 2026-07-24 Later, not carried
2026-07-28 Inbox item
(C) 2026-07-24 Review PR @github
z 2026-07-28 2026-07-01 Cancelled one
x 2026-07-28 2026-07-02 Finished one
EOF2
before=$(fixture_commit_count)

fixture_run_cli new-day
assert_eq "$RUN_EXIT" "0" "new-day exits 0"
assert_contains "$RUN_STDOUT" "📦 Archived yesterday to journal.md" "reports the archive"
assert_contains "$RUN_STDOUT" "✅ Rotated 1 day-block(s); journal.md retains its last 5" "reports the rotation"
assert_contains "$RUN_STDOUT" "🗑️  Deleted 1 cancelled task(s)" "reports the cancelled task"
assert_contains "$RUN_STDOUT" "✅ New day started: " "reports the new day"
assert_contains "$RUN_STDOUT" "   High priority tasks: 1" "one Now task"
assert_contains "$RUN_STDOUT" "   GitHub tasks: 1" "one GitHub task"
assert_eq "$(fixture_commit_count)" "$((before + 1))" "one commit in the home"

journal=$(cat "$(fixture_home_dir)/journal/journal.md")
assert_eq "$(grep -c '^### [0-9]' <<<"$journal")" "5" "journal.md keeps five day-blocks"
assert_not_contains "$journal" "Old work" "the oldest left journal.md"
assert_contains "$journal" "### 2026-07-28 Tuesday" "yesterday is in journal.md"
assert_contains "$(cat "$(fixture_home_dir)/journal/journal-20260723-mtd.md")" "Old work" "and the oldest is in the month window"

today=$(fixture_read_today)
now=$(awk '/^\*\*Now:/{p=1;next} /^\*\*/{p=0} p' <<<"$today")
next=$(awk '/^\*\*Next:/{p=1;next} /^\*\*/{p=0} p' <<<"$today")
assert_contains "$now" "- [ ] Ship the release +daisy" "(A) to Now, without priority or date"
assert_contains "$next" "- [ ] Write the notes" "(B) to Next"
assert_contains "$today" "- [ ] Inbox item" "dated, unprioritised to Inbox"
assert_contains "$today" "- [ ] Review PR @github" "@github carried whatever its priority"
assert_not_contains "$today" "Later, not carried" "(C) is not carried"
assert_not_contains "$today" "{" "no placeholder left"

todo=$(fixture_read_todo)
assert_not_contains "$todo" "Cancelled one" "the cancelled task left todo.txt"
assert_contains "$todo" "x 2026-07-28 2026-07-02 Finished one" "a completed task stays until new-week"
