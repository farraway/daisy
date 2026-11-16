package main_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"daisy/internal/cli"
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
	code = cli.Run(o, strings.NewReader(""), &out, &errb, args)
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

// `daisy files <name>` prints the one real path the name stands for and
// nothing else: the plan PLAN.md links to, or a row the listing always
// prints. An unknown name is a usage error listing the names; no plan, or
// a PLAN.md that is no plan's link, is a failure naming the repair.
func TestFilesNamePrintsOnePath(t *testing.T) {
	m := newMachine(t)
	plan := filepath.Join(m.home, "plans", "261115120000.the-plan.plan.md")
	writeFile(t, plan, "# The plan\n")
	if err := os.Symlink(filepath.Join(m.ws, ".daisy", "plans", filepath.Base(plan)), filepath.Join(m.ws, "PLAN.md")); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(m.ws, ".daisy", "today.md")) // drift is the listing's to report, not a name's
	real, err := filepath.EvalSymlinks(m.home)
	if err != nil {
		t.Fatal(err)
	}
	for name, rel := range map[string]string{
		"plan":             "/plans/261115120000.the-plan.plan.md",
		"identity":         "/gitconfig",
		"today":            "/journal/today.md",
		"journal":          "/journal/journal.md",
		"todo":             "/tasks/todo.txt",
		"done":             "/tasks/done.txt",
		"alias":            "/tasks/alias.txt",
		"projects":         "/projects",
		"projects-archive": "/projects/_archive",
		"feedback":         "/feedback/feedback.md",
		"plans":            "/plans",
		"plans-archive":    "/plans/archive",
		"metrics":          "/metrics/metrics.csv",
		"prompts":          "/prompts",
		"agents":           "/AGENTS.md",
	} {
		code, out, errs := m.run(t, m.ws, "files", name)
		if code != 0 || out != real+rel+"\n" || errs != "" {
			t.Errorf("files %s: exit %d out %q err %q, want %q", name, code, out, errs, real+rel+"\n")
		}
	}

	code, out, errs := m.run(t, m.ws, "files", "nope")
	if code != 2 || out != "" || !strings.HasPrefix(errs, "Error: unknown name: nope; one of identity, today, journal, todo, done, alias, projects, projects-archive, feedback, plans, plans-archive, plan, metrics, prompts, agents\nUsage: daisy files [--json|<name>]\n") {
		t.Errorf("files nope: exit %d out %q err %q", code, out, errs)
	}

	os.Remove(filepath.Join(m.ws, "PLAN.md"))
	code, out, errs = m.run(t, m.ws, "files", "plan")
	if code != 1 || out != "" || !strings.Contains(errs, "no PLAN.md") || !strings.Contains(errs, "daisy plan new <description> [+project]") {
		t.Errorf("no plan: exit %d out %q err %q", code, out, errs)
	}
	os.Symlink(filepath.Join(m.home, "plans", "gone.plan.md"), filepath.Join(m.ws, "PLAN.md"))
	code, out, errs = m.run(t, m.ws, "files", "plan")
	if code != 1 || out != "" || !strings.Contains(errs, "PLAN.md is a broken link") {
		t.Errorf("broken link: exit %d out %q err %q", code, out, errs)
	}
	os.Remove(filepath.Join(m.ws, "PLAN.md"))
	writeFile(t, filepath.Join(m.ws, "PLAN.md"), "# Not linked\n")
	code, out, errs = m.run(t, m.ws, "files", "plan")
	if code != 1 || out != "" || !strings.Contains(errs, "PLAN.md is not a link") {
		t.Errorf("plain file: exit %d out %q err %q", code, out, errs)
	}
	// The plan is there, but PLAN.md reaches it through a binding link that
	// is gone: the repair is init's, not a new plan.
	os.Remove(filepath.Join(m.ws, "PLAN.md"))
	os.Symlink(filepath.Join(m.ws, ".daisy", "plans", filepath.Base(plan)), filepath.Join(m.ws, "PLAN.md"))
	os.Remove(filepath.Join(m.ws, ".daisy", "plans"))
	code, out, errs = m.run(t, m.ws, "files", "plan")
	if code != 1 || out != "" || !strings.Contains(errs, "the binding link .daisy/plans is missing") || !strings.Contains(errs, "Re-run 'daisy init testhome'") {
		t.Errorf("drifted binding: exit %d out %q err %q", code, out, errs)
	}
}

// objectKeys is an object's keys, sorted: what a --json test pins.
func objectKeys(t *testing.T, v any) []string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not an object: %#v", v)
	}
	return slices.Sorted(maps.Keys(m))
}

// `daisy files --json` is one JSON document on stdout and nothing on
// stderr, drift and all: every name `files <name>` takes under paths,
// plan's null without a current plan, the listing's conditional rows in
// their own keys, the drift lines under warnings; with a name it is a
// usage error.
func TestFilesJSON(t *testing.T) {
	m := newMachine(t)
	proj := filepath.Join(m.ws, "chessgrid.260807")
	writeFile(t, filepath.Join(proj, ".daisy", "home"), "testhome\n")
	for _, l := range env.Links {
		os.Symlink(filepath.Join(m.home, l.Target), filepath.Join(proj, ".daisy", l.Link))
	}
	writeFile(t, filepath.Join(m.home, "projects", "chessgrid.md"), "# chessgrid\n")
	writeFile(t, filepath.Join(m.home, "journal", "journal-20260101-last14.md"), "")
	writeFile(t, filepath.Join(m.home, "journal", "journal-2025.md"), "")
	writeFile(t, filepath.Join(m.home, "journal", "journal-notes.md"), "")
	os.Remove(filepath.Join(m.home, "feedback", "archive.md"))
	real, err := filepath.EvalSymlinks(m.home)
	if err != nil {
		t.Fatal(err)
	}
	decode := func(args ...string) map[string]any {
		t.Helper()
		code, out, errs := m.run(t, proj, args...)
		var doc map[string]any
		if code != 0 || errs != "" || json.Unmarshal([]byte(out), &doc) != nil {
			t.Fatalf("%q: exit %d out %q err %q", args, code, out, errs)
		}
		return doc
	}
	doc := decode("files", "--json")
	if got := objectKeys(t, doc); !slices.Equal(got, []string{"feedback_archive", "home", "identity", "identity_missing", "journal_windows", "paths", "pre_rotation", "warnings", "workspace", "workspace_projects"}) {
		t.Errorf("keys %q", got)
	}
	if got := objectKeys(t, doc["paths"]); !slices.Equal(got, []string{"agents", "alias", "done", "feedback", "identity", "journal", "metrics", "plan", "plans", "plans-archive", "projects", "projects-archive", "prompts", "today", "todo"}) {
		t.Errorf("paths keys %q", got)
	}
	paths := doc["paths"].(map[string]any)
	if doc["home"] != "testhome" || doc["identity"] != "testhome" || doc["workspace"] != proj || doc["identity_missing"] != false ||
		paths["todo"] != real+"/tasks/todo.txt" || paths["identity"] != real+"/gitconfig" || paths["agents"] != real+"/AGENTS.md" || paths["plan"] != nil ||
		doc["pre_rotation"] != nil || doc["feedback_archive"] != nil {
		t.Errorf("document %v", doc)
	}
	windows, _ := json.Marshal(doc["journal_windows"])
	if string(windows) != `[{"name":"journal-20260101-last14.md","path":"`+real+`/journal/journal-20260101-last14.md"},{"name":"journal-2025.md","path":"`+real+`/journal/journal-2025.md"}]` {
		t.Errorf("journal_windows %s", windows)
	}
	projects, _ := json.Marshal(doc["workspace_projects"])
	if string(projects) != `{"names":["chessgrid"],"source":"convention"}` {
		t.Errorf("workspace_projects %s", projects)
	}
	if w, ok := doc["warnings"].([]any); !ok || len(w) != 0 {
		t.Errorf("warnings %#v", doc["warnings"])
	}

	// The conditional rows, a current plan, a missing identity file and
	// drift: still nothing on stderr, the drift under warnings.
	writeFile(t, filepath.Join(m.home, "journal", "journal-pre-rotation.md"), "")
	writeFile(t, filepath.Join(m.home, "feedback", "archive.md"), "")
	writeFile(t, filepath.Join(m.home, "plans", "p.plan.md"), "# P\n")
	os.Symlink(filepath.Join(m.home, "plans", "p.plan.md"), filepath.Join(proj, "PLAN.md"))
	os.Remove(filepath.Join(proj, ".daisy", "today.md"))
	os.Rename(filepath.Join(m.home, "gitconfig"), filepath.Join(m.home, "gitconfig.away"))
	doc = decode("files", "--json")
	paths = doc["paths"].(map[string]any)
	if doc["pre_rotation"] != real+"/journal/journal-pre-rotation.md" || doc["feedback_archive"] != real+"/feedback/archive.md" ||
		paths["plan"] != real+"/plans/p.plan.md" || doc["identity_missing"] != true || paths["identity"] != m.home+"/gitconfig" {
		t.Errorf("document %v", doc)
	}
	if w, _ := doc["warnings"].([]any); !slices.Contains(w, any("⚠️  the binding link .daisy/today.md is missing or does not point into the home; re-run 'daisy init testhome'")) {
		t.Errorf("warnings %q", w)
	}

	code, out, errs := m.run(t, proj, "files", "--json", "plan")
	if code != 2 || out != "" || !strings.HasPrefix(errs, "Error: --json lists every path; with <name>, its path alone is printed\nUsage: daisy files [--json|<name>]\n") {
		t.Errorf("--json with a name: exit %d out %q err %q", code, out, errs)
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
		if code != 0 || errs != "" || !strings.HasPrefix(out, "Usage: daisy <command> [args...]\n\nWorkspace\n") || !strings.Contains(out, "\n  status [--json]                   Home, identity, projects, today's counts, active plan\n") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	for usage, args := range map[string][]string{"status [--json]": {"status", "--help"}, "files [--json|<name>]": {"files", "-h"}} {
		code, out, errs := m.run(t, m.ws, args...)
		if code != 0 || errs != "" || !strings.HasPrefix(out, "Usage: daisy "+usage+"\n") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	// The tree names only commands this binary runs, one line per name, and
	// a long usage never runs into its description.
	_, out, _ := m.run(t, m.ws)
	if !strings.Contains(out, "\n  plan new ") || !strings.Contains(out, "\nSwarm\n  swarm init [--defaults] [--api <url>]") || !strings.Contains(out, "\n  swarm resume ") {
		t.Errorf("tree does not describe this build:\n%s", out)
	}
	if !strings.Contains(out, "\n  tasks [--all] [--todo|--done] [--json] [pattern]\n                                    Search tasks (read-only)\n") {
		t.Errorf("long usage not wrapped:\n%s", out)
	}
}

type fakeProcess struct{ code int }

func (p fakeProcess) Wait() (int, error) { return p.code, nil }
func (p fakeProcess) Kill() error        { return nil }
