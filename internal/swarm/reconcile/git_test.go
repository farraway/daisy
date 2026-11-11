package reconcile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// productIn is up, a product's upstream at <base>/app.git (made, empty),
// and ws, a swarm repository's clone whose origin is <base>/swarm-app.git,
// so that the declared repository app is reached at up (section 7.1,
// step 1).
func productIn(t *testing.T) (ws, up string) {
	t.Helper()
	base := t.TempDir()
	up = filepath.Join(base, "app.git")
	os.MkdirAll(up, 0o755)
	ws = t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", filepath.Join(base, "swarm-app.git")}} {
		if _, errb, err := ops.Real().Exec(ws, "git", args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
	return ws, up
}

// The clone a member sees (section 7.1, step 3), with real git: no remote,
// a local base, the role's identity at the commit address, /.swarm/ and
// /.notes/ excluded, and the hook stamping the role's co-author on each
// commit the member makes, once, even when the message already carries it.
func TestCloneAndTheCommitMsgHook(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	o := ops.Real()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, "git", args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	ws, up := productIn(t)
	state := t.TempDir()
	git(up, "init", "-q", "-b", "main")
	git(up, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first")
	git(up, "branch", "swarm/1-1")
	g := Git{O: o, WS: ws, State: state}
	repo := declare.Repo{Name: "app", Repo: "acme/app"}
	if err := g.Fetch(repo); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "work")
	co := "claude-opus-5-5 <noreply@anthropic.com>"
	sha, err := g.Clone(repo, "swarm/1-1", "main", dir, "worker", "swarm@example.net", co)
	if err != nil {
		t.Fatal(err)
	}
	if sha != git(up, "rev-parse", "main") || git(dir, "remote") != "" || git(dir, "rev-parse", "main") != sha {
		t.Errorf("sha %s, remotes %q", sha, git(dir, "remote"))
	}
	if git(dir, "config", "user.name") != "worker (Daisy swarm)" || git(dir, "config", "user.email") != "swarm@example.net" {
		t.Error("identity")
	}
	// The mount points under /work are excluded: a worker's `git add -A`
	// commits nothing of /work/.swarm, the notes or a resource.
	for _, f := range []string{".swarm/out.md", ".notes/n.md", ".resources/docs/README.md"} {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755)
		os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	git(dir, "add", "-A")
	if staged := git(dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("git add -A staged %q", staged)
	}
	for _, f := range []string{".swarm", ".notes", ".resources"} {
		os.RemoveAll(filepath.Join(dir, f))
	}
	os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0o644)
	git(dir, "add", "a")
	git(dir, "commit", "-q", "-m", "Add a")
	git(dir, "commit", "-q", "--allow-empty", "-m", "Again\n\nCo-Authored-By: "+co)
	for _, rev := range []string{"HEAD~1", "HEAD"} {
		msg := git(dir, "log", "-1", "--format=%B", rev)
		if strings.Count(msg, "Co-Authored-By: "+co) != 1 {
			t.Errorf("%s:\n%s", rev, msg)
		}
	}
	if a := git(dir, "log", "-1", "--format=%an <%ae>"); a != "worker (Daisy swarm) <swarm@example.net>" {
		t.Errorf("author %s", a)
	}

	// An empty co_author is no trailer: no hook, and a commit without one.
	bare := filepath.Join(t.TempDir(), "work")
	if _, err := g.Clone(repo, "swarm/1-1", "main", bare, "worker", "swarm@example.net", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bare, ".git", "hooks", "commit-msg")); err == nil {
		t.Error("a commit-msg hook for an empty co_author")
	}
	git(bare, "commit", "-q", "--allow-empty", "-m", "Plain")
	if msg := git(bare, "log", "-1", "--format=%B"); msg != "Plain" {
		t.Errorf("an empty co_author's commit:\n%s", msg)
	}
}

// Collection's git (section 7.6, steps 3 and 5) with real git: a
// protected file renamed away is changed by its old path too; a push the
// remote refuses is ErrPushRejected, and one that cannot reach it is an
// error to retry, not a rejection.
func TestChangedAndPush(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	o := ops.Real()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, "git", args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	ws, up := productIn(t)
	state := t.TempDir()
	git(up, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(up, "AGENTS.md"), []byte("Rules.\n"), 0o644)
	git(up, "add", "AGENTS.md")
	git(up, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "-m", "first")
	g := Git{O: o, WS: ws, State: state}
	repo := declare.Repo{Name: "app", Repo: "acme/app"}
	if err := g.Fetch(repo); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "work")
	if _, err := g.Clone(repo, "main", "main", dir, "worker", "swarm@example.net", "c <c@example.net>"); err != nil {
		t.Fatal(err)
	}
	git(dir, "checkout", "-q", "-b", "swarm/1-1")
	git(dir, "mv", "AGENTS.md", "NOTES.md")
	os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	os.WriteFile(filepath.Join(dir, ".github", "workflows", "é.yml"), []byte("on: push\n"), 0o644)
	git(dir, "add", ".github")
	git(dir, "commit", "-q", "-m", "Rename")
	sha, err := g.FetchRun("app", dir, "swarm/1-1", "refs/swarm-runs/1-worker-1")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := g.Changed("app", "main", sha)
	if err != nil || len(Protected(changed, []string{"AGENTS.md"})) == 0 {
		t.Errorf("changed %q (%v): the protected file renamed away is not among them", changed, err)
	}
	// A path git would quote (a byte outside ASCII) is named as it is.
	if hits := Protected(changed, []string{".github/"}); len(hits) != 1 || hits[0] != ".github/workflows/é.yml" {
		t.Errorf("changed %q: the workflow with a non-ASCII name is not protected", changed)
	}

	// A branch of that name the remote holds already, unrelated: refused.
	git(up, "checkout", "-q", "--orphan", "other")
	git(up, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "other")
	git(up, "branch", "-q", "swarm/1-1")
	git(up, "checkout", "-q", "main")
	if err := g.Push(repo, "refs/swarm-runs/1-worker-1", "swarm/1-1", sha); !errors.Is(err, ErrPushRejected) {
		t.Errorf("refused push: %v, want ErrPushRejected", err)
	}
	// The forge unreachable: an error, not a rejection.
	git(ws, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	if err := g.Push(repo, "refs/swarm-runs/1-worker-1", "swarm/1-1", sha); err == nil || errors.Is(err, ErrPushRejected) {
		t.Errorf("unreachable push: %v, want an error that is not ErrPushRejected", err)
	}
}

// The clones a member and a check see hold nothing of the host's but the
// objects (section 8, rule 3): no reflog naming the host's git identity or
// the mirror's path, no FETCH_HEAD.
func TestClonesForgetTheHost(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(global, []byte("[user]\n\tname = Host Person\n\temail = host@example.net\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	o := ops.Real()
	git := func(dir string, args ...string) {
		t.Helper()
		if _, errb, err := o.Exec(dir, "git", args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
	ws, up := productIn(t)
	state := t.TempDir()
	git(up, "init", "-q", "-b", "main")
	git(up, "commit", "-q", "--allow-empty", "-m", "first")
	g := Git{O: o, WS: ws, State: state}
	repo := declare.Repo{Name: "app", Repo: "acme/app"}
	if err := g.Fetch(repo); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "work")
	if _, err := g.Clone(repo, "main", "main", work, "worker", "swarm@example.net", "c <c@example.net>"); err != nil {
		t.Fatal(err)
	}
	if err := g.Branch(work, "swarm/1-1"); err != nil { // as launch does for a new slice
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(work, ".git", "logs")); err == nil {
		t.Error("a new slice's branch left host git's reflog")
	}
	git(work, "commit", "-q", "--allow-empty", "-m", "Work")
	sha, err := g.FetchRun("app", work, "swarm/1-1", "refs/swarm-runs/1-worker-1")
	if err != nil {
		t.Fatal(err)
	}
	check := filepath.Join(t.TempDir(), "check")
	if err := g.CheckClone("app", check, "refs/swarm-runs/1-worker-1", sha); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{work, check} {
		filepath.WalkDir(filepath.Join(dir, ".git"), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.Contains(p, string(filepath.Separator)+"objects"+string(filepath.Separator)) {
				return err
			}
			b, _ := os.ReadFile(p)
			if strings.Contains(string(b), "host@example.net") || strings.Contains(string(b), g.Mirror("app")) {
				t.Errorf("%s names the host: %q", p, b)
			}
			return nil
		})
	}
}

// A member's clone that reaches outside itself is not fetched from: a
// planted alternates file or a ref that is a link to another repository's
// would have host git read that repository's objects (section 8).
func TestFetchRunRefusesAForeignClone(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	o := ops.Real()
	git := func(dir string, args ...string) {
		t.Helper()
		if _, errb, err := o.Exec(dir, "git", args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
	}
	ws, up := productIn(t)
	state, other := t.TempDir(), t.TempDir()
	for _, d := range []string{up, other} {
		git(d, "init", "-q", "-b", "main")
		git(d, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first")
	}
	g := Git{O: o, WS: ws, State: state}
	if err := g.Fetch(declare.Repo{Name: "app", Repo: "acme/app"}); err != nil {
		t.Fatal(err)
	}
	for name, plant := range map[string]func(work string){
		"alternates": func(work string) {
			os.WriteFile(filepath.Join(work, ".git", "objects", "info", "alternates"), []byte(filepath.Join(other, ".git", "objects")+"\n"), 0o644)
		},
		"a linked ref": func(work string) {
			p := filepath.Join(work, ".git", "refs", "heads", "swarm", "1-1")
			os.Remove(p)
			os.Symlink(filepath.Join(other, ".git", "refs", "heads", "main"), p)
		},
		"a linked objects directory": func(work string) {
			os.RemoveAll(filepath.Join(work, ".git", "objects", "pack"))
			os.Symlink(filepath.Join(other, ".git", "objects", "pack"), filepath.Join(work, ".git", "objects", "pack"))
		},
	} {
		work := filepath.Join(t.TempDir(), "work")
		if _, err := g.Clone(declare.Repo{Name: "app", Repo: "acme/app"}, "main", "main", work, "worker", "swarm@example.net", "c <c@example.net>"); err != nil {
			t.Fatal(err)
		}
		git(work, "checkout", "-q", "-b", "swarm/1-1")
		git(work, "commit", "-q", "--allow-empty", "-m", "Work")
		plant(work)
		if _, err := g.FetchRun("app", work, "swarm/1-1", "refs/swarm-runs/1-worker-1"); !errors.Is(err, ErrForeignClone) || strings.Contains(err.Error(), work) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The round r > 1 Task text's diff, run as written in a reviewer's clone
// (section 7.4): after the worker merged base, it shows the worker's
// change since the previous head and leaves out what the merge brought
// in, a file the branch does not change and one base added.
func TestRoundDiffLeavesOutABaseMerge(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	o := ops.Real()
	dir := t.TempDir()
	run := func(name string, args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, name, args...)
		if err != nil {
			t.Fatalf("%s %v: %v %s", name, args, err, errb)
		}
		return string(out)
	}
	commit := func(files map[string]string, msg string) {
		for f, text := range files {
			os.WriteFile(filepath.Join(dir, f), []byte(text), 0o644)
		}
		run("git", "add", "-A")
		run("git", "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "-m", msg)
	}
	run("git", "init", "-q", "-b", "main")
	commit(map[string]string{"a.go": "a1\n", "x.go": "x1\n"}, "base")
	run("git", "checkout", "-q", "-b", "swarm/1-1")
	commit(map[string]string{"a.go": "a2\n"}, "slice")
	run("git", "branch", "aaa") // the previous head the fixture's reviews name
	run("git", "checkout", "-q", "main")
	commit(map[string]string{"x.go": "x2\n", "y.go": "y\n"}, "another slice merged")
	run("git", "checkout", "-q", "swarm/1-1")
	run("git", "-c", "user.name=T", "-c", "user.email=t@example.net", "merge", "-q", "--no-edit", "main")
	commit(map[string]string{"a.go": "a3\n"}, "the revision")

	f, at, pr, _ := planned(t)
	f.Push(pr, "bbb")
	f.Review(pr, forge.Commented, marker("sympathetic", 2)+"VERDICT: approve")
	pulls, _ := f.Pulls(1, []string{"swarm-bot"})
	j := job(f, at.N, "adversary", "swarm:review-0-ready")
	j.Pulls, j.Pull = pulls, thread(f, pr)
	sys, _ := Render(j)
	m := regexp.MustCompile(`code changed since[^(]*\("([^"]+)"\)`).FindStringSubmatch(sys)
	if m == nil {
		t.Fatalf("no diff in the round 2 Task:\n%s", sys)
	}
	diff := run("sh", "-c", m[1])
	if !strings.Contains(diff, "-a2\n+a3\n") || strings.Contains(diff, "x.go") || strings.Contains(diff, "y.go") {
		t.Errorf("%s:\n%s", m[1], diff)
	}
}

// Conflicts reads a head against base on the mirror with real git: a head
// that merges is clean; one that conflicts names each conflicting path
// once, a name with a space as it is; a head the mirror does not hold is
// an error, not a conflict (merge-tree exits 1 for both).
func TestConflicts(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	o := ops.Real()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, "git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.net"}, args...)...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(dir string, files map[string]string) {
		for name, text := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git(dir, "add", "-A")
		git(dir, "commit", "-q", "-m", "c")
	}
	state := t.TempDir()
	m := filepath.Join(state, "git", "app")
	os.MkdirAll(m, 0o755)
	git(m, "init", "-q", "-b", "main")
	write(m, map[string]string{"a.go": "a\n", "b c.go": "b\n", "d.go": "d\n"})
	git(m, "checkout", "-q", "-b", "swarm/1-1")
	write(m, map[string]string{"a.go": "slice\n", "b c.go": "slice\n"})
	conflicting := git(m, "rev-parse", "HEAD")
	git(m, "checkout", "-q", "-b", "swarm/2-1", "main")
	write(m, map[string]string{"e.go": "e\n"})
	clean := git(m, "rev-parse", "HEAD")
	git(m, "checkout", "-q", "main")
	write(m, map[string]string{"a.go": "base\n", "b c.go": "base\n", "d.go": "base\n"})
	g := Git{O: o, State: state}
	if paths, ok, err := g.Conflicts("app", "main", clean); err != nil || !ok || paths != nil {
		t.Errorf("clean: %q %v %v", paths, ok, err)
	}
	if paths, ok, err := g.Conflicts("app", "main", conflicting); err != nil || ok || !reflect.DeepEqual(paths, []string{"a.go", "b c.go"}) {
		t.Errorf("conflicting: %q %v %v", paths, ok, err)
	}
	if _, ok, err := g.Conflicts("app", "main", strings.Repeat("0", 40)); err == nil || ok {
		t.Errorf("unknown head: %v %v", ok, err)
	}
}

// The mirrors' remotes (section 7.1, step 1; section 13, revision 6's
// Step 3, the first row), with real git and upstream bare repositories
// reached by URL: an entry named by its repo is reached at the swarm
// repository's clone's origin with its last element replaced, its
// ssh_command recorded in the mirror's config, changed and unset with the
// configuration, and the fetch and the push read them there, no product's
// clone on the machine; and a mirror made before the config recorded
// anything (an older Daisy's: no remote at all) fetches and pushes.
func TestMirrorsRecordTheirRemote(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	o := ops.Real()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, "git", args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	forgeDir := t.TempDir() // the forge: <owner>/<repo>.git under it
	for _, name := range []string{"app", "lib"} {
		src := t.TempDir()
		git(src, "init", "-q", "-b", "main")
		git(src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", name)
		git("", "clone", "-q", "--bare", src, filepath.Join(forgeDir, "acme", name+".git"))
	}
	config := func(m, key string) string {
		out, _, err := o.Exec(m, "git", "config", "--local", "--get", key)
		if err != nil {
			return "<unset>"
		}
		return strings.TrimSpace(string(out))
	}

	clone := filepath.Join(t.TempDir(), "swarm-app")
	os.MkdirAll(clone, 0o755)
	git(clone, "init", "-q")
	git(clone, "remote", "add", "origin", filepath.Join(forgeDir, "acme", "swarm-app.git"))
	state := t.TempDir()
	g := Git{O: o, WS: clone, State: state}
	ssh := "ssh -i /keys/lib"
	lib := declare.Repo{Name: "lib", Repo: "acme/lib", SSHCommand: &ssh}
	app := declare.Repo{Name: "app", Repo: "acme/app"}
	for _, r := range []declare.Repo{app, lib} {
		if err := g.Fetch(r); err != nil {
			t.Fatal(err)
		}
	}
	if got := config(g.Mirror("lib"), "remote.origin.url"); got != filepath.Join(forgeDir, "acme", "lib.git") {
		t.Errorf("lib's url %s", got)
	}
	if got := config(g.Mirror("lib"), "core.sshCommand"); got != ssh {
		t.Errorf("lib's ssh command %s", got)
	}
	if got := config(g.Mirror("app"), "core.sshCommand"); got != "<unset>" {
		t.Errorf("app's ssh command %s", got)
	}
	if git(g.Mirror("lib"), "log", "-1", "--format=%s", "main") != "lib" {
		t.Error("lib's mirror did not fetch lib")
	}
	other := "ssh -i /keys/other"
	lib.SSHCommand = &other
	if err := g.Fetch(lib); err != nil || config(g.Mirror("lib"), "core.sshCommand") != other {
		t.Errorf("changed: %v %s", err, config(g.Mirror("lib"), "core.sshCommand"))
	}
	lib.SSHCommand = nil
	if err := g.Fetch(lib); err != nil || config(g.Mirror("lib"), "core.sshCommand") != "<unset>" {
		t.Errorf("unset: %v %s", err, config(g.Mirror("lib"), "core.sshCommand"))
	}

	// The push reads the mirror's config: with the clone's origin moved
	// after the fetch, it reaches the new URL, recorded first.
	moved := t.TempDir()
	git("", "clone", "-q", "--bare", filepath.Join(forgeDir, "acme", "app.git"), filepath.Join(moved, "acme", "app.git"))
	git(clone, "remote", "set-url", "origin", filepath.Join(moved, "acme", "swarm-app.git"))
	sha := git(g.Mirror("app"), "rev-parse", "main")
	git(g.Mirror("app"), "update-ref", "refs/swarm-runs/1-worker-1", sha)
	if err := g.Push(app, "refs/swarm-runs/1-worker-1", "swarm/1-1", sha); err != nil {
		t.Fatal(err)
	}
	if git(filepath.Join(moved, "acme", "app.git"), "rev-parse", "swarm/1-1") != sha || config(g.Mirror("app"), "remote.origin.url") != filepath.Join(moved, "acme", "app.git") {
		t.Errorf("the push did not reach the recorded URL")
	}

	// A mirror an older Daisy made, with no remote.
	git(clone, "remote", "set-url", "origin", filepath.Join(forgeDir, "acme", "swarm-app.git"))
	old := Git{O: o, WS: clone, State: t.TempDir()}
	os.MkdirAll(old.Mirror("app"), 0o755)
	git(old.Mirror("app"), "init", "--bare", "--quiet")
	git(old.Mirror("app"), "fetch", "--quiet", filepath.Join(forgeDir, "acme", "app.git"), "+refs/heads/*:refs/heads/*")
	git(old.Mirror("app"), "update-ref", "refs/swarm-runs/1-worker-2", sha)
	app.SSHCommand = &ssh
	if err := old.Push(app, "refs/swarm-runs/1-worker-2", "swarm/1-2", sha); err != nil {
		t.Fatalf("an old mirror's push: %v", err)
	}
	if err := old.Fetch(app); err != nil {
		t.Fatalf("an old mirror's fetch: %v", err)
	}
	if config(old.Mirror("app"), "remote.origin.url") != filepath.Join(forgeDir, "acme", "app.git") || config(old.Mirror("app"), "core.sshCommand") != ssh ||
		git(filepath.Join(forgeDir, "acme", "app.git"), "rev-parse", "swarm/1-2") != sha {
		t.Errorf("the old mirror's config: %s, %s", config(old.Mirror("app"), "remote.origin.url"), config(old.Mirror("app"), "core.sshCommand"))
	}
}
