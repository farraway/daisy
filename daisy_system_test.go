package daisy_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/cli"
	"daisy/internal/health"
	"daisy/internal/ops"
)

// rootRepo makes the temp root a repository with the directories the
// health check asks for and AGENTS.md holding the generated Command
// Reference.
func (m *machine) rootRepo(t *testing.T) {
	t.Helper()
	for _, d := range []string{"prompts", "skills"} {
		writeFile(t, filepath.Join(m.root, d, ".keep"), "")
	}
	writeFile(t, filepath.Join(m.root, "AGENTS.md"), "# Root\n\n"+cli.Reference())
	writeFile(t, filepath.Join(m.root, "tests", "run.sh"), "#!/bin/sh\n")
	os.Chmod(filepath.Join(m.root, "tests", "run.sh"), 0o755)
	m.gitIn(t, m.root, "init", "-q")
	m.gitIn(t, m.root, "config", "user.name", "Fixture Author")
	m.gitIn(t, m.root, "config", "user.email", "fixture@example.invalid")
	m.gitIn(t, m.root, "add", "-A")
	m.gitIn(t, m.root, "commit", "-q", "-m", "fixture: initial root")
}

func (m *machine) gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errb, err := ops.Real().Exec(dir, "git", args...)
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, errb)
	}
	return strings.TrimSpace(string(out))
}

// The health check runs every table hook in-process, in table order, under
// the command's name; `files` has none, and nothing is exec'd but git.
func TestHealthcheckComponentsFromTheTable(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	m.rootRepo(t)
	writeFile(t, filepath.Join(m.home, "tasks", "done.txt"), "")
	o := m.dayOps(m.ws, false)
	o.ExecFn = func(dir, name string, args ...string) ([]byte, []byte, error) {
		if name != "git" {
			t.Errorf("health check exec'd %s %v", name, args)
		}
		return ops.Real().Exec(dir, name, args...)
	}
	code, out, errs := runWith(o, "healthcheck", "--force")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q\n%s", code, errs, out)
	}
	var got []string
	for _, l := range strings.Split(out, "\n") {
		if _, name, ok := strings.Cut(l, "Component: "); ok {
			got = append(got, name)
		}
	}
	want := "list|new-day|new-week|log|done|tasks|projects|rotate|plan new|plan pickup|plan archive|home new|feedback|optimize|eval|test"
	if strings.Join(got, "|") != want {
		t.Errorf("components %q", got)
	}
	if _, err := os.Stat(filepath.Join(m.home, health.StampName)); err != nil {
		t.Error("a pass wrote no stamp")
	}
	// A hook failing is reported with its message; the run fails.
	os.Remove(filepath.Join(m.home, "tasks", "done.txt"))
	code, _, errs = runWith(o, "healthcheck", "--force")
	if code != 1 || !strings.Contains(errs, "Component: new-week failed health check\n  Error: done.txt not found\n") {
		t.Errorf("exit %d stderr %q", code, errs)
	}
	if code, _, errs := runWith(o, "healthcheck", "extra"); code != 2 || !strings.Contains(errs, "Usage: daisy healthcheck [--force]") {
		t.Errorf("surplus: exit %d %q", code, errs)
	}
}

// The pre-flight of a day command is the same check, in-process: a root
// that is no repository refuses `log` before anything is written.
func TestPreflightIsInProcess(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	m.rootRepo(t)
	before := m.read(t, "journal/today.md")
	o := m.dayOps(m.ws, false)
	if code, out, errs := runWith(o, "log", "x"); code != 0 || !strings.Contains(out, "Logged") {
		t.Fatalf("healthy: exit %d out %q err %q", code, out, errs)
	}
	os.RemoveAll(filepath.Join(m.root, "prompts"))
	os.Remove(filepath.Join(m.home, health.StampName))
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), before)
	if code, _, errs := runWith(o, "log", "y"); code != 1 || errs != "Error: System health check failed\n  Run: daisy healthcheck\n" {
		t.Errorf("unhealthy: exit %d err %q", code, errs)
	}
	if m.read(t, "journal/today.md") != before {
		t.Error("a refused log wrote today.md")
	}
	// A passed check (cached, or the override) never lets a mutation write
	// into a home that cannot commit.
	os.Rename(filepath.Join(m.home, ".git"), filepath.Join(m.home, "git-aside"))
	for _, args := range [][]string{{"log", "z"}, {"done", "Delta"}, {"new-day"}} {
		if code, _, errs := runWith(m.dayOps(m.ws, true), args...); code != 1 || !strings.Contains(errs, "not in its own git repository") {
			t.Errorf("%v: exit %d err %q", args, code, errs)
		}
	}
	if m.read(t, "journal/today.md") != before || m.read(t, "tasks/todo.txt") != todoBasic {
		t.Error("a mutation wrote into a home that cannot commit")
	}
}

// commit: the home's repository once, or with --root the root's minus the
// homes inside it, whose staged changes stay staged; nothing to commit is
// exit 0; the removed --home/--not-home forms are usage errors.
func TestCommit(t *testing.T) {
	m := newMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	m.rootRepo(t)
	o := m.dayOps(m.ws, true)
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) new\n")
	code, out, errs := runWith(o, "commit", "home change")
	if hash := m.git(t, "rev-parse", "--short", "HEAD"); code != 0 || out != "📝 Committed: home change ("+hash+")\n" || errs != "" {
		t.Errorf("home: exit %d out %q err %q", code, out, errs)
	}
	if code, _, errs := runWith(o, "commit", "again"); code != 0 || errs != "No changes to commit in home 'testhome'\n" {
		t.Errorf("home no-op: exit %d err %q", code, errs)
	}

	inner := filepath.Join(m.root, "data", "zebranest")
	writeFile(t, filepath.Join(inner, "tasks", "todo.txt"), "v1\n")
	m.gitIn(t, m.root, "add", "-A")
	m.gitIn(t, m.root, "commit", "-q", "-m", "a home inside the root")
	writeFile(t, filepath.Join(m.config, "homes"), "testhome="+m.home+"\nzebranest="+inner+"\n")
	writeFile(t, filepath.Join(m.root, "code.txt"), "code\n")
	writeFile(t, filepath.Join(m.root, "home", "stray", "note.txt"), "root content\n")
	writeFile(t, filepath.Join(inner, "tasks", "todo.txt"), "v2\n")
	m.gitIn(t, m.root, "add", "data/zebranest/tasks/todo.txt")
	code, out, errs = runWith(o, "commit", "--root", "root change")
	if code != 0 || !strings.HasPrefix(out, "📝 Committed: root change (") || errs != "" {
		t.Fatalf("root: exit %d out %q err %q", code, out, errs)
	}
	if files := m.gitIn(t, m.root, "show", "--name-only", "--format=", "HEAD"); files != "code.txt\nhome/stray/note.txt" {
		t.Errorf("root commit holds %q", files)
	}
	if staged := m.gitIn(t, m.root, "diff", "--cached", "--name-only"); staged != "data/zebranest/tasks/todo.txt" {
		t.Errorf("staged %q", staged)
	}
	if code, _, errs := runWith(o, "commit", "--root", "nothing"); code != 0 || errs != "No changes to commit in root (excluding data/zebranest)\n" {
		t.Errorf("root no-op: exit %d err %q", code, errs)
	}

	for _, args := range [][]string{{"commit"}, {"commit", "--home", "msg"}, {"commit", "--not-home"}, {"commit", "--home"}, {"commit", "a", "b"}, {"commit", "--root"}} {
		if code, out, errs := runWith(o, args...); code != 2 || out != "" || !strings.Contains(errs, "Usage: daisy commit [--root] <message>") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	os.RemoveAll(filepath.Join(m.root, ".git"))
	if code, _, errs := runWith(o, "commit", "--root", "x"); code != 1 || errs != "Error: DAISY_ROOT is not a git repository\n" {
		t.Errorf("root not a repository: exit %d err %q", code, errs)
	}
}

// check-secrets reads .env.sh without running it: values from the file,
// else the environment, eight characters shown; a line it cannot read is
// named; no file is exit 1 with the copy hint. It needs only the
// workspace, not a resolvable home, and writes nothing.
func TestCheckSecrets(t *testing.T) {
	m := newMachine(t)
	m.vars["DAISY_SECRET_JIRA_API_TOKEN"] = "fromtheenvironment"
	o := m.dayOps(m.ws, true)
	code, out, errs := runWith(o, "check-secrets")
	if code != 1 || out != "Checking Daisy environment configuration...\n\n❌ .env.sh not found in workspace\n   Copy template: cp $DAISY_ROOT/templates/env.sh.template .env.sh\n" || errs != "" {
		t.Errorf("no file: exit %d out %q err %q", code, out, errs)
	}
	writeFile(t, filepath.Join(m.ws, ".env.sh"), "export DAISY_ROOT=\"/path/to/daisy\"\nexport DAISY_SECRET_GITHUB_TOKEN=\"ghp_ÄÖÜ1234567\"\nexport DAISY_SECRET_WEBEX_API_TOKEN=\"\"\necho hi\n")
	os.Remove(filepath.Join(m.config, "homes"))
	code, out, errs = runWith(o, "check-secrets")
	want := "Checking Daisy environment configuration...\n\nCore paths:\n✅ DAISY_ROOT: /path/to...\n⚠️  DAISY_HOME: Not configured\n\nAPI tokens:\n⚠️  Webex API: Not configured\n⚠️  Backstage: Not configured\n✅ JIRA API: fromthee...\n✅ GitHub: ghp_ÄÖÜ1...\n\nNote: Only showing first 8 characters of tokens for security\n"
	if code != 0 || out != want || errs != "⚠️  .env.sh line 4: not an assignment; it and every line after it are not read\n" {
		t.Errorf("exit %d err %q out\n%s", code, errs, out)
	}
	if code, _, errs := runWith(m.dayOps(t.TempDir(), true), "check-secrets"); code != 1 || !strings.Contains(errs, "Not in a Daisy workspace") {
		t.Errorf("outside a workspace: exit %d err %q", code, errs)
	}
}

// test: build, go test, then the suite with the root's bin/ first on PATH,
// from the root, wherever it is run; a filter skips go test; the first
// failing stage stops the run.
func TestTestRunsBuildGoTestThenSuite(t *testing.T) {
	m := newMachine(t)
	m.vars["PATH"] = "/usr/bin"
	var started []string
	fail := ""
	o := m.dayOps(t.TempDir(), true)
	o.StartFn = func(dir, name string, stdin io.Reader, stdout, stderr io.Writer, args ...string) (ops.Process, error) {
		if dir != m.root {
			t.Errorf("%s run in %s", name, dir)
		}
		started = append(started, name+" "+strings.Join(args, " "))
		if name == fail {
			return fakeProcess{code: 1}, nil
		}
		return fakeProcess{}, nil
	}
	if code, _, errs := runWith(o, "test"); code != 0 || errs != "" {
		t.Errorf("exit %d err %q", code, errs)
	}
	want := []string{"make -C " + m.root + " --silent build", "go test ./...", "env PATH=" + m.root + "/bin:/usr/bin " + m.root + "/tests/run.sh"}
	if strings.Join(started, "|") != strings.Join(want, "|") {
		t.Errorf("started %q", started)
	}
	started = nil
	runWith(o, "test", "healthcheck")
	if len(started) != 2 || started[1] != "env PATH="+m.root+"/bin:/usr/bin "+m.root+"/tests/run.sh healthcheck" {
		t.Errorf("filtered: %q", started)
	}
	for stage, msg := range map[string]string{"make": "could not build bin/daisy", "go": "go test failed", "env": ""} {
		started, fail = nil, stage
		code, _, errs := runWith(o, "test")
		if code != 1 || !strings.Contains(errs, msg) || (msg == "" && errs != "") {
			t.Errorf("%s failing: exit %d err %q", stage, code, errs)
		}
	}
}

// AGENTS.md's Command Reference is the table's, byte for byte: the same
// comparison the health check makes, so `make check` fails on drift too.
func TestCommandReferenceIsAgentsMd(t *testing.T) {
	b, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	block, ok := health.ReferenceBlock(string(b))
	if !ok || block != cli.Reference() {
		t.Errorf("AGENTS.md's Command Reference is not `daisy help --markdown`; replace it with:\n%s", cli.Reference())
	}
	m := newMachine(t)
	if code, out, _ := m.run(t, m.ws, "help", "--markdown"); code != 0 || out != cli.Reference() {
		t.Errorf("help --markdown: exit %d %q", code, out)
	}
	// Every command is a row; a pipe in a usage cannot split its cell.
	for _, line := range strings.Split(cli.Reference(), "\n") {
		if strings.HasPrefix(line, "| ") && strings.Count(strings.ReplaceAll(line, `\|`, ""), "|") != 4 {
			t.Errorf("row splits: %q", line)
		}
	}
}
