package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// todoSorted is todo.txt grouped as docs/task-format.md orders it: (A),
// (B), the inbox, (C), (D), then the closed lines.
const todoSorted = `(A) 2026-07-20 Apple task
(B) 2026-07-20 Banana task
2026-07-20 Inbox thing
(C) 2026-07-20 Cherry task
(D) 2026-07-20 Date task
x 2026-07-21 2026-07-20 Finished thing
`

// add writes `(P) <today> <description>` into todo.txt after the last open
// line that sorts at or before it, and an (A) or (B) task into today.md's
// section as a new day would carry it, in one commit holding both.
func TestAddTask(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	code, out, errs := runWith(m.dayOps(m.ws, true), "add", "Call ~mom +family @phone due:2026-08-01", "A")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if out != "✅ Added: (A) 2026-07-28 Call ~mom +family @phone due:2026-08-01; today.md: added to Now (commit "+hash+")\n" {
		t.Errorf("stdout %q", out)
	}
	if got := m.read(t, "tasks/todo.txt"); got != "(A) 2026-07-28 Call ~mom +family @phone due:2026-08-01\n"+todoBasic {
		t.Errorf("todo.txt:\n%s", got)
	}
	wantToday := strings.Replace(todayBasic, "- [ ] Beta task +work\n", "- [ ] Beta task +work\n- [ ] Call ~mom +family @phone due:2026-08-01\n", 1)
	if got := m.read(t, "journal/today.md"); got != wantToday {
		t.Errorf("today.md:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s, want exactly one more", m.commits(t))
	}
	if files := m.git(t, "show", "--name-only", "--format=", "HEAD"); files != "journal/today.md\ntasks/todo.txt" {
		t.Errorf("committed %q", files)
	}
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "Added: (A) Call ~mom +family @phone due:2026-08-01" {
		t.Errorf("message %q", msg)
	}

	// (B) goes to Next after its last line, the blank lines before the
	// next label kept; an @git task to GitHub PRs, as a new day carries it.
	code, out, errs = runWith(m.dayOps(m.ws, true), "add", "--co-author", "X <x@y>", "Write the release notes", "B")
	if code != 0 || errs != "" || !strings.HasPrefix(out, "✅ Added: (B) 2026-07-28 Write the release notes; today.md: added to Next (commit ") {
		t.Errorf("B: exit %d out %q err %q", code, out, errs)
	}
	if body := m.git(t, "log", "-1", "--format=%B"); body != "Added: (B) Write the release notes\n\nCo-Authored-By: X <x@y>" {
		t.Errorf("B: message %q", body)
	}
	if !strings.Contains(m.read(t, "journal/today.md"), "- [ ] Epsilon task\n- [ ] Write the release notes\n\n\n**Inbox:**") {
		t.Errorf("B: today.md:\n%s", m.read(t, "journal/today.md"))
	}
	if code, _, errs = runWith(m.dayOps(m.ws, true), "add", "Review PR 12 @github", "B"); code != 0 || errs != "" {
		t.Errorf("@git: exit %d err %q", code, errs)
	}
	if !strings.Contains(m.read(t, "journal/today.md"), "**GitHub PRs:**\n- [ ] Review PR 12 @github\n\n\n#### Log") {
		t.Errorf("@git: today.md:\n%s", m.read(t, "journal/today.md"))
	}

	// (C), (D) and none go to todo.txt only, today.md and its bytes untouched.
	today := m.read(t, "journal/today.md")
	for _, tc := range []struct{ desc, prio, line, note string }{
		{"Tidy the attic", "C", "(C) 2026-07-28 Tidy the attic", "not added, the next new day does not carry it"},
		{"Look into the noise", "none", "2026-07-28 Look into the noise", "not added, the next new day carries it to Inbox"},
	} {
		code, out, errs := runWith(m.dayOps(m.ws, true), "add", tc.desc, tc.prio)
		if code != 0 || errs != "" || !strings.HasPrefix(out, "✅ Added: "+tc.line+"; today.md: "+tc.note+" (commit ") {
			t.Errorf("%s: exit %d out %q err %q", tc.prio, code, out, errs)
		}
		if !strings.Contains(m.read(t, "tasks/todo.txt"), tc.line+"\n") {
			t.Errorf("%s: todo.txt:\n%s", tc.prio, m.read(t, "tasks/todo.txt"))
		}
		if files := m.git(t, "show", "--name-only", "--format=", "HEAD"); files != "tasks/todo.txt" {
			t.Errorf("%s: committed %q", tc.prio, files)
		}
	}
	if m.read(t, "journal/today.md") != today {
		t.Error("a (C) or inbox task changed today.md")
	}
}

// The insertion point: after the last open line that sorts at or before
// the new one — (A), (B), none, (C), (D) — at the top when none does; a
// closed line is never an anchor.
func TestAddTaskPlacesByPriority(t *testing.T) {
	const closed = "x 2026-07-21 2026-07-20 Finished thing\n"
	for _, tc := range []struct{ todo, prio, want string }{
		{todoSorted, "A", "(A) 2026-07-20 Apple task\n(A) 2026-07-28 New\n(B) 2026-07-20 Banana task\n2026-07-20 Inbox thing\n(C) 2026-07-20 Cherry task\n(D) 2026-07-20 Date task\n" + closed},
		{todoSorted, "B", "(A) 2026-07-20 Apple task\n(B) 2026-07-20 Banana task\n(B) 2026-07-28 New\n2026-07-20 Inbox thing\n(C) 2026-07-20 Cherry task\n(D) 2026-07-20 Date task\n" + closed},
		{todoSorted, "none", "(A) 2026-07-20 Apple task\n(B) 2026-07-20 Banana task\n2026-07-20 Inbox thing\n2026-07-28 New\n(C) 2026-07-20 Cherry task\n(D) 2026-07-20 Date task\n" + closed},
		{todoSorted, "C", "(A) 2026-07-20 Apple task\n(B) 2026-07-20 Banana task\n2026-07-20 Inbox thing\n(C) 2026-07-20 Cherry task\n(C) 2026-07-28 New\n(D) 2026-07-20 Date task\n" + closed},
		{todoSorted, "D", "(A) 2026-07-20 Apple task\n(B) 2026-07-20 Banana task\n2026-07-20 Inbox thing\n(C) 2026-07-20 Cherry task\n(D) 2026-07-20 Date task\n(D) 2026-07-28 New\n" + closed},
		// no (A): before the first line that sorts after it
		{"(B) 2026-07-20 Banana task\n", "A", "(A) 2026-07-28 New\n(B) 2026-07-20 Banana task\n"},
		// no inbox line: after all (B) tasks, before (C)
		{"(B) 2026-07-20 Banana task\n(C) 2026-07-20 Cherry task\n", "none", "(B) 2026-07-20 Banana task\n2026-07-28 New\n(C) 2026-07-20 Cherry task\n"},
		// unsorted: after every open line that sorts at or before it
		{todoBasic, "C", strings.Replace(todoBasic, "(D)", "(C) 2026-07-28 New\n(D)", 1)},
		// only closed lines, or none at all
		{closed, "B", "(B) 2026-07-28 New\n" + closed},
		{"", "B", "(B) 2026-07-28 New\n"},
	} {
		m := newMachine(t)
		m.seed(t)
		writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), tc.todo)
		m.initHomeRepo(t)
		if code, _, errs := runWith(m.dayOps(m.ws, true), "add", "New", tc.prio); code != 0 || errs != "" {
			t.Errorf("%s into %q: exit %d err %q", tc.prio, tc.todo, code, errs)
		} else if got := m.read(t, "tasks/todo.txt"); got != tc.want {
			t.Errorf("%s into %q:\n%s", tc.prio, tc.todo, got)
		}
	}
}

// A refusal writes nothing and commits nothing: a priority that is not
// one of the five words is a usage error; a description that starts with
// a priority, holds a line break or is blank is refused.
func TestAddTaskRefusals(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	todo, today := m.read(t, "tasks/todo.txt"), m.read(t, "journal/today.md")
	for _, args := range [][]string{{"add"}, {"add", "Task"}, {"add", "Task", ""}, {"add", "", "B"}, {"add", "Task", "B", "C"}, {"add", "Task", "E"}, {"add", "Task", "b"}, {"add", "Task", "(B)"}, {"add", "Task", "inbox"}} {
		if code, out, errs := runWith(m.dayOps(m.ws, true), args...); code != 2 || out != "" || !strings.Contains(errs, "Usage: daisy add <description> A|B|C|D|none") {
			t.Errorf("%q: exit %d out %q err %q", args, code, out, errs)
		}
	}
	if _, _, errs := runWith(m.dayOps(m.ws, true), "add", "Task", "E"); !strings.HasPrefix(errs, "Error: not a priority: E; give A, B, C, D or none\n") {
		t.Errorf("E: stderr %q", errs)
	}
	for desc, want := range map[string]string{
		"(A) Call Mom":     "Error: the description starts with a priority, (A); give it as the priority argument: daisy add \"Call Mom\" A\n",
		"Two\nlines":       "Error: a task is one line; the description holds a line break\n",
		"Carriage\rreturn": "Error: a task is one line; the description holds a line break\n",
		"   ":              "Error: the description is blank\n",
	} {
		if code, out, errs := runWith(m.dayOps(m.ws, true), "add", desc, "B"); code != 1 || out != "" || errs != want {
			t.Errorf("%q: exit %d out %q err %q", desc, code, out, errs)
		}
	}
	if code, _, errs := runWith(m.dayOps(m.ws, false), "add", "Task", "B"); code != 1 || !strings.Contains(errs, "System health check failed") {
		t.Errorf("health check failing: exit %d err %q", code, errs)
	}
	if m.read(t, "tasks/todo.txt") != todo || m.read(t, "journal/today.md") != today || m.commits(t) != "1" {
		t.Error("a refusal changed a file or committed")
	}
}

// today.md absent, or without the section the task belongs in: todo.txt
// still takes the task and is committed alone, and stderr says why
// today.md did not.
func TestAddTaskWithoutTodaysSection(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), strings.Replace(todayBasic, "**Next:**\n", "", 1))
	m.initHomeRepo(t)
	today := m.read(t, "journal/today.md")
	code, out, errs := runWith(m.dayOps(m.ws, true), "add", "Orphan", "B")
	if code != 0 || errs != "⚠️  today.md has no **Next:** section; only todo.txt changed\n" || !strings.HasPrefix(out, "✅ Added: (B) 2026-07-28 Orphan; today.md: unchanged (commit ") {
		t.Errorf("no section: exit %d out %q err %q", code, out, errs)
	}
	if m.read(t, "journal/today.md") != today || !strings.Contains(m.read(t, "tasks/todo.txt"), "(B) 2026-07-28 Orphan\n") || m.commits(t) != "2" {
		t.Error("no section: todo.txt not added and committed alone")
	}
	os.Remove(filepath.Join(m.home, "journal", "today.md"))
	code, out, errs = runWith(m.dayOps(m.ws, true), "add", "Lonely", "A")
	if code != 0 || errs != "⚠️  today.md not found; only todo.txt changed\n" || !strings.HasPrefix(out, "✅ Added: (A) 2026-07-28 Lonely; today.md: unchanged (commit ") {
		t.Errorf("no today.md: exit %d out %q err %q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(m.home, "journal", "today.md")); err == nil {
		t.Error("no today.md: one was written")
	}
}

// cancel resolves one open task as done does; in todo.txt the line becomes
// `z <today> <created> <text>`, its priority dropped, at the end of the
// file; in today.md its `- [ ]` becomes `- [z]`; one commit.
func TestCancelTask(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	code, out, errs := runWith(m.dayOps(m.ws, true), "cancel", "delta TARGET")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if out != "✅ Cancelled: Delta target task +daisy (commit "+hash+")\n" {
		t.Errorf("stdout %q", out)
	}
	wantTodo := strings.Replace(todoBasic, "(B) 2026-07-27 Delta target task +daisy\n", "", 1) + "z 2026-07-28 2026-07-27 Delta target task +daisy\n"
	if got := m.read(t, "tasks/todo.txt"); got != wantTodo {
		t.Errorf("todo.txt:\n%s", got)
	}
	if got := m.read(t, "journal/today.md"); got != strings.Replace(todayBasic, "- [ ] Delta target task +daisy\n", "- [z] Delta target task +daisy\n", 1) {
		t.Errorf("today.md:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s, want exactly one more", m.commits(t))
	}
	if files := m.git(t, "show", "--name-only", "--format=", "HEAD"); files != "journal/today.md\ntasks/todo.txt" {
		t.Errorf("committed %q", files)
	}
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "Cancelled: Delta target task +daisy" {
		t.Errorf("message %q", msg)
	}

	// A task new-day carried whole (undated (A)) is found as it carried it;
	// one absent from today.md is cancelled in todo.txt with a warning.
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) Undated urgent\n(B) 2026-07-27 Ghost task +daisy\n")
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), todayBasic+"- [ ] (A) Undated urgent\n")
	if code, _, errs = runWith(m.dayOps(m.ws, true), "cancel", "undated"); code != 0 || errs != "" || !strings.HasSuffix(m.read(t, "journal/today.md"), "- [z] (A) Undated urgent\n") {
		t.Errorf("carried whole: exit %d err %q today %q", code, errs, m.read(t, "journal/today.md"))
	}
	code, out, errs = runWith(m.dayOps(m.ws, true), "cancel", "Ghost")
	if code != 0 || errs != "⚠️  Task not found in today.md (may not be in active tasks)\n" || !strings.HasPrefix(out, "✅ Cancelled: Ghost task +daisy (commit ") {
		t.Errorf("absent from today.md: exit %d out %q err %q", code, out, errs)
	}
	if got := m.read(t, "tasks/todo.txt"); got != "z 2026-07-28 Undated urgent\nz 2026-07-28 2026-07-27 Ghost task +daisy\n" {
		t.Errorf("todo.txt:\n%s", got)
	}
}

// cancel and priority refuse as done does: no match, several matches
// (listed), a closed line never a candidate; nothing written or committed.
func TestCancelAndPriorityRefusals(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), todoBasic+"(B) 2026-07-27 Shared token Alpha +daisy\n(B) 2026-07-27 Shared token Beta +daisy\nx 2026-07-01 2026-06-01 Shared token old +daisy\nz 2026-07-01 2026-06-01 Gone for good\n")
	m.initHomeRepo(t)
	todo, today := m.read(t, "tasks/todo.txt"), m.read(t, "journal/today.md")
	for _, extra := range [][]string{{"cancel"}, {"priority", "A"}} {
		run := func(pattern string) (int, string, string) {
			args := append([]string{extra[0], pattern}, extra[1:]...)
			return runWith(m.dayOps(m.ws, true), args...)
		}
		if code, out, errs := run("Zeta"); code != 1 || out != "" || errs != "No task found matching: Zeta\n" {
			t.Errorf("%s no match: exit %d out %q err %q", extra[0], code, out, errs)
		}
		if code, out, errs := run("Gone"); code != 1 || out != "" || errs != "No task found matching: Gone\n" {
			t.Errorf("%s cancelled line: exit %d out %q err %q", extra[0], code, out, errs)
		}
		if code, out, errs := run("shared TOKEN"); code != 1 || out != "" || errs != "Multiple tasks match \"shared TOKEN\":\n  1) (B) 2026-07-27 Shared token Alpha +daisy\n  2) (B) 2026-07-27 Shared token Beta +daisy\n" {
			t.Errorf("%s several: exit %d out %q err %q", extra[0], code, out, errs)
		}
	}
	for _, args := range [][]string{{"cancel"}, {"cancel", ""}, {"cancel", "a", "b"}, {"priority", "Delta"}, {"priority", "Delta", "E"}, {"priority", "Delta", "a"}, {"priority", "", "A"}, {"priority", "Delta", "A", "B"}} {
		if code, _, errs := runWith(m.dayOps(m.ws, true), args...); code != 2 || !strings.Contains(errs, "Usage: daisy "+args[0]+" <pattern>") {
			t.Errorf("%q: exit %d err %q", args, code, errs)
		}
	}
	for _, args := range [][]string{{"cancel", "Delta"}, {"priority", "Delta", "A"}} {
		if code, _, errs := runWith(m.dayOps(m.ws, false), args...); code != 1 || !strings.Contains(errs, "System health check failed") {
			t.Errorf("%v health check failing: exit %d err %q", args, code, errs)
		}
	}
	if m.read(t, "tasks/todo.txt") != todo || m.read(t, "journal/today.md") != today || m.commits(t) != "1" {
		t.Error("a refusal changed a file or committed")
	}
}

// priority rewrites the line with the new priority (creation date and
// text kept) at the new priority's place in todo.txt, and moves today.md's
// line to the section a new day would carry it to: (A) Now, (B) Next, none
// Inbox, (C) and (D) out of today.md; one commit.
func TestPriorityTask(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	code, out, errs := runWith(m.dayOps(m.ws, true), "priority", "gamma", "A")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if out != "✅ Priority: Gamma task +daisy (C) → (A); today.md: moved to Now (commit "+hash+")\n" {
		t.Errorf("stdout %q", out)
	}
	wantTodo := "(A) 2026-07-24 Gamma task +daisy\n" + strings.Replace(todoBasic, "(C) 2026-07-24 Gamma task +daisy\n", "", 1)
	if got := m.read(t, "tasks/todo.txt"); got != wantTodo {
		t.Errorf("todo.txt:\n%s", got)
	}
	wantToday := strings.Replace(todayBasic, "- [ ] Gamma task +daisy\n", "", 1)
	wantToday = strings.Replace(wantToday, "- [ ] Beta task +work\n", "- [ ] Beta task +work\n- [ ] Gamma task +daisy\n", 1)
	if got := m.read(t, "journal/today.md"); got != wantToday {
		t.Errorf("today.md:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s, want exactly one more", m.commits(t))
	}
	if files := m.git(t, "show", "--name-only", "--format=", "HEAD"); files != "journal/today.md\ntasks/todo.txt" {
		t.Errorf("committed %q", files)
	}
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "Priority: Gamma task +daisy (C) → (A)" {
		t.Errorf("message %q", msg)
	}

	for _, tc := range []struct{ pattern, prio, out, todoHas, todayHas, todayLacks string }{
		// out of today.md: (D) is not carried
		{"Alpha", "D", "Alpha task +daisy (B) → (D); today.md: removed, the next new day does not carry it", "(D) 2026-07-20 Epsilon task\n(D) 2026-07-27 Alpha task +daisy\n", "", "- [ ] Alpha task +daisy\n"},
		// to the inbox: a dated task without a priority goes to Inbox
		{"Epsilon", "none", "Epsilon task (D) → none; today.md: moved to Inbox", "(B) 2026-07-27 Delta target task +daisy\n2026-07-20 Epsilon task\n", "**Next:**\n- [ ] Delta target task +daisy\n\n\n**Inbox:**\n- [ ] Workout\n- [ ] Epsilon task\n", ""},
		// not in today.md, now (B): added to Next
		{"Zeta", "B", "Zeta task (C) → (B); today.md: added to Next", "(B) 2026-07-26 Zeta task\n", "**Next:**\n- [ ] Delta target task +daisy\n- [ ] Zeta task\n\n\n**Inbox:**", ""},
	} {
		if tc.pattern == "Zeta" {
			writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), m.read(t, "tasks/todo.txt")+"(C) 2026-07-26 Zeta task\n")
		}
		code, out, errs := runWith(m.dayOps(m.ws, true), "priority", tc.pattern, tc.prio)
		if code != 0 || errs != "" || !strings.HasPrefix(out, "✅ Priority: "+tc.out+" (commit ") {
			t.Errorf("%s %s: exit %d out %q err %q", tc.pattern, tc.prio, code, out, errs)
		}
		if got := m.read(t, "tasks/todo.txt"); !strings.Contains(got, tc.todoHas) {
			t.Errorf("%s %s: todo.txt:\n%s", tc.pattern, tc.prio, got)
		}
		got := m.read(t, "journal/today.md")
		if !strings.Contains(got, tc.todayHas) || tc.todayLacks != "" && strings.Contains(got, tc.todayLacks) {
			t.Errorf("%s %s: today.md:\n%s", tc.pattern, tc.prio, got)
		}
	}

	// not in today.md and not (A) or (B): todo.txt alone, today.md untouched
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), m.read(t, "tasks/todo.txt")+"(D) 2026-07-26 Omicron task\n")
	today := m.read(t, "journal/today.md")
	code, out, errs = runWith(m.dayOps(m.ws, true), "priority", "Omicron", "none")
	if code != 0 || errs != "" || !strings.HasPrefix(out, "✅ Priority: Omicron task (D) → none; today.md: not in it, the next new day carries it to Inbox (commit ") {
		t.Errorf("Omicron: exit %d out %q err %q", code, out, errs)
	}
	if m.read(t, "journal/today.md") != today || !strings.Contains(m.read(t, "tasks/todo.txt"), "\n2026-07-26 Omicron task\n") {
		t.Errorf("Omicron: today.md changed or todo.txt wrong:\n%s", m.read(t, "tasks/todo.txt"))
	}

	// the priority it already has: nothing written, nothing committed
	before, todo := m.commits(t), m.read(t, "tasks/todo.txt")
	code, out, errs = runWith(m.dayOps(m.ws, true), "priority", "Beta", "B")
	if code != 0 || errs != "" || out != "✅ Beta task +work is already (B); nothing changed\n" || m.commits(t) != before || m.read(t, "tasks/todo.txt") != todo {
		t.Errorf("same priority: exit %d out %q err %q", code, out, errs)
	}

	// an undated line carried whole changes its text with its priority
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) Undated urgent\n")
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), strings.Replace(todayBasic, "**Now:**\n", "**Now:**\n- [ ] (A) Undated urgent\n", 1))
	if code, _, errs = runWith(m.dayOps(m.ws, true), "priority", "Undated", "B"); code != 0 || errs != "" {
		t.Errorf("undated: exit %d err %q", code, errs)
	}
	if got := m.read(t, "journal/today.md"); strings.Contains(got, "(A) Undated") || !strings.Contains(got, "- [ ] Epsilon task\n- [ ] (B) Undated urgent\n") || m.read(t, "tasks/todo.txt") != "(B) Undated urgent\n" {
		t.Errorf("undated: today.md:\n%s\ntodo.txt %q", got, m.read(t, "tasks/todo.txt"))
	}
}
