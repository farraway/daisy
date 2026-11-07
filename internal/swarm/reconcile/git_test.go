package reconcile

import (
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
}
