package cmd

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"daisy/internal/ops"
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
// note (the workspace holds its built marker alone, written before the
// launch). A run whose text is no plan (BLOCKED:) has no builder plan,
// and its builder.md is not read.
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
		if tree := a.tree(1); len(tree) > 0 {
			t.Errorf("the workspace was written: %q", tree)
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

// A worker run with commits writes its slice's kept, whatever its
// outcome (section 7.8, slices/<s>/kept): a blocked run, `1-worker-1
// <the commit fetched from it>`, its run ref kept as before. A run whose
// commits fail the secret check, or change a protected path, writes
// none, the earlier kept staying; and so does a blocked run whose
// commits hold a secret, never checked before. A pushed run names its
// commit, the branch's.
func TestWorkspaceKept(t *testing.T) {
	secret := "oauth-token-0123456789abcdef0123456789" // the worker's env value
	a := newApplyWS(t)
	a.planned(oneSlice)
	mirror := a.state("git/app")
	unblock := func() { a.forge.Relabel(issue1, forge.Blocked, "") }

	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: which version?\n", Work: in("app", commit(map[string]string{"main.go": "half\n"}, "Half"))})
	a.pass()
	first := "1-worker-1 " + a.gitRunIn(mirror, "rev-parse", "refs/swarm-runs/1-worker-1") + "\n"
	if got, ok := a.jobFile(1, "slices/1/kept"); !ok || got != first {
		t.Fatalf("kept after the blocked run (%v): %q, want %q\n%s", ok, got, first, a.result("1-worker-1"))
	}

	for i, canned := range []runner.Canned{
		{Out: "Done.\n", Work: in("app", commit(map[string]string{"sub/key.txt": secret + "\n"}, "v1"))},
		{Out: "Done.\n", Work: in("app", commit(map[string]string{"main.go": "v1\n", ".gitea/workflows/ci.yml": "on: push\n"}, "v1"))},
		{Out: "BLOCKED: stuck.\n", Work: in("app", commit(map[string]string{"main.go": "token " + secret + "\n"}, "v1"))},
	} {
		unblock()
		a.runner.Script(1, "worker", canned)
		a.pass()
		run := "1-worker-" + strconv.Itoa(i+2)
		if r := a.result(run); !strings.Contains(r, "\nkept refs/swarm-runs/"+run+"\n") {
			t.Fatalf("%s kept no ref:\n%s", run, r)
		}
		if got, _ := a.jobFile(1, "slices/1/kept"); got != first {
			t.Errorf("kept after %s: %q, want %q\n%s", run, got, first, a.result(run))
		}
	}

	unblock()
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	head := a.pulls(1)[0].HeadSHA
	if got, ok := a.jobFile(1, "slices/1/kept"); !ok || got != "1-worker-5 "+head+"\n" {
		t.Errorf("kept after the push (%v): %q, the branch's head %s\n%s", ok, got, head, a.result("1-worker-5"))
	}
}

// A check that failed writes checks/<sha>.log (section 7.8): the `#
// Check` line, then its whole output, not the 20 lines of the tail the
// report carries; a check that passed writes none. The whole output is
// secret-checked before the file is written: a secret outside the tail is
// the failure secret in output naming check.out, and no file is written.
func TestWorkspaceCheckLog(t *testing.T) {
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, "line "+strconv.Itoa(i))
	}
	output := strings.Join(lines, "\n") + "\n"

	t.Run("failed", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", work("v1"))
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: output})
		a.pass()
		head := a.pulls(1)[0].HeadSHA
		want := "make check on " + head + ": exit 1\n\n" + output
		if got, ok := a.jobFile(1, "checks/"+head+".log"); !ok || got != want {
			t.Errorf("checks/%s.log (%v):\n%s", head, ok, got)
		}
		if body := a.pulls(1)[0].Body; strings.Contains(body, "line 10\n") || !strings.Contains(body, "line 11\n") {
			t.Errorf("the report's tail is not the last 20 lines:\n%s", body)
		}
	})

	t.Run("passed", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", work("v1"))
		a.runner.Script(1, "check", runner.Canned{Stdout: output})
		a.pass()
		if len(a.pulls(1)) != 1 {
			t.Fatalf("no pull request:\n%s", a.result("1-worker-1"))
		}
		if _, err := os.Stat(a.state("jobs/1/checks")); !errors.Is(err, iofs.ErrNotExist) {
			t.Errorf("a passed check wrote checks/: %v", err)
		}
	})

	t.Run("a secret outside the tail", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", work("v1"))
		a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: "env: oauth-token-0123456789abcdef0123456789\n" + output})
		a.pass()
		if r := a.result("1-worker-1"); !strings.Contains(r, "\nfailed: secret in output: check.out\n") {
			t.Errorf("result:\n%s", r)
		}
		if _, err := os.Stat(a.state("jobs/1/checks")); !errors.Is(err, iofs.ErrNotExist) {
			t.Errorf("checks/ was written: %v", err)
		}
	})
}

// roundFile is slice 1's rounds/<r>.md of issue 1, and whether it is
// there.
func (a *applyWS) roundFile(r int) (string, bool) {
	a.t.Helper()
	return a.jobFile(1, filepath.Join("slices", "1", "rounds", strconv.Itoa(r)+".md"))
}

// A round's file is written once the round completes (section 7.8,
// slices/<s>/rounds/<r>.md), not before: with one member a pass, the
// adversary's review alone writes none, the sympathetic reviewer's
// completes round 1 and writes its head and both reviews' finding lines,
// the adversary's first. A revision that merges main, which another
// slice moved, then changes main.go is reviewed in round 2, whose file
// holds the diff since round 1 in the pull request's files: main.go's
// change, not the merged file's. The worker's answer without a commit is
// reviewed in round 3, on the same head, which names the answer.
func TestWorkspaceRoundFile(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
	a.inReview(oneSlice)
	h1 := a.pulls(1)[0].HeadSHA
	nonBlocking := "VERDICT: approve\n\nFine.\n[non-blocking] main.go:1 could say why.\n"
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: requestChanges}, runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: nonBlocking}, runner.Canned{Out: approve}, runner.Canned{Out: approve})
	a.pass()
	if _, ok := a.roundFile(1); ok {
		t.Fatal("round 1 was written before it completed")
	}
	a.pass()
	want1 := "# Round 1 on " + h1 + "\n\nSlice 1, pull request app#1. The round before: none.\n\n" +
		"## adversary, run 1: request-changes\n\nWhole: /job/memory/texts/1-adversary-1.md\n\n[blocking] main.go:1 says v1; the plan wants v2.\n\n" +
		"## sympathetic, run 1: approve\n\nWhole: /job/memory/texts/1-sympathetic-1.md\n\n[non-blocking] main.go:1 could say why.\n"
	if got, ok := a.roundFile(1); !ok || got != want1 {
		t.Fatalf("rounds/1.md (%v):\n%s\nwant:\n%s", ok, got, want1)
	}

	a.land("app", map[string]string{"other.txt": "another slice\n"}, "another slice merged")
	a.runner.Script(1, "worker", runner.Canned{Out: "v2, main merged.\n\nNot established: none\n", Work: func(work string) error {
		work = filepath.Join(work, "app") // the checkout, under /job/work
		if _, errb, err := ops.Real().Exec(work, "git", "merge", "-q", "--no-edit", "main"); err != nil {
			return &execErr{[]string{"merge"}, string(errb)}
		}
		return commit(map[string]string{"main.go": "v2\n"}, "v2")(work)
	}})
	a.pass()
	h2 := a.pulls(1)[0].HeadSHA
	if got := labels(a.issue(1)); h2 == h1 || !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) {
		t.Fatalf("the revision: %v, head %s\n%s", got, h2, a.result("1-worker-2"))
	}
	a.pass()
	a.pass()
	got, ok := a.roundFile(2)
	head2 := "# Round 2 on " + h2 + "\n\nSlice 1, pull request app#1. The round before: 1 on " + h1 + ".\n\n" +
		"## adversary, run 2: request-changes\n\nWhole: /job/memory/texts/1-adversary-2.md\n\n[blocking] main.go:1 says v1; the plan wants v2.\n\n" +
		"## sympathetic, run 2: approve\n\nWhole: /job/memory/texts/1-sympathetic-2.md\n\n## Changes since round 1\n\n```diff\ndiff --git a/main.go b/main.go\n"
	if !ok || !strings.HasPrefix(got, head2) || !strings.HasSuffix(got, "\n@@ -1 +1 @@\n-v1\n+v2\n```\n") || strings.Contains(got, "other.txt") {
		t.Fatalf("rounds/2.md (%v):\n%s", ok, got)
	}

	a.runner.Script(1, "worker", runner.Canned{Out: "NO CHANGE: main.go:1 says v2 already.\n\nNot established: none\n"})
	a.pass()
	a.pass()
	a.pass()
	want3 := "# Round 3 on " + h2 + "\n\nSlice 1, pull request app#1. The round before: 2 on " + h2 + ".\n\n" +
		"## adversary, run 3: approve\n\nWhole: /job/memory/texts/1-adversary-3.md\n\n" +
		"## sympathetic, run 3: approve\n\nWhole: /job/memory/texts/1-sympathetic-3.md\n\n" +
		"## Changes since round 2\n\nNo commit since round 2: the worker answered without a commit, /job/memory/texts/1-worker-3.md.\n"
	if got, ok := a.roundFile(3); !ok || got != want3 {
		t.Errorf("rounds/3.md (%v):\n%s\nwant:\n%s\n%s", ok, got, want3, a.stdout.String())
	}
}

// A round whose previous round's head is no longer in the mirror (section
// 7.8, A round's file): the maintainer force-pushes the slice's branch
// past round 1's head, the worker's revision builds on the new branch,
// and the mirror loses round 1's head; round 2's file says in one
// sentence that the changes cannot be shown, naming the head, and its
// review's collection still makes its transition, T9 on two approvals
// (the issue then held at the merge gate).
func TestWorkspaceRoundFileCommitGone(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
	a.inReview(oneSlice)
	h1 := a.pulls(1)[0].HeadSHA
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve}, runner.Canned{Out: approve})
	a.pass()
	a.pass()
	if _, ok := a.roundFile(1); !ok {
		t.Fatalf("no rounds/1.md\n%s", a.stdout.String())
	}

	// The maintainer rewrites swarm/1-1 from main, without round 1's head.
	src := t.TempDir()
	a.gitRunIn("", "clone", "-q", a.upstream("app"), src)
	a.gitRunIn(src, "checkout", "-q", "-B", "swarm/1-1", "origin/main")
	if err := commit(map[string]string{"main.go": "v1, rewritten\n"}, "v1 rewritten")(src); err != nil {
		t.Fatal(err)
	}
	a.gitRunIn(src, "push", "-q", "--force", "origin", "swarm/1-1")
	a.runner.Script(1, "worker", runner.Canned{Out: "v2.\n\nNot established: none\n", Work: in("app", commit(map[string]string{"main.go": "v2\n"}, "v2"))})
	a.pass()
	h2 := a.pulls(1)[0].HeadSHA
	if got := labels(a.issue(1)); h2 == h1 || !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) {
		t.Fatalf("the revision: %v, head %s\n%s", got, h2, a.result("1-worker-2"))
	}
	mirror := a.state("git/app")
	a.gitRunIn(mirror, "reflog", "expire", "--expire=now", "--all")
	a.gitRunIn(mirror, "gc", "-q", "--prune=now")
	if _, _, err := ops.Real().Exec(mirror, "git", "cat-file", "-e", h1+"^{commit}"); err == nil {
		t.Fatalf("round 1's head %s is still in the mirror", h1)
	}

	a.pass()
	a.pass()
	got, ok := a.roundFile(2)
	want := "# Round 2 on " + h2 + "\n\nSlice 1, pull request app#1. The round before: 1 on " + h1 + ".\n\n" +
		"## adversary, run 2: approve\n\nWhole: /job/memory/texts/1-adversary-2.md\n\n" +
		"## sympathetic, run 2: approve\n\nWhole: /job/memory/texts/1-sympathetic-2.md\n\n" +
		"## Changes since round 1\n\nThe changes since round 1 cannot be shown: " + h1 + " is not in the mirror.\n"
	if !ok || got != want {
		t.Errorf("rounds/2.md (%v):\n%s\nwant:\n%s\n%s", ok, got, want, a.stdout.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", "swarm:gated"}) {
		t.Errorf("after round 2: %v, want T9's approved, held at the merge gate\n%s", got, a.stdout.String())
	}
}

// tree is every file of issue n's workspace but its built marker, by its
// path in the workspace, with its bytes.
func (a *applyWS) tree(n int) map[string]string {
	a.t.Helper()
	root := a.state(filepath.Join("jobs", strconv.Itoa(n)))
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d iofs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel != "built" {
			b, err := os.ReadFile(p)
			out[rel] = string(b)
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		a.t.Fatal(err)
	}
	return out
}

// built: issue n's workspace holds its built marker (section 7.8,
// Rollout).
func (a *applyWS) built(n int) bool {
	_, err := os.Lstat(a.state(filepath.Join("jobs", strconv.Itoa(n), "built")))
	return err == nil
}

// The claim of Step R1-4 (section 7.8, Rollout): a job's workspace
// replayed over its runs is byte-identical to the one its collections
// wrote. A job runs through a plan with a builder plan (two slices), a
// worker's question and the architect's answer, a report whose check
// failed, a round requesting changes, a revision and an approving round,
// to the merge gate, where no role runs. Its workspace, removed whole,
// is built again by the next pass, file for file; `status` before it
// writes none. Removed but for texts/, as a job the R1-1 build collected
// holds it, it is completed the same way, nothing in texts/ duplicated;
// a replay that cannot read an out.md blocks the issue and writes no
// built marker, which it writes last, and once the file is readable and
// the block removed, the pass after it completes the workspace.
func TestWorkspaceReplay(t *testing.T) {
	plan := "## Steps\n\n1. app: change main.go to v1; the test proves it.\n2. app: add notes.md.\n\n## Test Plan\n\n1. main.go says v1.\n"
	builder := "## Slice 1\n\nEdit main.go.\n\n## Slice 2\n\nWrite notes.md.\n"
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
	a.runner.Script(1, "architect", runner.Canned{Out: plan, IO: builderPlan(builder)})
	a.pass()
	a.forge.SetLabel(issue1, forge.PlanAccepted)
	a.runner.Script(1, "worker", runner.Canned{Out: "QUESTION: v1 or v1.0?\n"})
	a.pass()
	a.runner.Script(1, "architect", runner.Canned{Out: "v1.\n"})
	a.pass()
	a.runner.Script(1, "worker", work("v1"))
	a.runner.Script(1, "check", runner.Canned{Code: 1, Stdout: "FAIL main_test.go\n"})
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve}, runner.Canned{Out: approve})
	a.pass()
	a.runner.Script(1, "worker", work("v2"))
	a.runner.Script(1, "check", runner.Canned{Stdout: "ok\n"})
	a.pass()
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", "swarm:gated"}) {
		t.Fatalf("the job did not reach the merge gate: %v\n%s", got, a.stdout.String())
	}
	collected := a.tree(1)
	for _, f := range []string{"plan.md", "builder.md", "slices/1/slice.md", "slices/2/slice.md", "slices/1/kept", "slices/1/rounds/1.md", "slices/1/rounds/2.md",
		"texts/1-architect-1.md", "texts/1-worker-1.md", "texts/1-architect-2.md", "texts/1-worker-2.md", "texts/1-adversary-1.md", "texts/1-sympathetic-1.md",
		"texts/1-worker-3.md", "texts/1-adversary-2.md", "texts/1-sympathetic-2.md"} {
		if _, ok := collected[f]; !ok {
			t.Fatalf("the collections wrote no %s: %q", f, a.texts(1))
		}
	}
	if len(collected) != 17 || !a.built(1) {
		t.Fatalf("the collections' workspace (built %v): %d files\n%v", a.built(1), len(collected), collected)
	}
	job := a.state("jobs/1")
	same := func(when string) {
		t.Helper()
		got := a.tree(1)
		for f, want := range collected {
			if got[f] != want {
				t.Errorf("%s: %s differs from the collection's:\n%s\nwant:\n%s", when, f, got[f], want)
			}
		}
		for f := range got {
			if _, ok := collected[f]; !ok {
				t.Errorf("%s: %s, which no collection wrote", when, f)
			}
		}
		if !a.built(1) {
			t.Errorf("%s: no built marker", when)
		}
	}

	if err := os.RemoveAll(job); err != nil {
		t.Fatal(err)
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(job); !errors.Is(err, iofs.ErrNotExist) {
		t.Fatalf("status wrote the workspace: %v", err)
	}
	a.pass()
	same("rebuilt whole")

	for f := range collected {
		if !strings.HasPrefix(f, "texts/") {
			os.Remove(filepath.Join(job, f))
		}
	}
	os.Remove(filepath.Join(job, "built"))
	out := a.state("runs/1-worker-3/io/out.md")
	if err := os.Chmod(out, 0o000); err != nil {
		t.Fatal(err)
	}
	a.pass()
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || a.built(1) {
		t.Fatalf("a replay that cannot read an out.md: %v, built %v\n%s", got, a.built(1), a.stdout.String())
	}
	os.Chmod(out, 0o644)
	a.forge.Relabel(issue1, forge.Blocked, "")
	a.pass()
	same("completed over texts/")
}

// A worker run that faulted at its check (exit 125: the check could not
// run) keeps its commits and writes no checks/ file, its collection
// ending at the fault before step 4; its replay writes the same.
func TestWorkspaceReplayFaultedCheck(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.runner.Script(1, "check", runner.Canned{Code: 125})
	a.pass()
	if r := a.result("1-worker-1"); !strings.Contains(r, "\noutcome fault\n") {
		t.Fatalf("the run did not fault:\n%s", r)
	}
	collected := a.tree(1)
	if _, ok := collected["slices/1/kept"]; !ok {
		t.Fatalf("the collection kept nothing: %v", collected)
	}
	os.RemoveAll(a.state("jobs/1"))
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.pass()
	if got := a.tree(1); !reflect.DeepEqual(got, collected) || !a.built(1) {
		t.Errorf("replayed (built %v):\n%v\nthe collection's:\n%v", a.built(1), got, collected)
	}
}

// A plan taken cut from its post (section 7.8, Rollout): the plan's
// out.md gone and the job without a workspace, the replay writes the
// plan and its text as the forge holds them, the cut's note replaced by
// one that says the rest is not available, and blocks the issue beside
// its stage, the reconciler's note naming the missing file, rather than
// count its slices short. Another pass posts no second note. A workspace
// whose texts/ holds the plan whole, as a collection since R1-1 wrote
// it, is completed from it: the whole plan, its text left as it is, and
// no block.
func TestWorkspaceReplayCutPlan(t *testing.T) {
	t.Run("texts/ holds it whole", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
		a.runner.Script(1, "architect", runner.Canned{Out: bigPlan})
		a.pass()
		if err := os.Remove(a.state("runs/1-architect-1/io/out.md")); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"plan.md", "slices", "built"} {
			if err := os.RemoveAll(a.state("jobs/1/" + f)); err != nil {
				t.Fatal(err)
			}
		}
		a.pass()
		for _, f := range []string{"plan.md", "texts/1-architect-1.md"} {
			if got, ok := a.jobFile(1, f); !ok || got != bigPlan {
				t.Errorf("%s (%v) ends:\n%s", f, ok, got[max(0, len(got)-200):])
			}
		}
		if got := labels(a.issue(1)); slices.Contains(got, "swarm:blocked") || !a.built(1) {
			t.Errorf("after the replay: %v, built %v", got, a.built(1))
		}
	})

	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
	a.runner.Script(1, "architect", runner.Canned{Out: bigPlan})
	a.pass()
	posted := a.issue(1).Comments[0].Body
	cut := strings.LastIndex(posted, "\n[truncated by Daisy; ")
	if cut < 0 {
		t.Fatalf("the plan was not cut on the forge:\n%s", posted[max(0, len(posted)-300):])
	}
	want := strings.TrimPrefix(posted[:cut], mark("architect", 1)) + "\n[truncated by Daisy; the rest is not available to you]"
	if err := os.Remove(a.state("runs/1-architect-1/io/out.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(a.state("jobs/1")); err != nil {
		t.Fatal(err)
	}
	a.pass()
	for _, f := range []string{"plan.md", "texts/1-architect-1.md"} {
		if got, ok := a.jobFile(1, f); !ok || got != want {
			t.Errorf("%s (%v) ends:\n%s", f, ok, got[max(0, len(got)-200):])
		}
	}
	if got, ok := a.jobFile(1, "slices/1/slice.md"); !ok || got != "1. app: change main.go.\n" {
		t.Errorf("slices/1/slice.md (%v): %q", ok, got)
	}
	note := mark("reconciler", 1) + "The plan of 1-architect-1 is cut on the forge and runs/1-architect-1/io/out.md is gone, so its slices cannot be counted (docs/swarm.md section 7.8, Rollout). Restore the file and remove <state>/jobs/1, or relabel the issue swarm:proposal-accepted for a new plan; then remove swarm:blocked to resume.\n\ncc @jane"
	is := a.issue(1)
	if got := labels(is); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", "swarm:blocked", "swarm:gated"}) || !a.built(1) {
		t.Errorf("after the replay: %v, built %v\n%s", got, a.built(1), a.stdout.String())
	}
	if bs := a.bodies(1); len(bs) != 2 || bs[1] != note {
		t.Errorf("the thread's last: %q", bs[len(bs)-1])
	}
	os.Remove(a.state("jobs/1/built"))
	a.pass()
	if bs := a.bodies(1); len(bs) != 2 {
		t.Errorf("a second replay posted again: %d comments", len(bs))
	}
}

// unusableNote is the reconciler's note that blocks issue n, marked with
// run k, for its file Daisy cannot use (section 7.7, a job's file Daisy
// cannot use).
func unusableNote(n, k int, file, cause string) string {
	return mark("reconciler", k) + "Daisy cannot use " + file + ": " + cause + " (docs/swarm.md section 7.8, Rollout). Make the file readable, or remove it if it is an out.md, its run's text then taken from its post, or under jobs/" + strconv.Itoa(n) + "/, which the replay writes again; then remove swarm:blocked to resume.\n\ncc @jane"
}

// The claim of Step R1-4b (sections 7.7 and 7.8, Rollout; the
// maintainer's rule of 2026-11-18): an out.md that cannot be read blocks
// its issue alone. Two issues wait for their worker; the first's plan,
// cut on the forge, has an out.md that cannot be read. Read by the
// replay of a job without its workspace, it blocks the first issue with
// the reconciler's note naming the file, writes no built marker,
// launches nothing for it, and the pass goes on: the second issue's
// worker is launched and apply exits 0. The next pass posts no second
// note. So does a workspace directory that cannot be read, its built
// marker's read failing. For a job built, nothing reads that out.md any
// more, the plan being in plan.md and texts/ (Step R1-10b of the plan
// implementing the 2026-11-19 swarm roadmap): its worker is launched.
func TestUnreadableOutputBlocksItsIssueAlone(t *testing.T) {
	for _, c := range []struct {
		name         string
		replay       bool
		closed, file string // closed made unreadable, the file the note names
	}{
		{"the replay", true, "runs/1-architect-1/io/out.md", "runs/1-architect-1/io/out.md"},
		{"a built job's out.md, read by nothing", false, "runs/1-architect-1/io/out.md", ""},
		{"the workspace", false, "jobs/1", "jobs/1/built"},
	} {
		replay := c.replay
		t.Run(c.name, func(t *testing.T) {
			a := newApplyWS(t)
			issue2 := forge.Ref{Repo: issue1.Repo, N: 2}
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
			a.forge.CreateIssue("Notes", "Add notes.", []string{forge.ProposalAccepted})
			a.runner.Script(1, "architect", runner.Canned{Out: bigPlan})
			a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
			a.pass()
			a.forge.SetLabel(issue1, forge.PlanAccepted)
			a.forge.SetLabel(issue2, forge.PlanAccepted)
			a.runner.Script(1, "worker", work("v1"))
			a.runner.Script(2, "worker", work("v1"))
			if replay {
				os.RemoveAll(a.state("jobs/1"))
			}
			closed := a.state(c.closed)
			fi, err := os.Stat(closed)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(closed, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(closed, fi.Mode().Perm()) })
			before := len(a.bodies(1))
			a.pass()
			if c.file == "" {
				if got := labels(a.issue(1)); slices.Contains(got, forge.Blocked) || !strings.Contains(a.result("1-worker-1"), "\ncollected\n") || len(a.bodies(1)) != before {
					t.Errorf("#1: %v, its worker:\n%s", got, a.result("1-worker-1"))
				}
				return
			}
			note := unusableNote(1, 1, c.file, "permission denied")
			if bs := a.bodies(1); len(bs) != before+1 || bs[len(bs)-1] != note {
				t.Errorf("the thread's last:\n%q\nwant:\n%q", bs[len(bs)-1], note)
			}
			if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || (c.closed != "jobs/1" && a.built(1) != !replay) {
				t.Errorf("#1: %v, built %v\n%s", got, a.built(1), a.stdout.String())
			}
			if a.result("1-worker-1") != "" {
				t.Errorf("#1's worker was launched:\n%s", a.result("1-worker-1"))
			}
			if !strings.Contains(a.result("2-worker-1"), "\ncollected\n") {
				t.Errorf("#2's worker was not launched:\n%s", a.stdout.String())
			}
			a.pass()
			if bs := a.bodies(1); len(bs) != before+1 {
				t.Errorf("a second pass posted again: %q", bs[before:])
			}
		})
	}
}

// A worker's report posted with no check result, its repository then
// declaring none, cannot be replayed once the repository declares one
// (section 7.8, Rollout): the issue is blocked with the reconciler's
// note naming the run's out.md and the condition, and no built marker.
func TestReplayOfAReportWithNoCheckBlocks(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Repos[0].Check = nil })
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) {
		t.Fatalf("the worker's report was not posted: %v\n%s", got, a.stdout.String())
	}
	a.edit(func(c *declare.Config) { c.Repos[0].Check = []string{"make", "check"} })
	os.RemoveAll(a.state("jobs/1"))
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.pass()
	note := unusableNote(1, 1, "runs/1-worker-1/io/out.md", "its run records no check result, which its repository now declares")
	if bs := a.bodies(1); bs[len(bs)-1] != note {
		t.Errorf("the thread's last:\n%q\nwant:\n%q", bs[len(bs)-1], note)
	}
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || a.built(1) {
		t.Errorf("after the replay: %v, built %v", got, a.built(1))
	}
}

// refusesReviews is the git-following forge whose Reviews fails, as a
// forge that refuses a read does.
type refusesReviews struct{ gitForge }

func (refusesReviews) Reviews(forge.Ref) ([]forge.Review, error) {
	return nil, errors.New("503 Service Unavailable")
}

// A forge's error during a replay is not the job's (section 7.8,
// Rollout): it is the pass's error, as any per-issue forge read's is,
// the issue not blocked, no note posted and no built marker written, so
// the next pass replays the job again.
func TestReplayForgeErrorIsThePasss(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	os.RemoveAll(a.state("jobs/1"))
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	before := len(a.bodies(1))
	a.tick()
	if err := a.apply(refusesReviews{a.git()}, "--once"); err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Errorf("apply: %v", err)
	}
	if got := labels(a.issue(1)); slices.Contains(got, forge.Blocked) || len(a.bodies(1)) != before || a.built(1) {
		t.Errorf("after a forge error: %v, %d notes, built %v", got, len(a.bodies(1))-before, a.built(1))
	}
	a.pass()
	if !a.built(1) {
		t.Errorf("the next pass did not replay the job")
	}
}

// With the merge gate off, an approved job whose plan.md cannot be read
// is blocked by the merge step, which reads the plan from the job's
// workspace to count its slices (section 7.8, Daisy's own reads), with
// the note naming the file, and nothing merged; apply exits 0 (section
// 7.7).
func TestMergeStepUnreadablePlanBlocks(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(bigSlices)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	if got := labels(a.issue(1)); got[0] != forge.ReviewApproved {
		t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	plan := a.state("jobs/1/plan.md")
	if err := os.Chmod(plan, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(plan, 0o644) })
	a.pass()
	note := unusableNote(1, 1, "jobs/1/plan.md", "permission denied")
	if bs := a.bodies(1); bs[len(bs)-1] != note {
		t.Errorf("the thread's last:\n%q\nwant:\n%q", bs[len(bs)-1], note)
	}
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || a.pulls(1)[0].Merged {
		t.Errorf("after the merge step: %v, merged %v\n%s", got, a.pulls(1)[0].Merged, a.stdout.String())
	}
}

// swarm merge refuses a job with no built workspace whose plan, as the
// replay builds it, is taken cut from its post, its out.md gone (section
// 11, From R1; section 7.8, Rollout): exit 1 with the spec's text before
// any write, nothing changed on the forge or on disk; so does a built
// job whose plan.md is not there, its plan built in memory the same way
// (section 7.8, Daisy's own reads). With the workspace built, the verb
// counts the slices of its plan.md.
func TestSwarmMergeRefusesACutPlanWithoutWorkspace(t *testing.T) {
	for _, c := range []struct {
		name        string
		built, plan bool
	}{{"no workspace", false, false}, {"built, no plan.md", true, false}, {"built", true, true}} {
		built := c.built
		t.Run(c.name, func(t *testing.T) {
			m := newMaintainerWS(t)
			m.forge.CreateIssue("A", mark("owner", 1)+"a", []string{"swarm:proposal-posted"})
			cut := bigSlices[:strings.LastIndex(bigSlices[:59000], "\n")] + "\n" + cutNote("runs/1-architect-1")
			m.forge.Comment(m.ref(1), mark("architect", 1)+cut)
			pr := m.approvedPull(1, "c1")
			if built {
				os.MkdirAll(m.state("jobs/1"), 0o755)
				os.WriteFile(m.state("jobs/1/built"), nil, 0o644)
			}
			if c.plan {
				os.WriteFile(m.state("jobs/1/plan.md"), []byte(bigSlices), 0o644)
				m.ok(swarmMerge, "Maintainer jane on acme/swarm-app\n#1 merged pull app#1 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n#1 pull requests listed\n", "1")
				return
			}
			before := m.writesAt(m.ref(1), pr)
			err := m.verb(swarmMerge, "1")
			want := "Error: swarm: #1: the plan of 1-architect-1 is cut on the forge and runs/1-architect-1/io/out.md is gone, so its slices cannot be counted (docs/swarm.md section 7.8, Rollout); nothing changed"
			if err == nil || err.Error() != want {
				t.Errorf("merge: %v\nwant: %s", err, want)
			}
			if after := m.writesAt(m.ref(1), pr); !reflect.DeepEqual(after, before) {
				t.Errorf("writes on the forge:\n%q\nbefore:\n%q", after, before)
			}
			if p, _ := m.forge.Pulls(1, []string{"swarm-bot"}); len(p) != 1 || p[0].Merged {
				t.Errorf("merged: %+v", p)
			}
			if _, err := os.Stat(m.state("jobs")); !built && !errors.Is(err, iofs.ErrNotExist) {
				t.Errorf("the verb wrote the workspace: %v", err)
			}
		})
	}
}
