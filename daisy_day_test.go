package daisy_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy"
	"daisy/internal/env"
	"daisy/internal/ops"
)

const todayBasic = `### 2026-07-28 Tuesday

#### Agenda
- 0900 Plan Day

#### Tasks

**Now:**
- [ ] Alpha task +daisy
- [ ] Beta task +work

**Next:**
- [ ] Gamma task +daisy
- [ ] Delta target task +daisy
- [ ] Epsilon task


**Inbox:**
- [ ] Workout


**GitHub PRs:**


#### Log

- 0900 New day started

#### Retrospective

* **Successes:**
* **Misses:**
* **What would a Sage do next:**
`

const todoBasic = `(B) 2026-07-27 Alpha task +daisy
(B) 2026-07-27 Beta task +work
(C) 2026-07-24 Gamma task +daisy
(B) 2026-07-27 Delta target task +daisy
(D) 2026-07-20 Epsilon task
`

// git makes the home its own repository with an identity and one commit, as
// the bash fixture does.
func (m *machine) git(t *testing.T, args ...string) string {
	t.Helper()
	out, errb, err := ops.Real().Exec(m.home, "git", args...)
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, errb)
	}
	return strings.TrimSpace(string(out))
}

func (m *machine) initHomeRepo(t *testing.T) {
	t.Helper()
	m.git(t, "init", "-q")
	m.git(t, "config", "user.name", "Fixture Author")
	m.git(t, "config", "user.email", "fixture@example.invalid")
	m.git(t, "add", "-A")
	m.git(t, "commit", "-q", "-m", "fixture: initial home")
}

func (m *machine) commits(t *testing.T) string { return m.git(t, "rev-list", "--count", "HEAD") }

func (m *machine) seed(t *testing.T) {
	t.Helper()
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), todayBasic)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), todoBasic)
}

func (m *machine) read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(m.home, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// dayOps is the Fake for log and done: the clock pinned, and the bash health
// check (a script the temp root does not have) answered by healthOK.
func (m *machine) dayOps(cwd string, healthOK bool) *ops.Fake {
	return &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		GetwdFn:  func() (string, error) { return cwd, nil },
		NowFn:    func() time.Time { return time.Date(2026, 7, 28, 14, 23, 0, 0, time.Local) },
		ExecFn: func(dir, name string, args ...string) ([]byte, []byte, error) {
			if strings.HasSuffix(name, "/daisy/scripts/healthcheck.sh") {
				if dir != m.ws && dir != cwd {
					return nil, nil, errors.New("health check run outside the workspace: " + dir)
				}
				if healthOK {
					return nil, nil, nil
				}
				return nil, []byte("✗ broken"), errors.New("exit status 1")
			}
			return ops.Real().Exec(dir, name, args...)
		},
	}
}

func runWith(o ops.Ops, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = daisy.Run(o, strings.NewReader(""), &out, &errb, args)
	return code, out.String(), errb.String()
}

func TestLogAppendsAndCommitsOnce(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	code, out, errs := runWith(m.dayOps(m.ws, true), "log", "Shipped", "the fix", "~mel")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if out != "✅ Logged: 1423 Shipped the fix ~mel\n📝 Committed: Log: 1423 - Shipped the fix ~mel ("+hash+")\n" {
		t.Errorf("stdout %q", out)
	}
	want := strings.Replace(todayBasic, "- 0900 New day started\n", "- 0900 New day started\n- 1423 Shipped the fix ~mel\n", 1)
	if got := m.read(t, "journal/today.md"); got != want {
		t.Errorf("today.md:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s, want exactly one more", m.commits(t))
	}
	if files := m.git(t, "show", "--name-only", "--format=", "HEAD"); files != "journal/today.md" {
		t.Errorf("committed %q", files)
	}
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "Log: 1423 - Shipped the fix ~mel" {
		t.Errorf("message %q", msg)
	}
	// the summary is the first 60 characters of the message
	long := strings.Repeat("abcdefghij", 7)
	runWith(m.dayOps(m.ws, true), "log", long)
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "Log: 1423 - "+long[:60] {
		t.Errorf("long message %q", msg)
	}
}

func TestLogRefusals(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	for _, args := range [][]string{{"log"}, {"log", ""}, {"log", "", ""}} {
		code, out, errs := runWith(m.dayOps(m.ws, true), args...)
		if code != 2 || out != "" || !strings.Contains(errs, "Usage: daisy log <message...>") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	code, out, errs := runWith(m.dayOps(m.ws, true), "log", "", "kept")
	if code != 0 || !strings.HasPrefix(out, "✅ Logged: 1423 kept\n") {
		t.Errorf("empty word dropped: exit %d out %q err %q", code, out, errs)
	}
	code, out, errs = runWith(m.dayOps(m.ws, false), "log", "x")
	if code != 1 || out != "" || errs != "Error: System health check failed\n  Run: daisy healthcheck\n" {
		t.Errorf("health check failing: exit %d out %q err %q", code, out, errs)
	}
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), "#### Tasks\n- [ ] a\n")
	code, _, errs = runWith(m.dayOps(m.ws, true), "log", "x")
	if code != 1 || errs != "Error: Log section not found in today.md\n" {
		t.Errorf("no Log section: exit %d err %q", code, errs)
	}
	os.Remove(filepath.Join(m.home, "journal", "today.md"))
	code, _, errs = runWith(m.dayOps(m.ws, true), "log", "x")
	if code != 1 || errs != "Error: today.md not found\n" {
		t.Errorf("no today.md: exit %d err %q", code, errs)
	}
	if m.commits(t) != "2" { // the one `kept` entry, nothing from a refusal
		t.Errorf("a refusal committed: %s commits", m.commits(t))
	}
}

func TestDoneCompletesAndCommitsOnce(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	code, out, errs := runWith(m.dayOps(m.ws, true), "done", "delta TARGET")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if out != "✅ Done: Delta target task +daisy (commit "+hash+")\n" {
		t.Errorf("stdout %q", out)
	}
	wantTodo := strings.Replace(todoBasic, "(B) 2026-07-27 Delta target task +daisy\n", "", 1) + "x 2026-07-28 2026-07-27 Delta target task +daisy\n"
	if got := m.read(t, "tasks/todo.txt"); got != wantTodo {
		t.Errorf("todo.txt:\n%s", got)
	}
	wantToday := strings.Replace(todayBasic, "- [ ] Delta target task +daisy\n", "- [x] Delta target task +daisy\n", 1)
	wantToday = strings.Replace(wantToday, "- 0900 New day started\n", "- 0900 New day started\n- 1423 Done: Delta target task +daisy\n", 1)
	if got := m.read(t, "journal/today.md"); got != wantToday {
		t.Errorf("today.md:\n%s", got)
	}
	if m.commits(t) != "2" {
		t.Errorf("commits %s, want exactly one more", m.commits(t))
	}
	if files := m.git(t, "show", "--name-only", "--format=", "HEAD"); files != "journal/today.md\ntasks/todo.txt" {
		t.Errorf("committed %q", files)
	}
	if msg := m.git(t, "log", "-1", "--format=%s"); msg != "Completed: Delta target task +daisy" {
		t.Errorf("message %q", msg)
	}
}

func TestDoneRefusals(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), todoBasic+"(B) 2026-07-27 Shared token Alpha +daisy\n(B) 2026-07-27 Shared token Beta +daisy\nx 2026-07-01 2026-06-01 Shared token old +daisy\n")
	todo, today := m.read(t, "tasks/todo.txt"), m.read(t, "journal/today.md")
	code, out, errs := runWith(m.dayOps(m.ws, true), "done", "Zeta")
	if code != 1 || out != "" || errs != "No task found matching: Zeta\n" {
		t.Errorf("no match: exit %d out %q err %q", code, out, errs)
	}
	code, out, errs = runWith(m.dayOps(m.ws, true), "done", "shared TOKEN")
	if code != 1 || out != "" || errs != "Multiple tasks match \"shared TOKEN\":\n  1) (B) 2026-07-27 Shared token Alpha +daisy\n  2) (B) 2026-07-27 Shared token Beta +daisy\n" {
		t.Errorf("several: exit %d out %q err %q", code, out, errs)
	}
	for _, args := range [][]string{{"done"}, {"done", ""}, {"done", "a", "b"}} {
		if code, _, errs := runWith(m.dayOps(m.ws, true), args...); code != 2 || !strings.Contains(errs, "Usage: daisy done <pattern>") {
			t.Errorf("%v: exit %d err %q", args, code, errs)
		}
	}
	code, _, errs = runWith(m.dayOps(m.ws, false), "done", "Delta")
	if code != 1 || !strings.Contains(errs, "System health check failed") {
		t.Errorf("health check failing: exit %d err %q", code, errs)
	}
	if m.read(t, "tasks/todo.txt") != todo || m.read(t, "journal/today.md") != today || m.commits(t) != "1" {
		t.Error("a refusal changed a file or committed")
	}
}

func TestDoneWithoutTheTaskInToday(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(B) 2026-07-27 Ghost task not in journal +daisy\n")
	code, out, errs := runWith(m.dayOps(m.ws, true), "done", "Ghost")
	if code != 0 || !strings.HasPrefix(out, "✅ Done: Ghost task not in journal +daisy (commit ") || errs != "⚠️  Task not found in today.md (may not be in active tasks)\n" {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
	if !strings.HasPrefix(m.read(t, "tasks/todo.txt"), "x 2026-07-28 2026-07-27 Ghost task not in journal +daisy\n") || m.commits(t) != "2" {
		t.Error("todo.txt not completed and committed")
	}
	// an undated line: the description is still the text, not the priority
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) Alpha task +daisy\n")
	code, _, errs = runWith(m.dayOps(m.ws, true), "done", "Alpha")
	if code != 0 || errs != "" || !strings.Contains(m.read(t, "journal/today.md"), "- [x] Alpha task +daisy\n") || m.read(t, "tasks/todo.txt") != "x 2026-07-28 Alpha task +daisy\n" {
		t.Errorf("undated: exit %d err %q todo %q", code, errs, m.read(t, "tasks/todo.txt"))
	}
}

func TestDoneJournalFailureStillCommits(t *testing.T) {
	m := newMachine(t)
	m.initHomeRepo(t)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(B) 2026-07-27 Ship widget without log section +daisy\n")
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), "**Now:**\n- [ ] Ship widget without log section +daisy\n")
	code, out, errs := runWith(m.dayOps(m.ws, true), "done", "Ship widget")
	hash := m.git(t, "rev-parse", "--short", "HEAD")
	if code != 1 || out != "" {
		t.Errorf("exit %d out %q", code, out)
	}
	if errs != "Error: journal entry failed — todo.txt and today.md are committed ("+hash+"), but nothing was recorded in today.md's #### Log section\nError: Log section not found in today.md\n" {
		t.Errorf("stderr %q", errs)
	}
	if !strings.HasPrefix(m.read(t, "tasks/todo.txt"), "x 2026-07-28 2026-07-27 Ship widget") || !strings.Contains(m.read(t, "journal/today.md"), "- [x] Ship widget") || m.commits(t) != "2" {
		t.Error("the edits were not made and committed")
	}
}

// Through Run with every git call scripted: one `git commit` per done and
// per log, and the commit lands in the home's repository.
func TestOneCommitPerMutationScripted(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	homeReal, _ := filepath.EvalSymlinks(m.home)
	for _, args := range [][]string{{"done", "Delta"}, {"log", "a line"}} {
		m.seed(t)
		var calls []string
		o := m.dayOps(m.ws, true)
		o.ExecFn = func(dir, name string, args ...string) ([]byte, []byte, error) {
			if name != "git" {
				return nil, nil, nil // the health check
			}
			calls = append(calls, dir+": git "+strings.Join(args, " "))
			switch args[0] {
			case "rev-parse":
				if args[1] == "--show-toplevel" {
					return []byte(homeReal + "\n"), nil, nil
				}
				return []byte("abc1234\n"), nil, nil
			case "status":
				return []byte(" M tasks/todo.txt\n"), nil, nil
			}
			return nil, nil, nil
		}
		code, out, errs := runWith(o, args...)
		if code != 0 || errs != "" || !strings.Contains(out, "abc1234") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
		commits := 0
		for _, c := range calls {
			if strings.HasPrefix(c, m.home+": git commit") {
				commits++
			}
		}
		if commits != 1 {
			t.Errorf("%v made %d commits: %q", args, commits, calls)
		}
	}
}

func TestTasks(t *testing.T) {
	m := newMachine(t)
	proj := filepath.Join(m.ws, "proj.260807")
	writeFile(t, filepath.Join(proj, ".daisy", "home"), "testhome\n")
	for _, l := range env.Links {
		os.Symlink(filepath.Join(m.home, l.Target), filepath.Join(proj, ".daisy", l.Link))
	}
	writeFile(t, filepath.Join(m.home, "projects", "proj.md"), "# proj\n")
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(B) 2026-07-27 Task one +proj\n(C) 2026-07-20 Task two +other\n\n(B) 2026-07-27 -1 point deduction task +proj\n(B) 2026-07-27 Release version 1.0 +PROJ\n(B) 2026-07-27 Release version 1X0 +proj-dev\n")
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "x 2026-07-01 2026-06-01 Old completed +proj\nx 2026-07-02 2026-06-02 Unrelated +other")
	for _, tc := range []struct {
		args        []string
		code        int
		out, errHas string
	}{
		{[]string{"tasks"}, 0, "(B) 2026-07-27 Task one +proj\n(B) 2026-07-27 -1 point deduction task +proj\n(B) 2026-07-27 Release version 1.0 +PROJ\nx 2026-07-01 2026-06-01 Old completed +proj\n", ""},
		{[]string{"tasks", "--all"}, 0, "(B) 2026-07-27 Task one +proj\n(C) 2026-07-20 Task two +other\n(B) 2026-07-27 -1 point deduction task +proj\n(B) 2026-07-27 Release version 1.0 +PROJ\n(B) 2026-07-27 Release version 1X0 +proj-dev\nx 2026-07-01 2026-06-01 Old completed +proj\nx 2026-07-02 2026-06-02 Unrelated +other\n", ""},
		{[]string{"tasks", "--all", "--todo", "+other"}, 0, "(C) 2026-07-20 Task two +other\n", ""},
		{[]string{"tasks", "--done"}, 0, "x 2026-07-01 2026-06-01 Old completed +proj\n", ""},
		{[]string{"tasks", "--todo", "-1 point"}, 0, "(B) 2026-07-27 -1 point deduction task +proj\n", ""},
		{[]string{"tasks", "--todo", "version 1.0"}, 0, "(B) 2026-07-27 Release version 1.0 +PROJ\n", ""},
		{[]string{"tasks", "--todo", "Nonexistent"}, 1, "", "No tasks match"},
		{[]string{"tasks", "a", "b"}, 2, "", "Usage: daisy tasks"},
		{[]string{"tasks", "--done", ""}, 0, "x 2026-07-01 2026-06-01 Old completed +proj\n", ""},
	} {
		code, out, errs := runWith(m.dayOps(proj, true), tc.args...)
		if code != tc.code || out != tc.out || (tc.errHas == "" && errs != "") || !strings.Contains(errs, tc.errHas) {
			t.Errorf("%v: exit %d\nout  %q\nwant %q\nerr %q", tc.args, code, out, tc.out, errs)
		}
	}
	os.Remove(filepath.Join(m.home, "tasks", "done.txt"))
	if code, out, errs := runWith(m.dayOps(proj, true), "tasks", "--all", "+proj"); code != 0 || errs != "" || !strings.Contains(out, "Task one") {
		t.Errorf("without done.txt: exit %d out %q err %q", code, out, errs)
	}
	// an unbound workspace lists everything and says so
	code, out, errs := runWith(m.dayOps(m.ws, true), "tasks")
	if code != 0 || !strings.Contains(out, "Task two +other") || errs != "⚠️  This workspace resolves to no project; listing every project's tasks.\n  Bind it by creating .daisy/project, or pass --all to ask for this deliberately.\n" {
		t.Errorf("unbound: exit %d out %q err %q", code, out, errs)
	}
	os.Remove(filepath.Join(m.home, "tasks", "todo.txt"))
	if code, out, errs := runWith(m.dayOps(m.ws, true), "tasks", "--all"); code != 1 || out != "" || !strings.Contains(errs, "todo.txt") {
		t.Errorf("no files: exit %d out %q err %q", code, out, errs)
	}
}

func TestProjects(t *testing.T) {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.home, "projects", "beta.md"), "")
	writeFile(t, filepath.Join(m.home, "projects", "alpha.md"), "")
	writeFile(t, filepath.Join(m.home, "projects", "notes.txt"), "")
	real := env.RealPath(ops.Real().FS(), m.home)
	code, out, errs := runWith(m.dayOps(m.ws, true), "projects")
	if code != 0 || errs != "" || out != "Active projects:\n  alpha                          "+real+"/projects/alpha.md\n  beta                           "+real+"/projects/beta.md\n" {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
	code, out, _ = runWith(m.dayOps(m.ws, true), "projects", "--archived")
	if code != 0 || out != "Archived projects: none\n" {
		t.Errorf("archived: exit %d out %q", code, out)
	}
	if code, _, errs := runWith(m.dayOps(m.ws, true), "projects", "x"); code != 2 || !strings.Contains(errs, "Usage: daisy projects") {
		t.Errorf("surplus argument: exit %d err %q", code, errs)
	}
}

// --healthcheck as the sole argument runs the command's own precondition
// check: nothing on success, one Error line and exit 1 otherwise.
func TestHealthcheckHooks(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	for _, cmd := range []string{"done", "log", "tasks", "projects"} {
		if code, out, errs := runWith(m.dayOps(m.ws, true), cmd, "--healthcheck"); code != 0 || out != "" || errs != "" {
			t.Errorf("%s --healthcheck: exit %d out %q err %q", cmd, code, out, errs)
		}
	}
	os.Remove(filepath.Join(m.home, "tasks", "todo.txt"))
	if code, _, errs := runWith(m.dayOps(m.ws, true), "done", "--healthcheck"); code != 1 || errs != "Error: todo.txt not found\n" {
		t.Errorf("done without todo.txt: exit %d err %q", code, errs)
	}
	os.Remove(filepath.Join(m.home, "journal", "today.md"))
	for _, cmd := range []string{"done", "log"} {
		if code, _, errs := runWith(m.dayOps(m.ws, true), cmd, "--healthcheck"); code != 1 || errs != "Error: today.md not found\n" {
			t.Errorf("%s without today.md: exit %d err %q", cmd, code, errs)
		}
	}
	os.RemoveAll(filepath.Join(m.home, "tasks"))
	os.RemoveAll(filepath.Join(m.home, "projects"))
	if code, _, errs := runWith(m.dayOps(m.ws, true), "tasks", "--healthcheck"); code != 1 || errs != "Error: tasks directory not found\n" {
		t.Errorf("tasks: exit %d err %q", code, errs)
	}
	if code, _, errs := runWith(m.dayOps(m.ws, true), "projects", "--healthcheck"); code != 1 || errs != "Error: projects directory not found\n" {
		t.Errorf("projects: exit %d err %q", code, errs)
	}
	if code, _, _ := runWith(m.dayOps(m.ws, true), "status", "--healthcheck"); code != 2 {
		t.Errorf("status has no hook: exit %d", code)
	}
}
