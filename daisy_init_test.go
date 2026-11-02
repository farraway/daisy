package daisy_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"daisy"
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
	}
	var out, errb bytes.Buffer
	code := daisy.Run(o, strings.NewReader(""), &out, &errb, args)
	return code, out.String(), errb.String()
}

func initMachine(t *testing.T) *machine {
	m := newMachine(t)
	writeFile(t, filepath.Join(m.home, "skills", "nested", "myskill", "SKILL.md"), "---\nname: myskill\ndescription: \"Use when: testing\"\n---\nbody\n")
	writeFile(t, filepath.Join(m.home, "skills", "nested", "myskill", "ref.md"), "ref\n")
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
		for _, f := range []string{".gitignore", ".cursorignore", ".claude/settings.local.json", ".cursor/rules/myskill.mdc", ".claude/skills/myskill/ref.md", ".daisy/AGENTS.md"} {
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

// A refused init writes nothing: every refusal is decided before the first write.
func TestRefusedInitWritesNothing(t *testing.T) {
	m := initMachine(t)
	ws := scratch(t)
	m.pinned(t, ws, "init", "pelicanmoor")
	for _, obstacle := range []func(){
		func() { writeFile(t, filepath.Join(ws, ".claude", "settings.local.json"), "{ not json") },
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
	code := daisy.Run(o, strings.NewReader(""), &out, &errb, []string{"init", "testhome"})
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

// install against a temp HOME: the link, the rc line and the permissions;
// applied twice equals once; plain install refuses once installed.
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
	settings, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(string(settings), `"Read(/`+m.root+`/**)"`) || !strings.Contains(string(settings), `"Bash(daisy:*)"`) {
		t.Errorf("settings %s", settings)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "s", "SKILL.md")); err != nil {
		t.Error("the root skill was not installed")
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
	m.vars["DAISY_ROOT"] = other
	m.pinned(t, m.ws, "install")
	if rc, _ := os.ReadFile(filepath.Join(home, ".zshenv")); string(rc) != "export PATH=x\n\n# daisy\nexport DAISY_ROOT=\""+other+"\"\n" {
		t.Errorf("rc after a move %q", rc)
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
	if code := daisy.Run(o, strings.NewReader(""), &out, &errb, []string{"install", "--update"}); code != 0 || !strings.Contains(out.String(), "DAISY_ROOT: "+m.root+"\n") {
		t.Errorf("exit %d out %q err %q", code, out.String(), errb.String())
	}
	o.ExecutableFn = func() (string, error) { return filepath.Join(t.TempDir(), "daisy"), nil }
	errb.Reset()
	if code := daisy.Run(o, strings.NewReader(""), &out, &errb, []string{"install", "--update"}); code != 1 || !strings.Contains(errb.String(), "DAISY_ROOT is not set") {
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
