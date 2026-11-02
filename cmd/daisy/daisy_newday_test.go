package main_test

import (
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/journal"
	"daisy/internal/ops"
)

const todoForNewDay = `(A) 2026-07-27 Ship the release +daisy
(B) 2026-07-27 Write the notes
(B) Undated next
(C) 2026-07-24 Later
2026-07-28 Inbox item ~mel
(C) 2026-07-24 Review PR @github
z 2026-07-28 2026-07-01 Cancelled one
x 2026-07-28 2026-07-02 Finished one
x 2026-07-28 2026-07-03 Finished two +daisy
`

// days is a journal.md of day-blocks for the dates, as new-day leaves it.
func days(dates ...string) string {
	var bs []string
	for _, d := range dates {
		bs = append(bs, "### "+d+" Day\n\nWork on "+d+"\n")
	}
	return strings.Join(bs, "\n---\n\n")
}

// at is dayOps with the clock at t.
func (m *machine) at(t time.Time) *ops.Fake {
	o := m.dayOps(m.ws, true)
	o.NowFn = func() time.Time { return t }
	return o
}

func (m *machine) template(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(m.root, "templates", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNewDay(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	writeFile(t, filepath.Join(m.home, "journal", "journal.md"), days("2026-07-23", "2026-07-24", "2026-07-25", "2026-07-26", "2026-07-27"))
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), todoForNewDay)
	m.initHomeRepo(t)
	now := time.Date(2026, 7, 29, 8, 5, 0, 0, time.Local)

	code, out, errs := runWith(m.at(now), "new-day")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if want := "📦 Archived yesterday to journal.md\n" +
		"✅ Rotated 1 day-block(s); journal.md retains its last 5\n" +
		"🗑️  Deleted 1 cancelled task(s)\n" +
		"✅ New day started: 2026-07-29 Wednesday\n" +
		"   High priority tasks: 1\n   Next priority tasks: 2\n   Inbox tasks: 1\n   GitHub tasks: 1\n" +
		"📝 Committed: New day: 2026-07-29 Wednesday (" + hash + ")\n"; out != want {
		t.Errorf("stdout:\n%s", out)
	}
	wantToday := journal.Fill(m.template(t, "journal-day.md"), now, journal.Carried{
		Now: []string{"Ship the release +daisy"}, Next: []string{"Write the notes", "(B) Undated next"},
		Inbox: []string{"Inbox item ~mel"}, GitHub: []string{"Review PR @github"},
	})
	if got := m.read(t, "journal/today.md"); got != wantToday {
		t.Errorf("today.md:\n%s", got)
	}
	// yesterday is journal.md's last block, as written; the oldest moved out
	if got := m.read(t, "journal/journal.md"); got != days("2026-07-24", "2026-07-25", "2026-07-26", "2026-07-27")+"\n---\n\n"+todayBasic {
		t.Errorf("journal.md:\n%s", got)
	}
	if got := m.read(t, "journal/journal-20260723-mtd.md"); !strings.Contains(got, "### 2026-07-23 Day\n") {
		t.Errorf("mtd:\n%s", got)
	}
	if got := m.read(t, "tasks/todo.txt"); got != strings.Replace(todoForNewDay, "z 2026-07-28 2026-07-01 Cancelled one\n", "", 1) {
		t.Errorf("todo.txt:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s, want exactly one more", m.commits(t))
	}
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "New day: 2026-07-29 Wednesday" {
		t.Errorf("message %q", msg)
	}
}

func TestNewWeek(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), todoForNewDay)
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "x 2026-07-01 2026-06-01 Old\n")
	m.initHomeRepo(t)
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.Local)

	code, out, errs := runWith(m.at(now), "new-week")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if want := "📦 Archived yesterday to journal.md\n" +
		"🗑️  Deleted 1 cancelled task(s)\n" +
		"📦 Archived 2 completed task(s) to done.txt\n" +
		"✅ New week started: 2026-08-03 Monday\n" +
		"   High priority tasks: 1\n   Next priority tasks: 2\n   Inbox tasks: 1\n   GitHub tasks: 1\n" +
		"📝 Committed: New week: 2026-08-03 Monday (" + hash + ")\n"; out != want {
		t.Errorf("stdout:\n%s", out)
	}
	if got := m.read(t, "tasks/done.txt"); got != "x 2026-07-01 2026-06-01 Old\nx 2026-07-28 2026-07-02 Finished one\nx 2026-07-28 2026-07-03 Finished two +daisy\n" {
		t.Errorf("done.txt:\n%s", got)
	}
	if got := m.read(t, "tasks/todo.txt"); got != "(A) 2026-07-27 Ship the release +daisy\n(B) 2026-07-27 Write the notes\n(B) Undated next\n(C) 2026-07-24 Later\n2026-07-28 Inbox item ~mel\n(C) 2026-07-24 Review PR @github\n" {
		t.Errorf("todo.txt:\n%s", got)
	}
	if got := m.read(t, "journal/today.md"); got != journal.Fill(m.template(t, "journal-week.md"), now, journal.Carry(strings.Split(m.read(t, "tasks/todo.txt"), "\n"))) {
		t.Errorf("today.md:\n%s", got)
	}
	if got := m.read(t, "journal/journal.md"); got != todayBasic {
		t.Errorf("journal.md: the first archive has no separator:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s", m.commits(t))
	}
	// a done.txt without a final newline is left alone when nothing moves
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "x 2026-07-01 2026-06-01 Old")
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) 2026-08-01 Open\n")
	runWith(m.at(now.Add(time.Minute)), "new-week")
	if got := m.read(t, "tasks/done.txt"); got != "x 2026-07-01 2026-06-01 Old" {
		t.Errorf("done.txt touched with nothing to archive: %q", got)
	}
	// and is not glued to when something does
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "x 2026-07-01 2026-06-01 Old")
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "x 2026-08-03 2026-08-01 New\n")
	runWith(m.at(now.Add(time.Hour)), "new-week")
	if got := m.read(t, "tasks/done.txt"); got != "x 2026-07-01 2026-06-01 Old\nx 2026-08-03 2026-08-01 New\n" {
		t.Errorf("done.txt:\n%q", got)
	}
}

// A refusal changes nothing and commits nothing: the health check, a
// missing template or todo.txt, and a journal rotate cannot split.
func TestNewDayRefusals(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	now := time.Date(2026, 7, 29, 8, 5, 0, 0, time.Local)
	before := func() map[string]string {
		got := map[string]string{}
		for _, f := range []string{"journal/today.md", "journal/journal.md", "tasks/todo.txt"} {
			got[f] = m.read(t, f)
		}
		return got
	}

	o := m.at(now)
	o.GetenvFn = m.dayOps(m.ws, false).GetenvFn
	if code, out, errs := runWith(o, "new-day"); code != 1 || out != "" || errs != "Error: System health check failed\n  Run: daisy healthcheck\n" {
		t.Errorf("health check: exit %d out %q err %q", code, out, errs)
	}

	writeFile(t, filepath.Join(m.home, "journal", "journal.md"), "stray note\n\n"+days("2026-07-27"))
	want := before()
	code, out, errs := runWith(m.at(now), "new-day")
	if code != 1 || out != "" || !strings.HasPrefix(errs, "Error: rotate: journal.md has non-blank content before its first '### YYYY-MM-DD' heading") {
		t.Errorf("unsplittable journal: exit %d out %q err %q", code, out, errs)
	}
	if got := before(); !maps.Equal(got, want) {
		t.Errorf("a refusal wrote: %q", got)
	}

	os.Remove(filepath.Join(m.root, "templates", "journal-week.md"))
	if code, _, errs := runWith(m.at(now), "new-week"); code != 1 || errs != templateMissing(m, "journal-week.md") {
		t.Errorf("no template: exit %d err %q", code, errs)
	}
	os.Remove(filepath.Join(m.home, "tasks", "todo.txt"))
	if code, _, errs := runWith(m.at(now), "new-day"); code != 1 || errs != "Error: todo.txt not found\n" {
		t.Errorf("no todo.txt: exit %d err %q", code, errs)
	}
	if code, _, errs := runWith(m.at(now), "new-day", "extra"); code != 2 || !strings.Contains(errs, "Usage: daisy new-day") {
		t.Errorf("argument: exit %d err %q", code, errs)
	}
	if m.commits(t) != "1" {
		t.Errorf("a refusal committed: %s commits", m.commits(t))
	}
}

func TestRotate(t *testing.T) {
	m := newMachine(t)
	m.initHomeRepo(t)
	jpath := filepath.Join(m.home, "journal", "journal.md")
	for _, in := range []string{"", days("2026-07-01", "2026-07-02", "2026-07-03", "2026-07-04", "2026-07-05")} {
		writeFile(t, jpath, in)
		if code, out, errs := runWith(m.at(time.Now()), "rotate"); code != 0 || out != "" || errs != "" {
			t.Errorf("%q: exit %d out %q err %q", in, code, out, errs)
		}
	}
	os.Remove(jpath)
	if code, out, errs := runWith(m.at(time.Now()), "rotate"); code != 0 || out != "" || errs != "" {
		t.Errorf("no journal.md: exit %d out %q err %q", code, out, errs)
	}
	writeFile(t, jpath, days("2026-07-01", "2026-07-02", "2026-07-03", "2026-07-04", "2026-07-05", "2026-07-06", "2026-07-07"))
	code, out, errs := runWith(m.at(time.Now()), "rotate")
	if code != 0 || errs != "" || out != "✅ Rotated 2 day-block(s); journal.md retains its last 5\n" {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
	if got := m.read(t, "journal/journal.md"); got != days("2026-07-03", "2026-07-04", "2026-07-05", "2026-07-06", "2026-07-07") {
		t.Errorf("journal.md:\n%s", got)
	}
	if m.commits(t) != "1" {
		t.Errorf("rotate committed")
	}
	// a rolling window that is a link is refused, not skipped
	writeFile(t, filepath.Join(m.home, "store"), "### 2026-07-01 Day\n")
	os.Symlink(filepath.Join(m.home, "store"), filepath.Join(m.home, "journal", "journal-20260601-last14.md"))
	writeFile(t, jpath, days("2026-07-08", "2026-07-09", "2026-07-10", "2026-07-11", "2026-07-12"))
	if code, out, errs := runWith(m.at(time.Now()), "rotate"); code != 0 || out != "" || errs != "" {
		t.Errorf("linked window, nothing to move: exit %d out %q err %q", code, out, errs)
	}
	writeFile(t, jpath, days("2026-07-08", "2026-07-09", "2026-07-10", "2026-07-11", "2026-07-12", "2026-07-13"))
	before := m.read(t, "journal/journal.md")
	if code, _, errs := runWith(m.at(time.Now()), "rotate"); code != 1 || !strings.Contains(errs, "journal-20260601-last14.md is not a regular file") {
		t.Errorf("linked window: exit %d err %q", code, errs)
	}
	if m.read(t, "journal/journal.md") != before {
		t.Errorf("linked window: journal.md written")
	}
}

func TestDayHealthcheckHooks(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "")
	for _, cmd := range []string{"new-day", "new-week", "rotate"} {
		if code, out, errs := runWith(m.dayOps(m.ws, true), cmd, "--healthcheck"); code != 0 || out != "" || errs != "" {
			t.Errorf("%s --healthcheck: exit %d out %q err %q", cmd, code, out, errs)
		}
	}
	for _, tc := range []struct{ remove, cmd, err string }{
		{"journal/journal.md", "rotate", "Error: journal.md not found\n"},
		{"tasks/done.txt", "new-week", "Error: done.txt not found\n"},
		{"journal", "new-day", "Error: journal directory not found\n"},
		{"tasks/todo.txt", "new-day", "Error: todo.txt not found\n"},
	} {
		os.RemoveAll(filepath.Join(m.home, tc.remove))
		if code, _, errs := runWith(m.dayOps(m.ws, true), tc.cmd, "--healthcheck"); code != 1 || errs != tc.err {
			t.Errorf("%s without %s: exit %d err %q", tc.cmd, tc.remove, code, errs)
		}
	}
	os.Remove(filepath.Join(m.root, "templates", "journal-day.md"))
	if code, _, errs := runWith(m.dayOps(m.ws, true), "new-day", "--healthcheck"); code != 1 || errs != templateMissing(m, "journal-day.md") {
		t.Errorf("no template: exit %d err %q", code, errs)
	}
}

// templateMissing is the refusal naming both places a template is looked
// for, the home's first.
func templateMissing(m *machine, name string) string {
	return "Error: Template missing: " + name + "\n  Looked for: " + m.home + "/templates/" + name + ", then " + m.root + "/templates/" + name + "\n"
}

// A home's template is used in place of the root's: the new today.md is
// the root's as today with the one line the home changed. With the root's
// gone the home's still serves, the health hook included.
func TestNewDayHomeTemplate(t *testing.T) {
	const rootLine, homeLine = "- [ ] Check calendar for upcoming events\n", "- [ ] Check calendar for upcoming events (Home calendar)\n"
	for _, cmd := range []string{"new-day", "new-week"} {
		m := newMachine(t)
		m.seed(t)
		writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "")
		m.initHomeRepo(t)
		name := map[string]string{"new-day": "journal-day.md", "new-week": "journal-week.md"}[cmd]
		root := m.template(t, name)
		if strings.Count(root, rootLine) != 1 {
			t.Fatalf("%s: the root's template has no single calendar line", name)
		}
		writeFile(t, filepath.Join(m.home, "templates", name), strings.Replace(root, rootLine, homeLine, 1))
		now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.Local)
		carried := journal.Carry(strings.Split(m.read(t, "tasks/todo.txt"), "\n"))

		if code, _, errs := runWith(m.at(now), cmd); code != 0 || errs != "" {
			t.Fatalf("%s: exit %d stderr %q", cmd, code, errs)
		}
		if got, want := m.read(t, "journal/today.md"), strings.Replace(journal.Fill(root, now, carried), rootLine, homeLine, 1); got != want {
			t.Errorf("%s: today.md is not the root's with the home's line:\n%s", cmd, got)
		}

		os.Remove(filepath.Join(m.root, "templates", name))
		if code, out, errs := runWith(m.dayOps(m.ws, true), cmd, "--healthcheck"); code != 0 || out != "" || errs != "" {
			t.Errorf("%s --healthcheck, the home's template only: exit %d out %q err %q", cmd, code, out, errs)
		}
		if code, _, errs := runWith(m.at(now.Add(time.Hour)), cmd); code != 0 || errs != "" {
			t.Errorf("%s, the home's template only: exit %d err %q", cmd, code, errs)
		}
		if got := m.read(t, "journal/today.md"); !strings.Contains(got, homeLine) {
			t.Errorf("%s, the home's template only: today.md:\n%s", cmd, got)
		}
		os.Remove(filepath.Join(m.home, "templates", name))
		if code, _, errs := runWith(m.dayOps(m.ws, true), cmd, "--healthcheck"); code != 1 || errs != templateMissing(m, name) {
			t.Errorf("%s --healthcheck, neither: exit %d err %q", cmd, code, errs)
		}
	}
}

// The stateful invariant, asserted after every new-day from late November
// to mid-January: journal.md holds the last five archived days; every older
// day is in exactly one month file (the month-to-date window or a closed
// journal-YYYYMM.md) and that file is its month's, in exactly one year file
// likewise, and in the last-14 window exactly when it is among the fourteen
// most recently rotated; it is in no other file. Each rolling window is
// named by its first block.
func TestRotationInvariantAcrossMonthAndYear(t *testing.T) {
	m := newMachine(t)
	m.initHomeRepo(t)
	var archived []string
	for d := time.Date(2026, 11, 20, 8, 0, 0, 0, time.Local); d.Before(time.Date(2027, 1, 15, 0, 0, 0, 0, time.Local)); d = d.AddDate(0, 0, 1) {
		if code, _, errs := runWith(m.at(d), "new-day"); code != 0 {
			t.Fatalf("%s: exit %d %s", d, code, errs)
		}
		checkRotation(t, filepath.Join(m.home, "journal"), archived)
		archived = append(archived, d.Format(time.DateOnly))
	}
	if len(archived) < 40 {
		t.Fatal("the walk did not run")
	}
}

func checkRotation(t *testing.T, dir string, archived []string) {
	t.Helper()
	keep := max(0, len(archived)-journal.Retain)
	rotated, retained := archived[:keep], archived[keep:]
	headings := func(name string) []string {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		var ds []string
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "### ") && len(l) >= 14 {
				ds = append(ds, l[4:14])
			}
		}
		return ds
	}
	if got := headings("journal.md"); !slices.Equal(got, retained) {
		t.Fatalf("after %v: journal.md holds %v", archived, got)
	}
	entries, _ := os.ReadDir(dir)
	type window struct {
		name, kind, period string // kind: last14, month, year
		dates              []string
	}
	var windows []window
	rolling := map[string]int{}
	for _, e := range entries {
		n := e.Name()
		if n == "journal.md" || n == "today.md" {
			continue
		}
		w := window{name: n, dates: headings(n)}
		match := func(g string) bool { ok, _ := path.Match(g, n); return ok }
		switch {
		case match("journal-*-last14.md"):
			w.kind = "last14"
		case match("journal-*-mtd.md"):
			w.kind, w.period = "month", w.dates[0][:7]
		case match("journal-*-ytd.md"):
			w.kind, w.period = "year", w.dates[0][:4]
		case match("journal-[0-9][0-9][0-9][0-9][0-9][0-9].md"):
			w.kind, w.period = "month", n[8:12]+"-"+n[12:14]
		case match("journal-[0-9][0-9][0-9][0-9].md"):
			w.kind, w.period = "year", n[8:12]
		default:
			t.Fatalf("unexpected file %s", n)
		}
		if strings.HasSuffix(n, "-last14.md") || strings.HasSuffix(n, "-mtd.md") || strings.HasSuffix(n, "-ytd.md") {
			rolling[w.kind]++
			if want := "journal-" + strings.ReplaceAll(w.dates[0], "-", ""); !strings.HasPrefix(n, want+"-") {
				t.Errorf("%s is not named by its first block %s", n, w.dates[0])
			}
		}
		for _, d := range w.dates {
			if !slices.Contains(rotated, d) {
				t.Errorf("%s holds %s, which was not rotated", n, d)
			}
			if w.period != "" && !strings.HasPrefix(d, w.period) {
				t.Errorf("%s (%s) holds %s", n, w.period, d)
			}
		}
		windows = append(windows, w)
	}
	for kind, n := range rolling {
		if n > 1 {
			t.Errorf("%d %s windows", n, kind)
		}
	}
	recent := rotated[max(0, len(rotated)-journal.Last14):]
	for _, d := range rotated {
		in := map[string]int{}
		for _, w := range windows {
			for _, wd := range w.dates {
				if wd == d {
					in[w.kind]++
				}
			}
		}
		wantLast14 := 0
		if slices.Contains(recent, d) {
			wantLast14 = 1
		}
		if in["month"] != 1 || in["year"] != 1 || in["last14"] != wantLast14 {
			t.Errorf("after %d days, %s is in %v", len(archived), d, in)
		}
	}
}
