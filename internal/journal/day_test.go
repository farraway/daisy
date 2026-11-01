package journal_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/journal"
)

// Which open todo.txt lines a new day carries, into which section, and with
// what text: the bash's rules, one shape per row.
func TestCarry(t *testing.T) {
	lines := []string{
		"(A) 2026-07-27 Ship it +daisy",
		"(A) Undated urgent",               // no date: carried whole
		"(B) 2026-07-27 Next thing",        //
		"(C) 2026-07-24 Later",             // C and D are not carried
		"(D) 2026-07-20 Someday",           //
		"2026-07-28 Inbox item ~mel",       // dated, no priority: Inbox
		"Undated, unprioritised",           // not carried
		"(C) 2026-07-24 Review PR @github", // @git/@github first, any priority
		"(E) 2026-07-24 Odd priority @git", // priority outside A-D: carried whole
		"@git with nothing else",
		"x 2026-07-28 2026-07-01 Done +daisy @git", // completed: skipped
		"",
	}
	c := journal.Carry(lines)
	for _, tc := range []struct {
		name      string
		got, want []string
	}{
		{"Now", c.Now, []string{"Ship it +daisy", "(A) Undated urgent"}},
		{"Next", c.Next, []string{"Next thing"}},
		{"Inbox", c.Inbox, []string{"Inbox item ~mel"}},
		{"GitHub", c.GitHub, []string{"Review PR @github", "(E) 2026-07-24 Odd priority @git", "@git with nothing else"}},
	} {
		if !slices.Equal(tc.got, tc.want) {
			t.Errorf("%s: %q", tc.name, tc.got)
		}
	}
}

// The template's placeholders filled; each section a `- [ ] ` line per
// task; the file ends in exactly one newline whatever the template's tail.
func TestFillDayTemplate(t *testing.T) {
	tpl := read(t, "../../templates/journal-day.md")
	now := time.Date(2026, 12, 31, 7, 5, 0, 0, time.UTC)
	got := journal.Fill(tpl, now, journal.Carried{
		Now:    []string{"Ship it +daisy", "R&D \\& notes"},
		Inbox:  []string{"Inbox item"},
		GitHub: []string{"Review PR @github"},
	})
	want := strings.TrimRight(tpl, "\n") + "\n"
	for _, r := range [][2]string{
		{"{DATE}", "2026-12-31"}, {"{DAY}", "Thursday"}, {"{TIME}", "0705"},
		{"{HIGH_PRIORITY_TASKS}", "- [ ] Ship it +daisy\n- [ ] R&D \\& notes\n"},
		{"{NEXT_PRIORITY_TASKS}", ""},
		{"{INBOX_TASKS}", "- [ ] Inbox item\n"},
		{"{GITHUB_TASKS}", "- [ ] Review PR @github\n"},
	} {
		want = strings.ReplaceAll(want, r[0], r[1])
	}
	if got != want {
		t.Errorf("got:\n%s", got)
	}
	if !strings.HasPrefix(got, "### 2026-12-31 Thursday\n") || !strings.Contains(got, "- 0705 New day started\n") || strings.Contains(got, "{") {
		t.Errorf("placeholders left or misplaced:\n%s", got)
	}
}

// The week template opens with last week's retrospective before the day's
// heading; it is filled the same way.
func TestFillWeekTemplate(t *testing.T) {
	got := journal.Fill(read(t, "../../templates/journal-week.md"), time.Date(2027, 1, 4, 9, 0, 0, 0, time.UTC), journal.Carried{})
	if !strings.HasPrefix(got, "#### Weekly Retrospective\n") || !strings.Contains(got, "\n---\n\n### 2027-01-04 Monday\n") || strings.Contains(got, "{") {
		t.Errorf("got:\n%s", got)
	}
}
