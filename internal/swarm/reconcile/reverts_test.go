package reconcile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
)

// The step's claim (Step 29 of the plan remediating the 2026-11-15
// design review, talk A13): a swarm whose mirror holds N merged swarm
// pull requests, one of them reverted on base, reads 1 of N. Not
// counted: a merge of a branch no worker run pushed, its revert, a
// revert on a branch that is not base, a pushed head never merged, and,
// with since, a merge made before it.
func TestPullReverts(t *testing.T) {
	o := ops.Real()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, errb, err := o.Exec(dir, "git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.net"}, args...)...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, errb)
		}
		return strings.TrimSpace(string(out))
	}
	work, state := t.TempDir(), t.TempDir()
	git(work, "init", "-q", "-b", "main")
	git(work, "commit", "-q", "--allow-empty", "-m", "first")
	// branch makes a commit adding a file on a new branch off main: its
	// head.
	branch := func(name string) string {
		git(work, "checkout", "-q", "-b", name, "main")
		os.WriteFile(filepath.Join(work, strings.ReplaceAll(name, "/", "-")), []byte(name+"\n"), 0o644)
		git(work, "add", "-A")
		git(work, "commit", "-q", "-m", name+" work")
		head := git(work, "rev-parse", "HEAD")
		git(work, "checkout", "-q", "main")
		return head
	}
	// merge merges the branch into main with a merge commit, as the
	// forge does, on the day given (ops' git drops GIT_COMMITTER_DATE):
	// the merge commit.
	merge := func(name, day string) string {
		cmd := exec.Command("git", "-c", "user.name=T", "-c", "user.email=t@example.net", "merge", "-q", "--no-ff", "-m", "Merge pull request from "+name, name)
		cmd.Dir, cmd.Env = work, append(os.Environ(), "GIT_COMMITTER_DATE="+day+"T12:00:00Z")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git merge %s: %v %s", name, err, out)
		}
		return git(work, "rev-parse", "HEAD")
	}
	h1, h2, h3, other := branch("swarm/1-1"), branch("swarm/2-1"), branch("swarm/3-1"), branch("feature")
	unmerged := branch("swarm/4-1")
	merge("swarm/1-1", "2026-11-01")
	m2 := merge("swarm/2-1", "2026-11-02")
	m3 := merge("swarm/3-1", "2026-11-03")
	mOther := merge("feature", "2026-11-04")
	git(work, "revert", "--no-edit", "-m", "1", m2)
	git(work, "revert", "--no-edit", "-m", "1", mOther)
	git(work, "checkout", "-q", "-b", "elsewhere")
	git(work, "revert", "--no-edit", "-m", "1", m3) // not on base
	git(work, "checkout", "-q", "main")

	mirror := filepath.Join(state, "git", "app")
	os.MkdirAll(mirror, 0o755)
	git(mirror, "init", "-q", "--bare")
	git(mirror, "fetch", "-q", work, "+refs/heads/*:refs/heads/*")
	fetched := time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(mirror, "FETCH_HEAD"), fetched, fetched); err != nil {
		t.Fatal(err)
	}
	for run, result := range map[string]string{
		"1-worker-1": "branch swarm/1-1 in app\npushed " + h1 + "\noutcome posted\n",
		"2-worker-1": "branch swarm/2-1 in app\npushed " + h2 + "\noutcome posted\n",
		"3-worker-2": "branch swarm/3-1 in app\npushed " + h3 + "\noutcome posted\n",
		"4-worker-1": "branch swarm/4-1 in app\npushed " + unmerged + "\noutcome posted\n",
		// A reviewer's run names the head it read, never a push.
		"5-adversary-1": "start " + other + "\noutcome posted\n",
		// A repository the swarm has no mirror of: nothing to read.
		"6-worker-1": "branch swarm/6-1 in lib\npushed " + other + "\noutcome posted\n",
	} {
		os.MkdirAll(filepath.Join(state, "runs", run), 0o755)
		os.WriteFile(filepath.Join(state, "runs", run, "result"), []byte(result), 0o644)
	}

	got, err := ReadPullReverts(o, state, "main", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if want := (PullReverts{Merged: 3, Reverted: 1, Fetched: fetched}); !got.Fetched.Equal(want.Fetched) || got.Merged != want.Merged || got.Reverted != want.Reverted {
		t.Errorf("ReadPullReverts = %+v, want %+v", got, want)
	}
	since := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	if got, err := ReadPullReverts(o, state, "main", since); err != nil || got.Merged != 2 || got.Reverted != 1 {
		t.Errorf("ReadPullReverts since %s = %+v, %v; want 1 of 2", since, got, err)
	}
	// A base the mirror does not hold, and a state with no mirror: none.
	for _, c := range []struct{ state, base string }{{state, "trunk"}, {t.TempDir(), "main"}} {
		if got, err := ReadPullReverts(o, c.state, c.base, time.Time{}); err != nil || got != (PullReverts{}) {
			t.Errorf("ReadPullReverts(%s, %s) = %+v, %v; want none", c.state, c.base, got, err)
		}
	}
}
