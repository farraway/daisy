package main_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/plan"
)

// planOps pins the clock; git is real, so a commit is a commit.
func (m *machine) planOps(cwd string) *ops.Fake {
	return &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		GetwdFn:  func() (string, error) { return cwd, nil },
		NowFn:    func() time.Time { return time.Date(2026, 11, 1, 9, 30, 5, 0, time.Local) },
	}
}

func (m *machine) planTemplate(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(m.root, "templates", "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// snapshot is every file under dir, links as their targets.
func snapshot(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, dir := range dirs {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || strings.Contains(p, "/.git/") || strings.HasSuffix(p, "/.git") {
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				l, _ := os.Readlink(p)
				files[p] = "-> " + l
			} else if info.Mode().IsRegular() {
				b, _ := os.ReadFile(p)
				files[p] = string(b)
			}
			return nil
		})
	}
	return files
}

func sameFiles(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for p, b := range before {
		if a, ok := after[p]; !ok || a != b {
			t.Errorf("%s: %s changed (%q → %q)", what, p, b, a)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("%s: %s appeared", what, p)
		}
	}
}

func TestPlanNewCreatesAndLinks(t *testing.T) {
	m := newMachine(t)
	o := m.planOps(m.ws)
	code, out, errs := runWith(o, "plan", "new", "Fix four defects", "+testproj")
	if code != 0 {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	name := "261101093005.testproj.Fix-four-defects.plan.md"
	got := m.read(t, filepath.Join("plans", name))
	want := plan.Fill(m.planTemplate(t), "Fix four defects", "testproj", "2026-11-01 09:30", m.root)
	if got != want || !strings.HasPrefix(got, "# Fix four defects — +testproj\n\n**Project:** +testproj\n**Created:** 2026-11-01 09:30\n") {
		t.Fatalf("plan file:\n%s", got)
	}
	if l, _ := os.Readlink(filepath.Join(m.ws, "PLAN.md")); l != ".daisy/plans/"+name {
		t.Fatalf("PLAN.md → %q", l)
	}
	if b, _ := os.ReadFile(filepath.Join(m.ws, ".gitignore")); string(b) != "PLAN.md\n" {
		t.Fatalf(".gitignore = %q", b)
	}
	wantOut := "  ✓ Created .gitignore with PLAN.md\n✅ Plan created: " + name + "\n   Linked as: " + m.ws + "/PLAN.md\n"
	if out != wantOut {
		t.Fatalf("stdout:\n%q\nwant\n%q", out, wantOut)
	}

	// A second plan is refused, and so is one over a dangling PLAN.md link.
	before := snapshot(t, m.home, m.ws)
	if code, _, errs := runWith(o, "plan", "new", "Another"); code != 1 || !strings.Contains(errs, "PLAN.md already exists") {
		t.Fatalf("second plan new = %d: %s", code, errs)
	}
	os.Remove(filepath.Join(m.ws, "PLAN.md"))
	os.Symlink(".daisy/plans/gone.plan.md", filepath.Join(m.ws, "PLAN.md"))
	before = snapshot(t, m.home, m.ws)
	if code, _, errs := runWith(o, "plan", "new", "Another"); code != 1 || !strings.Contains(errs, "PLAN.md already exists") {
		t.Fatalf("plan new over a dangling link = %d: %s", code, errs)
	}
	sameFiles(t, "refused plan new", before, snapshot(t, m.home, m.ws))
}

func TestPlanNewSpec(t *testing.T) {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.ws, ".gitignore"), "node_modules")
	code, out, errs := runWith(m.planOps(m.ws), "plan", "new", "--spec", "A draft")
	if code != 0 {
		t.Fatalf("plan new --spec = %d: %s", code, errs)
	}
	spec := filepath.Join(m.ws, "A-draft_PLAN.md")
	if b, _ := os.ReadFile(spec); string(b) != plan.Fill(m.planTemplate(t), "A draft", "", "2026-11-01 09:30", m.root) {
		t.Fatalf("spec:\n%s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(m.ws, ".gitignore")); string(b) != "node_modules\n*_PLAN.md\n" {
		t.Fatalf(".gitignore = %q", b)
	}
	if _, err := os.Lstat(filepath.Join(m.ws, "PLAN.md")); err == nil {
		t.Fatal("--spec linked PLAN.md")
	}
	if entries, _ := os.ReadDir(filepath.Join(m.home, "plans")); len(entries) != 1 { // archive/
		t.Fatalf("--spec wrote into the home: %v", entries)
	}
	wantOut := "  ✓ Added *_PLAN.md to .gitignore\n✅ Spec created: A-draft_PLAN.md\n   Location: " + spec + "\n   Not registered with Daisy — promote later with: daisy plan pickup " + spec + "\n"
	if out != wantOut {
		t.Fatalf("stdout:\n%q\nwant\n%q", out, wantOut)
	}
	if code, _, errs := runWith(m.planOps(m.ws), "plan", "new", "--spec", "A draft"); code != 1 || !strings.Contains(errs, "A-draft_PLAN.md already exists") {
		t.Fatalf("second spec = %d: %s", code, errs)
	}
}

// The slug seam end to end (case 55): a long project is refused with the
// limit and what to change, and nothing is written.
func TestPlanNewRefusals(t *testing.T) {
	m := newMachine(t)
	o := m.planOps(m.ws)
	before := snapshot(t, m.home, m.ws)
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"a short description", "+" + strings.Repeat("p", 250)}, 1, "the limit is 255"},
		{[]string{"!!!"}, 1, "empty slug"},
		{[]string{"x", "+a/b"}, 1, "cannot be part of a file name"},
		{nil, 2, "missing argument"},
		{[]string{"a", "b", "c"}, 2, "unexpected argument: c"},
		{[]string{"--specc", "Fix the parser"}, 2, "unknown flag: --specc"}, // a mistyped flag, not a description
	} {
		code, _, errs := runWith(o, append([]string{"plan", "new"}, tc.args...)...)
		if code != tc.code || !strings.Contains(errs, tc.want) {
			t.Errorf("plan new %q = %d: %s", tc.args, code, errs)
		}
	}
	sameFiles(t, "refused plan new", before, snapshot(t, m.home, m.ws))
}

// A plan new whose last write fails takes back the plan and the link.
func TestPlanNewRollsBack(t *testing.T) {
	m := newMachine(t)
	if err := os.Mkdir(filepath.Join(m.ws, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(m.ws, "sub", "ignore"), "x\n")
	os.Symlink("sub/ignore", filepath.Join(m.ws, ".gitignore"))
	os.Chmod(filepath.Join(m.ws, "sub"), 0o555)
	defer os.Chmod(filepath.Join(m.ws, "sub"), 0o755)
	before := snapshot(t, m.home, m.ws)
	code, _, errs := runWith(m.planOps(m.ws), "plan", "new", "Doomed")
	if code != 1 || !strings.Contains(errs, "Nothing was created") {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	sameFiles(t, "rolled-back plan new", before, snapshot(t, m.home, m.ws))
}

func TestPlanPickup(t *testing.T) {
	m := newMachine(t)
	m.initHomeRepo(t)
	sub := filepath.Join(m.ws, "drafts")
	writeFile(t, filepath.Join(sub, "x_PLAN.md"), "intro\n# A draft — +testproj\n\nbody\n")
	code, out, errs := runWith(m.planOps(sub), "plan", "pickup", "x_PLAN.md")
	if code != 0 {
		t.Fatalf("plan pickup = %d: %s", code, errs)
	}
	name := "261101093005.testproj.A-draft.plan.md"
	if got := m.read(t, filepath.Join("plans", name)); got != "intro\n# A draft — +testproj\n\nbody\n" {
		t.Fatalf("copy = %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(sub, "x_PLAN.md")); len(b) == 0 {
		t.Fatal("the original was touched")
	}
	if l, _ := os.Readlink(filepath.Join(m.ws, "PLAN.md")); l != ".daisy/plans/"+name {
		t.Fatalf("PLAN.md → %q", l)
	}
	head := m.git(t, "rev-parse", "--short", "HEAD")
	wantOut := "✅ Picked up: x_PLAN.md → " + name + "\n   Linked as: " + m.ws + "/PLAN.md\n   Original left in place at: x_PLAN.md\n📝 Committed: Pick up plan: " + name + " (" + head + ")\n"
	if out != wantOut || m.commits(t) != "2" || m.git(t, "status", "--porcelain") != "" {
		t.Fatalf("stdout:\n%q\nwant\n%q\ncommits %s", out, wantOut, m.commits(t))
	}

	before := snapshot(t, m.home, m.ws)
	for _, tc := range []struct{ path, want string }{
		{"x_PLAN.md", "PLAN.md already exists"},
		{"missing.md", "File not found: missing.md"},
	} {
		if code, _, errs := runWith(m.planOps(sub), "plan", "pickup", tc.path); code != 1 || !strings.Contains(errs, tc.want) {
			t.Errorf("plan pickup %s = %d: %s", tc.path, code, errs)
		}
	}
	os.Remove(filepath.Join(m.ws, "PLAN.md"))
	writeFile(t, filepath.Join(sub, "bare.md"), "no heading\n")
	before = snapshot(t, m.home, m.ws)
	if code, _, errs := runWith(m.planOps(sub), "plan", "pickup", "bare.md"); code != 1 || !strings.Contains(errs, "header") {
		t.Errorf("plan pickup without a header = %d: %s", code, errs)
	}
	sameFiles(t, "refused pickup", before, snapshot(t, m.home, m.ws))
}

// archiveFixture is case 54's: a plan linked as PLAN.md, a project file
// linking it, an unrelated one, and an archived project linking it too.
func (m *machine) archiveFixture(t *testing.T) (name string) {
	t.Helper()
	name = "260101010101.testproj.a-plan.plan.md"
	writeFile(t, filepath.Join(m.home, "plans", name), "# a plan\n")
	writeFile(t, filepath.Join(m.home, "projects", "testproj.md"), "# testproj\n\n## Plans\n\n- [../plans/"+name+"](../plans/"+name+") — status: built\n")
	writeFile(t, filepath.Join(m.home, "projects", "other.md"), "# other\n\n- [../plans/260202020202.other.unrelated.plan.md](../plans/260202020202.other.unrelated.plan.md)")
	writeFile(t, filepath.Join(m.home, "projects", "_archive", "old.md"), "see ../../plans/"+name+"\n")
	if err := os.Symlink(filepath.Join(m.home, "plans", name), filepath.Join(m.ws, "PLAN.md")); err != nil {
		t.Fatal(err)
	}
	m.initHomeRepo(t)
	return name
}

// The repoint seam end to end (case 54).
func TestPlanArchive(t *testing.T) {
	m := newMachine(t)
	name := m.archiveFixture(t)
	other := m.read(t, "projects/other.md")
	code, out, errs := runWith(m.planOps(m.ws), "plan", "archive")
	if code != 0 {
		t.Fatalf("plan archive = %d: %s", code, errs)
	}
	if got := m.read(t, "projects/testproj.md"); got != "# testproj\n\n## Plans\n\n- [../plans/archive/"+name+"](../plans/archive/"+name+") — status: built\n" {
		t.Fatalf("testproj.md = %q", got)
	}
	if got := m.read(t, "projects/_archive/old.md"); got != "see ../../plans/archive/"+name+"\n" {
		t.Fatalf("_archive/old.md = %q", got)
	}
	if got := m.read(t, "projects/other.md"); got != other {
		t.Fatalf("other.md changed: %q", got)
	}
	if m.read(t, "plans/archive/"+name) != "# a plan\n" {
		t.Fatal("plan not in the archive")
	}
	if _, err := os.Lstat(filepath.Join(m.home, "plans", name)); err == nil {
		t.Fatal("plan still in plans/")
	}
	if _, err := os.Lstat(filepath.Join(m.ws, "PLAN.md")); err == nil {
		t.Fatal("PLAN.md still there")
	}
	head := m.git(t, "rev-parse", "--short", "HEAD")
	wantOut := "✅ Archived: " + name + "\n  ✓ Repointed links in 2 project file(s)\n📝 Committed: Archive plan: " + name + " (" + head + ")\n"
	if out != wantOut || m.commits(t) != "2" || m.git(t, "status", "--porcelain") != "" {
		t.Fatalf("stdout:\n%q\nwant\n%q\ncommits %s", out, wantOut, m.commits(t))
	}
}

func TestPlanArchiveRefusals(t *testing.T) {
	m := newMachine(t)
	name := m.archiveFixture(t)
	writeFile(t, filepath.Join(m.home, "plans", "archive", name), "older\n")
	before := snapshot(t, m.home, m.ws)
	if code, _, errs := runWith(m.planOps(m.ws), "plan", "archive"); code != 1 || !strings.Contains(errs, "already exists") {
		t.Fatalf("archive over an archived plan = %d: %s", code, errs)
	}
	sameFiles(t, "refused archive", before, snapshot(t, m.home, m.ws))

	os.Remove(filepath.Join(m.home, "plans", "archive", name))
	os.Remove(filepath.Join(m.ws, "PLAN.md"))
	for _, tc := range []struct {
		setup func()
		want  string
	}{
		{func() {}, "PLAN.md does not exist"},
		{func() { writeFile(t, filepath.Join(m.ws, "PLAN.md"), "x") }, "not a Daisy plan"},
		{func() {
			os.Remove(filepath.Join(m.ws, "PLAN.md"))
			os.Symlink(".daisy/plans/gone.plan.md", filepath.Join(m.ws, "PLAN.md"))
		}, "The symlink may be broken"},
	} {
		tc.setup()
		if code, _, errs := runWith(m.planOps(m.ws), "plan", "archive"); code != 1 || !strings.Contains(errs, tc.want) {
			t.Errorf("plan archive = %d: %s, want %q", code, errs, tc.want)
		}
	}
	if code, _, _ := runWith(m.planOps(m.ws), "plan", "archive", "extra"); code != 2 {
		t.Errorf("plan archive extra = %d, want 2", code)
	}
}

// An archive whose repoint cannot be written takes everything back: the
// plan, the link and the project files already rewritten.
func TestPlanArchiveRollsBack(t *testing.T) {
	m := newMachine(t)
	m.archiveFixture(t)
	before := snapshot(t, m.home, m.ws)
	dir := filepath.Join(m.home, "projects", "_archive")
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	code, _, errs := runWith(m.planOps(m.ws), "plan", "archive")
	if code != 1 || !strings.Contains(errs, "Nothing was archived") {
		t.Fatalf("plan archive = %d: %s", code, errs)
	}
	sameFiles(t, "rolled-back archive", before, snapshot(t, m.home, m.ws))
	if m.commits(t) != "1" {
		t.Fatal("a rolled-back archive committed")
	}
}

func TestPlanNoun(t *testing.T) {
	m := newMachine(t)
	code, _, errs := runWith(m.planOps(m.ws), "plan")
	if code != 2 || !strings.Contains(errs, "daisy plan <command>") || !strings.Contains(errs, "plan pickup <path>") {
		t.Fatalf("bare plan = %d: %s", code, errs)
	}
	for _, verb := range []string{"new", "pickup", "archive"} {
		if code, out, errs := runWith(m.planOps(m.ws), "plan", verb, "--healthcheck"); code != 0 || out != "" || errs != "" {
			t.Errorf("plan %s --healthcheck = %d: %q %q", verb, code, out, errs)
		}
	}
	os.Remove(filepath.Join(m.root, "templates", "plan.md"))
	if code, _, errs := runWith(m.planOps(m.ws), "plan", "new", "--healthcheck"); code != 1 || !strings.Contains(errs, "templates/plan.md") {
		t.Errorf("plan new --healthcheck without the template = %d: %s", code, errs)
	}
}

// An empty word is no description: the project after it must not take its
// place (pass 1, F1), for any command whose arguments have places.
func TestEmptyWordKeepsItsPlace(t *testing.T) {
	m := newMachine(t)
	before := snapshot(t, m.home, m.ws)
	for _, args := range [][]string{
		{"plan", "new", "", "+testproj"},
		{"plan", "new", "--spec", "", "Real words"},
		{"home", "add", "", m.home},
		{"init", "", m.ws},
	} {
		if code, _, errs := runWith(m.planOps(m.ws), args...); code != 2 || !strings.Contains(errs, "empty argument") {
			t.Errorf("%q = %d: %s", args, code, errs)
		}
	}
	sameFiles(t, "empty words", before, snapshot(t, m.home, m.ws))
	if code, _, errs := runWith(m.planOps(m.ws), "plan", "new", "x", ""); code != 0 {
		t.Errorf("a trailing empty word is no argument: %d %s", code, errs)
	}
}

// Dotfiles are not project files (pass 1, F4); a picked-up spec keeps its
// mode, as cp did.
func TestArchiveSkipsDotfilesPickupKeepsMode(t *testing.T) {
	m := newMachine(t)
	name := "260101010101.testproj.a-plan.plan.md"
	hidden := "../plans/" + name + "\n"
	writeFile(t, filepath.Join(m.home, "projects", ".hidden.md"), hidden)
	m.archiveFixture(t)
	if code, out, errs := runWith(m.planOps(m.ws), "plan", "archive"); code != 0 || !strings.Contains(out, "in 2 project file(s)") {
		t.Fatalf("archive = %d: %s %s", code, out, errs)
	}
	if m.read(t, "projects/.hidden.md") != hidden {
		t.Fatal("a dotfile was repointed")
	}
	spec := filepath.Join(m.ws, "s_PLAN.md")
	writeFile(t, spec, "# Spec\n")
	os.Chmod(spec, 0o600)
	if code, _, errs := runWith(m.planOps(m.ws), "plan", "pickup", spec); code != 0 {
		t.Fatalf("pickup = %d: %s", code, errs)
	}
	if fi, _ := os.Stat(filepath.Join(m.home, "plans", "261101093005.Spec.plan.md")); fi == nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("copy mode = %v", fi)
	}
}

// archiveFails fails the repoint (any write under projects/) and the move
// of the plan back out of the archive: a rollback that cannot finish.
type archiveFails struct{ ops.FS }

func (f archiveFails) WriteFile(name string, data []byte, perm os.FileMode) error {
	if strings.Contains(name, "/projects/") {
		return errors.New("forced write failure")
	}
	return f.FS.WriteFile(name, data, perm)
}

func (f archiveFails) Rename(from, to string) error {
	if strings.Contains(from, "/plans/archive/") {
		return errors.New("forced rename failure")
	}
	return f.FS.Rename(from, to)
}

// A rollback that fails says what it left, never "Nothing was archived".
func TestArchiveRollbackFailureIsReported(t *testing.T) {
	m := newMachine(t)
	m.archiveFixture(t)
	code, _, errs := runWith(withFS{m.planOps(m.ws), archiveFails{ops.Real().FS()}}, "plan", "archive")
	if code != 1 || strings.Contains(errs, "Nothing was archived") || !strings.Contains(errs, "rollback failed") || !strings.Contains(errs, "forced rename failure") {
		t.Fatalf("archive = %d: %s", code, errs)
	}
}
