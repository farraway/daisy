package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"daisy/internal/cli"
	"daisy/internal/ops"
)

// gitIn runs git in dir and returns its stdout.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return string(out)
}

// scratch is a repository with history, a tracked .gitignore holding a
// user's own lines (one of them an entry init writes too), and an
// untracked file: what init then clean must hand back as it was.
func scratch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\nPLAN.md\n\n")
	writeFile(t, filepath.Join(dir, "README"), "hello\n")
	writeFile(t, filepath.Join(dir, "notes.txt"), "untracked\n")
	gitIn(t, dir, "add", ".gitignore", "README")
	gitIn(t, dir, "-c", "user.name=S", "-c", "user.email=s@x", "commit", "-q", "-m", "scratch")
	return dir
}

// tree is every path under dir but .git, with its kind, mode and content
// or link target: equal trees are equal byte for byte.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == ".git" {
			return filepath.SkipDir
		}
		fi, _ := os.Lstat(p)
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			files[rel] = "link " + target
		case fi.IsDir():
			files[rel] = fmt.Sprintf("dir %v", fi.Mode().Perm())
		default:
			b, _ := os.ReadFile(p)
			files[rel] = fmt.Sprintf("file %v %q", fi.Mode().Perm(), b)
		}
		return nil
	})
	return files
}

func diffTrees(a, b map[string]string) []string {
	var d []string
	for k, v := range a {
		if b[k] != v {
			d = append(d, fmt.Sprintf("%s: %q → %q", k, v, b[k]))
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			d = append(d, fmt.Sprintf("%s: added %q", k, b[k]))
		}
	}
	sort.Strings(d)
	return d
}

// keys is the repository's local values of the three identity keys.
func keys(t *testing.T, dir string) string {
	var b strings.Builder
	for _, k := range []string{"user.name", "user.email", "core.sshCommand"} {
		out, _ := exec.Command("git", "-C", dir, "config", "--local", "--get-all", "-z", k).Output()
		fmt.Fprintf(&b, "%s=%q;", k, out)
	}
	return b.String()
}

// pinned runs daisy with the clock pinned, so AGENTS.md's Built line is too.
func (m *machine) pinned(t *testing.T, cwd string, args ...string) (int, string, string) {
	t.Helper()
	o := &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		GetwdFn:  func() (string, error) { return cwd, nil },
		NowFn:    func() time.Time { return time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC) },
		// The binary is the root's own, as `make build` leaves it.
		ExecutableFn: func() (string, error) { return filepath.Join(m.root, "bin", "daisy"), nil },
	}
	var out, errb bytes.Buffer
	code := cli.Run(o, strings.NewReader(""), &out, &errb, args)
	return code, out.String(), errb.String()
}

func initMachine(t *testing.T) *machine {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.home, "skills", "nested", "myskill", "SKILL.md"), "---\nname: myskill\ndescription: \"Use when: testing\"\n---\nbody\n")
	writeFile(t, filepath.Join(m.home, "skills", "nested", "myskill", "ref.md"), "ref\n")
	writeFile(t, filepath.Join(m.root, "skills", "rootskill", "SKILL.md"), "---\nname: rootskill\ndescription: \"Use when: the root's\"\n---\nbody\n")
	os.MkdirAll(filepath.Join(m.home, "identities"), 0o755)
	writeFile(t, filepath.Join(m.home, "identities", "pelicanmoor"), "name=Pelican Moorhen\nemail=pelican@example.invalid\nsshkey=~/.ssh/id_p\n")
	return m
}

// Applying init twice equals applying it once, on every file it writes, the
// binding's links and the repository's identity keys — for the default
// identity and a named one with a key.
func TestInitTwiceEqualsOnce(t *testing.T) {
	m := initMachine(t)
	for _, id := range []string{"testhome", "pelicanmoor"} {
		ws := scratch(t)
		writeFile(t, filepath.Join(ws, ".claude", "settings.local.json"), "{\"permissions\": {\"allow\": [\"Bash(ls:*)\"]}, \"x\": 1}\n")
		if code, _, errs := m.pinned(t, ws, "init", id); code != 0 {
			t.Fatalf("%s: first init exit %d: %s", id, code, errs)
		}
		once, onceKeys := tree(t, ws), keys(t, ws)
		if code, out, errs := m.pinned(t, ws, "init", id); code != 0 || !strings.Contains(out, "Already initialized with identity: "+id) {
			t.Fatalf("%s: second init exit %d: %s%s", id, code, out, errs)
		}
		if d := diffTrees(once, tree(t, ws)); len(d) > 0 {
			t.Errorf("%s: the second init changed:\n%s", id, strings.Join(d, "\n"))
		}
		if k := keys(t, ws); k != onceKeys {
			t.Errorf("%s: keys %s → %s", id, onceKeys, k)
		}
		for _, f := range []string{".gitignore", ".cursorignore", ".claude/settings.local.json", ".cursor/rules/myskill.mdc", ".claude/skills/myskill/ref.md", ".cursor/rules/rootskill.mdc", ".claude/skills/rootskill/SKILL.md", ".agents/skills/rootskill/SKILL.md", ".daisy/AGENTS.md"} {
			if _, ok := once[f]; !ok {
				t.Errorf("%s: init wrote no %s", id, f)
			}
		}
		if !strings.Contains(once[".daisy/AGENTS.md"], `# Output: `+ws+`/.daisy/AGENTS.md\n`) {
			t.Errorf("AGENTS.md does not name where it is: %s", once[".daisy/AGENTS.md"])
		}
	}
}

// init then clean hands the repository back as it was: `git status`, every
// file, and the three keys exactly — whether or not they were set by hand
// before init, multi-valued and in Daisy's key form included.
func TestInitThenCleanRestores(t *testing.T) {
	m := initMachine(t)
	for name, preset := range map[string][][]string{
		"no keys": nil,
		"hand-set keys": {
			{"user.name", "Hand Set"}, {"--add", "user.name", "\tSecond\nvalue"},
			{"user.email", "hand@example.invalid"}, {"core.sshCommand", "ssh -v"},
		},
	} {
		for _, id := range []string{"testhome", "pelicanmoor"} {
			ws := scratch(t)
			for _, args := range preset {
				gitIn(t, ws, append([]string{"config", "--local"}, args...)...)
			}
			status, before, files := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all"), keys(t, ws), tree(t, ws)
			if code, _, errs := m.pinned(t, ws, "init", id); code != 0 {
				t.Fatalf("%s/%s: init exit %d: %s", name, id, code, errs)
			}
			if keys(t, ws) == before {
				t.Fatalf("%s/%s: init applied nothing", name, id)
			}
			if code, _, errs := m.pinned(t, ws, "clean", "-f"); code != 0 {
				t.Fatalf("%s/%s: clean exit %d: %s", name, id, code, errs)
			}
			if got := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all"); got != status {
				t.Errorf("%s/%s: git status %q → %q", name, id, status, got)
			}
			if got := keys(t, ws); got != before {
				t.Errorf("%s/%s: keys %s → %s", name, id, before, got)
			}
			if d := diffTrees(files, tree(t, ws)); len(d) > 0 {
				t.Errorf("%s/%s: files differ:\n%s", name, id, strings.Join(d, "\n"))
			}
		}
	}
}

// The root bound as a workspace (D44): a root holding this root's own
// .gitignore, bound to a home whose skill (myskill) the root's .gitignore
// does not name, stays as git has it through init, a re-init, `plan new`,
// `plan new --spec` and `clean -f`: nothing untracked, and .gitignore byte
// for byte as the root ships it. Its lines ignore what init and plan write
// wholesale, under the root's own comment, so init adds no block and clean
// finds none to strip.
func TestBindingTheRootLeavesItAsGitHasIt(t *testing.T) {
	m := initMachine(t)
	shipped, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(m.root, ".gitignore"), string(shipped))
	gitIn(t, m.root, "init", "-q")
	gitIn(t, m.root, "add", "-A")
	gitIn(t, m.root, "-c", "user.name=S", "-c", "user.email=s@x", "commit", "-q", "-m", "root")
	// After each command, the files it leaves in the root that git would
	// otherwise publish: there, and ignored.
	for _, c := range []struct {
		args  []string
		there []string
	}{
		{[]string{"init", "testhome"}, []string{".daisy/AGENTS.md", ".claude/skills/myskill/SKILL.md", ".agents/skills/rootskill/SKILL.md", ".cursor/rules/myskill.mdc", ".claude/settings.local.json", ".cursorignore"}},
		{[]string{"init", "testhome"}, []string{".claude/skills/myskill/SKILL.md"}},
		{[]string{"plan", "new", "Ship it"}, []string{"PLAN.md"}},
		{[]string{"plan", "new", "--spec", "A draft"}, []string{"A-draft_PLAN.md"}},
		{[]string{"clean", "-f"}, []string{"PLAN.md", "A-draft_PLAN.md"}}, // clean never removes a plan
	} {
		if code, _, errs := runWith(m.planOps(m.root), c.args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", c.args, code, errs)
		}
		for _, p := range c.there {
			if _, err := os.Lstat(filepath.Join(m.root, p)); err != nil {
				t.Errorf("after %v: %v", c.args, err)
			}
		}
		if st := gitIn(t, m.root, "status", "--porcelain", "--untracked-files=all"); st != "" {
			t.Errorf("after %v, git status in the root:\n%s", c.args, st)
		}
		if b, _ := os.ReadFile(filepath.Join(m.root, ".gitignore")); string(b) != string(shipped) {
			t.Errorf("after %v, the root's .gitignore is\n%s", c.args, b)
		}
	}
	if _, err := os.Lstat(filepath.Join(m.root, ".claude", "skills", "myskill")); err == nil {
		t.Error("clean left the home's skill in the root")
	}
}

// init installs the root's skills beside the home's, in .claude/skills and
// its Codex twin .agents/skills, each with its Cursor stub, a nested one
// by its directory's name, byte for byte as the root has it; a home skill
// of the same name wins: its override of the root's (the root's directory,
// the home's SKILL.md), or, at another path, the home's alone. The root's
// Read and Edit rules join .claude/settings.local.json. git ignores all of
// it, and clean takes all of it out. Nothing is written under the user's
// ~/.claude.
func TestInitInstallsRootSkills(t *testing.T) {
	m := initMachine(t)
	writeFile(t, filepath.Join(m.root, "skills", "rootskill", "ref.md"), "the root's ref\n")
	writeFile(t, filepath.Join(m.root, "skills", "group", "deep", "SKILL.md"), "---\nname: deep\ndescription: nested\n---\n")
	writeFile(t, filepath.Join(m.root, "skills", "both", "SKILL.md"), "---\nname: both\ndescription: the root's\n---\n")
	writeFile(t, filepath.Join(m.root, "skills", "both", "ref.md"), "both's ref\n")
	writeFile(t, filepath.Join(m.home, "skills", "both", "SKILL.md"), "---\nname: both\ndescription: the home's\n---\n")
	writeFile(t, filepath.Join(m.root, "skills", "elsewhere", "SKILL.md"), "---\nname: elsewhere\ndescription: the root's\n---\n")
	writeFile(t, filepath.Join(m.home, "skills", "mine", "elsewhere", "SKILL.md"), "---\nname: elsewhere\ndescription: the home's\n---\n")
	ws := scratch(t)
	status := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all")
	if code, out, errs := m.pinned(t, ws, "init", "testhome"); code != 0 || !strings.Contains(out, "Installed 5 skill(s)") {
		t.Fatalf("init exit %d: %s%s", code, out, errs)
	}
	for _, tree := range []string{".claude/skills", ".agents/skills"} {
		for n, src := range map[string]string{"rootskill": "skills/rootskill", "deep": "skills/group/deep"} {
			if d := diffTrees(treeOf(t, filepath.Join(m.root, src)), treeOf(t, filepath.Join(ws, tree, n))); len(d) > 0 {
				t.Errorf("%s/%s is not the root's %s:\n%s", tree, n, src, strings.Join(d, "\n"))
			}
		}
		for f, want := range map[string]string{"both/SKILL.md": "the home's", "both/ref.md": "both's ref", "elsewhere/SKILL.md": "the home's"} {
			if b, _ := os.ReadFile(filepath.Join(ws, tree, f)); !strings.Contains(string(b), want) {
				t.Errorf("%s/%s: %q, want %q", tree, f, b, want)
			}
		}
	}
	twins(t, ws)
	for _, n := range []string{"rootskill", "deep", "both", "elsewhere"} {
		if b, _ := os.ReadFile(filepath.Join(ws, ".cursor", "rules", n+".mdc")); !strings.Contains(string(b), "Read `.claude/skills/"+n+"/SKILL.md`") {
			t.Errorf("no stub for %s: %q", n, b)
		}
	}
	settings, _ := os.ReadFile(filepath.Join(ws, ".claude", "settings.local.json"))
	for _, r := range []string{`"Read(//` + strings.TrimPrefix(m.root, "/") + `/**)"`, `"Edit(//` + strings.TrimPrefix(m.root, "/") + `/**)"`, `"Bash(daisy:*)"`, `"Read(.daisy/**)"`} {
		if !strings.Contains(string(settings), r) {
			t.Errorf("settings lack %s: %s", r, settings)
		}
	}
	if got := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all"); got != " M .gitignore\n"+status {
		t.Errorf("git sees what init wrote: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(m.vars["HOME"], ".claude")); err == nil {
		t.Error("init wrote under ~/.claude")
	}
	if code, _, errs := m.pinned(t, ws, "clean", "-f"); code != 0 {
		t.Fatalf("clean exit %d: %s", code, errs)
	}
	for _, p := range []string{".claude", ".agents", ".cursor"} {
		if _, err := os.Lstat(filepath.Join(ws, p)); err == nil {
			t.Errorf("clean left %s: %v", p, tree(t, filepath.Join(ws, p)))
		}
	}
}

// A root skill whose name the workspace already uses for a skill or a
// Cursor rule of its own (one init's stub does not claim) is not
// installed: init warns, writes none of its copies, stub or ignore lines,
// and clean leaves the workspace's own as it was. A root skill init did
// install is refreshed, and a home skill's name is still the home's
// choice (TestStubIsTheRecordOfInstalledSkills).
func TestInitKeepsAWorkspacesOwnSkillOfARootName(t *testing.T) {
	m := initMachine(t)
	writeFile(t, filepath.Join(m.root, "skills", "css", "SKILL.md"), "---\nname: css\ndescription: the root's\n---\n")
	ws := scratch(t)
	writeFile(t, filepath.Join(ws, ".claude", "skills", "rootskill", "SKILL.md"), "the project's own\n")
	writeFile(t, filepath.Join(ws, ".cursor", "rules", "css.mdc"), "Use BEM.\n")
	gitIn(t, ws, "add", ".claude", ".cursor")
	gitIn(t, ws, "-c", "user.name=S", "-c", "user.email=s@x", "commit", "-q", "-m", "own")
	status := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all")
	for i := 0; i < 2; i++ {
		code, _, errs := m.pinned(t, ws, "init", "testhome")
		if code != 0 {
			t.Fatalf("init exit %d: %s", code, errs)
		}
		for _, want := range []string{"The root's skill rootskill is not installed: this workspace has its own " + filepath.Join(ws, ".claude", "skills", "rootskill"), "The root's skill css is not installed: this workspace has its own " + filepath.Join(ws, ".cursor", "rules", "css.mdc")} {
			if !strings.Contains(errs, want) {
				t.Errorf("no warning %q: %s", want, errs)
			}
		}
	}
	if b, _ := os.ReadFile(filepath.Join(ws, ".claude", "skills", "rootskill", "SKILL.md")); string(b) != "the project's own\n" {
		t.Errorf("the workspace's skill became %q", b)
	}
	for _, p := range []string{".agents/skills/rootskill", ".claude/skills/css", ".agents/skills/css", ".cursor/rules/rootskill.mdc"} {
		if _, err := os.Lstat(filepath.Join(ws, p)); err == nil {
			t.Errorf("init wrote %s", p)
		}
	}
	if got := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all"); got != " M .gitignore\n"+status {
		t.Errorf("git status after init %q", got)
	}
	if code, _, errs := m.pinned(t, ws, "clean", "-f"); code != 0 {
		t.Fatalf("clean exit %d: %s", code, errs)
	}
	if got := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all"); got != status {
		t.Errorf("git status after clean %q, want %q", got, status)
	}
}

// treeOf is tree with each directory's mode left out: a copy's directories
// are made by the copy.
func treeOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := tree(t, dir)
	for k, v := range files {
		if strings.HasPrefix(v, "dir ") {
			delete(files, k)
		}
	}
	return files
}

// A refused init writes nothing: every refusal is decided before the first write.
func TestRefusedInitWritesNothing(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	m.pinned(t, ws, "init", "pelicanmoor")
	gitconfig := filepath.Join(m.home, "gitconfig")
	unreadable := filepath.Join(m.home, "skills", "nested", "myskill", "secret.md")
	defaultID, err := os.ReadFile(gitconfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, obstacle := range []func(){
		func() { writeFile(t, filepath.Join(ws, ".claude", "settings.local.json"), "{ not json") },
		// The default identity may be incomplete, never malformed.
		func() {
			writeFile(t, gitconfig, "name=Fixture Author\nemail=fixture@example.invalid\nsshkey=~/.ssh/my key\n")
		},
		func() { writeFile(t, gitconfig, "sshkey=~/.ssh/my key\n") }, // incomplete and malformed
		// A home skill's file that cannot be read is found before the first write.
		func() { writeFile(t, unreadable, "x\n"); os.Chmod(unreadable, 0o000) },
		func() {
			os.Remove(filepath.Join(ws, ".daisy", "identity"))
			os.Mkdir(filepath.Join(ws, ".daisy", "identity"), 0o755)
		},
	} {
		obstacle()
		files, k := tree(t, ws), keys(t, ws)
		code, out, errs := m.pinned(t, ws, "init", "testhome")
		if code != 1 || out != "" || errs == "" {
			t.Errorf("exit %d out %q err %q", code, out, errs)
		}
		if d := diffTrees(files, tree(t, ws)); len(d) > 0 || keys(t, ws) != k {
			t.Errorf("a refused init wrote:\n%s\nkeys %s → %s", strings.Join(d, "\n"), k, keys(t, ws))
		}
		os.RemoveAll(filepath.Join(ws, ".claude", "settings.local.json"))
		os.Remove(filepath.Join(ws, ".daisy", "identity"))
		writeFile(t, gitconfig, string(defaultID))
		os.Remove(unreadable)
		m.pinned(t, ws, "init", "pelicanmoor")
	}
}

// failingSwap fails the rename of a stage onto .daisy, once: the swap's
// second half, after .daisy/ has been moved aside.
type failingSwap struct {
	ops.FS
	failed bool
}

func (f *failingSwap) Rename(from, to string) error {
	if !f.failed && strings.Contains(from, ".daisy.stage.") && filepath.Base(to) == ".daisy" {
		f.failed = true
		return errors.New("forced failure")
	}
	return f.FS.Rename(from, to)
}

type withFS struct {
	ops.Ops
	fs ops.FS
}

func (o withFS) FS() ops.FS { return o.fs }

// The swap failing rolls everything back: .daisy/ comes back from aside, the
// keys are as they were byte for byte (a hand-set value included), and
// nothing is left beside .daisy/.
func TestSwapFailureRollsBack(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	m.pinned(t, ws, "init", "pelicanmoor")
	writeFile(t, filepath.Join(ws, ".daisy", "project"), "alpha\n")
	gitIn(t, ws, "config", "--local", "--add", "user.email", "multi\nline")
	daisyBefore, k := tree(t, filepath.Join(ws, ".daisy")), keys(t, ws)
	o := withFS{&ops.Fake{GetenvFn: func(k string) string { return m.vars[k] }, GetwdFn: func() (string, error) { return ws, nil }}, &failingSwap{FS: ops.Real().FS()}}
	var out, errb bytes.Buffer
	code := cli.Run(o, strings.NewReader(""), &out, &errb, []string{"init", "testhome"})
	if code != 1 || !strings.Contains(errb.String(), "forced failure") || !strings.Contains(errb.String(), "did not finish") {
		t.Errorf("exit %d err %q", code, errb.String())
	}
	if d := diffTrees(daisyBefore, tree(t, filepath.Join(ws, ".daisy"))); len(d) > 0 {
		t.Errorf(".daisy/ is not as it was:\n%s", strings.Join(d, "\n"))
	}
	if keys(t, ws) != k {
		t.Errorf("keys %s → %s", k, keys(t, ws))
	}
	if left, _ := filepath.Glob(filepath.Join(ws, ".daisy.*")); len(left) > 0 {
		t.Errorf("left beside .daisy/: %v", left)
	}
}

// install against a temp HOME: the link and the rc line, and nothing under
// ~/.claude (skills and permissions are init's, per workspace); applied
// twice equals once; plain install refuses once installed.
func TestInstall(t *testing.T) {
	m := newMachine(t)
	home := m.vars["HOME"]
	m.vars["SHELL"] = "/bin/zsh"
	writeFile(t, filepath.Join(m.root, "skills", "s", "SKILL.md"), "---\nname: s\n---\n")
	writeFile(t, filepath.Join(home, ".zshenv"), "export PATH=x\n")
	code, out, errs := m.pinned(t, m.ws, "install")
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, out, errs)
	}
	if l, _ := os.Readlink(filepath.Join(home, "bin", "daisy")); l != filepath.Join(m.root, "bin", "daisy") {
		t.Errorf("link %q", l)
	}
	if !strings.Contains(errs, "is not built yet") {
		t.Errorf("no warning that the binary is not built: %q", errs)
	}
	rc, _ := os.ReadFile(filepath.Join(home, ".zshenv"))
	if string(rc) != "export PATH=x\n\n# daisy\nexport DAISY_ROOT=\""+m.root+"\"\n" {
		t.Errorf("rc %q", rc)
	}
	if _, err := os.Lstat(filepath.Join(home, ".claude")); err == nil {
		t.Errorf("install wrote under ~/.claude: %v", tree(t, filepath.Join(home, ".claude")))
	}
	once := tree(t, home)
	if code, _, errs := m.pinned(t, m.ws, "install"); code != 1 || !strings.Contains(errs, "already installed") {
		t.Errorf("second install: exit %d err %q", code, errs)
	}
	if code, _, _ := m.pinned(t, m.ws, "install", "--update"); code != 0 {
		t.Errorf("update exit %d", code)
	}
	if d := diffTrees(once, tree(t, home)); len(d) > 0 {
		t.Errorf("install --update changed:\n%s", strings.Join(d, "\n"))
	}
	// A moved root: the rc line is replaced in place.
	other := t.TempDir()
	os.MkdirAll(filepath.Join(other, "templates"), 0o755)
	m.root, m.vars["DAISY_ROOT"] = other, other
	m.pinned(t, m.ws, "install")
	if rc, _ := os.ReadFile(filepath.Join(home, ".zshenv")); string(rc) != "export PATH=x\n\n# daisy\nexport DAISY_ROOT=\""+other+"\"\n" {
		t.Errorf("rc after a move %q", rc)
	}
}

// install --update takes back what earlier installs put under ~/.claude
// and nothing else: of the skills, only the copies of a root skill (one an
// older root installed, its every file a version in the root's history),
// never a user's own skill, a root-named one the user edited or synced,
// where Claude Code's own skill sync writes; of the settings, only Daisy's
// rules, in every form an install wrote, the rest in place. With none of
// Daisy's rules there, no file under ~/.claude is written at all.
func TestInstallRemovesOnlyRootSkillCopies(t *testing.T) {
	m := newMachine(t)
	home := m.vars["HOME"]
	skills := filepath.Join(home, ".claude", "skills")
	gitIn(t, m.root, "init", "-q")
	writeFile(t, filepath.Join(m.root, "skills", "s", "SKILL.md"), "---\nname: s\n---\nv1\n")
	writeFile(t, filepath.Join(m.root, "skills", "e", "SKILL.md"), "---\nname: e\n---\n")
	gitIn(t, m.root, "add", "skills")
	gitIn(t, m.root, "-c", "user.name=R", "-c", "user.email=r@x", "commit", "-q", "-m", "one")
	writeFile(t, filepath.Join(m.root, "skills", "s", "SKILL.md"), "---\nname: s\n---\nv2\n")
	gitIn(t, m.root, "-c", "user.name=R", "-c", "user.email=r@x", "commit", "-q", "-am", "two")
	writeFile(t, filepath.Join(skills, "s", "SKILL.md"), "---\nname: s\n---\nv1\n") // the older root's copy
	writeFile(t, filepath.Join(skills, "e", "SKILL.md"), "---\nname: e\n---\nmy edit\n")
	writeFile(t, filepath.Join(skills, "mine", "SKILL.md"), "mine\n")
	writeFile(t, filepath.Join(skills, "synced", "abc", "SKILL.md"), "Claude Code's\n")
	userOnly := "{\n  \"model\": \"x\",\n  \"permissions\": {\"allow\": [\"Bash(ls:*)\"]}\n}\n"
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settingsPath, userOnly)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	filepath.Walk(filepath.Join(home, ".claude"), func(p string, _ os.FileInfo, _ error) error { return os.Chtimes(p, past, past) })
	before := tree(t, filepath.Join(home, ".claude"))

	code, out, errs := m.pinned(t, m.ws, "install", "--update")
	if code != 0 || !strings.Contains(out, "Removed 1 copy of a root skill") || !strings.Contains(out, "skills/e") {
		t.Fatalf("exit %d out %q err %q", code, out, errs)
	}
	after := tree(t, filepath.Join(home, ".claude"))
	for k := range before {
		if k == "skills/s" || strings.HasPrefix(k, "skills/s/") {
			if _, ok := after[k]; ok {
				t.Errorf("%s survived", k)
			}
			delete(before, k)
		}
	}
	if d := diffTrees(before, after); len(d) > 0 {
		t.Errorf("~/.claude changed beyond the copy's removal:\n%s", strings.Join(d, "\n"))
	}
	filepath.Walk(filepath.Join(home, ".claude"), func(p string, fi os.FileInfo, _ error) error {
		if fi != nil && !fi.IsDir() && !fi.ModTime().Equal(past) {
			t.Errorf("%s was written", p)
		}
		return nil
	})

	// Daisy's rules, as installs wrote them, come out; the rest stays.
	r := m.root
	writeFile(t, settingsPath, "{\n  \"model\": \"x\",\n  \"permissions\": {\n    \"allow\": [\n      \"Read(/"+r+"/**)\",\n      \"Bash(ls:*)\",\n      \"Edit(/"+r+"/**)\",\n      \"Bash(daisy:*)\",\n      \"Read(//"+r+"/**)\",\n      \"Edit("+r+"/**)\",\n      \"Bash("+r+"/daisy/scripts/*)\"\n    ],\n    \"deny\": []\n  }\n}\n")
	if code, out, errs := m.pinned(t, m.ws, "install", "--update"); code != 0 || !strings.Contains(out, "Removed Daisy's permissions from ~/.claude/settings.json") {
		t.Fatalf("exit %d out %q err %q", code, out, errs)
	}
	if b, _ := os.ReadFile(settingsPath); string(b) != "{\n  \"model\": \"x\",\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(ls:*)\"\n    ],\n    \"deny\": []\n  }\n}\n" {
		t.Errorf("settings %s", b)
	}
	// Nothing but Daisy's rules: the file goes, as clean's does.
	writeFile(t, settingsPath, "{\"permissions\": {\"allow\": [\"Bash(daisy:*)\"]}}\n")
	m.pinned(t, m.ws, "install", "--update")
	if _, err := os.Lstat(settingsPath); err == nil {
		t.Error("a settings file holding only Daisy's rules was kept")
	}
}

// Plain install is "already installed" only when the link and the rc line
// both name this root. The link right but the rc line naming another root
// (a moved root installed only with --update): it writes the rc line, the
// fix a root-mismatch refusal prints; and the other way round.
func TestInstallRewritesAStaleRcLine(t *testing.T) {
	m := newMachine(t)
	home := m.vars["HOME"]
	m.vars["SHELL"] = "/bin/zsh"
	writeFile(t, filepath.Join(home, ".zshenv"), "export DAISY_ROOT=\"/old\"\n")
	if code, _, errs := m.pinned(t, m.ws, "install", "--update"); code != 0 {
		t.Fatalf("update: exit %d err %q", code, errs)
	}
	code, out, errs := m.pinned(t, m.ws, "install")
	if rc, _ := os.ReadFile(filepath.Join(home, ".zshenv")); code != 0 || string(rc) != "export DAISY_ROOT=\""+m.root+"\"\n" {
		t.Errorf("exit %d out %q err %q rc %q", code, out, errs, rc)
	}
	// And the other way: the rc line right, the link naming another root.
	link := filepath.Join(home, "bin", "daisy")
	os.Remove(link)
	os.Symlink("/old/bin/daisy", link)
	if code, _, errs := m.pinned(t, m.ws, "install"); code != 0 {
		t.Errorf("stale link: exit %d err %q", code, errs)
	}
	if l, _ := os.Readlink(link); l != filepath.Join(m.root, "bin", "daisy") {
		t.Errorf("link %q", l)
	}
	// A stale export below the right line wins in the shell: not installed.
	writeFile(t, filepath.Join(home, ".zshenv"), "export DAISY_ROOT=\""+m.root+"\"\n\texport DAISY_ROOT=\"/old\"\n")
	code, out, errs = m.pinned(t, m.ws, "install")
	if rc, _ := os.ReadFile(filepath.Join(home, ".zshenv")); code != 0 || string(rc) != "export DAISY_ROOT=\""+m.root+"\"\n\texport DAISY_ROOT=\""+m.root+"\"\n" || !strings.Contains(out, "(was: /old)") {
		t.Errorf("second line stale: exit %d out %q err %q rc %q", code, out, errs, rc)
	}
	// The lines checked are the lines rewritten, so install converges: a
	// comment or a line using $DAISY_ROOT is neither (fish too), and an
	// unquoted right line is right.
	for _, c := range []struct{ shell, rc, text string }{
		{"/bin/zsh", ".zshenv", "# DAISY_ROOT=\"/old\" until the move\nexport DAISY_ROOT=\"" + m.root + "\"\n"},
		{"/bin/zsh", ".zshenv", "export DAISY_ROOT=" + m.root + "\n"},
		{"/bin/zsh", ".zshenv", "export DAISY_ROOT='" + m.root + "'\n"},
		{"/usr/bin/fish", ".config/fish/config.fish", "set -gx DAISY_ROOT '" + m.root + "'\n"},
		{"/bin/zsh", ".zshenv", "# export DAISY_ROOT=\"/old\"\nif true; then\n  export DAISY_ROOT=\"" + m.root + "\"\nfi\n"},
		{"/usr/bin/fish", ".config/fish/config.fish", "set -gx DAISY_ROOT \"" + m.root + "\"\nset -gx PATH $DAISY_ROOT/bin $PATH\n"},
	} {
		m.vars["SHELL"] = c.shell
		writeFile(t, filepath.Join(home, c.rc), c.text)
		if code, out, errs := m.pinned(t, m.ws, "install"); code != 1 || !strings.Contains(errs, "already installed") {
			t.Errorf("%s %q: exit %d out %q err %q", c.shell, c.text, code, out, errs)
		}
	}
}

// A $DAISY_ROOT naming another root than the binary's: install and install
// --update exit 1 naming both, and HOME is left as it was.
func TestInstallRefusesAnotherRoot(t *testing.T) {
	m := newMachine(t)
	home := m.vars["HOME"]
	m.vars["SHELL"] = "/bin/zsh"
	writeFile(t, filepath.Join(home, ".zshenv"), "export PATH=x\n")
	stale := t.TempDir()
	os.MkdirAll(filepath.Join(stale, "templates"), 0o755)
	m.vars["DAISY_ROOT"] = stale
	before := tree(t, home)
	for _, args := range [][]string{{"install"}, {"install", "--update"}} {
		code, out, errs := m.pinned(t, m.ws, args...)
		if code != 1 || out != "" || !strings.Contains(errs, stale) || !strings.Contains(errs, "DAISY_ROOT="+m.root+" "+m.root+"/bin/daisy install  (") {
			t.Errorf("%v: exit %d out %q err %q", args, code, out, errs)
		}
	}
	if d := diffTrees(before, tree(t, home)); len(d) > 0 {
		t.Errorf("HOME changed:\n%s", strings.Join(d, "\n"))
	}
}

// With nothing set, install finds the root it was built in.
func TestInstallBootstrapsFromTheBinary(t *testing.T) {
	m := newMachine(t)
	delete(m.vars, "DAISY_ROOT")
	o := &ops.Fake{
		GetenvFn:     func(k string) string { return m.vars[k] },
		GetwdFn:      func() (string, error) { return m.ws, nil },
		ExecutableFn: func() (string, error) { return filepath.Join(m.root, "bin", "daisy"), nil },
	}
	var out, errb bytes.Buffer
	if code := cli.Run(o, strings.NewReader(""), &out, &errb, []string{"install", "--update"}); code != 0 || !strings.Contains(out.String(), "DAISY_ROOT: "+m.root+"\n") {
		t.Errorf("exit %d out %q err %q", code, out.String(), errb.String())
	}
	o.ExecutableFn = func() (string, error) { return filepath.Join(t.TempDir(), "daisy"), nil }
	errb.Reset()
	if code := cli.Run(o, strings.NewReader(""), &out, &errb, []string{"install", "--update"}); code != 1 || !strings.Contains(errb.String(), "DAISY_ROOT is not set") {
		t.Errorf("not in a root: exit %d err %q", code, errb.String())
	}
}

func TestIdentityList(t *testing.T) {
	m := initMachine(t)
	code, out, errs := m.pinned(t, m.ws, "identity", "list")
	if code != 0 || out != "testhome       testhome\npelicanmoor    testhome\n" || errs != "" {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
}

// A path init must write that cannot take what it writes is refused before
// the first write: no repository, no rule, nothing.
func TestInitRefusesAnUnwritablePathFirst(t *testing.T) {
	m := initMachine(t)
	for _, obstacle := range []func(ws string){
		func(ws string) { os.MkdirAll(filepath.Join(ws, ".claude", "commands", "daisy.md"), 0o755) },
		func(ws string) { writeFile(t, filepath.Join(ws, ".claude"), "x\n") },
		func(ws string) { writeFile(t, filepath.Join(ws, ".cursor", "rules"), "x\n") },
		func(ws string) { os.MkdirAll(filepath.Join(ws, ".cursor", "rules", "myskill.mdc"), 0o755) },
		func(ws string) { writeFile(t, filepath.Join(ws, ".claude", "skills"), "x\n") },
		func(ws string) { writeFile(t, filepath.Join(ws, ".agents", "skills"), "x\n") },
		func(ws string) {
			os.MkdirAll(filepath.Join(ws, "somedir"), 0o755)
			os.MkdirAll(filepath.Join(ws, ".claude", "commands"), 0o755)
			os.Symlink("../../somedir", filepath.Join(ws, ".claude", "commands", "daisy.md"))
		},
	} {
		ws := t.TempDir()
		obstacle(ws)
		files := tree(t, ws)
		code, out, errs := m.pinned(t, ws, "init", "testhome")
		if code != 1 || out != "" || !strings.Contains(errs, "move it aside") {
			t.Errorf("exit %d out %q err %q", code, out, errs)
		}
		if d := diffTrees(files, tree(t, ws)); len(d) > 0 {
			t.Errorf("wrote:\n%s", strings.Join(d, "\n"))
		}
		if _, err := os.Stat(filepath.Join(ws, ".git")); err == nil {
			t.Error("made a repository")
		}
	}
}

// The same with a home that has no skills.
func TestInitRefusesAFileAtSkillsForASkillLessHome(t *testing.T) {
	m := newMachine(t)
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, ".claude", "skills"), "x\n")
	files := tree(t, ws)
	if code, _, _ := m.pinned(t, ws, "init", "testhome"); code != 1 {
		t.Errorf("exit %d", code)
	}
	if d := diffTrees(files, tree(t, ws)); len(d) > 0 {
		t.Errorf("wrote:\n%s", strings.Join(d, "\n"))
	}
}

// clean takes out the skills init installed even when the home is gone:
// the .gitignore block names them.
func TestCleanRemovesSkillsOfAGoneHome(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	m.pinned(t, ws, "init", "testhome")
	writeFile(t, filepath.Join(m.config, "homes"), "testhome=/nonexistent\n")
	if code, _, errs := m.pinned(t, ws, "clean", "-f"); code != 0 {
		t.Fatalf("exit %d %s", code, errs)
	}
	for _, p := range []string{".claude/skills/myskill", ".cursor/rules/myskill.mdc"} {
		if _, err := os.Lstat(filepath.Join(ws, p)); err == nil {
			t.Errorf("%s left behind", p)
		}
	}
}

// A user's own skill stays through clean, whatever .gitignore says: only a
// stub in init's form marks a skill as init's. A gone home's skills, odd
// names included, still go.
func TestCleanKeepsAUsersOwnSkill(t *testing.T) {
	m := initMachine(t)
	writeFile(t, filepath.Join(m.home, "skills", "_draft", "SKILL.md"), "---\ndescription: d\n---\n")
	ws := scratch(t)
	writeFile(t, filepath.Join(ws, ".gitignore"), "node_modules/\n.cursor/rules/myskill.mdc\n")
	m.pinned(t, ws, "init", "testhome")
	writeFile(t, filepath.Join(ws, ".claude", "skills", "mine", "SKILL.md"), "mine\n")
	writeFile(t, filepath.Join(ws, ".cursor", "rules", "mine.mdc"), "my rule\n")
	writeFile(t, filepath.Join(ws, ".cursor", "rules", "..mdc"), "x\n")
	f, _ := os.OpenFile(filepath.Join(ws, ".gitignore"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(".claude/skills/mine/\n.cursor/rules/mine.mdc\n.claude/skills/../\n")
	f.Close()
	writeFile(t, filepath.Join(m.config, "homes"), "testhome=/nonexistent\n")
	if code, _, errs := m.pinned(t, ws, "clean", "-f"); code != 0 {
		t.Fatalf("exit %d %s", code, errs)
	}
	for _, p := range []string{".claude/skills/mine/SKILL.md", ".cursor/rules/mine.mdc", ".cursor/rules/..mdc"} {
		if _, err := os.Stat(filepath.Join(ws, p)); err != nil {
			t.Errorf("%s removed", p)
		}
	}
	for _, p := range []string{".claude/skills/myskill", ".cursor/rules/myskill.mdc", ".claude/skills/_draft", ".cursor/rules/_draft.mdc"} {
		if _, err := os.Lstat(filepath.Join(ws, p)); err == nil {
			t.Errorf("%s left behind", p)
		}
	}
}

// Which skills are init's is the stub alone: a description that keeps
// quotes at its ends after init's strip still marks its skill; a stub
// reached through a link, or a home skill whose stub is the user's own, does not.
func TestStubIsTheRecordOfInstalledSkills(t *testing.T) {
	m := initMachine(t)
	writeFile(t, filepath.Join(m.home, "skills", "dqend", "SKILL.md"), "---\ndescription: \"Use when asked to \"ship\"\"\n---\n")
	ws := scratch(t)
	m.pinned(t, ws, "init", "testhome")
	elsewhere := t.TempDir()
	b, _ := os.ReadFile(filepath.Join(ws, ".cursor", "rules", "myskill.mdc"))
	writeFile(t, filepath.Join(elsewhere, "foo.mdc"), strings.ReplaceAll(string(b), "myskill", "foo"))
	os.Symlink(filepath.Join(elsewhere, "foo.mdc"), filepath.Join(ws, ".cursor", "rules", "foo.mdc"))
	writeFile(t, filepath.Join(ws, ".claude", "skills", "foo", "SKILL.md"), "my own work\n")
	writeFile(t, filepath.Join(ws, ".claude", "skills", "deploy", "SKILL.md"), "mine\n")
	writeFile(t, filepath.Join(ws, ".cursor", "rules", "deploy.mdc"), "my deploy rule\n")
	writeFile(t, filepath.Join(m.home, "skills", "deploy", "SKILL.md"), "---\ndescription: d\n---\n")
	os.RemoveAll(filepath.Join(m.home, "skills", "dqend"))
	m.pinned(t, ws, "init", "testhome") // sweeps dqend; deploy now installed over the user's
	if _, err := os.Lstat(filepath.Join(ws, ".claude", "skills", "dqend")); err == nil {
		t.Error("the stale dqend copy survived init's sweep")
	}
	if _, err := os.Stat(filepath.Join(ws, ".claude", "skills", "foo", "SKILL.md")); err != nil {
		t.Error("a skill behind a linked stub was swept")
	}
	writeFile(t, filepath.Join(ws, ".cursor", "rules", "deploy.mdc"), "my deploy rule again\n")
	m.pinned(t, ws, "clean", "-f")
	if _, err := os.Stat(filepath.Join(ws, ".cursor", "rules", "deploy.mdc")); err != nil {
		t.Error("clean removed a rule that is not init's stub")
	}
}

// A home skill named like a core rule is refused before any write; a stub
// standing as a link is replaced by init's own file, so clean still finds it.
func TestSkillStubsAreInitsOwnFiles(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	other := filepath.Join(t.TempDir(), "user.mdc")
	writeFile(t, other, "the user's\n")
	os.MkdirAll(filepath.Join(ws, ".cursor", "rules"), 0o755)
	os.Symlink(other, filepath.Join(ws, ".cursor", "rules", "myskill.mdc"))
	if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
		t.Fatalf("exit %d %s", code, errs)
	}
	if b, _ := os.ReadFile(other); string(b) != "the user's\n" {
		t.Errorf("written through the link: %q", b)
	}
	m.pinned(t, ws, "clean", "-f")
	if _, err := os.Lstat(filepath.Join(ws, ".claude", "skills", "myskill")); err == nil {
		t.Error("clean left the skill behind")
	}
	writeFile(t, filepath.Join(m.home, "skills", "daisy", "SKILL.md"), "---\ndescription: d\n---\n")
	ws2 := t.TempDir()
	if code, out, _ := m.pinned(t, ws2, "init", "testhome"); code != 1 || out != "" {
		t.Errorf("a skill named daisy: exit %d out %q", code, out)
	}
}

// A core rule standing as a link is replaced too, not written through.
func TestCoreRulesAreInitsOwnFiles(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	m.pinned(t, ws, "init", "testhome")
	rules := filepath.Join(ws, ".cursor", "rules")
	os.Remove(filepath.Join(rules, "daisy.mdc"))
	os.Symlink("myskill.mdc", filepath.Join(rules, "daisy.mdc"))
	dir := t.TempDir()
	os.Remove(filepath.Join(rules, "daisy-plan.mdc"))
	os.Symlink(dir, filepath.Join(rules, "daisy-plan.mdc"))
	if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
		t.Fatalf("exit %d %s", code, errs)
	}
	want, _ := os.ReadFile(filepath.Join(m.root, "templates", "cursor-rule.mdc"))
	if b, _ := os.ReadFile(filepath.Join(rules, "daisy.mdc")); string(b) != string(want) {
		t.Errorf("daisy.mdc is %q", b)
	}
	if fi, _ := os.Lstat(filepath.Join(rules, "daisy-plan.mdc")); !fi.Mode().IsRegular() {
		t.Error("the link to a directory was not replaced")
	}
	if b, _ := os.ReadFile(filepath.Join(rules, "myskill.mdc")); !strings.Contains(string(b), "Read `.claude/skills/myskill/SKILL.md`") {
		t.Errorf("myskill.mdc is %q", b)
	}
}

// recordingFS notes every path a command writes (successfully), a directory with a
// trailing slash: files, links, directories made one at a time (a stage, a
// skill's copy) and rename targets (the swap). MkdirAll's parents are not
// writes of a path git could publish, so they are not noted.
type recordingFS struct {
	ops.FS
	written []string
}

func (r *recordingFS) note(p string) {
	if fi, err := os.Lstat(p); err == nil && fi.IsDir() {
		p += "/"
	}
	r.written = append(r.written, p)
}

func (r *recordingFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	err := r.FS.WriteFile(name, data, perm)
	if err == nil {
		r.note(name)
	}
	return err
}

func (r *recordingFS) Mkdir(path string, perm os.FileMode) error {
	err := r.FS.Mkdir(path, perm)
	if err == nil {
		r.note(path)
	}
	return err
}

func (r *recordingFS) Symlink(oldname, newname string) error {
	err := r.FS.Symlink(oldname, newname)
	if err == nil {
		r.note(newname)
	}
	return err
}

func (r *recordingFS) Rename(oldpath, newpath string) error {
	err := r.FS.Rename(oldpath, newpath)
	if err == nil {
		r.note(newpath)
	}
	return err
}

// ignoredBy is the paths of rel (relative to a repository) that the one
// ignore line entry matches, as git reads it.
func ignoredBy(t *testing.T, entry string, rel []string) []string {
	t.Helper()
	probe := t.TempDir()
	gitIn(t, probe, "init", "-q", "--template=") // no info/exclude of the machine's
	writeFile(t, filepath.Join(probe, ".gitignore"), entry+"\n")
	cmd := exec.Command("git", "-C", probe, "-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(rel, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil && cmd.ProcessState.ExitCode() != 1 { // 1: nothing matched
		t.Fatalf("check-ignore %q: %v", entry, err)
	}
	return strings.Fields(string(out))
}

// The block init writes into .gitignore is exactly what Daisy writes into
// a workspace, both ways: every path init (run twice, so .daisy/ is moved
// aside and a stage swapped in), and `plan new` after it, writes there
// but .gitignore itself is ignored by the block; and every entry of the
// block ignores at least one of those paths — PLAN.md only because `plan
// new` links it — and no broader: an entry is a written path, or a name
// with a trailing * standing for exactly the suffix of the names init
// generates beside .daisy/, and none ignores a user's own file. Every
// .cursorignore line re-includes an entry of the block.
func TestInitIgnoresWhatItWrites(t *testing.T) {
	m := initMachine(t)
	ws := t.TempDir()
	gitIn(t, ws, "init", "-q")
	rec := &recordingFS{FS: ops.Real().FS()}
	var block []byte // .gitignore as init left it, before plan new runs
	for i, args := range [][]string{{"init", "testhome"}, {"init", "testhome"}, {"plan", "new", "Ship it"}} {
		o := withFS{&ops.Fake{
			GetenvFn: func(k string) string { return m.vars[k] },
			GetwdFn:  func() (string, error) { return ws, nil },
			NowFn:    func() time.Time { return time.Date(2026, 11, 1, 9, 30, i, 0, time.UTC) },
		}, rec}
		if args[0] == "plan" {
			b, err := os.ReadFile(filepath.Join(ws, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			block = b
		}
		if code, _, errs := runWith(o, args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, errs)
		}
	}

	var written []string
	seen := map[string]bool{}
	for _, p := range rec.written {
		rel, err := filepath.Rel(ws, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || rel == ".gitignore" || seen[rel] || seen[rel+"/"] {
			continue // outside the workspace (the home), or the file the user commits
		}
		if strings.HasSuffix(p, "/") {
			rel += "/"
		}
		seen[rel] = true
		written = append(written, rel)
	}
	for _, want := range []string{".daisy/", "PLAN.md", ".cursorignore", ".claude/skills/myskill/"} {
		if !seen[want] {
			t.Fatalf("no %s among the paths written: %v", want, written)
		}
	}
	var stage, previous bool
	for _, p := range written {
		stage = stage || strings.HasPrefix(p, ".daisy.stage.")
		previous = previous || strings.HasPrefix(p, ".daisy.previous.")
	}
	if !stage || !previous {
		t.Fatalf("init made no stage (%v) or set no .daisy/ aside (%v): %v", stage, previous, written)
	}

	// plan new adds PLAN.md to .gitignore when it is missing: the block
	// must already hold it.
	if b, _ := os.ReadFile(filepath.Join(ws, ".gitignore")); string(b) != string(block) {
		t.Fatalf("plan new changed the .gitignore init wrote:\n%s\n→\n%s", block, b)
	}
	b := block
	lines := splitNonEmpty(string(b))
	if len(lines) < 2 || lines[0] != "# Daisy workspace config (per-machine)" {
		t.Fatalf(".gitignore is not one Daisy block:\n%s", b)
	}
	entries := lines[1:]
	for _, e := range entries {
		if len(ignoredBy(t, e, written)) == 0 {
			t.Errorf("the entry %q ignores nothing Daisy writes into the workspace (%v)", e, written)
		}
		// No broader than its cause: a path Daisy writes, or a name Daisy
		// writes with a trailing * for its generated suffix.
		if lit, glob := strings.CutSuffix(e, "*"); strings.Contains(lit, "*") || !(glob && generatedSuffix(lit, ignoredBy(t, e, written)) || !glob && seen[e]) {
			t.Errorf("the entry %q is not a path Daisy writes, nor such a name ending in *", e)
		}
	}
	// And so it ignores nothing of the user's beside Daisy's own files.
	user := []string{"README.md", "src/main.go", "plan.md", "OTHER_PLAN.txt", ".daisyrc", ".daisy.bak", ".env.sh",
		".claude/settings.json", ".claude/commands/other.md", ".claude/skills/other/SKILL.md", ".cursor/rules/user.mdc", ".cursor/rules/docs.mdc", ".claude/commands/deploy.md", ".cursor/mcp.json", "Procfile", ".claude/skills/my-notes/SKILL.md", ".agents/skills/other/SKILL.md", ".agents/config.toml", ".daisy.sample", ".daisy.plan.md"}
	if hit := ignoredBy(t, strings.Join(entries, "\n"), user); len(hit) > 0 {
		t.Errorf("the block ignores the user's own %v", hit)
	}
	ignored := map[string]bool{}
	for _, p := range ignoredBy(t, strings.Join(entries, "\n"), written) {
		ignored[p] = true
	}
	for _, p := range written {
		if !ignored[p] {
			t.Errorf("%s is written into the workspace and not ignored by the block %v", p, entries)
		}
	}

	c, err := os.ReadFile(filepath.Join(ws, ".cursorignore"))
	if err != nil {
		t.Fatal(err)
	}
	cl := splitNonEmpty(string(c))
	if len(cl) < 2 || cl[0] != "# Allow Cursor to index daisy paths (gitignored but needed for agent context)" {
		t.Fatalf(".cursorignore is not one Daisy block:\n%s", c)
	}
	for _, l := range cl[1:] {
		if !strings.HasPrefix(l, "!") || !slices.Contains(entries, l[1:]) {
			t.Errorf(".cursorignore line %q re-includes no entry of the .gitignore block", l)
		}
	}
	// And the other way: every core Cursor rule init ignores, and .daisy/
	// and PLAN.md, Cursor may index.
	for _, e := range entries {
		if (strings.HasPrefix(e, ".cursor/rules/daisy") || e == ".daisy/" || e == "PLAN.md") && !slices.Contains(cl, "!"+e) {
			t.Errorf("the block ignores %s and .cursorignore does not re-include it", e)
		}
	}
}

// init adds no entry a .gitignore of the repository already ignores (the
// user's own .claude/), but still adds one only this machine ignores (in
// .git/info/exclude or a core.excludesFile, which a clone elsewhere lacks)
// and one a negated line re-includes.
func TestInitSkipsOnlyWhatTheRepositoryIgnores(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	writeFile(t, filepath.Join(ws, ".gitignore"), ".claude/\n*.md\n!PLAN.md\n")
	writeFile(t, filepath.Join(ws, ".git", "info", "exclude"), ".daisy/\n")
	machine := filepath.Join(t.TempDir(), ".gitignore") // named as a repository's, and not one
	writeFile(t, machine, ".cursorignore\n")
	gitIn(t, ws, "config", "core.excludesFile", machine)
	if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
		t.Fatalf("init exit %d: %s", code, errs)
	}
	b, _ := os.ReadFile(filepath.Join(ws, ".gitignore"))
	lines := splitNonEmpty(string(b))
	h := slices.Index(lines, "# Daisy workspace config (per-machine)")
	if h < 0 {
		t.Fatalf("no block:\n%s", b)
	}
	block := lines[h+1:]
	for _, e := range []string{".daisy/", ".cursorignore", "PLAN.md", ".cursor/rules/daisy.mdc", ".agents/skills/myskill/"} {
		if !slices.Contains(block, e) {
			t.Errorf("the block lacks %s:\n%s", e, b)
		}
	}
	for _, e := range block {
		if strings.HasPrefix(e, ".claude/") {
			t.Errorf("the block adds %s, which the repository's .claude/ ignores:\n%s", e, b)
		}
	}
}

// plan new adds no PLAN.md line a .gitignore of the workspace already
// ignores the plan by, as init adds no entry it has: init, plan new and
// clean leave such a workspace's .gitignore and git status as they were.
func TestPlanNewAddsNoLineGitAlreadyHas(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	writeFile(t, filepath.Join(ws, ".gitignore"), "/PLAN.md\n")
	gitIn(t, ws, "-c", "user.name=S", "-c", "user.email=s@x", "commit", "-q", "-a", "-m", "anchored")
	status := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all")
	for _, args := range [][]string{{"init", "testhome"}, {"plan", "new", "Ship it"}, {"clean", "-f"}} {
		if code, out, errs := runWith(m.planOps(ws), args...); code != 0 || strings.Contains(out, "to .gitignore") && args[0] == "plan" {
			t.Fatalf("%v: exit %d: %s%s", args, code, out, errs)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(ws, ".gitignore")); string(b) != "/PLAN.md\n" {
		t.Errorf(".gitignore after init, plan new and clean:\n%s", b)
	}
	if got := gitIn(t, ws, "status", "--porcelain", "--untracked-files=all"); got != status {
		t.Errorf("git status %q → %q", status, got)
	}
}

// A directory linked elsewhere — an agent directory, or a skill's copy
// itself — is outside the repository's tree, which git refuses to be asked
// about: init still binds the workspace and ignores the paths it writes
// there, as before.
func TestInitIgnoresThroughALinkedAgentDirectory(t *testing.T) {
	m := initMachine(t)
	for name, link := range map[string]func(ws string) (string, string){
		".claude elsewhere": func(ws string) (string, string) { return t.TempDir(), filepath.Join(ws, ".claude") },
		"a skill's copy elsewhere": func(ws string) (string, string) {
			return t.TempDir(), filepath.Join(ws, ".claude", "skills", "myskill")
		},
		"a skill's twin linked to its copy": func(ws string) (string, string) {
			os.MkdirAll(filepath.Join(ws, ".claude", "skills", "myskill"), 0o755)
			return "../../.claude/skills/myskill", filepath.Join(ws, ".agents", "skills", "myskill")
		},
	} {
		ws := scratch(t)
		target, at := link(ws)
		os.MkdirAll(filepath.Dir(at), 0o755)
		if err := os.Symlink(target, at); err != nil {
			t.Fatal(err)
		}
		if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
			t.Fatalf("%s: init exit %d: %s", name, code, errs)
		}
		b, _ := os.ReadFile(filepath.Join(ws, ".gitignore"))
		for _, e := range []string{".claude/commands/daisy.md", ".claude/skills/myskill/", ".agents/skills/myskill/"} {
			if !slices.Contains(splitNonEmpty(string(b)), e) {
				t.Errorf("%s: the block lacks %s:\n%s", name, e, b)
			}
		}
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// generatedName is the names init generates beside .daisy/ (env's
// leftover shape), its fixed part and its generated suffix: the one place
// a * in the block stands for a suffix.
var generatedName = regexp.MustCompile(`^(\.daisy\.previous\.)[0-9]+(?:/|$)|^(\.daisy\.stage\.)[A-Za-z0-9]{6}(?:/|$)`)

// generatedSuffix: paths is not empty and each is a generated name whose
// fixed part is lit, so lit + "*" stands for exactly the generated suffix.
func generatedSuffix(lit string, paths []string) bool {
	for _, p := range paths {
		m := generatedName.FindStringSubmatch(p)
		if m == nil || m[1]+m[2] != lit {
			return false
		}
	}
	return len(paths) > 0
}

// twins fails unless .agents/skills holds .claude/skills byte for byte,
// beside the daisy skill init generates there.
func twins(t *testing.T, ws string) {
	t.Helper()
	claude, agents := tree(t, filepath.Join(ws, ".claude", "skills")), tree(t, filepath.Join(ws, ".agents", "skills"))
	delete(agents, "daisy")
	delete(agents, "daisy/SKILL.md")
	if d := diffTrees(claude, agents); len(d) > 0 {
		t.Errorf(".agents/skills is not .claude/skills' twin:\n%s", strings.Join(d, "\n"))
	}
}

// Codex: init writes the daisy skill (front matter name and a one-line
// description; the body sends the request through .daisy/AGENTS.md) and
// every skill's copy in .agents/skills as in .claude/skills; re-init sweeps
// a stale copy from both trees.
func TestInitWritesCodexTwinTree(t *testing.T) {
	m := initMachine(t)
	writeFile(t, filepath.Join(m.home, "skills", "other", "SKILL.md"), "---\nname: other\ndescription: o\n---\n")
	ws := scratch(t)
	// A copy is replaced whole in either tree, whatever stands there.
	for _, tree := range []string{".claude", ".agents"} {
		os.MkdirAll(filepath.Join(ws, tree, "skills", "myskill", "SKILL.md"), 0o755)
	}
	if code, out, errs := m.pinned(t, ws, "init", "testhome"); code != 0 || !strings.Contains(out, "Installed Codex skill: daisy") {
		t.Fatalf("exit %d out %q err %q", code, out, errs)
	}
	b, err := os.ReadFile(filepath.Join(ws, ".agents", "skills", "daisy", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	front, body, ok := strings.Cut(strings.TrimPrefix(doc, "---\n"), "\n---\n")
	if !strings.HasPrefix(doc, "---\n") || !ok || !regexp.MustCompile(`(?m)^name: daisy$`).MatchString(front) || !regexp.MustCompile(`(?m)^description: \S.{20,300}$`).MatchString(front) {
		t.Errorf("front matter: %q", doc)
	}
	if !strings.Contains(body, "Read `.daisy/AGENTS.md`") {
		t.Errorf("body: %q", body)
	}
	for _, n := range []string{"myskill/ref.md", "other/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(ws, ".agents", "skills", n)); err != nil {
			t.Errorf("no .agents/skills/%s", n)
		}
	}
	twins(t, ws)

	os.RemoveAll(filepath.Join(m.home, "skills", "other"))
	if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
		t.Fatalf("re-init: %s", errs)
	}
	for _, tree := range []string{".claude", ".agents"} {
		if _, err := os.Lstat(filepath.Join(ws, tree, "skills", "other")); err == nil {
			t.Errorf("the stale %s/skills/other survived re-init", tree)
		}
	}
	twins(t, ws)
}

// A home's own repository is not a workspace: binding it would apply an
// identity there, and every commit of the home would carry it.
func TestInitRefusesAHomesOwnRepository(t *testing.T) {
	m := initMachine(t)
	m.seed(t)
	m.initHomeRepo(t)
	before := keys(t, m.home)
	code, out, errs := m.pinned(t, m.home, "init", "pelicanmoor")
	if code != 1 || out != "" || !strings.Contains(errs, "is home 'testhome''s own repository") {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
	if keys(t, m.home) != before {
		t.Errorf("keys %s → %s", before, keys(t, m.home))
	}
}

// A leftover stage with no record (an init killed before writing it) does
// not take a key of Daisy's form before the binding's own claim does: the
// binding's record alone holds what the key held before, and it is put
// back (init then clean leaves the keys as they were).
func TestInitRestoresBeforeARecordlessStage(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	gitIn(t, ws, "config", "--local", "core.sshCommand", "ssh -v")
	if code, _, errs := m.pinned(t, ws, "init", "pelicanmoor"); code != 0 {
		t.Fatal(errs)
	}
	os.Mkdir(filepath.Join(ws, ".daisy.stage.abcdef"), 0o700)
	if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
		t.Fatal(errs)
	}
	if got := strings.TrimSpace(gitIn(t, ws, "config", "--local", "--get-all", "core.sshCommand")); got != "ssh -v" {
		t.Errorf("core.sshCommand after the switch: %q", got)
	}
}

// A home that is a linked worktree of the workspace's repository shares its
// config: binding the workspace would apply an identity the home's commits
// then carry, so init refuses it too.
func TestInitRefusesAHomeSharingTheRepository(t *testing.T) {
	m := initMachine(t)
	proj := scratch(t)
	h := filepath.Join(t.TempDir(), "wthome")
	gitIn(t, proj, "worktree", "add", "-q", "-b", "daisy-data", h)
	copyTree(t, "templates/home", h)
	writeFile(t, filepath.Join(h, "identities", "persona"), "name=Persona Gannet\nemail=persona@example.invalid\n")
	gitIn(t, h, "-c", "user.name=Home Owner", "-c", "user.email=owner@example.invalid", "add", "-A")
	gitIn(t, h, "-c", "user.name=Home Owner", "-c", "user.email=owner@example.invalid", "commit", "-q", "-m", "home")
	if code, out, errs := m.pinned(t, m.ws, "home", "add", "wthome", h); code != 0 {
		t.Fatalf("home add: %d %s %s", code, out, errs)
	}
	code, out, errs := m.pinned(t, proj, "init", "persona")
	if code != 1 || out != "" || !strings.Contains(errs, "is home 'wthome''s own repository") {
		t.Errorf("exit %d out %q err %q", code, out, errs)
	}
}

// The other way round: an unbound workspace (no claim of its own) with a
// stage that holds a record (an init killed after applying, before its
// swap): the stage's record puts back what the key held before.
func TestInitRestoresAnInterruptedFirstInit(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	gitIn(t, ws, "config", "--local", "core.sshCommand", "ssh -v")
	if code, _, errs := m.pinned(t, ws, "init", "pelicanmoor"); code != 0 {
		t.Fatal(errs)
	}
	stage := filepath.Join(ws, ".daisy.stage.abcdef")
	os.Mkdir(stage, 0o700)
	for _, f := range []string{"gitconfig", "gitconfig.before"} {
		os.Rename(filepath.Join(ws, ".daisy", f), filepath.Join(stage, f))
	}
	os.RemoveAll(filepath.Join(ws, ".daisy"))
	if code, _, errs := m.pinned(t, ws, "init", "testhome"); code != 0 {
		t.Fatal(errs)
	}
	if got := strings.TrimSpace(gitIn(t, ws, "config", "--local", "--get-all", "core.sshCommand")); got != "ssh -v" {
		t.Errorf("core.sshCommand: %q", got)
	}
}

// A bare `daisy init` is `daisy init` with the workspace's own binding: run
// anywhere in a workspace bound to a named identity, after the root's prompt
// and the home's skills changed, it keeps .daisy/identity, the record of the
// applied identity, what the keys held before it and the keys themselves,
// and rebuilds .daisy/AGENTS.md and the skills. The home is the binding's:
// a second home defining the same identity, which makes `init pelicanmoor`
// ambiguous, does not move it. In a folder with no binding it is the
// missing argument, exit 2, naming `daisy init <identity>`, and writes nothing.
func TestBareInitReinitsTheBinding(t *testing.T) {
	m := initMachine(t)
	agentsPrompt := filepath.Join(m.root, "prompts", "daisy.md")
	writeFile(t, agentsPrompt, "## Trigger\n\nThe root as it was.\n")
	ws := scratch(t)
	gitIn(t, ws, "config", "--local", "user.email", "hand@example.invalid")
	if code, _, errs := m.pinned(t, ws, "init", "pelicanmoor"); code != 0 {
		t.Fatalf("init: exit %d: %s", code, errs)
	}
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(ws, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	binding := map[string]string{}
	for _, f := range []string{".daisy/identity", ".daisy/gitconfig", ".daisy/gitconfig.before"} {
		binding[f] = read(f)
	}
	if !strings.Contains(binding[".daisy/gitconfig.before"], "hand@example.invalid") {
		t.Fatalf("init recorded no hand-set key: %q", binding[".daisy/gitconfig.before"])
	}
	k := keys(t, ws)

	writeFile(t, agentsPrompt, "## Trigger\n\nThe root as it is now.\n")
	writeFile(t, filepath.Join(m.home, "skills", "later", "SKILL.md"), "---\nname: later\ndescription: l\n---\n")
	second := filepath.Join(t.TempDir(), "second")
	writeFile(t, filepath.Join(second, "identities", "pelicanmoor"), "name=Other Pelican\nemail=other@example.invalid\n")
	writeFile(t, filepath.Join(m.config, "homes"), "testhome="+m.home+"\nsecond="+second+"\n")
	if code, _, errs := m.pinned(t, ws, "init", "pelicanmoor"); code != 1 || !strings.Contains(errs, "more than one home") {
		t.Fatalf("init pelicanmoor with two homes defining it: exit %d: %s", code, errs)
	}

	sub := filepath.Join(ws, "sub")
	os.Mkdir(sub, 0o755)
	code, out, errs := m.pinned(t, sub, "init")
	if code != 0 || !strings.Contains(out, "Initializing Daisy in: "+ws+"\n") || !strings.Contains(out, "Already initialized with identity: pelicanmoor") {
		t.Fatalf("bare init: exit %d out %q err %q", code, out, errs)
	}
	for f, was := range binding {
		if got := read(f); got != was {
			t.Errorf("%s: %q → %q", f, was, got)
		}
	}
	if got := keys(t, ws); got != k {
		t.Errorf("keys %s → %s", k, got)
	}
	if got := read(".daisy/home"); got != "testhome\n" {
		t.Errorf(".daisy/home is %q", got)
	}
	if agents := read(".daisy/AGENTS.md"); !strings.Contains(agents, "The root as it is now.") || strings.Contains(agents, "The root as it was.") {
		t.Errorf("AGENTS.md not rebuilt: %s", agents)
	}
	for _, tree := range []string{".claude", ".agents"} {
		if _, err := os.Stat(filepath.Join(ws, tree, "skills", "later", "SKILL.md")); err != nil {
			t.Errorf("the home's new skill is not in %s/skills: %v", tree, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(sub, ".daisy")); err == nil {
		t.Errorf("bare init bound the subdirectory")
	}

	unbound := t.TempDir()
	code, out, errs = m.pinned(t, unbound, "init")
	if code != 2 || out != "" || !strings.Contains(errs, "Error: missing argument") || !strings.Contains(errs, "daisy init <identity>") || !strings.Contains(errs, "Usage: daisy init") {
		t.Errorf("bare init in an unbound folder: exit %d out %q err %q", code, out, errs)
	}
	if entries, _ := os.ReadDir(unbound); len(entries) != 0 {
		t.Errorf("bare init in an unbound folder wrote %v", entries)
	}
}
