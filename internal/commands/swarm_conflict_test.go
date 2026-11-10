package commands

import (
	"os"
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

// land commits files to base (main) in the upstream of the declared
// repository name, as the merge of another pull request moves it.
func (a *applyWS) land(name string, files map[string]string, msg string) {
	a.t.Helper()
	src := a.t.TempDir()
	if _, errb, err := ops.Real().Exec("", "git", "clone", "-q", a.upstream(name), src); err != nil {
		a.t.Fatalf("clone: %v %s", err, errb)
	}
	for _, kv := range [][2]string{{"user.name", "T"}, {"user.email", "t@example.net"}} {
		if _, errb, err := ops.Real().Exec(src, "git", "config", kv[0], kv[1]); err != nil {
			a.t.Fatalf("config: %v %s", err, errb)
		}
	}
	if err := commit(files, msg)(src); err != nil {
		a.t.Fatal(err)
	}
	if _, errb, err := ops.Real().Exec(src, "git", "push", "-q", "origin", "main"); err != nil {
		a.t.Fatalf("push: %v %s", err, errb)
	}
}

// direction is the note `swarm merge`'s T25 posts on app#2, approved at
// head, for issue 1's slice 1 (section 12): the forge's report, which
// names no file.
func direction(head string) string {
	return "Pull request acme/app#2 was approved at " + head[:7] + ", but the forge reports it not mergeable into main, which has moved since swarm/1-1 last took it in. " +
		`Merge main into swarm/1-1 with a merge commit ("git merge --no-edit main"), resolve each conflict, keeping what both sides meant, and commit; the reviewers then review the new head.`
}

// conflictIn is the merge step's T25 note on app#2, approved at head,
// naming the files that conflict on the mirror (section 12).
func conflictIn(head, files string) string {
	return "Pull request acme/app#2 was approved at " + head[:7] + ", but it does not merge into main, which has moved since swarm/1-1 last took it in: they conflict in " + files + ". " +
		`Merge main into swarm/1-1 with a merge commit ("git merge --no-edit main"), resolve each conflict, keeping what both sides meant, and commit; the reviewers then review the new head.`
}

// notMergeable takes issue 1 through the pair's approval with the merge
// gate off, then lands a main.go on main that conflicts with the slice's
// and has the forge report app#2 not mergeable; the next pass's merge step
// is T25. It returns the approved head.
func (a *applyWS) notMergeable() string {
	a.t.Helper()
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	head := a.pulls(1)[0].HeadSHA
	a.land("app", map[string]string{"main.go": "other\n"}, "another slice merged")
	a.forge.SetMergeable(forge.Ref{Repo: "app", N: 2}, forge.MergeableNo)
	a.pass()
	return head
}

// T25 by the merge step: a pull request made unmergeable by a merge to
// base, which conflicts on the mirror too, goes back to the worker with
// the direction naming the file on its thread, no note on the issue and
// no block; the worker merges main, resolves the
// conflict and the merge is pushed; the pair reviews the new head (round
// 2) and the merge step merges it. The maintainer does nothing.
func TestSwarmApplyNotMergeableBackToTheWorker(t *testing.T) {
	a := newApplyWS(t)
	approved := a.notMergeable()
	pr := forge.Ref{Repo: "app", N: 2}
	note := mark("reconciler", 1) + conflictIn(approved, "`main.go`")
	comments, _ := a.forge.PullComments(pr)
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested"}) || len(a.bodies(1)) != 1 ||
		len(comments) == 0 || comments[len(comments)-1].Body != note || comments[len(comments)-1].Author != "swarm-bot" ||
		!strings.Contains(a.stdout.String(), "#1 not mergeable: back to the worker\n#1 swarm:review-2-approved → swarm:review-3-changes-requested\n") {
		t.Fatalf("T25: %v %q %q\n%s", got, a.bodies(1), comments, a.stdout.String())
	}

	a.runner.Script(1, "worker", runner.Canned{Out: "Merged main; main.go keeps both.\n\nNot established: none\n", Work: func(work string) error {
		if _, _, err := ops.Real().Exec(work, "git", "merge", "--no-edit", "main"); err == nil {
			t.Error("main merged without a conflict")
		}
		if err := os.WriteFile(work+"/main.go", []byte("v1\nother\n"), 0o644); err != nil {
			return err
		}
		for _, args := range [][]string{{"add", "main.go"}, {"commit", "-q", "--no-edit"}} {
			if _, errb, err := ops.Real().Exec(work, "git", args...); err != nil {
				return &execErr{args, string(errb)}
			}
		}
		return nil
	}})
	a.pass() // the worker: T20, then T12
	sys, prompt := a.file("1-worker-2", "io/system.md"), a.file("1-worker-2", "io/prompt.md")
	if !strings.Contains(sys, `The newest entries of pull request app#2's thread need an answer`) ||
		!strings.Contains(sys, `Before anything else, run "git merge --no-edit main"; if it conflicts, resolve each conflict`) || !strings.Contains(prompt, conflictIn(approved, "`main.go`")) {
		t.Errorf("the worker's files:\n%s\n%s", sys, prompt)
	}
	head := a.pulls(1)[0].HeadSHA
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) || head == approved {
		t.Fatalf("T12: %v, head %s\n%s", got, head, a.stdout.String())
	}
	if _, errb, err := ops.Real().Exec(a.upstream("app"), "git", "merge-base", "--is-ancestor", "main", head); err != nil {
		t.Errorf("the pushed head %s does not hold main: %v %s", head, err, errb)
	}

	a.forge.SetMergeable(pr, forge.MergeableYes) // the forge's check after the push
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair on the new head: T9
	if sys := a.file("1-adversary-2", "io/system.md"); !strings.Contains(sys, "This is round 2. First say, for each blocking finding of your review of "+approved) {
		t.Errorf("the adversary's second round:\n%s", sys)
	}
	a.pass() // the merge step: T15
	ps := a.pulls(1)
	if is := a.issue(1); is.Open || !ps[0].Merged || ps[0].HeadSHA != head || !strings.Contains(a.stdout.String(), "#1 merged pull app#2 (slice 1 of 1)\n#1 closed (last slice merged)\n") {
		t.Errorf("merged: %+v %+v\n%s", is, ps, a.stdout.String())
	}
	if evs := a.forge.Events(issue1); wroteAs(evs, "jane") {
		t.Errorf("the maintainer wrote: %v", evs)
	}
}

// wroteAs: any event of evs is by login.
func wroteAs(evs []forge.Event, login string) bool {
	for _, e := range evs {
		if e.Actor == login {
			return true
		}
	}
	return false
}

// T25's relabel refused: the next pass posts the direction once, the
// thread's last word already, and makes the relabel.
func TestSwarmApplyNotMergeableRetried(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	approved := a.pulls(1)[0].HeadSHA
	pr := forge.Ref{Repo: "app", N: 2}
	a.land("app", map[string]string{"main.go": "other\n"}, "another slice merged")
	a.forge.SetMergeable(pr, forge.MergeableNo)
	a.tick()
	if err := a.apply(&flaky{gitForge: a.git(), relabels: 1, to: "swarm:review-3-changes-requested"}, "--once"); err == nil {
		t.Fatal("the refused relabel was not the pass's error")
	}
	a.pass()
	n := 0
	comments, _ := a.forge.PullComments(pr)
	for _, c := range comments {
		if c.Body == mark("reconciler", 1)+conflictIn(approved, "`main.go`") {
			n++
		}
	}
	if got := labels(a.issue(1)); n != 1 || !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested"}) {
		t.Errorf("after the retry: %d directions, %v", n, got)
	}
}

// A worker that cannot resolve the conflict says so, naming the files: the
// existing BLOCKED: path, its text on the pull request and the issue
// blocked at review-3-changes-requested for the maintainer.
func TestSwarmApplyConflictBlocked(t *testing.T) {
	a := newApplyWS(t)
	a.notMergeable()
	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: main conflicts in main.go\n\nBoth sides rewrite main.go; which wins is the maintainer's call.\n"})
	a.pass()
	comments, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 2})
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested", "swarm:blocked", forge.Gated}) ||
		!strings.HasPrefix(comments[len(comments)-1].Body, mark("worker", 2)+"BLOCKED: main conflicts in main.go") ||
		a.bodies(1)[len(a.bodies(1))-1] != mark("reconciler", 2)+"worker run 2 is blocked (at swarm:review-3-changes-requested). Answer, then remove swarm:blocked to resume." {
		t.Errorf("blocked: %v %q %q\n%s", got, a.bodies(1), comments, a.stdout.String())
	}
}

// A refused merge whose mergeability the forge has not settled is tried
// again at the next pass, neither blocked nor sent back: one the forge
// reports unknown, and one read after the merge step merged into its
// repository in the same pass (Gitea recomputing it reads false).
func TestSwarmMergeStepUnsettled(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	a.forge.SetMergeable(forge.Ref{Repo: "app", N: 2}, forge.MergeableUnknown)
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved"}) || len(a.bodies(1)) != 1 ||
		!strings.Contains(a.stdout.String(), "#1 merge of pull app#2 refused (405 Please try again later); its mergeability is not settled, so the next pass tries again\n") {
		t.Fatalf("unknown: %v %q\n%s", got, a.bodies(1), a.stdout.String())
	}
	a.forge.SetMergeable(forge.Ref{Repo: "app", N: 2}, forge.MergeableYes)
	a.pass()
	if a.issue(1).Open {
		t.Fatalf("not merged once settled:\n%s", a.stdout.String())
	}

	// Two approved issues: #2's pull request is read after the step merged
	// #1's into the same repository.
	a = newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
		c.MaxMembers = 4 // both pairs in one pass
	})
	for _, n := range []int{1, 2} {
		a.forge.CreateIssue("Drift check "+strconv.Itoa(n), "Check the drift.", []string{"swarm:proposal-accepted"})
		a.runner.Script(n, "architect", runner.Canned{Out: oneSlice})
		a.runner.Script(n, "worker", work("v"+strconv.Itoa(n)))
		a.runner.Script(n, "adversary", runner.Canned{Out: approve})
		a.runner.Script(n, "sympathetic", runner.Canned{Out: approve})
	}
	a.pass() // the architects
	for _, n := range []int{1, 2} {
		a.forge.SetLabel(forge.Ref{Repo: "app", N: n}, "swarm:plan-1-accepted")
	}
	a.pass() // the workers
	a.pass() // the pairs: T9
	second := a.pulls(2)[0]
	a.forge.SetMergeable(forge.Ref{Repo: second.Repo, N: second.Number}, forge.MergeableNo)
	a.pass()
	if a.issue(1).Open || !reflect.DeepEqual(labels(a.issue(2)), []string{"swarm:review-2-approved"}) ||
		!strings.Contains(a.stdout.String(), "#2 merge of pull app#"+strconv.Itoa(second.Number)+" refused (405 Please try again later); its mergeability is not settled, so the next pass tries again\n") {
		t.Fatalf("the same pass: %v\n%s", labels(a.issue(2)), a.stdout.String())
	}
	a.land("app", map[string]string{"main.go": "other\n"}, "another slice merged") // the mirror's main as #1's merge moved the forge's
	a.pass()
	if got := labels(a.issue(2)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested"}) || !strings.Contains(a.stdout.String(), "#2 not mergeable: back to the worker\n") {
		t.Errorf("the next pass: %v\n%s", got, a.stdout.String())
	}
}

// A refused merge the forge reports not mergeable whose head merges into
// base on the mirror (Gitea's checker recomputing reads false): nothing
// posted, the issue left approved, and the next pass, the forge settled,
// merges it.
func TestSwarmMergeStepRecomputing(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	pr := forge.Ref{Repo: "app", N: 2}
	a.land("app", map[string]string{"other.go": "other\n"}, "another slice merged, touching no file of the slice")
	a.forge.SetMergeable(pr, forge.MergeableNo)
	a.pass()
	comments, _ := a.forge.PullComments(pr)
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved"}) || len(comments) != 0 || len(a.bodies(1)) != 1 ||
		!strings.Contains(a.stdout.String(), "#1 merge of pull app#2 refused (405 Please try again later); its head merges into main on the mirror, so the forge is taken to be recomputing and the next pass tries again\n") {
		t.Fatalf("recomputing: %v %q %q\n%s", got, comments, a.bodies(1), a.stdout.String())
	}
	a.forge.SetMergeable(pr, forge.MergeableYes)
	a.pass()
	if a.issue(1).Open {
		t.Fatalf("not merged once settled:\n%s", a.stdout.String())
	}
}

// T25's second time on one head (section 12): the worker sent back
// answers without a commit, the pair approves the same head again, and
// it still conflicts: the issue is blocked at review-2-approved with the
// reconciler's note naming the file, no second direction posted; the
// pass after posts nothing more.
func TestSwarmMergeStepSentBackTwice(t *testing.T) {
	a := newApplyWS(t)
	approved := a.notMergeable()
	pr := forge.Ref{Repo: "app", N: 2}
	a.runner.Script(1, "worker", runner.Canned{Out: "NO CHANGE: main.go already holds what main means.\n\nNot established: none\n"})
	a.pass() // the worker: T12 without a commit
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair on the same head: T9
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved"}) || a.pulls(1)[0].HeadSHA != approved {
		t.Fatalf("approved again: %v\n%s", got, a.stdout.String())
	}
	a.pass() // the merge step
	note := mark("reconciler", 2) + "Pull request acme/app#2 was sent back to merge main at " + approved[:7] + " once already, was approved again at that head, and still does not merge into main (they conflict in `main.go`) (at swarm:review-2-approved). Merge main into swarm/1-1, or relabel the issue, then remove swarm:blocked."
	comments, _ := a.forge.PullComments(pr)
	directions := 0
	for _, c := range comments {
		if strings.Contains(c.Body, "was approved at "+approved[:7]+", but ") {
			directions++
		}
	}
	b := a.bodies(1)
	if got := labels(a.issue(1)); !slices.Equal(got[:2], []string{"swarm:review-2-approved", "swarm:blocked"}) || b[len(b)-1] != note || directions != 1 ||
		!strings.Contains(a.stdout.String(), "#1 not mergeable a second time at "+approved[:7]+": blocked\n") {
		t.Fatalf("the second time: %v %d directions\n%q\n%s", got, directions, b[len(b)-1], a.stdout.String())
	}
	a.pass()
	if len(a.bodies(1)) != len(b) {
		t.Errorf("posted again: %q", a.bodies(1)[len(b):])
	}
}

// unknownHead reads every review as on a commit no repository holds.
type unknownHead struct{ gitForge }

func (u unknownHead) Reviews(pr forge.Ref) ([]forge.Review, error) {
	rs, err := u.gitForge.Reviews(pr)
	for i := range rs {
		rs[i].Commit = strings.Repeat("0", 40)
	}
	return rs, err
}

// An approved head the mirror cannot read: the merge step warns and the
// forge's report stands, T25 with the direction naming no file.
func TestSwarmMergeStepMirrorCannotTell(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	pr := forge.Ref{Repo: "app", N: 2}
	a.forge.SetMergeable(pr, forge.MergeableNo)
	a.tick()
	if err := a.apply(unknownHead{a.git()}, "--once"); err != nil {
		t.Fatal(err)
	}
	zero := strings.Repeat("0", 40)
	comments, _ := a.forge.PullComments(pr)
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested"}) || len(comments) != 1 || comments[0].Body != mark("reconciler", 1)+direction(zero) ||
		!strings.Contains(a.stderr.String(), "⚠ #1: pull app#2's head 0000000 not merged on the mirror (git merge-tree ") {
		t.Errorf("%v %q\n%s", got, comments, a.stderr.String())
	}
}
