package journal_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/journal"
)

// block is a day-block as new-day archives it: heading, a body line, and
// the separator new-day writes before the next one.
func block(date string) string { return "### " + date + " Day\n\nWork on " + date + "\n" }

// days joins blocks the way journal.md holds them.
func days(dates ...string) string {
	var bs []string
	for _, d := range dates {
		bs = append(bs, block(d))
	}
	return strings.Join(bs, "\n---\n\n")
}

func TestRotateNothingToMove(t *testing.T) {
	for _, in := range []string{
		"",
		"\n\n  \n",
		days("2026-10-01"),
		days("2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05"),
		// five blocks with untidy separators: not rewritten, since nothing moves
		days("2026-10-01", "2026-10-02") + "\n---\n\n\n---\n" + days("2026-10-03", "2026-10-04", "2026-10-05") + "\n\n",
	} {
		r, err := journal.Rotate(in, nil, nil)
		if err != nil || r.Moved != 0 || len(r.Write) != 0 || len(r.Remove) != 0 {
			t.Errorf("%q: %+v %v", in, r, err)
		}
	}
}

func TestRotateRefusesContentBeforeTheFirstHeading(t *testing.T) {
	_, err := journal.Rotate("stray note\n\n"+days("2026-10-01"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "non-blank content before its first '### YYYY-MM-DD' heading") {
		t.Errorf("err %v", err)
	}
	// blank lines before the first heading are not content
	if _, err := journal.Rotate(" \n\t\n"+days("2026-10-01"), nil, nil); err != nil {
		t.Errorf("blank lead: %v", err)
	}
}

// One block moved into empty windows: the three rolling files, named by
// the block's date, each with its front matter; journal.md keeps five.
func TestRotateOneBlockFormat(t *testing.T) {
	in := days("2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06")
	r, err := journal.Rotate(in, nil, nil)
	if err != nil || r.Moved != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	if want := days("2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06"); r.Journal != want {
		t.Errorf("journal:\n%s", r.Journal)
	}
	front := func(label string) string {
		return "# Journal Archive: " + label + "\n\n## Table of Contents\n- 2026-10-01\n\n## Summary\n\n1 entries, 2026-10-01 to 2026-10-01.\n\n---\n\n" + block("2026-10-01")
	}
	want := map[string]string{
		"journal-20261001-last14.md": front("Last 14 Entries"),
		"journal-20261001-mtd.md":    front("2026-10"),
		"journal-20261001-ytd.md":    front("2026"),
	}
	if fmt.Sprint(r.Write) != fmt.Sprint(want) || len(r.Remove) != 0 {
		t.Errorf("write %q remove %q", r.Write, r.Remove)
	}
}

// A block's trailing blank lines and one trailing `---` are the boundary,
// not content; a preamble written before a heading (new-week's Weekly
// Retrospective) stays with the block it follows.
func TestRotateBlockBoundaries(t *testing.T) {
	first := "### 2026-10-01 Day\n\nwork\n  \n---\n\n#### Weekly Retrospective\n\n* ok\n\n---\n\n\t\n"
	in := first + days("2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06")
	r, err := journal.Rotate(in, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Write["journal-20261001-mtd.md"]
	if want := "\n---\n\n### 2026-10-01 Day\n\nwork\n  \n---\n\n#### Weekly Retrospective\n\n* ok\n"; !strings.HasSuffix(got, want) {
		t.Errorf("mtd:\n%q", got)
	}
	// only one trailing separator is the boundary; a second is content
	in = "### 2026-10-01 Day\n\nwork\n\n---\n---\n\n" + days("2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06")
	r, _ = journal.Rotate(in, nil, nil)
	if got := r.Write["journal-20261001-ytd.md"]; !strings.HasSuffix(got, "\n---\n\n### 2026-10-01 Day\n\nwork\n\n---\n") {
		t.Errorf("ytd:\n%q", got)
	}
	// the retained blocks are trimmed and rejoined too
	in = days("2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05") + "\n\n---\n\n\n" + block("2026-10-06") + "\n\n"
	r, _ = journal.Rotate(in, nil, nil)
	if want := days("2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06"); r.Journal != want {
		t.Errorf("journal:\n%q", r.Journal)
	}
}

// The month and year files close at the calendar boundary into
// journal-YYYYMM.md / journal-YYYY.md and a new window opens, named by the
// new block's date; the last-14 window drops its oldest beyond fourteen
// and is renamed by its new first block.
func TestRotateWindowsAcrossBoundaries(t *testing.T) {
	windows := map[string]string{}
	var all []string
	for d := 1; d <= 30; d++ { // 2026-12-13 .. 2027-01-11
		day := dateAfter("2026-12-12", d)
		all = append(all, day)
		start := max(0, len(all)-6)
		r, err := journal.Rotate(days(all[start:]...), windows, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range r.Remove {
			delete(windows, n)
		}
		for n, c := range r.Write {
			windows[n] = c
		}
	}
	// 30 blocks seen, 5 retained in journal.md, 25 rotated: 2026-12-13 ..
	// 2027-01-06.
	names := slices.Sorted(maps.Keys(windows))
	want := []string{"journal-2026.md", "journal-202612.md", "journal-20261224-last14.md", "journal-20270101-mtd.md", "journal-20270101-ytd.md"}
	if !slices.Equal(names, want) {
		t.Fatalf("windows %q", names)
	}
	headings := func(name string) []string {
		var ds []string
		for _, l := range strings.Split(windows[name], "\n") {
			if strings.HasPrefix(l, "### ") {
				ds = append(ds, l[4:14])
			}
		}
		return ds
	}
	if h := headings("journal-202612.md"); len(h) != 19 || h[0] != "2026-12-13" || h[18] != "2026-12-31" {
		t.Errorf("month file %q", h)
	}
	if h := headings("journal-20261224-last14.md"); len(h) != 14 || h[0] != "2026-12-24" || h[13] != "2027-01-06" {
		t.Errorf("last14 %q", h)
	}
	if h := headings("journal-20270101-ytd.md"); len(h) != 6 {
		t.Errorf("ytd %q", h)
	}
	if !strings.HasPrefix(windows["journal-2026.md"], "# Journal Archive: 2026\n\n## Table of Contents\n- 2026-12-13\n") ||
		!strings.Contains(windows["journal-2026.md"], "\n19 entries, 2026-12-13 to 2026-12-31.\n") {
		t.Errorf("year file front matter:\n%s", windows["journal-2026.md"][:200])
	}
}

// A rolling month or year file with no day-block in it cannot say which
// month it is; the bash died part-way (set -e) with the last-14 window
// already written. Go refuses before anything is written.
func TestRotateRefusesAWindowWithoutAHeading(t *testing.T) {
	in := days("2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06")
	_, err := journal.Rotate(in, map[string]string{"journal-20260901-mtd.md": "# notes\n"}, nil)
	if err == nil || !strings.Contains(err.Error(), "journal-20260901-mtd.md") {
		t.Errorf("err %v", err)
	}
}

func dateAfter(base string, n int) string {
	t, _ := time.Parse(time.DateOnly, base)
	return t.AddDate(0, 0, n).Format(time.DateOnly)
}

// A preamble ending in `---` before the first heading (new-week's Weekly
// Retrospective archived into an empty journal) has no block before it to
// stay with: it stays at the top of journal.md, never rotated.
func TestRotateKeepsALeadingRetrospective(t *testing.T) {
	retro := "#### Weekly Retrospective\n\n* **Successes:** \n\n---\n\n"
	in := retro + days("2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06")
	r, err := journal.Rotate(in, nil, nil)
	if err != nil || r.Moved != 1 || !strings.HasPrefix(r.Journal, retro+"### 2026-10-02") {
		t.Errorf("%+v %v", r, err)
	}
	if _, err := journal.Rotate("#### Weekly Retrospective\n\n---\n", nil, nil); err == nil {
		t.Error("a preamble with no block after it was taken")
	}
}

// A month or year window never closes into a file of its name that exists
// already, in the directory or written earlier in the same run: blocks out
// of date order would replace it, so the rotation is refused, nothing
// written (L513).
func TestRotateNeverReplacesAClosedWindow(t *testing.T) {
	retained := []string{"2026-10-10", "2026-10-11", "2026-10-12", "2026-10-13", "2026-10-14"}
	for _, tc := range []struct {
		name     string
		moved    []string
		existing map[string]bool
		closes   string
	}{
		// September closes when October comes, October when September
		// comes back, and September would close a second time.
		{"in the run", []string{"2026-09-28", "2026-10-01", "2026-09-29", "2026-10-02"}, nil, "journal-202609.md"},
		{"on disk, a month", []string{"2026-09-29", "2026-10-01"}, map[string]bool{"journal-202609.md": true}, "journal-202609.md"},
		{"on disk, a year", []string{"2025-12-31", "2026-01-01"}, map[string]bool{"journal-2025.md": true}, "journal-2025.md"},
	} {
		in := days(append(tc.moved, retained...)...)
		r, err := journal.Rotate(in, nil, tc.existing)
		if err == nil || !strings.Contains(err.Error(), "would replace "+tc.closes) || !strings.Contains(err.Error(), "Nothing written") {
			t.Errorf("%s: %+v %v", tc.name, r, err)
		}
	}
	// In date order, the same blocks close each period once.
	if _, err := journal.Rotate(days("2026-09-28", "2026-09-29", "2026-10-01", "2026-10-02", "2026-10-10", "2026-10-11", "2026-10-12", "2026-10-13", "2026-10-14"), nil, map[string]bool{"journal-202608.md": true}); err != nil {
		t.Errorf("in order: %v", err)
	}
}
