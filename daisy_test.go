package daisy_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy"
	"daisy/internal/env"
	"daisy/internal/ops"
)

// machine is a temp root, registry, home (copied from templates/home) and a
// workspace bound to it: what the bash fixture builds, built the same way.
type machine struct {
	root, config, local, home, ws string
	vars                          map[string]string
}

func newMachine(t *testing.T) *machine {
	t.Helper()
	m := &machine{root: t.TempDir(), config: t.TempDir(), local: t.TempDir(), ws: t.TempDir(), vars: map[string]string{}}
	m.home = filepath.Join(t.TempDir(), "testhome")
	copyTree(t, "templates/home", m.home)
	copyTree(t, "templates", filepath.Join(m.root, "templates"))
	writeFile(t, filepath.Join(m.home, "gitconfig"), "name=Fixture Author\nemail=fixture@example.invalid\n")
	writeFile(t, filepath.Join(m.config, "homes"), "testhome="+m.home+"\n")
	writeFile(t, filepath.Join(m.ws, ".daisy", "home"), "testhome\n")
	for _, l := range env.Links {
		if err := os.Symlink(filepath.Join(m.home, l.Target), filepath.Join(m.ws, ".daisy", l.Link)); err != nil {
			t.Fatal(err)
		}
	}
	m.vars["DAISY_ROOT"], m.vars["DAISY_CONFIG"], m.vars["DAISY_LOCAL"], m.vars["HOME"] = m.root, m.config, m.local, t.TempDir()
	return m
}

func (m *machine) run(t *testing.T, cwd string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	o := &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		GetwdFn:  func() (string, error) { return cwd, nil },
	}
	var out, errb bytes.Buffer
	code = daisy.Run(o, strings.NewReader(""), &out, &errb, args)
	return code, out.String(), errb.String()
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The first functional test: status against a temp root, registry and home
// prints the fixture's home name and workspace path, so the fixture is
// honoured and nothing reaches a real home.
func TestStatusHonoursTheFixture(t *testing.T) {
	m := newMachine(t)
	code, out, errs := m.run(t, m.ws, "status")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	if !strings.Contains(out, "Daisy | home: testhome | identity: testhome | workspace: "+m.ws+"\n") {
		t.Errorf("stdout %q", out)
	}
	if !strings.Contains(out, "Projects: (none)\n") || !strings.Contains(out, "Tasks:  ") || !strings.Contains(out, "Backlog: ") {
		t.Errorf("stdout %q", out)
	}
	if errs != "" {
		t.Errorf("stderr %q", errs)
	}
}

func TestStatusCountsAndPlan(t *testing.T) {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.home, "journal", "today.md"), strings.Join([]string{
		"### 2026-07-28 Tuesday", "", "#### Tasks", "", "**Now:**", "- [ ] Alpha task", "- [x] Beta task", "- [ ] Gamma", "",
		"#### Log", "", "- 0900 New day started", "- 1015 Something", "not an entry", "", "#### Retrospective", "- 1100 not a log entry", "",
	}, "\n"))
	writeFile(t, filepath.Join(m.home, "tasks", "todo.txt"), "(A) one\nx 2026-01-01 done\nz cancelled\n\n(B) two\n")
	writeFile(t, filepath.Join(m.home, "plans", "p.plan.md"), "##   The Plan Title\n\nbody\n")
	if err := os.Symlink(filepath.Join(m.home, "plans", "p.plan.md"), filepath.Join(m.ws, "PLAN.md")); err != nil {
		t.Fatal(err)
	}
	code, out, _ := m.run(t, m.ws, "status")
	if code != 0 {
		t.Fatal(out)
	}
	for _, want := range []string{"Tasks:  2 pending, 1 completed today\n", "Log:    2 entries\n", "Backlog: 2 tasks in todo.txt\n", "\nPlan:   The Plan Title\n        " + env.RealPath(ops.Real().FS(), filepath.Join(m.home, "plans", "p.plan.md")) + "\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	os.Remove(filepath.Join(m.home, "journal", "today.md"))
	_, out, _ = m.run(t, m.ws, "status")
	if !strings.Contains(out, "  No today.md — run 'daisy new-day' to start.\n") {
		t.Errorf("stdout %q", out)
	}
}

func TestFilesListsRealPaths(t *testing.T) {
	m := newMachine(t)
	proj := filepath.Join(m.ws, "chessgrid.260807")
	writeFile(t, filepath.Join(proj, ".daisy", "home"), "testhome\n")
	for _, l := range env.Links {
		os.Symlink(filepath.Join(m.home, l.Target), filepath.Join(proj, ".daisy", l.Link))
	}
	writeFile(t, filepath.Join(m.home, "projects", "chessgrid.md"), "# chessgrid\n")
	writeFile(t, filepath.Join(m.home, "journal", "journal-20260101-last14.md"), "")
	writeFile(t, filepath.Join(m.home, "journal", "journal-2025.md"), "")
	writeFile(t, filepath.Join(m.home, "journal", "journal-202601.md"), "")
	writeFile(t, filepath.Join(m.home, "journal", "journal-notes.md"), "")
	os.Symlink("/nonexistent", filepath.Join(m.home, "journal", "journal-20260101-mtd.md"))
	code, out, errs := m.run(t, proj, "files")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d stderr %q", code, errs)
	}
	real := env.RealPath(ops.Real().FS(), m.home)
	for _, want := range []string{
		"Daisy | home: testhome | identity: testhome | workspace: " + proj + "\n",
		"Identity:\n  testhome     " + real + "/gitconfig\n",
		"  todo.txt     " + real + "/tasks/todo.txt\n",
		"  workspace    +chessgrid (from directory name)\n",
		"  journal-20260101-last14.md " + real + "/journal/journal-20260101-last14.md\n",
		"  journal-2025.md " + real + "/journal/journal-2025.md\n",
		"  journal-202601.md " + real + "/journal/journal-202601.md\n",
		"AGENTS.md:     " + real + "/AGENTS.md\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "journal-notes.md") || strings.Contains(out, "journal-20260101-mtd.md") {
		t.Errorf("a file outside the window shapes was listed:\n%s", out)
	}

	writeFile(t, filepath.Join(proj, ".daisy", "project"), "typoed-name\n")
	code, out, errs = m.run(t, proj, "files")
	if code != 0 || !strings.Contains(out, "+typoed-name (from .daisy/project)") || !strings.Contains(errs, "typoed-name") {
		t.Errorf("exit %d\n%s\n%s", code, out, errs)
	}
}

func TestStatusAndFilesReportDrift(t *testing.T) {
	m := newMachine(t)
	os.Remove(filepath.Join(m.ws, ".daisy", "today.md"))
	for _, cmd := range []string{"status", "files"} {
		code, _, errs := m.run(t, m.ws, cmd)
		if code != 0 {
			t.Errorf("%s exit %d", cmd, code)
		}
		if !strings.Contains(errs, "⚠️  the binding link .daisy/today.md is missing or does not point into the home; re-run 'daisy init testhome'\n") {
			t.Errorf("%s stderr %q", cmd, errs)
		}
	}
}

func TestStatusRefusesBrokenBindings(t *testing.T) {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.ws, ".daisy", "home"), "second\n")
	code, out, errs := m.run(t, m.ws, "status")
	if code != 1 || out != "" || !strings.Contains(errs, "not registered") || !strings.Contains(errs, "daisy home add second") {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
	code, _, errs = m.run(t, t.TempDir(), "files")
	if code != 1 || !strings.Contains(errs, "Not in a Daisy workspace") {
		t.Errorf("exit %d err %q", code, errs)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	m := newMachine(t)
	for _, args := range [][]string{{"status", "--bogus"}, {"status", "extra"}, {"files", "-x"}} {
		code, out, errs := m.run(t, m.ws, args...)
		if code != 2 || out != "" || !strings.Contains(errs, "Usage: daisy "+args[0]) {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	code, out, errs := m.run(t, m.ws, "no-such-command")
	if code != 2 || out != "" || !strings.HasPrefix(errs, "Unknown command: no-such-command\n\nUsage: daisy <command> [args...]\n") {
		t.Errorf("unknown: exit %d out %q err %q", code, out, errs)
	}
}

func TestHelpForms(t *testing.T) {
	m := newMachine(t)
	for _, args := range [][]string{{}, {"help"}, {"--help"}, {"-h"}} {
		code, out, errs := m.run(t, m.ws, args...)
		if code != 0 || errs != "" || !strings.HasPrefix(out, "Usage: daisy <command> [args...]\n\nWorkspace\n") || !strings.Contains(out, "\n  status                            Home, identity, projects, today's counts, active plan\n") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	for _, args := range [][]string{{"status", "--help"}, {"files", "-h"}} {
		code, out, errs := m.run(t, m.ws, args...)
		if code != 0 || errs != "" || !strings.HasPrefix(out, "Usage: daisy "+args[0]+"\n") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	// The tree names only commands this binary runs, one line per name, and
	// a long usage never runs into its description.
	_, out, _ := m.run(t, m.ws)
	if !strings.Contains(out, "\n  plan new ") || strings.Contains(out, "swarm") {
		t.Errorf("tree does not describe this build:\n%s", out)
	}
	if !strings.Contains(out, "\n  tasks [--all] [--todo|--done] [pattern]\n                                    Search tasks (read-only)\n") {
		t.Errorf("long usage not wrapped:\n%s", out)
	}
}

type fakeProcess struct{ code int }

func (p fakeProcess) Wait() (int, error) { return p.code, nil }
func (p fakeProcess) Kill() error        { return nil }
