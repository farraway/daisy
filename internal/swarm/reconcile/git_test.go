package reconcile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// The clone a member sees (section 7.1, step 3), with real git: no remote,
// a local base, the role's identity at the commit address, /.swarm/
// excluded, and the hook stamping the role's co-author on each commit the
// member makes, once, even when the message already carries it.
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
	ws, state := t.TempDir(), t.TempDir()
	up := t.TempDir()
	git(up, "init", "-q", "-b", "main")
	git(up, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first")
	git(up, "branch", "swarm/1-1")
	os.MkdirAll(filepath.Join(ws, "app"), 0o755)
	git(filepath.Join(ws, "app"), "init", "-q")
	git(filepath.Join(ws, "app"), "remote", "add", "origin", up)
	g := Git{O: o, WS: ws, State: state}
	repo := declare.Repo{Name: "app", Path: "app"}
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
	if b, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude")); !strings.HasSuffix(string(b), "/.swarm/\n") {
		t.Errorf("exclude %q", b)
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
	ws, state, up := t.TempDir(), t.TempDir(), t.TempDir()
	git(up, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(up, "AGENTS.md"), []byte("Rules.\n"), 0o644)
	git(up, "add", "AGENTS.md")
	git(up, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "-m", "first")
	os.MkdirAll(filepath.Join(ws, "app"), 0o755)
	git(filepath.Join(ws, "app"), "init", "-q")
	git(filepath.Join(ws, "app"), "remote", "add", "origin", up)
	g := Git{O: o, WS: ws, State: state}
	repo := declare.Repo{Name: "app", Path: "app"}
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
	git(filepath.Join(ws, "app"), "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
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
	ws, state, up := t.TempDir(), t.TempDir(), t.TempDir()
	git(up, "init", "-q", "-b", "main")
	git(up, "commit", "-q", "--allow-empty", "-m", "first")
	os.MkdirAll(filepath.Join(ws, "app"), 0o755)
	git(filepath.Join(ws, "app"), "init", "-q")
	git(filepath.Join(ws, "app"), "remote", "add", "origin", up)
	g := Git{O: o, WS: ws, State: state}
	repo := declare.Repo{Name: "app", Path: "app"}
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
	ws, state, up, other := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, d := range []string{up, other} {
		git(d, "init", "-q", "-b", "main")
		git(d, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "first")
	}
	os.MkdirAll(filepath.Join(ws, "app"), 0o755)
	git(filepath.Join(ws, "app"), "init", "-q")
	git(filepath.Join(ws, "app"), "remote", "add", "origin", up)
	g := Git{O: o, WS: ws, State: state}
	if err := g.Fetch(declare.Repo{Name: "app", Path: "app"}); err != nil {
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
		if _, err := g.Clone(declare.Repo{Name: "app", Path: "app"}, "main", "main", work, "worker", "swarm@example.net", "c <c@example.net>"); err != nil {
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
