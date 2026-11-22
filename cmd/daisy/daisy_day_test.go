package main_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/cli"
	"daisy/internal/commands"
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

// dayOps is the Fake for the day commands: the clock pinned, and the
// in-process health check passed by DAISY_HEALTHCHECK_PASSED when healthOK;
// otherwise it runs for real, and fails, since the temp root is no
// repository.
func (m *machine) dayOps(cwd string, healthOK bool) *ops.Fake {
	return &ops.Fake{
		GetenvFn: func(k string) string {
			if k == "DAISY_HEALTHCHECK_PASSED" && healthOK {
				return "1"
			}
			return m.vars[k]
		},
		GetwdFn: func() (string, error) { return cwd, nil },
		NowFn:   func() time.Time { return time.Date(2026, 7, 28, 14, 23, 0, 0, time.Local) },
	}
}

func runWith(o ops.Ops, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = cli.Run(o, strings.NewReader(""), &out, &errb, args)
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

// DAISY_CO_AUTHOR, set by an agent, ends each home commit with one
// Co-Authored-By trailer; unset (a person at the keyboard) there is none,
// and a value not shaped `Name <email>` is warned about and left out, the
// commit made all the same. A message already ending with the trailer
// does not get a second, whatever the user's trailer.ifexists says (here
// `add`, which alone would add it again).
func TestHomeCommitsCoAuthor(t *testing.T) {
	const who = "Claude Opus 5.5 <noreply@anthropic.com>"
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	m.git(t, "config", "trailer.ifexists", "add")
	trailers := func() string {
		return m.git(t, "log", "-1", "--format=%(trailers:key=Co-Authored-By,valueonly,separator=|)")
	}
	code, _, errs := runWith(m.dayOps(m.ws, true), "log", "by hand")
	if code != 0 || errs != "" {
		t.Fatalf("unset: exit %d stderr %q", code, errs)
	}
	if body := m.git(t, "log", "-1", "--format=%B"); body != "Log: 1423 - by hand" {
		t.Errorf("unset: message %q", body)
	}
	m.vars["DAISY_CO_AUTHOR"] = who
	code, out, errs := runWith(m.dayOps(m.ws, true), "log", "by an agent")
	if code != 0 || errs != "" {
		t.Fatalf("set: exit %d stderr %q", code, errs)
	}
	if !strings.HasSuffix(out, "📝 Committed: Log: 1423 - by an agent ("+m.git(t, "rev-parse", "--short", "HEAD")+")\n") {
		t.Errorf("set: stdout %q", out)
	}
	if body := m.git(t, "log", "-1", "--format=%B"); body != "Log: 1423 - by an agent\n\nCo-Authored-By: "+who {
		t.Errorf("set: message %q", body)
	}
	if got := trailers(); got != who {
		t.Errorf("set: trailers %q", got)
	}
	writeFile(t, filepath.Join(m.home, "tasks", "inbox.txt"), "x\n")
	code, _, errs = runWith(m.dayOps(m.ws, true), "commit", "Add inbox\n\nCo-Authored-By: "+who)
	if code != 0 || errs != "" {
		t.Fatalf("commit: exit %d stderr %q", code, errs)
	}
	if got := trailers(); got != who {
		t.Errorf("a message holding the trailer: trailers %q", got)
	}
	for i, bad := range []string{"Claude Opus 5.5", "noreply@anthropic.com", "Claude <noreply@anthropic.com>\nCo-Authored-By: Other <o@x>", "<noreply@anthropic.com>"} {
		m.vars["DAISY_CO_AUTHOR"] = bad
		before := m.commits(t)
		code, _, errs = runWith(m.dayOps(m.ws, true), "log", "bad "+string(rune('a'+i)))
		if code != 0 || m.commits(t) == before {
			t.Errorf("%q: exit %d, commits %s, stderr %q", bad, code, m.commits(t), errs)
		}
		if !strings.Contains(errs, "DAISY_CO_AUTHOR") || !strings.Contains(errs, "Name <email>") {
			t.Errorf("%q: stderr %q", bad, errs)
		}
		if got := trailers(); got != "" {
			t.Errorf("%q: trailers %q", bad, got)
		}
	}
}

// --co-author "Name <email>", leading a committing command's arguments,
// ends its commit with exactly that one trailer, over DAISY_CO_AUTHOR
// (which is then not read: a malformed one draws no warning). A value of
// another shape is a usage error before anything is written; a command
// that does not commit does not take the flag.
func TestCoAuthorFlag(t *testing.T) {
	const who = "X <x@y>"
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	body := func() string { return m.git(t, "log", "-1", "--format=%B") }
	code, _, errs := runWith(m.dayOps(m.ws, true), "log", "--co-author", who, "m")
	if code != 0 || errs != "" {
		t.Fatalf("log: exit %d stderr %q", code, errs)
	}
	if got := body(); got != "Log: 1423 - m\n\nCo-Authored-By: "+who {
		t.Errorf("log: message %q", got)
	}
	for _, env := range []string{"Claude Opus 5.5 <noreply@anthropic.com>", "not a line"} {
		m.vars["DAISY_CO_AUTHOR"] = env
		code, _, errs = runWith(m.dayOps(m.ws, true), "log", "--co-author", who, "over", env)
		if code != 0 || errs != "" {
			t.Fatalf("over %q: exit %d stderr %q", env, code, errs)
		}
		if got := body(); got != "Log: 1423 - over "+env+"\n\nCo-Authored-By: "+who {
			t.Errorf("over %q: message %q", env, got)
		}
	}
	delete(m.vars, "DAISY_CO_AUTHOR")
	code, _, errs = runWith(m.dayOps(m.ws, true), "done", "--co-author", who, "Delta")
	if code != 0 || body() != "Completed: Delta target task +daisy\n\nCo-Authored-By: "+who {
		t.Errorf("done: exit %d stderr %q message %q", code, errs, body())
	}
	writeFile(t, filepath.Join(m.home, "tasks", "inbox.txt"), "x\n")
	code, _, errs = runWith(m.dayOps(m.ws, true), "commit", "--co-author", who, "Add inbox")
	if code != 0 || body() != "Add inbox\n\nCo-Authored-By: "+who {
		t.Errorf("commit: exit %d stderr %q message %q", code, errs, body())
	}
	today := m.read(t, "journal/today.md")
	for _, bad := range [][]string{{"X"}, {"x@y"}, {"<x@y>"}, {" X <x@y>"}, {"X <x@y>\nCo-Authored-By: Y <y@z>"}, {"X", "--co-author", who}} {
		before := m.commits(t)
		code, out, errs := runWith(m.dayOps(m.ws, true), append(append([]string{"log", "--co-author"}, bad...), "bad")...)
		if code != 2 || out != "" || !strings.HasPrefix(errs, "Error: --co-author ") || !strings.Contains(errs, "'Name <email>'") || !strings.Contains(errs, "Usage: daisy log") {
			t.Errorf("%q: exit %d stdout %q stderr %q", bad, code, out, errs)
		}
		if m.commits(t) != before || m.read(t, "journal/today.md") != today {
			t.Errorf("%q: written or committed", bad)
		}
	}
	code, _, errs = runWith(m.dayOps(m.ws, true), "tasks", "--co-author", who)
	if code != 2 || !strings.HasPrefix(errs, "Error: unknown flag: --co-author\n") {
		t.Errorf("tasks: exit %d stderr %q", code, errs)
	}
	_, out, _ := runWith(m.dayOps(m.ws, true), "log", "--help")
	if !strings.Contains(out, "--co-author \"Name <email>\"") || !strings.Contains(out, "DAISY_CO_AUTHOR") {
		t.Errorf("log --help: %q", out)
	}
	if _, out, _ = runWith(m.dayOps(m.ws, true), "tasks", "--help"); strings.Contains(out, "--co-author") {
		t.Errorf("tasks --help: %q", out)
	}
}

// Every command the table marks as committing, driven with --co-author,
// ends the commit it makes with that one trailer. The table is read, not
// listed here: a committing command without a case below fails this test
// until it gets one.
func TestEveryCommittingCommandCarriesTheTrailer(t *testing.T) {
	const who = "X <x@y>"
	type run struct {
		o     ops.Ops
		stdin string
		repo  string // where the commit lands
	}
	day := func(t *testing.T, at time.Time) run {
		m := newMachine(t)
		m.seed(t)
		m.initHomeRepo(t)
		o := m.dayOps(m.ws, true)
		if !at.IsZero() {
			o.NowFn = func() time.Time { return at }
		}
		return run{o: o, repo: m.home}
	}
	cases := map[string]struct {
		setup func(t *testing.T) run
		args  []string
	}{
		"log":      {func(t *testing.T) run { return day(t, time.Time{}) }, []string{"m"}},
		"add":      {func(t *testing.T) run { return day(t, time.Time{}) }, []string{"New task", "C"}},
		"done":     {func(t *testing.T) run { return day(t, time.Time{}) }, []string{"Delta"}},
		"cancel":   {func(t *testing.T) run { return day(t, time.Time{}) }, []string{"Gamma"}},
		"priority": {func(t *testing.T) run { return day(t, time.Time{}) }, []string{"Gamma", "A"}},
		"new-day":  {func(t *testing.T) run { return day(t, time.Date(2026, 7, 29, 8, 0, 0, 0, time.Local)) }, nil},
		"new-week": {func(t *testing.T) run { return day(t, time.Date(2026, 8, 3, 8, 0, 0, 0, time.Local)) }, nil},
		"commit": {func(t *testing.T) run {
			r := day(t, time.Time{})
			writeFile(t, filepath.Join(r.repo, "tasks", "inbox.txt"), "x\n")
			return r
		}, []string{"Add inbox"}},
		"plan pickup": {func(t *testing.T) run {
			m := newMachine(t)
			m.initHomeRepo(t)
			writeFile(t, filepath.Join(m.ws, "x_PLAN.md"), "# A draft — +testproj\n")
			return run{o: m.planOps(m.ws), repo: m.home}
		}, []string{"x_PLAN.md"}},
		"plan archive": {func(t *testing.T) run {
			m := newMachine(t)
			m.archiveFixture(t)
			return run{o: m.planOps(m.ws), repo: m.home}
		}, nil},
		"home new": {func(t *testing.T) run {
			m := newMachine(t)
			return run{o: m.planOps(m.ws), stdin: "Jane Doe\njane@example.net\n", repo: filepath.Join(m.config, "fresh")}
		}, []string{"fresh", "<repo>"}},
		"optimize apply": {func(t *testing.T) run {
			l := newLoop(t)
			return run{o: l.ops(), repo: l.home}
		}, []string{"--workflow", "widgetflow", "--rewrite", "<proposal>", "--yes"}},
	}
	rowRe := regexp.MustCompile("^\\| [^|]+ \\| `daisy ([a-z -]+?)(?: [^a-z]|`)")
	seen := map[string]bool{}
	for _, l := range strings.Split(cli.Reference(), "\n") {
		row := rowRe.FindStringSubmatch(l)
		if row == nil {
			continue
		}
		name := strings.TrimSpace(row[1])
		if flags, _, _ := cli.Flags(name); !slices.Contains(flags, commands.CoAuthorFlag) {
			continue
		}
		seen[name] = true
		tc, ok := cases[name]
		if !ok {
			t.Errorf("%s commits, and this test has no case for it: add one", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			r := tc.setup(t)
			args := append(strings.Fields(name), commands.CoAuthorFlag, who)
			for _, a := range tc.args {
				switch a {
				case "<repo>":
					a = r.repo
				case "<proposal>":
					a = filepath.Join(t.TempDir(), "new.md")
					writeFile(t, a, "## Trigger\n\nNew.\n")
				}
				args = append(args, a)
			}
			var out, errb bytes.Buffer
			if code := cli.Run(r.o, strings.NewReader(r.stdin), &out, &errb, args); code != 0 {
				t.Fatalf("%q: exit %d stdout %q stderr %q", args, code, out.String(), errb.String())
			}
			got, errs, err := ops.Real().Exec(r.repo, "git", "log", "-1", "--format=%(trailers:key=Co-Authored-By,valueonly,separator=|)")
			if err != nil {
				t.Fatalf("git log: %v %s", err, errs)
			}
			if strings.TrimSpace(string(got)) != who {
				t.Errorf("%q: trailers %q, want %q", args, got, who)
			}
		})
	}
	for name := range cases {
		if !seen[name] {
			t.Errorf("%s has a case here but does not commit: drop it", name)
		}
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
	// the completed line goes before the cancelled ones (docs/task-format.md)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) 2026-07-27 Open task\nx 2026-07-28 2026-07-01 Earlier done\nz 2026-07-28 2026-07-02 Dropped\n")
	if code, _, errs := runWith(m.dayOps(m.ws, true), "done", "Open task"); code != 0 {
		t.Fatalf("with a z line: exit %d err %q", code, errs)
	}
	if got := m.read(t, "tasks/todo.txt"); got != "x 2026-07-28 2026-07-01 Earlier done\nx 2026-07-28 2026-07-27 Open task\nz 2026-07-28 2026-07-02 Dropped\n" {
		t.Errorf("with a z line: todo.txt:\n%s", got)
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
	for _, args := range [][]string{{"done"}, {"done", ""}, {"done", "a", "b"}, {"done", "--dry-run"}} {
		if code, _, errs := runWith(m.dayOps(m.ws, true), args...); code != 2 || !strings.Contains(errs, "Usage: daisy done <pattern>") {
			t.Errorf("%v: exit %d err %q", args, code, errs)
		}
	}
	// A pattern with a dash and more than a flag's shape is a pattern.
	if code, _, errs := runWith(m.dayOps(m.ws, true), "done", "--dry-run support"); code != 1 || errs != "No task found matching: --dry-run support\n" {
		t.Errorf("--dry-run support: exit %d err %q", code, errs)
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
	} // as new-day carries it: an undated (A) line whole
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) Beta undated urgent\n")
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), todayBasic+"- [ ] (A) Beta undated urgent\n")
	if code, _, errs = runWith(m.dayOps(m.ws, true), "done", "Beta"); code != 0 || errs != "" || !strings.Contains(m.read(t, "journal/today.md"), "- [x] (A) Beta undated urgent\n") {
		t.Errorf("carried whole: exit %d err %q today %q", code, errs, m.read(t, "journal/today.md"))
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

// Through Run with every git call scripted: one `git commit` per done, log,
// new-day and new-week, and the commit lands in the home's repository.
func TestOneCommitPerMutationScripted(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	homeReal, _ := filepath.EvalSymlinks(m.home)
	os.MkdirAll(filepath.Join(m.home, ".git"), 0o755) // the scripted repository's git directory
	for _, args := range [][]string{{"done", "Delta"}, {"log", "a line"}, {"new-day"}, {"new-week"}} {
		m.seed(t)
		var calls []string
		o := m.dayOps(m.ws, true)
		if args[0] == "new-day" || args[0] == "new-week" { // the next week: a period not yet started
			o.NowFn = func() time.Time { return time.Date(2026, 8, 3, 9, 0, 0, 0, time.Local) }
		}
		o.ExecFn = func(dir, name string, args ...string) ([]byte, []byte, error) {
			if name != "git" {
				return nil, nil, nil // the health check
			}
			calls = append(calls, dir+": git "+strings.Join(args, " "))
			switch args[0] {
			case "rev-parse":
				switch args[1] {
				case "--show-toplevel":
					return []byte(homeReal + "\n"), nil, nil
				case "--path-format=absolute":
					if dir != m.home {
						return nil, nil, errors.New("not a repository")
					}
					return []byte(homeReal + "/.git\n"), nil, nil
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
			if strings.HasPrefix(c, m.home+": git -c trailer.ifexists=addIfDifferent commit") {
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
	// a .daisy/project naming a missing project is said on stderr, and
	// under warnings with --json, as status and tasks say it
	writeFile(t, filepath.Join(m.ws, ".daisy", "project"), "ghost\n")
	warning := "⚠️  .daisy/project names 'ghost', but " + filepath.Join(m.home, "projects", "ghost.md") + " does not exist"
	if code, out, errs := runWith(m.dayOps(m.ws, true), "projects"); code != 0 || errs != warning+"\n" || !strings.HasPrefix(out, "Active projects:\n") {
		t.Errorf("a missing project: exit %d out %q err %q", code, out, errs)
	}
	code, out, errs = runWith(m.dayOps(m.ws, true), "projects", "--json")
	var doc struct{ Warnings []string }
	if err := json.Unmarshal([]byte(out), &doc); code != 0 || errs != "" || err != nil || !slices.Equal(doc.Warnings, []string{warning}) {
		t.Errorf("--json: exit %d out %q err %q (%v)", code, out, errs, err)
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

// projects lists the `*.md` glob: no dotfile, in the files' byte order
// (`a-b.md` before `a.md`), as the bash's glob did.
func TestProjectsIsTheGlob(t *testing.T) {
	m := newMachine(t)
	for _, f := range []string{"a.md", "a-b.md", ".draft.md"} {
		writeFile(t, filepath.Join(m.home, "projects", f), "")
	}
	real := env.RealPath(ops.Real().FS(), m.home)
	code, out, errs := runWith(m.dayOps(m.ws, true), "projects")
	want := "Active projects:\n  a-b                            " + real + "/projects/a-b.md\n  a                              " + real + "/projects/a.md\n"
	if code != 0 || out != want {
		t.Errorf("exit %d err %q\ngot\n%s\nwant\n%s", code, errs, out, want)
	}
}
