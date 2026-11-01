package gitx_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/gitx"
	"daisy/internal/ops"
)

// script answers git by verb and records every call.
type script struct {
	calls   []string
	status  string
	fail    string // the verb that fails
	toplvls map[string]string
}

func (s *script) exec(dir, name string, args ...string) ([]byte, []byte, error) {
	s.calls = append(s.calls, name+" "+strings.Join(args, " "))
	if name != "git" {
		return nil, nil, errors.New("not git")
	}
	if args[0] == s.fail {
		return nil, []byte("fatal: scripted failure"), errors.New("exit status 1")
	}
	switch args[0] {
	case "rev-parse":
		if args[1] == "--show-toplevel" {
			if top, ok := s.toplvls[dir]; ok {
				return []byte(top + "\n"), nil, nil
			}
			return nil, []byte("fatal: not a git repository"), errors.New("exit status 128")
		}
		return []byte("abc1234\n"), nil, nil
	case "status":
		return []byte(s.status), nil, nil
	}
	return nil, nil, nil
}

func TestCommitIsOneCommit(t *testing.T) {
	s := &script{status: " M journal/today.md\n"}
	o := &ops.Fake{ExecFn: s.exec}
	repo := gitx.Repo{Dir: "/h"}
	hash, committed, err := repo.Commit(o, "Log: 1423 - x")
	if err != nil || !committed || hash != "abc1234" {
		t.Fatalf("hash %q committed %v err %v", hash, committed, err)
	}
	want := []string{"git status --porcelain -- .", "git add -- .", "git commit -m Log: 1423 - x", "git rev-parse --short HEAD"}
	if strings.Join(s.calls, "|") != strings.Join(want, "|") {
		t.Errorf("calls %q", s.calls)
	}
	commits := 0
	for _, c := range s.calls {
		if strings.HasPrefix(c, "git commit") {
			commits++
		}
	}
	if commits != 1 {
		t.Errorf("%d commits", commits)
	}
}

func TestCommitScopedAndClean(t *testing.T) {
	s := &script{status: "?? x\n"}
	o := &ops.Fake{ExecFn: s.exec}
	if _, _, err := (gitx.Repo{Dir: "/r/home/h", Scoped: true}).Commit(o, "m"); err != nil {
		t.Fatal(err)
	}
	if s.calls[2] != "git commit -m m -- ." {
		t.Errorf("scoped commit: %q", s.calls[2])
	}
	s = &script{status: ""}
	o = &ops.Fake{ExecFn: s.exec}
	hash, committed, err := (gitx.Repo{Dir: "/h"}).Commit(o, "m")
	if err != nil || committed || hash != "" || len(s.calls) != 1 {
		t.Errorf("clean tree: hash %q committed %v err %v calls %q", hash, committed, err, s.calls)
	}
}

func TestCommitFailuresNameTheStep(t *testing.T) {
	for _, verb := range []string{"add", "commit"} {
		s := &script{status: " M f\n", fail: verb}
		_, _, err := (gitx.Repo{Dir: "/h"}).Commit(&ops.Fake{ExecFn: s.exec}, "m")
		if err == nil || !strings.Contains(err.Error(), verb) || !strings.Contains(err.Error(), "scripted failure") {
			t.Errorf("%s failing: %v", verb, err)
		}
	}
}

// HomeRepo: the home's own repository, or the root's when the home lies
// inside it (then the commit is scoped to the home), else nothing.
func TestHomeRepo(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	own, _ := filepath.EvalSymlinks(t.TempDir())
	inside := filepath.Join(root, "home", "h")
	loose, _ := filepath.EvalSymlinks(t.TempDir())
	for _, d := range []string{own, inside} {
		os.MkdirAll(d, 0o755)
	}
	s := &script{toplvls: map[string]string{root: root, own: own, inside: root}}
	o := &ops.Fake{ExecFn: s.exec}
	if r, ok := gitx.HomeRepo(o, root, own); !ok || r.Dir != own || r.Scoped {
		t.Errorf("own repo: %+v %v", r, ok)
	}
	if r, ok := gitx.HomeRepo(o, root, inside); !ok || r.Dir != inside || !r.Scoped {
		t.Errorf("inside root: %+v %v", r, ok)
	}
	if _, ok := gitx.HomeRepo(o, root, loose); ok {
		t.Error("a home in no repository was accepted")
	}
}

// Against real git: the commit lands, carries the repository's own author
// even under a hook's environment, and nothing else is committed.
func TestCommitWithRealGit(t *testing.T) {
	dir := t.TempDir()
	o := ops.Real()
	run := func(args ...string) string {
		out, errb, err := o.Exec(dir, "git", args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("config", "user.name", "Fixture Author")
	run("config", "user.email", "fixture@example.invalid")
	os.WriteFile(filepath.Join(dir, "a"), []byte("1"), 0o644)
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	t.Setenv("GIT_AUTHOR_NAME", "Hook Author")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'user.email=hook@example.invalid'")
	os.WriteFile(filepath.Join(dir, "a"), []byte("2"), 0o644)
	repo, ok := gitx.HomeRepo(o, t.TempDir(), dir)
	if !ok {
		t.Fatal("not a repo")
	}
	hash, committed, err := repo.Commit(o, "second")
	if err != nil || !committed || hash == "" {
		t.Fatalf("hash %q committed %v err %v", hash, committed, err)
	}
	if got := run("log", "-1", "--format=%an <%ae> %s"); got != "Fixture Author <fixture@example.invalid> second" {
		t.Errorf("author: %q", got)
	}
	if got := run("rev-list", "--count", "HEAD"); got != "2" {
		t.Errorf("commits: %s", got)
	}
}
