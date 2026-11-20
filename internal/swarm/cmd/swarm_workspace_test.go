package cmd

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// text is issue n's workspace file texts/<run>.md (section 7.8), and
// whether it is there.
func (a *applyWS) text(n int, run string) (string, bool) {
	a.t.Helper()
	b, err := os.ReadFile(a.state(filepath.Join("jobs", strconv.Itoa(n), "texts", run+".md")))
	if errors.Is(err, iofs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		a.t.Fatal(err)
	}
	return string(b), true
}

// texts is the names in issue n's texts/, sorted.
func (a *applyWS) texts(n int) []string {
	entries, _ := os.ReadDir(a.state(filepath.Join("jobs", strconv.Itoa(n), "texts")))
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Each member text Daisy posts for a job is written whole to the job's
// workspace, texts/<n>-<role>-<k>.md, at the collection that posts it
// (section 7.8): the architect's plan, cut on the forge, whole; the
// worker's question; the architect's answer with the ANSWER: Daisy adds;
// the worker's report with its check block; a reviewer's BLOCKED: text,
// which is no review and is not held. The owner's batch run has no
// workspace, and the posts are as before.
func TestWorkspaceTexts(t *testing.T) {
	a := newApplyWS(t)
	a.release()
	a.runner.Script(0, "owner", runner.Canned{Out: "## Drift check\nBrief: docs/briefs/a.md — M1 The drift\nCheck the drift.\n", Stdout: claudeResult})
	a.pass()
	if _, err := os.Stat(a.state("jobs")); !errors.Is(err, iofs.ErrNotExist) {
		t.Fatalf("the owner's batch run wrote a workspace: %v", err)
	}

	a.forge.SetLabel(issue1, forge.ProposalAccepted)
	a.runner.Script(1, "architect", runner.Canned{Out: bigPlan})
	a.pass()
	if got, ok := a.text(1, "1-architect-1"); !ok || got != bigPlan {
		t.Fatalf("texts/1-architect-1.md (%v) is not the plan whole: %d bytes of %d", ok, len(got), len(bigPlan))
	}
	if bs := a.bodies(1); len(bs) != 1 || !strings.HasPrefix(bs[0], mark("architect", 1)) || !strings.Contains(bs[0], "\n[truncated by Daisy; the whole output is in runs/1-architect-1 and attached as ") {
		t.Errorf("the thread: %d comments; the last ends:\n%s", len(bs), bs[len(bs)-1][max(0, len(bs[len(bs)-1])-300):])
	}

	a.forge.SetLabel(issue1, forge.PlanAccepted)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: Should main.go say v1 or v1.0?\n"})
	a.pass()
	if got, ok := a.text(1, "1-worker-1"); !ok || got != "QUESTION: Should main.go say v1 or v1.0?\n" || a.lastComment(1) != mark("worker", 1)+got {
		t.Fatalf("the question: %q (%v), posted %q", got, ok, a.lastComment(1))
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "v1, as the Test Plan's first claim says.\n", Stdout: claudeResult})
	a.pass()
	if got, ok := a.text(1, "1-architect-2"); !ok || got != "ANSWER: v1, as the Test Plan's first claim says.\n" || a.lastComment(1) != mark("architect", 2)+got {
		t.Fatalf("the answer: %q (%v), posted %q", got, ok, a.lastComment(1))
	}

	a.runner.Script(1, "worker", work("v1"))
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok 1 test\n"})
	a.pass()
	pr := a.pulls(1)[0]
	report := "v1\n\nNot established: none\n\n**Check:** `make check` on " + pr.HeadSHA[:7] + ": exit 0\n\n```\nok 1 test\n```"
	if got, ok := a.text(1, "1-worker-2"); !ok || got != report || pr.Body != mark("worker", 2)+report+"\n\nRefs acme/swarm-app#1" {
		t.Fatalf("the report: %q (%v)\npull request's description: %q", got, ok, pr.Body)
	}

	a.runner.Script(1, "adversary", runner.Canned{Out: "BLOCKED: the plan names no test for v1.\n"})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got, ok := a.text(1, "1-adversary-1"); !ok || got != "BLOCKED: the plan names no test for v1.\n" {
		t.Errorf("the reviewer's BLOCKED: text: %q (%v)", got, ok)
	}
	want := []string{"1-adversary-1.md", "1-architect-1.md", "1-architect-2.md", "1-worker-1.md", "1-worker-2.md"}
	if _, ok := a.text(1, "1-sympathetic-1"); ok || strings.Join(a.texts(1), " ") != strings.Join(want, " ") {
		t.Errorf("texts/: %q", a.texts(1))
	}
}

// A review is held until its round completes (section 7.8, texts/): with
// one member a pass, the adversary's review of head H is posted and not
// written; the sympathetic reviewer's of H completes the round, and both
// are written. A push before the pair's second review leaves the first's
// round unable to complete: it is written at the next collection on the
// issue, from its run's out.md, whatever that collection posts (here a
// reviewer's BLOCKED: text, no review).
func TestWorkspaceHoldsAReviewUntilItsRound(t *testing.T) {
	t.Run("the round completes", func(t *testing.T) {
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass()
		if strings.Contains(a.stdout.String(), "sympathetic run 1: launched") || !strings.Contains(a.result("1-adversary-1"), "\nposted review\n") {
			t.Fatalf("pass 1 is not the adversary's alone:\n%s", a.stdout.String())
		}
		if _, ok := a.text(1, "1-adversary-1"); ok {
			t.Fatal("the adversary's review was written before its round completed")
		}
		a.pass()
		reviews, _ := a.forge.Reviews(forge.Ref{Repo: "app", N: 1})
		if len(reviews) != 2 || reviews[0].Body != mark("adversary", 1)+requestChanges || reviews[1].Body != mark("sympathetic", 1)+approve {
			t.Fatalf("the reviews as posted: %+v", reviews)
		}
		for run, want := range map[string]string{"1-adversary-1": requestChanges, "1-sympathetic-1": approve} {
			if got, ok := a.text(1, run); !ok || got != want {
				t.Errorf("texts/%s.md (%v): %q", run, ok, got)
			}
		}
	})

	t.Run("a push before the second review", func(t *testing.T) {
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
		a.inReview(oneSlice)
		blocked := "BLOCKED: the head moved under the review.\n"
		a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: blocked})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: blocked})
		a.pass()
		if _, ok := a.text(1, "1-adversary-1"); ok {
			t.Fatal("the adversary's review was written before its round completed")
		}
		dir := t.TempDir()
		gitIn(t, "", "clone", "-q", "--branch", "swarm/1-1", a.upstream("app"), dir)
		os.WriteFile(filepath.Join(dir, "main.go"), []byte("v1, by hand\n"), 0o644)
		gitIn(t, dir, "commit", "-q", "-am", "The maintainer's fix")
		gitIn(t, dir, "push", "-q", "origin", "swarm/1-1")
		a.pass()
		var next string
		for _, run := range []string{"1-adversary-2", "1-sympathetic-1"} {
			if strings.Contains(a.result(run), "\nposted comment\n") {
				next = run
			}
		}
		if got, ok := a.text(1, next); next == "" || !ok || got != blocked {
			t.Fatalf("pass 2's BLOCKED: text (%s, %v): %q\n%s", next, ok, got, a.stdout.String())
		}
		if got, ok := a.text(1, "1-adversary-1"); !ok || got != requestChanges {
			t.Errorf("the review whose round can no longer complete (%v): %q", ok, got)
		}
	})
}
