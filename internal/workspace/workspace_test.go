package workspace

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// The package's seam is its exported surface, Init, Reinit, ReadBinding and
// Clean, driven with an ops.Fake over a temp machine: a root holding the
// repository's templates, a registry, a home and a workspace that is a git
// repository. These tests pin what the functional tests in cmd/daisy reach
// only from above or not at all: init's refusals before its first write,
// the leftovers of an interrupted swap, a home without a default identity,
// and clean's settings, recordless bindings, stages and non-repositories.

type machine struct {
	o                *ops.Fake
	m                env.Env
	root, home, ws   string
	config, homeName string
	vars             map[string]string
}

func newMachine(t *testing.T) *machine {
	t.Helper()
	mc := &machine{root: t.TempDir(), config: t.TempDir(), ws: t.TempDir(), homeName: "testhome"}
	mc.home = filepath.Join(t.TempDir(), "testhome")
	copyTree(t, "../../templates/home", mc.home)
	copyTree(t, "../../templates", filepath.Join(mc.root, "templates"))
	writeFile(t, filepath.Join(mc.home, "gitconfig"), "name=Fixture Author\nemail=fixture@example.invalid\n")
	writeFile(t, filepath.Join(mc.config, "homes"), "testhome="+mc.home+"\n")
	mc.vars = map[string]string{"DAISY_ROOT": mc.root, "DAISY_CONFIG": mc.config, "DAISY_LOCAL": t.TempDir(), "HOME": t.TempDir()}
	mc.o = &ops.Fake{
		GetenvFn: func(k string) string { return mc.vars[k] },
		NowFn:    func() time.Time { return time.Date(2026, 11, 16, 9, 30, 0, 0, time.UTC) },
	}
	m, err := env.Machine(mc.o)
	if err != nil {
		t.Fatal(err)
	}
	mc.m = m
	git(t, mc.ws, "init", "-q")
	return mc
}

func (mc *machine) init(t *testing.T, name, target string) (stdout, stderr string, err error) {
	t.Helper()
	var out, warn bytes.Buffer
	err = Init(mc.o, mc.m, name, target, &out, &warn)
	return out.String(), warn.String(), err
}

func (mc *machine) clean(t *testing.T) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Clean(mc.o, mc.m, ReadBinding(mc.o, mc.m, mc.ws), &out)
	return out.String(), err
}

// mustInit binds the workspace to the home's default identity.
func (mc *machine) mustInit(t *testing.T) {
	t.Helper()
	if _, stderr, err := mc.init(t, mc.homeName, mc.ws); err != nil {
		t.Fatalf("init: %v\n%s", err, stderr)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// keys is the repository's local values of the identity keys.
func keys(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	for _, k := range []string{"user.name", "user.email", "core.sshCommand"} {
		out, _ := exec.Command("git", "-C", dir, "config", "--local", "--get-all", k).Output()
		fmt.Fprintf(&b, "%s=%q;", k, strings.TrimSpace(string(out)))
	}
	return b.String()
}

const noKeys = `user.name="";user.email="";core.sshCommand="";`

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
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// tree is every path under dir but .git, with its kind and content or
// link target: equal trees are equal byte for byte.
func tree(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		fi, _ := os.Lstat(p)
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			paths = append(paths, rel+" -> "+target)
		case fi.IsDir():
			paths = append(paths, rel+"/")
		default:
			b, _ := os.ReadFile(p)
			paths = append(paths, fmt.Sprintf("%s %q", rel, b))
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}

func exist(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func leftovers(t *testing.T, ws string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(ws, ".daisy.*"))
	return m
}

// Init refuses a target it cannot bind before its first write: the error
// names the problem, and the target is as it was, byte for byte.
func TestInitRefusesBeforeItsFirstWrite(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, mc *machine) string // the target
		wantErr string
	}{
		{"not a directory", func(t *testing.T, mc *machine) string {
			p := filepath.Join(mc.ws, "file")
			writeFile(t, p, "x\n")
			return p
		}, "is not a directory"},
		{".daisy a symlink", func(t *testing.T, mc *machine) string {
			elsewhere := t.TempDir()
			os.Symlink(elsewhere, filepath.Join(mc.ws, ".daisy"))
			return mc.ws
		}, "is a symlink; init binds a directory of its own"},
		{".daisy a file", func(t *testing.T, mc *machine) string {
			writeFile(t, filepath.Join(mc.ws, ".daisy"), "x\n")
			return mc.ws
		}, "exists and is not a directory"},
		{".daisy the root", func(t *testing.T, mc *machine) string {
			os.Symlink(mc.root, filepath.Join(mc.ws, ".daisy"))
			return mc.ws
		}, "is the Daisy root, not a workspace binding"},
		{"two bindings moved aside", func(t *testing.T, mc *machine) string {
			writeFile(t, filepath.Join(mc.ws, ".daisy.previous.1", "home"), "testhome\n")
			writeFile(t, filepath.Join(mc.ws, ".daisy.previous.2", "home"), "testhome\n")
			return mc.ws
		}, "no .daisy/ here but 2 directories an earlier init moved aside"},
		{"a directory where init writes its own file", func(t *testing.T, mc *machine) string {
			os.MkdirAll(filepath.Join(mc.ws, ".daisy", "AGENTS.md"), 0o755)
			return mc.ws
		}, "is a directory where init writes a file of its own"},
		{"a linked worktree", func(t *testing.T, mc *machine) string {
			git(t, mc.ws, "-c", "user.name=S", "-c", "user.email=s@example.invalid", "commit", "-q", "--allow-empty", "-m", "first")
			wt := filepath.Join(t.TempDir(), "wt")
			git(t, mc.ws, "worktree", "add", "-q", wt)
			return wt
		}, "is a linked worktree"},
		{"a template missing", func(t *testing.T, mc *machine) string {
			os.Remove(filepath.Join(mc.root, "templates", "claude-command.md"))
			return mc.ws
		}, "cannot read the template"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mc := newMachine(t)
			target := c.setup(t, mc)
			dir := target
			if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
				dir = filepath.Dir(target)
			}
			before, keysBefore := tree(t, dir), keys(t, mc.ws)
			_, _, err := mc.init(t, mc.homeName, target)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want one containing %q", err, c.wantErr)
			}
			if after := tree(t, dir); strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Errorf("a refused init wrote:\nbefore %q\nafter  %q", before, after)
			}
			if k := keys(t, mc.ws); k != keysBefore {
				t.Errorf("a refused init changed the keys: %s -> %s", keysBefore, k)
			}
		})
	}
}

// Reinit refuses a binding whose .daisy/identity is not an identity name,
// naming the file and the fix, and writes nothing.
func TestReinitRefusesABindingNamingNoIdentity(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	writeFile(t, filepath.Join(mc.ws, ".daisy", "identity"), "../escape\n")
	before := tree(t, mc.ws)
	b := ReadBinding(mc.o, mc.m, mc.ws)
	var out, warn bytes.Buffer
	err := Reinit(mc.o, mc.m, b, &out, &warn)
	if err == nil || !strings.Contains(err.Error(), "which is not an identity name") || !strings.Contains(err.Error(), "daisy init <identity>") {
		t.Fatalf("err = %v", err)
	}
	if after := tree(t, mc.ws); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("a refused reinit wrote")
	}
}

// With no .daisy/ and one .daisy.previous.* (an init interrupted mid-swap),
// init takes that one back as the binding: what the user kept in it stays,
// and no leftover remains.
func TestInitReclaimsABindingMovedAside(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	writeFile(t, filepath.Join(mc.ws, ".daisy", "project"), "kept\n")
	aside := filepath.Join(mc.ws, ".daisy.previous.42")
	if err := os.Rename(filepath.Join(mc.ws, ".daisy"), aside); err != nil {
		t.Fatal(err)
	}
	_, warn, err := mc.init(t, mc.homeName, mc.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn, "Reclaimed "+aside+" as .daisy/") {
		t.Errorf("stderr does not say the binding was reclaimed:\n%s", warn)
	}
	if b, _ := os.ReadFile(filepath.Join(mc.ws, ".daisy", "project")); string(b) != "kept\n" {
		t.Errorf(".daisy/project = %q, want the reclaimed binding's", b)
	}
	if l := leftovers(t, mc.ws); len(l) != 0 {
		t.Errorf("leftovers remain: %v", l)
	}
}

// With .daisy/ in place, a .daisy.previous.* beside it is what an earlier
// swap moved aside and init removes it, saying so; the binding is .daisy/'s.
func TestInitRemovesAPreviousBesideTheBinding(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	writeFile(t, filepath.Join(mc.ws, ".daisy", "project"), "current\n")
	writeFile(t, filepath.Join(mc.ws, ".daisy.previous.7", "project"), "stale\n")
	_, warn, err := mc.init(t, mc.homeName, mc.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn, "Removed "+filepath.Join(mc.ws, ".daisy.previous.7")) {
		t.Errorf("stderr does not name the removed leftover:\n%s", warn)
	}
	if b, _ := os.ReadFile(filepath.Join(mc.ws, ".daisy", "project")); string(b) != "current\n" {
		t.Errorf(".daisy/project = %q, want .daisy/'s own", b)
	}
	if l := leftovers(t, mc.ws); len(l) != 0 {
		t.Errorf("leftovers remain: %v", l)
	}
}

// A home with no complete default identity still binds: init warns with
// the file and the fix, records no identity and leaves the keys unset.
func TestInitWithoutADefaultIdentityBindsAndWarns(t *testing.T) {
	mc := newMachine(t)
	os.Remove(filepath.Join(mc.home, "gitconfig"))
	_, warn, err := mc.init(t, mc.homeName, mc.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn, "has no complete default identity at "+filepath.Join(mc.home, "gitconfig")) || !strings.Contains(warn, "re-run 'daisy init testhome'") {
		t.Errorf("stderr lacks the warning and its fix:\n%s", warn)
	}
	if exist(filepath.Join(mc.ws, ".daisy", "gitconfig")) {
		t.Error("init recorded an identity it did not apply")
	}
	if b, _ := os.ReadFile(filepath.Join(mc.ws, ".daisy", "home")); string(b) != "testhome\n" {
		t.Errorf(".daisy/home = %q", b)
	}
	if k := keys(t, mc.ws); k != noKeys {
		t.Errorf("keys = %s, want none", k)
	}
}

// Binding to another home says so and takes back the first home's
// identity before applying the second's: a key the first displaced gets
// back what it held before (its record, .daisy/gitconfig.before), and the
// second home's identity is applied.
func TestInitSwitchingHomeSwapsTheIdentity(t *testing.T) {
	mc := newMachine(t)
	writeFile(t, filepath.Join(mc.home, "gitconfig"), "name=Fixture Author\nemail=fixture@example.invalid\nsshkey=~/.ssh/id_fixture\n")
	git(t, mc.ws, "config", "core.sshCommand", "ssh -F hand-config")
	mc.mustInit(t)
	if !strings.Contains(keys(t, mc.ws), "id_fixture") {
		t.Fatalf("the first identity's key is not applied: %s", keys(t, mc.ws))
	}
	other := filepath.Join(t.TempDir(), "otherhome")
	copyTree(t, "../../templates/home", other)
	writeFile(t, filepath.Join(other, "gitconfig"), "name=Other Author\nemail=other@example.invalid\n")
	writeFile(t, filepath.Join(mc.config, "homes"), "testhome="+mc.home+"\notherhome="+other+"\n")
	out, _, err := mc.init(t, "otherhome", mc.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Switching home: testhome → otherhome") {
		t.Errorf("stdout does not say the home switched:\n%s", out)
	}
	if want := `user.name="Other Author";user.email="other@example.invalid";core.sshCommand="ssh -F hand-config";`; keys(t, mc.ws) != want {
		t.Errorf("keys = %s, want %s", keys(t, mc.ws), want)
	}
	if target, _ := os.Readlink(filepath.Join(mc.ws, ".daisy", "tasks")); target != filepath.Join(other, "tasks") {
		t.Errorf(".daisy/tasks -> %s, want the other home's", target)
	}
}

// A re-init failing in its final step says what it changed: the files it
// wrote before the step stay, and the previous git identity is said not
// to be put back only when the keys, read again, differ from what they
// held. A failed `git config --unset-all` changes nothing, so it is not
// said; a failed --add after the keys were cleared leaves them changed,
// so it is.
func TestInitFailedUnsetSaysWhatChanged(t *testing.T) {
	for _, tc := range []struct {
		fail     string // the git config option that fails
		putBack  bool   // whether the keys end as they were
		wantKeys string
	}{
		{"--unset-all", true, `user.name="Fixture Author";user.email="fixture@example.invalid";core.sshCommand="";`},
		{"--replace-all", false, noKeys},
	} {
		t.Run(tc.fail, func(t *testing.T) {
			mc := newMachine(t)
			mc.mustInit(t)
			k := keys(t, mc.ws)
			mc.o.ExecFn = func(dir, name string, args ...string) ([]byte, []byte, error) {
				if name == "git" && len(args) > 2 && args[0] == "config" && (args[2] == tc.fail || args[2] == "--add") {
					return nil, []byte("error: scripted failure"), fmt.Errorf("exit status 4")
				}
				return ops.Real().Exec(dir, name, args...)
			}
			_, _, err := mc.init(t, mc.homeName, mc.ws)
			if err == nil {
				t.Fatal("init did not fail")
			}
			msg := err.Error()
			if !strings.Contains(msg, "init did not finish") || !strings.Contains(msg, "the Cursor rules, the Claude command, the skills, .claude/settings.local.json and the ignore lines it wrote before that step stay as written") {
				t.Errorf("message %q", msg)
			}
			if said := strings.Contains(msg, "could not be put back"); said == tc.putBack {
				t.Errorf("'could not be put back' said %v with the keys %s (were %s): %q", said, keys(t, mc.ws), k, msg)
			}
			if got := keys(t, mc.ws); got != tc.wantKeys {
				t.Errorf("keys %s, want %s", got, tc.wantKeys)
			}
		})
	}
}

// A home skill overriding a root skill installs as the root's skill
// directory with the home's other files over it and the home's SKILL.md.
func TestInitInstallsAnOverridingSkillOverTheRootsDirectory(t *testing.T) {
	mc := newMachine(t)
	writeFile(t, filepath.Join(mc.root, "skills", "tool", "SKILL.md"), "---\nname: tool\ndescription: \"Use when: the root's\"\n---\nroot body\n")
	writeFile(t, filepath.Join(mc.root, "skills", "tool", "ref.md"), "root ref\n")
	writeFile(t, filepath.Join(mc.root, "skills", "tool", "only-root.md"), "root only\n")
	writeFile(t, filepath.Join(mc.home, "skills", "tool", "SKILL.md"), "---\nname: tool\ndescription: \"Use when: the home's\"\n---\nhome body\n")
	writeFile(t, filepath.Join(mc.home, "skills", "tool", "ref.md"), "home ref\n")
	mc.mustInit(t)
	for _, tree := range []string{".claude/skills", ".agents/skills"} {
		dir := filepath.Join(mc.ws, tree, "tool")
		for name, want := range map[string]string{"SKILL.md": "---\nname: tool\ndescription: \"Use when: the home's\"\n---\nhome body\n", "ref.md": "home ref\n", "only-root.md": "root only\n"} {
			if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != want {
				t.Errorf("%s/tool/%s = %q (%v), want %q", tree, name, b, err, want)
			}
		}
	}
}

// Clean keeps a user's own allow rule: the settings file stays, without
// Daisy's rules.
func TestCleanKeepsTheUsersOwnSettings(t *testing.T) {
	mc := newMachine(t)
	writeFile(t, filepath.Join(mc.ws, ".claude", "settings.local.json"), `{"permissions":{"allow":["Bash(make:*)"]}}`+"\n")
	mc.mustInit(t)
	out, err := mc.clean(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Removed Daisy permissions from .claude/settings.local.json") {
		t.Errorf("stdout:\n%s", out)
	}
	b, err := os.ReadFile(filepath.Join(mc.ws, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("the user's settings file is gone: %v", err)
	}
	if !strings.Contains(string(b), "Bash(make:*)") || strings.Contains(string(b), ".daisy/**") || strings.Contains(string(b), "Bash(daisy:*)") {
		t.Errorf("settings = %s, want the user's rule alone", b)
	}
}

// Clean refuses a settings file it cannot read as settings before it
// removes anything, naming the file and the fix.
func TestCleanRefusesUnreadableSettingsFirst(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	writeFile(t, filepath.Join(mc.ws, ".claude", "settings.local.json"), "{not json\n")
	before, keysBefore := tree(t, mc.ws), keys(t, mc.ws)
	_, err := mc.clean(t)
	if err == nil || !strings.Contains(err.Error(), "cannot be read as settings") || !strings.Contains(err.Error(), "re-run 'daisy clean'") {
		t.Fatalf("err = %v", err)
	}
	if after := tree(t, mc.ws); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Error("a refused clean removed files")
	}
	if keys(t, mc.ws) != keysBefore {
		t.Error("a refused clean changed the keys")
	}
}

// A binding without a .daisy/gitconfig record (one from before records)
// is unset against the identity file its home names, as that file is now.
func TestCleanWithoutARecordUnsetsTheHomesIdentity(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	os.Remove(filepath.Join(mc.ws, ".daisy", "gitconfig"))
	out, err := mc.clean(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cleared local git identity (user.name, user.email)") {
		t.Errorf("stdout:\n%s", out)
	}
	if k := keys(t, mc.ws); k != noKeys {
		t.Errorf("keys = %s, want none", k)
	}
}

// A binding naming no home, with no record, is cleaned of what Daisy wrote;
// a key not of Daisy's form is the user's and stays, and clean says so.
func TestCleanWithNoHomeAndNoRecordLeavesTheUsersKeys(t *testing.T) {
	mc := newMachine(t)
	os.MkdirAll(filepath.Join(mc.ws, ".daisy"), 0o755)
	git(t, mc.ws, "config", "user.email", "mine@example.invalid")
	out, err := mc.clean(t)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"no identity file resolves for home '?' identity '?'", "Local git identity left as is (nothing matched what Daisy writes)", "Removed .daisy/"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if k := keys(t, mc.ws); k != `user.name="";user.email="mine@example.invalid";core.sshCommand="";` {
		t.Errorf("keys = %s", k)
	}
}

// The stage an interrupted init left records what that init applied: clean
// takes that identity back too, and removes the stage.
func TestCleanReclaimsAnInterruptedStage(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	stage := filepath.Join(mc.ws, ".daisy.stage.abcDEF")
	writeFile(t, filepath.Join(stage, "gitconfig"), "name=Staged Author\nemail=staged@example.invalid\n")
	git(t, mc.ws, "config", "user.name", "Staged Author")
	git(t, mc.ws, "config", "user.email", "staged@example.invalid")
	out, err := mc.clean(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cleared local git identity (user.name, user.email)") {
		t.Errorf("stdout:\n%s", out)
	}
	if k := keys(t, mc.ws); k != noKeys {
		t.Errorf("keys = %s, want none", k)
	}
	if exist(stage) {
		t.Error("the stage remains")
	}
}

// .daisy.previous.* beside a binding is removed with it, each named.
func TestCleanRemovesWhatAnInterruptedSwapLeft(t *testing.T) {
	mc := newMachine(t)
	mc.mustInit(t)
	aside := filepath.Join(mc.ws, ".daisy.previous.9")
	writeFile(t, filepath.Join(aside, "home"), "testhome\n")
	out, err := mc.clean(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Removed "+aside+" (left by an interrupted init)") {
		t.Errorf("stdout:\n%s", out)
	}
	if exist(aside) {
		t.Error("the leftover remains")
	}
}

// In a directory that is no repository, clean checks no identity, says so,
// and removes the binding.
func TestCleanOutsideARepository(t *testing.T) {
	mc := newMachine(t)
	mc.ws = t.TempDir()
	os.MkdirAll(filepath.Join(mc.ws, ".daisy"), 0o755)
	writeFile(t, filepath.Join(mc.ws, ".daisy", "home"), "testhome\n")
	out, err := mc.clean(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Local git identity not checked: this workspace is not a repository") {
		t.Errorf("stdout:\n%s", out)
	}
	if exist(filepath.Join(mc.ws, ".daisy")) {
		t.Error(".daisy/ remains")
	}
}
