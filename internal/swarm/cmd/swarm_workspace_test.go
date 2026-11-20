package cmd

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"reflect"
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
	return a.jobFile(n, filepath.Join("texts", run+".md"))
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

// jobFile is issue n's workspace file rel (section 7.8), and whether it is
// there.
func (a *applyWS) jobFile(n int, rel string) (string, bool) {
	a.t.Helper()
	b, err := os.ReadFile(a.state(filepath.Join("jobs", strconv.Itoa(n), rel)))
	if errors.Is(err, iofs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		a.t.Fatal(err)
	}
	return string(b), true
}

// builderPlan writes the architect's builder plan to its run's io
// (section 7.8, The builder plan).
func builderPlan(text string) func(string) error {
	return func(io string) error { return os.WriteFile(filepath.Join(io, "builder.md"), []byte(text), 0o644) }
}

// The plan, the builder plan and the slices (section 7.8): the
// collection of an architect's plan writes plan.md, the plan whole;
// builder.md, the builder plan its run wrote beside it, whole and not
// posted; and each declared slice's slice.md, its item under ## Steps
// whole, then its ## Slice <s> section of the builder plan (## Slice
// 20 is not slice 2's), which ends at the next ## line outside a fenced
// block, or the builder plan's end. A revised plan written without a builder plan removes builder.md
// and leaves each slice its step alone. The posts are as before.
func TestWorkspacePlanBuilderAndSlices(t *testing.T) {
	a := newApplyWS(t)
	plan := "## Research\n\n- /work/app/main.go: the entry point.\n\n## Steps\n\n1. app: change main.go to v1; the test proves it.\n2. app: add notes.md.\n   Its claim: notes.md lists v1.\n\n   ```text\n   notes.md\n   ```\n\n## Test Plan\n\n1. main.go says v1.\n"
	builder := "# Builder plan\n\nRead before any slice.\n\n## Slice 1\n\nEdit main.go: print v1.\n\n## Slice 20\n\nNot slice 2.\n\n## Slice 2: notes\n\nWrite notes.md:\n\n```markdown\n## Slice 1\nv1\n```\n"
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
	a.runner.Script(1, "architect", runner.Canned{Out: plan, IO: builderPlan(builder)})
	a.pass()
	if got, ok := a.jobFile(1, "plan.md"); !ok || got != plan {
		t.Errorf("plan.md (%v): %q", ok, got)
	}
	if got, ok := a.jobFile(1, "builder.md"); !ok || got != builder {
		t.Errorf("builder.md (%v): %q", ok, got)
	}
	for s, want := range map[int]string{
		1: "1. app: change main.go to v1; the test proves it.\n\n## Slice 1\n\nEdit main.go: print v1.\n",
		2: "2. app: add notes.md.\n   Its claim: notes.md lists v1.\n\n   ```text\n   notes.md\n   ```\n\n## Slice 2: notes\n\nWrite notes.md:\n\n```markdown\n## Slice 1\nv1\n```\n",
	} {
		if got, ok := a.jobFile(1, filepath.Join("slices", strconv.Itoa(s), "slice.md")); !ok || got != want {
			t.Errorf("slices/%d/slice.md (%v):\n%s", s, ok, got)
		}
	}
	if bs := a.bodies(1); len(bs) != 1 || bs[0] != mark("architect", 1)+plan {
		t.Errorf("the thread: %q", bs)
	}

	revised := "## Steps\n\n1. app: change main.go to v2.\n2. app: add notes.md listing v2.\n\n## Test Plan\n\n1. main.go says v2.\n"
	a.forge.Say(issue1, "jane", "Make it v2.")
	a.runner.Script(1, "architect", runner.Canned{Out: revised})
	a.pass()
	if got, ok := a.jobFile(1, "plan.md"); !ok || got != revised {
		t.Errorf("plan.md after the revision (%v): %q", ok, got)
	}
	if _, ok := a.jobFile(1, "builder.md"); ok {
		t.Error("builder.md outlived its plan")
	}
	for s, want := range map[int]string{1: "1. app: change main.go to v2.\n", 2: "2. app: add notes.md listing v2.\n"} {
		if got, ok := a.jobFile(1, filepath.Join("slices", strconv.Itoa(s), "slice.md")); !ok || got != want {
			t.Errorf("slices/%d/slice.md after the revision (%v): %q", s, ok, got)
		}
	}
	if bs := a.bodies(1); len(bs) != 3 || bs[2] != mark("architect", 2)+revised || strings.Contains(strings.Join(bs, "\n"), "Builder plan") {
		t.Errorf("the thread: %q", bs)
	}
}

// A builder plan is secret-checked as the plan's text is (section 7.6,
// The secret check): a token in it fails the run, secret in output
// naming builder.md, and nothing of the run is written or posted but the
// note. A run whose text is no plan (BLOCKED:) has no builder plan, and
// its builder.md is not read.
func TestSecretInBuilderPlan(t *testing.T) {
	token := "oauth-token-0123456789abcdef0123456789" // the architect's env value
	t.Run("a plan", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
		a.runner.Script(1, "architect", runner.Canned{Out: archPlan, IO: builderPlan("## Slice 1\n\nleak " + token + "\n")})
		a.pass()
		if r := a.result("1-architect-1"); !strings.Contains(r, "failed: secret in output: builder.md\n") {
			t.Errorf("result:\n%s", r)
		}
		want := []string{mark("reconciler", 1) + "architect run 1 failed: secret in output: builder.md (at swarm:proposal-accepted). Remove swarm:blocked to resume.\n\ncc @jane"}
		if bs := a.bodies(1); !reflect.DeepEqual(bs, want) {
			t.Errorf("the thread: %q", bs)
		}
		if _, err := os.Stat(a.state(filepath.Join("jobs", "1"))); !errors.Is(err, iofs.ErrNotExist) {
			t.Errorf("the workspace was written: %v %q", err, a.texts(1))
		}
	})
	t.Run("no plan", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
		a.runner.Script(1, "architect", runner.Canned{Out: "BLOCKED: which repository?\n", IO: builderPlan("leak " + token + "\n")})
		a.pass()
		if r := a.result("1-architect-1"); !strings.HasSuffix(r, "outcome blocked\ncollected\n") {
			t.Errorf("result:\n%s", r)
		}
		if _, ok := a.jobFile(1, "builder.md"); ok {
			t.Error("a BLOCKED: run's builder.md was written")
		}
	})
}
