package health_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"daisy/internal/env"
	"daisy/internal/health"
	"daisy/internal/ops"
)

// machine is a temp root (a repository with the directories Check 4 asks
// for and AGENTS.md holding ref), a registry, a home with its own
// repository, and a workspace bound to it. Every variable the check reads
// is pinned, and git runs in the temp repositories only.
type machine struct {
	t                             *testing.T
	root, config, home, ws, uhome string
	vars                          map[string]string
	now                           time.Time
}

const ref = health.ReferenceBegin + "\n| Group | Command | Description |\n" + health.ReferenceEnd + "\n"

func newMachine(t *testing.T) *machine {
	t.Helper()
	real := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }
	m := &machine{t: t, root: real(t.TempDir()), config: t.TempDir(), ws: real(t.TempDir()), uhome: t.TempDir(), vars: map[string]string{}}
	m.home = filepath.Join(real(t.TempDir()), "testhome")
	for _, d := range []string{"prompts", "templates", "skills"} {
		m.write(filepath.Join(m.root, d, ".keep"), "")
	}
	m.write(filepath.Join(m.root, "AGENTS.md"), "# Root\n\n"+ref+"\nmore\n")
	m.repo(m.root)
	m.write(filepath.Join(m.home, "journal", ".keep"), "")
	m.repo(m.home)
	m.write(filepath.Join(m.config, "homes"), "testhome="+m.home+"\n")
	m.write(filepath.Join(m.ws, ".daisy", "home"), "testhome\n")
	m.vars = map[string]string{"DAISY_ROOT": m.root, "DAISY_CONFIG": m.config, "DAISY_LOCAL": t.TempDir(), "HOME": m.uhome, "USER": "pelicanmoor"}
	m.now = time.Unix(1_800_000_000, 0)
	return m
}

func (m *machine) write(p, s string) {
	m.t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *machine) git(dir string, args ...string) {
	m.t.Helper()
	if _, errb, err := ops.Real().Exec(dir, "git", args...); err != nil {
		m.t.Fatalf("git %v: %v %s", args, err, errb)
	}
}

func (m *machine) repo(dir string) {
	m.t.Helper()
	m.git(dir, "init", "-q")
	m.git(dir, "config", "user.name", "Fixture Author")
	m.git(dir, "config", "user.email", "fixture@example.invalid")
	m.git(dir, "add", "-A")
	m.git(dir, "commit", "-q", "-m", "initial")
}

func (m *machine) ops() *ops.Fake {
	return &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		GetwdFn:  func() (string, error) { return m.ws, nil },
		NowFn:    func() time.Time { return m.now },
	}
}

func (m *machine) env() env.Env {
	e, err := env.Resolve(m.ops())
	if err != nil {
		m.t.Fatal(err)
	}
	return e
}

func (m *machine) run(opt health.Options) (ok bool, stdout, stderr string) {
	var out, errb bytes.Buffer
	if opt.Reference == "" {
		opt.Reference = ref
	}
	ok = health.Run(m.ops(), &out, &errb, opt)
	return ok, out.String(), errb.String()
}

func (m *machine) stamp() string { return filepath.Join(m.home, health.StampName) }

// The term set: every registered name but the four placeholders, each
// usable home's identity (the first line a name has), $USER, the root's
// identity and $HOME/, each once without regard to case.
func TestTerms(t *testing.T) {
	m := newMachine(t)
	quill := filepath.Join(t.TempDir(), "quill")
	m.write(filepath.Join(quill, ".keep"), "")
	m.git(quill, "init", "-q")
	m.git(quill, "config", "user.name", "Zebra Quillfeather")
	m.git(quill, "config", "user.email", "ZQ@example.invalid")
	m.write(filepath.Join(m.config, "homes"), strings.Join([]string{
		"testhome=" + m.home,
		"zebrahome=" + filepath.Join(t.TempDir(), "absent"),
		"quill=" + quill,
		"work=" + filepath.Join(t.TempDir(), "absent"),
		"ZEBRAHOME=" + quill,
		"# comment=x",
	}, "\n")+"\n")
	m.vars["USER"] = "zq@EXAMPLE.invalid"
	m.git(m.root, "config", "user.name", "Root Author")
	got := health.Terms(m.ops(), m.env())
	want := []string{"Fixture Author", "fixture@example.invalid", "zebrahome", "quill", "Zebra Quillfeather", "ZQ@example.invalid", "Root Author", m.uhome + "/"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("terms\n got %q\nwant %q", got, want)
	}
	m.vars["HOME"] = "/"
	m.vars["USER"] = ""
	if got := health.Terms(m.ops(), m.env()); strings.Contains(strings.Join(got, "|"), "|/") {
		t.Errorf("HOME=/ is a term: %q", got)
	}
}

// Contains: no case, a whole word where the term's own edge is a word
// character, any neighbour where it is not.
func TestContains(t *testing.T) {
	for _, c := range []struct {
		text, term string
		want       bool
	}{
		{"this mentions ZebraHome by name", "zebrahome", true},
		{"zebrahomes", "zebrahome", false},
		{"_zebrahome", "zebrahome", false},
		{"zebrahomé", "zebrahom", false},
		{"heronry heron", "heron", true},
		{"heronry", "heron", false},
		{"see /home/pelican/src/x", "/home/pelican/", true},
		{"x/home/pelican/", "/home/pelican/", true},
		{"/home/pelicanmoor/", "/home/pelican/", false},
		{"mail zq@example.invalid.", "zq@example.invalid", true},
		{"Zebra\nQuillfeather", "Zebra Quillfeather", false},
		{"", "x", false},
	} {
		if got := health.Contains([]byte(c.text), c.term); got != c.want {
			t.Errorf("Contains(%q, %q) = %v", c.text, c.term, got)
		}
	}
}

// The publishable surface: tracked and untracked-unignored files, never an
// ignored one or a home inside the root; a symlink is its link text (what
// git publishes) and, for a link to a file, the file's content too.
func TestPublishableAndTexts(t *testing.T) {
	m := newMachine(t)
	m.write(filepath.Join(m.root, ".gitignore"), "ignored.md\n")
	m.write(filepath.Join(m.root, "ignored.md"), "x")
	m.write(filepath.Join(m.root, "untracked.md"), "x")
	inner := filepath.Join(m.root, "data", "zebranest")
	m.write(filepath.Join(inner, "today.md"), "x")
	m.write(filepath.Join(m.config, "homes"), "testhome="+m.home+"\nzebranest="+inner+"\n")
	if err := os.Symlink(filepath.Join(m.uhome, "private"), filepath.Join(m.root, "dirlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(m.root, "filelink")); err != nil {
		t.Fatal(err)
	}
	files, err := health.Publishable(m.ops(), m.env())
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(files, "|")
	for _, want := range []string{"AGENTS.md", "untracked.md", ".gitignore", "dirlink", "filelink"} {
		if !strings.Contains("|"+got+"|", "|"+want+"|") {
			t.Errorf("%s not publishable: %q", want, files)
		}
	}
	for _, not := range []string{"ignored.md", "zebranest"} {
		if strings.Contains(got, not) {
			t.Errorf("%s publishable: %q", not, files)
		}
	}
	fs := ops.Real().FS()
	texts, err := health.Texts(fs, filepath.Join(m.root, "dirlink"))
	if err != nil || len(texts) != 1 || string(texts[0]) != filepath.Join(m.uhome, "private") {
		t.Errorf("dangling link: %q %v", texts, err)
	}
	texts, _ = health.Texts(fs, filepath.Join(m.root, "filelink"))
	if len(texts) != 2 || string(texts[0]) != "AGENTS.md" || !strings.HasPrefix(string(texts[1]), "# Root") {
		t.Errorf("link to a file: %q", texts)
	}
	locked := filepath.Join(t.TempDir(), "locked")
	m.write(locked, "x")
	os.Chmod(locked, 0)
	os.Symlink(locked, filepath.Join(m.root, "lockedlink"))
	if texts, err := health.Texts(fs, filepath.Join(m.root, "lockedlink")); os.Geteuid() != 0 && (err != nil || len(texts) != 1) {
		t.Errorf("link to an unreadable file: %q %v", texts, err)
	}
	os.Chmod(filepath.Join(m.root, "AGENTS.md"), 0)
	if _, err := health.Texts(fs, filepath.Join(m.root, "AGENTS.md")); os.Geteuid() != 0 && err == nil {
		t.Error("an unreadable regular file was vouched for")
	}
	os.Chmod(filepath.Join(m.root, "AGENTS.md"), 0o644)
	if texts, _ := health.Texts(fs, filepath.Join(m.root, "prompts")); len(texts) != 0 {
		t.Errorf("a directory has text: %q", texts)
	}
}

// Check 6c end to end: a link whose text holds $HOME/ fails the scan (the
// bash never read link text), an ignored leak does not, and each file is
// named under its term.
func TestHygieneScan(t *testing.T) {
	m := newMachine(t)
	if ok, _, errs := m.run(health.Options{Force: true}); !ok {
		t.Fatalf("clean machine unhealthy: %s", errs)
	}
	if err := os.Symlink(filepath.Join(m.uhome, "private"), filepath.Join(m.root, "secret")); err != nil {
		t.Fatal(err)
	}
	m.write(filepath.Join(m.root, ".gitignore"), "ignored.md\n")
	m.write(filepath.Join(m.root, "ignored.md"), "pelicanmoor")
	m.write(filepath.Join(m.root, "leak.md"), "by PelicanMoor, in "+m.uhome+"/src\n")
	ok, _, errs := m.run(health.Options{Force: true})
	if ok {
		t.Fatal("leaks passed")
	}
	for _, want := range []string{
		"Publication hygiene: 'pelicanmoor' appears in publishable file(s)",
		"    leak.md\n",
		"Publication hygiene: '" + m.uhome + "/' appears",
		"    secret\n",
		"2 issue(s) found",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
	if strings.Contains(errs, "ignored.md") {
		t.Errorf("an ignored file was scanned:\n%s", errs)
	}
}

// A leak only in the index (staged, then cleaned in the worktree) is what
// a plain commit publishes; a term in a file's name is published too.
func TestHygieneReadsIndexAndPaths(t *testing.T) {
	m := newMachine(t)
	m.write(filepath.Join(m.root, "staged.md"), "by pelicanmoor\n")
	m.git(m.root, "add", "staged.md")
	m.write(filepath.Join(m.root, "staged.md"), "clean\n")
	m.write(filepath.Join(m.root, "notes", "PelicanMoor-notes.md"), "clean\n")
	ok, _, errs := m.run(health.Options{Force: true})
	if ok || !strings.Contains(errs, "    staged.md\n") || !strings.Contains(errs, "    notes/PelicanMoor-notes.md\n") {
		t.Errorf("ok %v stderr:\n%s", ok, errs)
	}
}

// The index is read by object id: an assume-unchanged entry and a path
// shaped like stage syntax are read too, while a gitlink and a conflicted
// merge in a clean tree are no failure.
func TestHygieneIndexByObject(t *testing.T) {
	m := newMachine(t)
	m.git(m.root, "checkout", "-q", "-b", "side")
	m.write(filepath.Join(m.root, "conf.txt"), "side\n")
	m.git(m.root, "add", "-A")
	m.git(m.root, "commit", "-q", "-m", "side")
	m.git(m.root, "checkout", "-q", "-")
	m.write(filepath.Join(m.root, "conf.txt"), "main\n")
	m.git(m.root, "add", "conf.txt")
	m.git(m.root, "commit", "-q", "-m", "main")
	if _, _, err := ops.Real().Exec(m.root, "git", "merge", "-q", "side"); err == nil {
		t.Fatal("the merge did not conflict")
	}
	if ok, _, errs := m.run(health.Options{Force: true}); !ok {
		t.Fatalf("clean tree mid-merge with a gitlink failed:\n%s", errs)
	}
	m.git(m.root, "merge", "--abort")
	head, _, _ := ops.Real().Exec(m.root, "git", "rev-parse", "HEAD")
	m.git(m.root, "update-index", "--add", "--cacheinfo", "160000,"+strings.TrimSpace(string(head))+",vend/sub")
	if ok, _, errs := m.run(health.Options{Force: true}); !ok {
		t.Fatalf("a gitlink failed the scan:\n%s", errs)
	}
	for _, name := range []string{"c.md", "0:leak.md"} {
		m.write(filepath.Join(m.root, name), "note pelicanmoor here\n")
		m.git(m.root, "add", "--", name)
		m.write(filepath.Join(m.root, name), "clean\n")
	}
	m.write(filepath.Join(m.root, "leak.md"), "clean\n")
	m.git(m.root, "add", "leak.md")
	m.git(m.root, "update-index", "--assume-unchanged", "c.md")
	ok, _, errs := m.run(health.Options{Force: true})
	if ok || !strings.Contains(errs, "    c.md\n") || !strings.Contains(errs, "    0:leak.md\n") || strings.Contains(errs, "    leak.md\n") {
		t.Errorf("ok %v stderr:\n%s", ok, errs)
	}
}

// What the root tracks is published wherever it lies: a file under a
// registered home inside the root is scanned once tracked, and only an
// untracked one is the home's.
func TestTrackedUnderAHomeIsScanned(t *testing.T) {
	m := newMachine(t)
	inner := filepath.Join(m.root, "data", "zebranest")
	m.write(filepath.Join(inner, "untracked.md"), "pelicanmoor\n")
	m.write(filepath.Join(inner, "notes.md"), "pelicanmoor\n")
	m.git(m.root, "add", "data/zebranest/notes.md")
	m.write(filepath.Join(m.config, "homes"), "testhome="+m.home+"\nzebranest="+inner+"\n")
	ok, _, errs := m.run(health.Options{Force: true})
	if ok || !strings.Contains(errs, "    data/zebranest/notes.md\n") || strings.Contains(errs, "untracked.md") {
		t.Errorf("ok %v stderr:\n%s", ok, errs)
	}
}

// A FIFO where today.md, journal.md, .last-optimized or AGENTS.md would be
// is skipped, never opened: the check does not hang.
func TestHealthNeverOpensAFIFO(t *testing.T) {
	m := newMachine(t)
	for _, p := range []string{filepath.Join(m.home, "journal", "today.md"), filepath.Join(m.home, "journal", "journal.md"), filepath.Join(m.home, "feedback", ".last-optimized")} {
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := syscall.Mkfifo(p, 0o644); err != nil {
			t.Skip(err)
		}
	}
	m.write(filepath.Join(m.home, "feedback", "feedback.md"), "")
	done := make(chan bool)
	go func() { ok, _, _ := m.run(health.Options{Force: true}); done <- ok }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the health check hung on a FIFO")
	}
}

// today.md that exists but cannot be read is a failure, not a pass.
func TestUnreadableToday(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}
	m := newMachine(t)
	p := filepath.Join(m.home, "journal", "today.md")
	m.write(p, "#### Agenda\n")
	os.Chmod(p, 0)
	defer os.Chmod(p, 0o644)
	if ok, _, errs := m.run(health.Options{Force: true}); ok || !strings.Contains(errs, "today.md cannot be read") {
		t.Errorf("ok %v stderr %q", ok, errs)
	}
}

// The cache: a pass stamps now+TTL; within it a run says so and checks
// nothing; --force, an expired, corrupt or too-far stamp, or TTL 0 run
// everything; a failure — even one that stops the run at Check 3 —
// removes the stamp; DAISY_HEALTHCHECK_PASSED=1 short-circuits unless
// --force.
func TestCache(t *testing.T) {
	m := newMachine(t)
	component := health.Component{Name: "probe", Check: func(env.Env) error { return nil }}
	opt := health.Options{Components: []health.Component{component}}
	ok, out, _ := m.run(opt)
	if !ok || !strings.Contains(out, "Component: probe") {
		t.Fatalf("first run: %v %s", ok, out)
	}
	if b, _ := os.ReadFile(m.stamp()); string(b) != strconv.FormatInt(m.now.Unix()+300, 10)+"\n" {
		t.Errorf("stamp %q", b)
	}
	m.now = m.now.Add(100 * time.Second)
	ok, out, _ = m.run(opt)
	if !ok || strings.Contains(out, "Component:") || !strings.Contains(out, "Checks cached — valid for 200s (--force to re-run)") {
		t.Errorf("cached run: %v %q", ok, out)
	}
	if _, out, _ = m.run(health.Options{Force: true, Components: opt.Components}); !strings.Contains(out, "Component:") {
		t.Errorf("--force was cached: %q", out)
	}
	for _, stamp := range []string{"not-a-number\n", "9999999999\n", "", strconv.FormatInt(m.now.Unix(), 10)} {
		m.write(m.stamp(), stamp)
		if _, out, _ = m.run(opt); !strings.Contains(out, "Component:") {
			t.Errorf("stamp %q honoured: %q", stamp, out)
		}
	}
	m.vars["DAISY_HEALTHCHECK_TTL"] = "0"
	os.Remove(m.stamp())
	if _, out, _ = m.run(opt); !strings.Contains(out, "Component:") {
		t.Errorf("TTL 0 cached")
	}
	if _, err := os.Stat(m.stamp()); err == nil {
		t.Error("TTL 0 wrote a stamp")
	}
	delete(m.vars, "DAISY_HEALTHCHECK_TTL")

	m.vars["DAISY_HEALTHCHECK_PASSED"] = "1"
	if ok, out, _ = m.run(opt); !ok || out != "" {
		t.Errorf("override: %v %q", ok, out)
	}
	if _, out, _ = m.run(health.Options{Force: true, Components: opt.Components}); !strings.Contains(out, "Component:") {
		t.Errorf("--force did not outrank the override")
	}
	delete(m.vars, "DAISY_HEALTHCHECK_PASSED")

	// A failure at Check 3, after a --force run past a valid stamp.
	m.run(opt)
	os.Rename(filepath.Join(m.home, ".git"), filepath.Join(m.home, "git-aside"))
	ok, _, errs := m.run(health.Options{Force: true})
	if ok || !strings.Contains(errs, "not in its own git repository") {
		t.Errorf("home without a repository: %v %s", ok, errs)
	}
	if _, err := os.Stat(m.stamp()); err == nil {
		t.Error("a failure at Check 3 left the stamp")
	}
	os.Rename(filepath.Join(m.home, "git-aside"), filepath.Join(m.home, ".git"))
	if ok, _, _ := m.run(opt); !ok {
		t.Fatal("restored home unhealthy")
	}
	os.Rename(filepath.Join(m.root, ".git"), filepath.Join(m.root, "git-aside"))
	if ok, _, _ := m.run(health.Options{Force: true}); ok {
		t.Error("root without a repository passed")
	}
	if _, err := os.Stat(m.stamp()); err == nil {
		t.Error("a root that is no repository left the stamp")
	}
}

// Check 3: the root must be a repository's top, the home in its own
// repository — never the root's or a foreign enclosing one; the fix is
// printed quoted for the shell.
func TestRepositories(t *testing.T) {
	m := newMachine(t)
	parent := filepath.Dir(m.home)
	os.Rename(filepath.Join(m.home, ".git"), filepath.Join(parent, "git-aside"))
	m.git(parent, "init", "-q")
	ok, _, errs := m.run(health.Options{Force: true})
	q := health.ShellQuote(m.home)
	if ok || !strings.Contains(errs, "Home 'testhome' at "+m.home+" is not in its own git repository\n  Run: git -C "+q+" init && git -C "+q+" add -A && git -C "+q+" commit -m 'New home: testhome'\n") {
		t.Errorf("foreign repository: %v\n%s", ok, errs)
	}
	os.RemoveAll(filepath.Join(parent, ".git"))
	os.Rename(filepath.Join(parent, "git-aside"), filepath.Join(m.home, ".git"))

	// A home inside the root's repository is refused: its commits would
	// land in the published root's history.
	inner := filepath.Join(m.root, "data", "zebranest")
	m.write(filepath.Join(inner, "journal", ".keep"), "")
	m.write(filepath.Join(m.config, "homes"), "testhome="+inner+"\n")
	if ok, _, errs := m.run(health.Options{Force: true}); ok || !strings.Contains(errs, "is not in its own git repository") {
		t.Errorf("home inside the root: %v %q", ok, errs)
	}

	// A home whose git directory is the root's (a gitdir file; a linked
	// worktree is the same) would commit into the root's history.
	m.write(filepath.Join(m.config, "homes"), "testhome="+m.home+"\n")
	os.Rename(filepath.Join(m.home, ".git"), filepath.Join(parent, "git-aside"))
	m.write(filepath.Join(m.home, ".git"), "gitdir: "+filepath.Join(m.root, ".git")+"\n")
	ok, _, errs = m.run(health.Options{Force: true})
	if ok || !strings.Contains(errs, "not in its own git repository (its .git points into the root's)") || !strings.Contains(errs, "  Run: rm "+m.home+"/.git && git -C "+m.root+" worktree prune && git -C "+m.home+" init") {
		t.Errorf("home borrowing the root's git directory: %v %q", ok, errs)
	}
	// A .git pointing at no repository (a pruned worktree) is removed first.
	m.write(filepath.Join(m.home, ".git"), "gitdir: "+filepath.Join(m.root, ".git", "worktrees", "gone")+"\n")
	ok, _, errs = m.run(health.Options{Force: true})
	if ok || !strings.Contains(errs, "(its .git points at no repository)\n  Run: rm "+m.home+"/.git && git -C "+m.home+" init") {
		t.Errorf("dangling .git: %v %q", ok, errs)
	}
	os.Remove(filepath.Join(m.home, ".git"))
	os.Rename(filepath.Join(parent, "git-aside"), filepath.Join(m.home, ".git"))

	os.RemoveAll(filepath.Join(m.root, ".git"))
	if ok, _, errs := m.run(health.Options{Force: true}); ok || !strings.Contains(errs, "DAISY_ROOT is not a git repository") {
		t.Errorf("root without a repository: %v %s", ok, errs)
	}
}

// A failing component is one issue, its message indented under it.
func TestComponentFailureCountsOnce(t *testing.T) {
	m := newMachine(t)
	bad := health.Component{Name: "plan new", Check: func(env.Env) error { return errors.New("Error: plans directory not found\nsecond line") }}
	ok, out, errs := m.run(health.Options{Force: true, Components: []health.Component{bad}})
	if ok || strings.Contains(out, "Component: plan new") {
		t.Fatalf("%v %q", ok, out)
	}
	if !strings.Contains(errs, "✗\033[0m Component: plan new failed health check\n  Error: plans directory not found\n  second line\n") || !strings.Contains(errs, "1 issue(s) found") {
		t.Errorf("stderr %q", errs)
	}
	os.RemoveAll(filepath.Join(m.root, "skills"))
	if _, _, errs := m.run(health.Options{Force: true}); !strings.Contains(errs, "Missing directory: skills") || !strings.Contains(errs, "1 issue(s) found") {
		t.Errorf("missing directory: %q", errs)
	}
}

// The healthy run, line by line, as the bash printed it.
func TestHealthyOutput(t *testing.T) {
	m := newMachine(t)
	m.write(filepath.Join(m.home, "feedback", "feedback.md"), "# Feedback\n\n## 2026-10-01 0900\nx\n")
	m.write(filepath.Join(m.home, "feedback", ".last-optimized"), "2026-10-02 1000 prompts/daisy\n")
	ok, out, errs := m.run(health.Options{Force: true})
	head, _, _ := ops.Real().Exec(m.root, "git", "rev-parse", "--short", "HEAD")
	g := "\033[0;32m✓\033[0m "
	want := g + "DAISY_ROOT: " + m.root + "\n" +
		g + "DAISY_HOME: " + m.home + " (home: testhome)\n" +
		g + "Git repository: " + string(head) +
		g + "today.md format valid\n" +
		g + "Publication hygiene: no identifying terms outside the homes\n" +
		g + "Feedback: 1 entries (last optimized: 2026-10-02 1000 prompts/daisy)\n" +
		"\n" + g + "System healthy\n"
	if !ok || out != want || errs != "" {
		t.Errorf("ok %v stderr %q stdout\n%s\nwant\n%s", ok, errs, out, want)
	}
}

// Check 6b: the block between the markers must be the generated one.
func TestReferenceDrift(t *testing.T) {
	m := newMachine(t)
	fs := ops.Real().FS()
	if p := health.ReferenceDrift(fs, m.root, ref); p != "" {
		t.Errorf("in step: %q", p)
	}
	if p := health.ReferenceDrift(fs, m.root, strings.Replace(ref, "Group", "Kind", 1)); !strings.Contains(p, "differs from the cli table") {
		t.Errorf("drift: %q", p)
	}
	m.write(filepath.Join(m.root, "AGENTS.md"), "# Root\n")
	if p := health.ReferenceDrift(fs, m.root, ref); !strings.Contains(p, "no Command Reference block") {
		t.Errorf("no block: %q", p)
	}
	os.Remove(filepath.Join(m.root, "AGENTS.md"))
	if p := health.ReferenceDrift(fs, m.root, ref); !strings.Contains(p, "cannot be read") {
		t.Errorf("no file: %q", p)
	}
	m.write(filepath.Join(m.root, "AGENTS.md"), ref)
	_, _, errs := m.run(health.Options{Force: true, Reference: "x"})
	if !strings.Contains(errs, "⚠\033[0m AGENTS.md's Command Reference differs") {
		t.Errorf("not warned: %q", errs)
	}
}

// Check 5's faults, as the bash's awk found them.
func TestTodayFaults(t *testing.T) {
	good := "### d\n\n#### Agenda\n\n#### Tasks\n\n#### Log\n\n- 0900 a\n- 0901 b\n\n#### Retrospective\n"
	for _, c := range []struct {
		name, text string
		want       []string
	}{
		{"good", good, nil},
		{"no heading", strings.Replace(good, "#### Agenda\n", "", 1), []string{"today.md missing required heading: #### Agenda"}},
		{"no blank after Log", strings.Replace(good, "#### Log\n\n", "#### Log\n", 1), []string{"today.md missing blank line after #### Log"}},
		{"Log last", "#### Agenda\n#### Tasks\n\n#### Retrospective\n#### Log\n", []string{"today.md missing blank line after #### Log"}},
		{"no blank before Retrospective", strings.Replace(good, "\n\n#### Retrospective", "\n#### Retrospective", 1), []string{"today.md missing blank line before #### Retrospective"}},
		{"gap in Log", strings.Replace(good, "- 0901 b\n", "\n- 0901 b\n", 1), []string{"today.md has 1 whitespace chunk(s) within log entries"}},
		{"two gaps", strings.Replace(good, "- 0901 b\n", "\n\n- 0901 b\n\n- 0902 c\n", 1), []string{"today.md has 2 whitespace chunk(s) within log entries"}},
		{"trailing blank only", strings.Replace(good, "- 0901 b\n\n", "- 0901 b\n\n\n", 1), nil},
	} {
		if got := health.TodayFaults(c.text); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: %q", c.name, got)
		}
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/tmp/a-b_c.d": "/tmp/a-b_c.d", "/é/x=y:z@w%v+u": "/é/x=y:z@w%v+u", "": "''",
		"/a b": `'/a b'`, "/x'y": `'/x'\''y'`, "~/a": `'~/a'`, "/a:~b": `'/a:~b'`, "/a#b,c!d^e": `'/a#b,c!d^e'`,
		"/a\tb": `$'/a\x09b'`, "/a\xffb'": `$'/a\xffb\''`,
	} {
		if got := health.ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
	// A quoted path reads back as itself in bash.
	for _, in := range []string{"/a b'c", "/a:~b", "/a\tb\xff'\\", "/é x"} {
		out, _, err := ops.Real().Exec(t.TempDir(), "bash", "-c", "printf %s "+health.ShellQuote(in))
		if err != nil || string(out) != in {
			t.Errorf("bash reads %q back as %q (%v)", in, out, err)
		}
	}
}

// Check 4c: a root-installed skill with no source warns, but synced, where
// Claude Code's own skill sync writes, is not Daisy's and never does.
func TestSkillDriftSparesSynced(t *testing.T) {
	m := newMachine(t)
	m.write(filepath.Join(m.uhome, ".claude", "skills", "synced", "abc", "SKILL.md"), "Claude Code's\n")
	m.write(filepath.Join(m.uhome, ".claude", "skills", "old-skill", "SKILL.md"), "stale\n")
	_, _, errs := m.run(health.Options{Force: true})
	if !strings.Contains(errs, "Root-installed skill 'old-skill'") {
		t.Errorf("no warning for the stale skill: %q", errs)
	}
	if strings.Contains(errs, "'synced'") {
		t.Errorf("synced warned as stale: %q", errs)
	}
}

// Check 4b covers Codex's tree as it covers Claude Code's: a copy with no
// source warns in either, naming its tree; the daisy skill init generates
// in .agents/skills has no source and never does.
func TestSkillDriftCoversBothTrees(t *testing.T) {
	m := newMachine(t)
	m.write(filepath.Join(m.ws, ".claude", "skills", "gone", "SKILL.md"), "stale\n")
	m.write(filepath.Join(m.ws, ".agents", "skills", "gone", "SKILL.md"), "stale\n")
	m.write(filepath.Join(m.ws, ".agents", "skills", "daisy", "SKILL.md"), "generated\n")
	_, _, errs := m.run(health.Options{Force: true})
	for _, tree := range []string{".claude/skills", ".agents/skills"} {
		if !strings.Contains(errs, "remove "+tree+"/gone manually") {
			t.Errorf("no warning for %s/gone: %q", tree, errs)
		}
	}
	if strings.Contains(errs, "'daisy'") {
		t.Errorf("the generated daisy skill warned as stale: %q", errs)
	}
}
