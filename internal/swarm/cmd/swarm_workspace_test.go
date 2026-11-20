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

	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: which version?\n", Work: commit(map[string]string{"main.go": "half\n"}, "Half")})
	a.pass()
	first := "1-worker-1 " + a.gitRunIn(mirror, "rev-parse", "refs/swarm-runs/1-worker-1") + "\n"
	if got, ok := a.jobFile(1, "slices/1/kept"); !ok || got != first {
		t.Fatalf("kept after the blocked run (%v): %q, want %q\n%s", ok, got, first, a.result("1-worker-1"))
	}

	for i, canned := range []runner.Canned{
		{Out: "Done.\n", Work: commit(map[string]string{"sub/key.txt": secret + "\n"}, "v1")},
		{Out: "Done.\n", Work: commit(map[string]string{"main.go": "v1\n", ".gitea/workflows/ci.yml": "on: push\n"}, "v1")},
		{Out: "BLOCKED: stuck.\n", Work: commit(map[string]string{"main.go": "token " + secret + "\n"}, "v1")},
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
	a.runner.Script(1, "worker", runner.Canned{Out: "v2.\n\nNot established: none\n", Work: commit(map[string]string{"main.go": "v2\n"}, "v2")})
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
