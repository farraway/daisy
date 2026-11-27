package cmd

import (
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

// approvedHead takes issue 1 through the pair's approval of its one
// slice, gate off when gateOff, and returns the approved head; its merge
// check has not run.
func (a *applyWS) approvedHead(gateOff bool) string {
	a.t.Helper()
	if gateOff {
		a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	}
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary approves
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	if got := labels(a.issue(1)); !slices.Contains(got, forge.ReviewApproved) {
		a.t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	return a.pulls(1)[0].HeadSHA
}

// mainAt is the commit main is at in the upstream of app.
func (a *applyWS) mainAt() string {
	a.t.Helper()
	out, errb, err := ops.Real().Exec(a.upstream("app"), "git", "rev-parse", "main")
	if err != nil {
		a.t.Fatalf("rev-parse: %v %s", err, errb)
	}
	return strings.TrimSpace(string(out))
}

// merges is the merge checks launched, by their labels.
func (a *applyWS) merges() []runner.Spec {
	var out []runner.Spec
	for _, l := range a.runner.Launches() {
		if strings.HasSuffix(l.Label, "/merge") {
			out = append(out, l)
		}
	}
	return out
}

// The merge check runs the repository's check on the commit that merges
// the approved head into base as it is now, not on the head alone
// (section 12.1, rule 3): main moved since the branch took it in, the
// check's clone at /job/work/app is a merge commit whose parents are
// main's commit and the head, holding both sides' files, in the run
// 1-merge-1, its container named and labelled for the merge.
func TestSwarmMergeCheckRunsOnBasePlusHead(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedHead(true)
	a.land("app", map[string]string{"other.go": "other\n"}, "another slice merged")
	base := a.mainAt()
	var parents, files string
	a.runner.Script(1, "merge", runner.Canned{Work: func(work string) error {
		out, _, err := ops.Real().Exec(work, "git", "log", "-1", "--format=%P")
		parents = strings.TrimSpace(string(out))
		if err != nil {
			return err
		}
		out, _, err = ops.Real().Exec(work, "git", "ls-files")
		files = strings.TrimSpace(string(out))
		return err
	}})
	a.pass()
	ms := a.merges()
	if len(ms) != 1 || ms[0].Name != "daisy-swarm.acme.swarm-app.1.merge.1" || ms[0].Label != "acme/swarm-app#1/merge" || ms[0].Workdir != "/job/work/app" ||
		!strings.Contains(strings.Join(ms[0].Argv, " "), "make check") {
		t.Fatalf("the merge check's launch: %+v\n%s", ms, a.stdout.String())
	}
	if parents != base+" "+head || !strings.Contains("\n"+files+"\n", "\nother.go\n") || !strings.Contains("\n"+files+"\n", "\nmain.go\n") {
		t.Errorf("checked %q (want %s %s), files %q", parents, base, head, files)
	}
	res := a.result("1-merge-1")
	if !strings.HasPrefix(res, "merge app "+head+" into "+base+" at ") || !strings.Contains(res, "\npassed\ncollected\n") ||
		!strings.Contains(a.stdout.String(), "#1 merge check of pull app#1: "+head[:7]+" merged into main at "+base[:7]+"\n") {
		t.Errorf("the run's result:\n%s\n%s", res, a.stdout.String())
	}
}

// The merge step merges no head without a passed merge check on base as
// it is now (section 12.1, rule 3): the pass after the approval waits,
// no merge asked of the forge, and starts the check; the pass after the
// check passed merges.
func TestSwarmMergeQueueMergesOnlyACheckedHead(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedHead(true)
	base := a.mainAt()
	a.pass()
	waits := "#1 merge of pull app#1 waits: its merge check on main at " + base[:7] + " has not passed (queued); the next pass tries again\n"
	if a.pulls(1)[0].Merged || !strings.Contains(a.stdout.String(), waits) || len(a.merges()) != 1 {
		t.Fatalf("merged before its check: %v\n%s", a.pulls(1)[0].Merged, a.stdout.String())
	}
	if !strings.Contains(a.result("1-merge-1"), "\npassed\n") {
		t.Fatalf("the check:\n%s", a.result("1-merge-1"))
	}
	a.pass()
	if ps := a.pulls(1); !ps[0].Merged || ps[0].HeadSHA != head || a.issue(1).Open || len(a.merges()) != 1 ||
		!strings.Contains(a.stdout.String(), "#1 merged pull app#1 (slice 1 of 1)\n") {
		t.Errorf("not merged once checked: %+v\n%s", ps, a.stdout.String())
	}
}

// A failed merge check goes back to the worker with its log (T32,
// section 12.1, rule 3): its output in the job's workspace as
// checks/<merge sha>.log, the reconciler's note on the pull request
// naming it, the issue at review-3-changes-requested, no block; the
// worker's next run reads the note, and nothing is merged.
func TestSwarmMergeCheckFailedBackToTheWorker(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedHead(true)
	base := a.mainAt()
	a.runner.Script(1, "merge", runner.Canned{Code: 2, Stdout: "main.go:1: undefined: other\n"})
	a.pass()
	res := a.result("1-merge-1")
	f := strings.Fields(strings.SplitN(res, "\n", 2)[0])
	sha := f[len(f)-1]
	note := mark("reconciler", 1) + "The merge check on pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) failed: its approved head " + head[:7] + " merged into main at " + base[:7] +
		" fails `make check` (exit 2). The output is /job/memory/checks/" + sha + ".log. Merge main into swarm/1-1, fix what fails and commit; the reviewers then review the new head."
	comments, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ChangesRequested}) || len(comments) == 0 || comments[len(comments)-1].Body != note ||
		!strings.Contains(res, "\nfailed: merge check: exit 2\ncollected\n") || a.pulls(1)[0].Merged {
		t.Fatalf("T32: %v %+v\n%s\n%s", got, comments, res, a.stdout.String())
	}
	log, err := os.ReadFile(a.state(filepath.Join("jobs", "1", "checks", sha+".log")))
	if err != nil || !strings.Contains(string(log), "main.go:1: undefined: other") || !strings.HasPrefix(string(log), "make check on "+sha+": exit 2 ("+head+" merged into main at "+base+")\n") {
		t.Errorf("the log: %q %v", log, err)
	}
	a.runner.Script(1, "worker", work("v2"))
	a.pass() // the worker: T20, then T12
	if prompt := a.file("1-worker-2", "io/prompt.md"); !strings.Contains(prompt, "The merge check on pull request") {
		t.Errorf("the worker's prompt:\n%s", prompt)
	}
}

// `swarm merge`, the merge gate on, refuses a head without a passed
// merge check on base as the mirror's last fetch left it, nothing
// changed (section 12.1, rule 3); once the queue's check has passed, it
// merges. Run as the comment command, which answers with the verb's
// words.
func TestSwarmMaintainerMergeWaitsForTheMergeCheck(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.maintainer()
	a.approvedHead(false)
	base := a.mainAt()
	pull := forge.Ref{Repo: "app", N: 1}
	id := a.say(pull, "jane", "/swarm merge")
	a.pass() // the command runs before the queue's check starts
	want := answer(id, "/swarm merge", "Refused:\n\n    Maintainer jane on acme/swarm-app\n    Error: swarm: #1: merge of pull app#1 waits: its merge check on main at "+base[:7]+" has not passed (queued); run merge again when it has; nothing changed\n")
	if got := a.answers(pull); len(got) != 1 || !strings.HasPrefix(got[0], want) || a.pulls(1)[0].Merged || a.janes(issue1) != nil {
		t.Fatalf("answers %q\n%s", got, a.stdout.String())
	}
	if !strings.Contains(a.result("1-merge-1"), "\npassed\n") {
		t.Fatalf("the check:\n%s", a.result("1-merge-1"))
	}
	a.say(pull, "jane", "/swarm merge")
	a.pass()
	if !a.pulls(1)[0].Merged || a.issue(1).Open {
		t.Errorf("not merged once checked: %v\n%s", labels(a.issue(1)), a.stdout.String())
	}
}

// When base moves, a passed merge check on the old commit is stale
// (section 12.1, rule 3): the queued head waits again and is checked
// again on base as it is now, then merged.
func TestSwarmQueuedHeadRecheckedWhenBaseMoves(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedHead(true)
	first := a.mainAt()
	a.pass() // the check on main at first, passed
	a.land("app", map[string]string{"other.go": "other\n"}, "another slice merged")
	moved := a.mainAt()
	a.pass()
	waits := "#1 merge of pull app#1 waits: its merge check on main at " + moved[:7] + " has not passed (queued); the next pass tries again\n"
	if a.pulls(1)[0].Merged || !strings.Contains(a.stdout.String(), waits) || len(a.merges()) != 2 ||
		!strings.HasPrefix(a.result("1-merge-1"), "merge app "+head+" into "+first+" ") || !strings.HasPrefix(a.result("1-merge-2"), "merge app "+head+" into "+moved+" ") {
		t.Fatalf("checked again: %d checks\n%s\n%s", len(a.merges()), a.result("1-merge-2"), a.stdout.String())
	}
	a.pass()
	if !a.pulls(1)[0].Merged || len(a.merges()) != 2 {
		t.Errorf("not merged once checked again:\n%s", a.stdout.String())
	}
}

// A required context that has not reported on the approved head holds
// the merge as a pending one does (section 12, The checks, folding in L346): main requires
// `ci / *`, the head reports only lint; once ci / test reports success,
// the head merges.
func TestSwarmMergeWaitsForAnUnreportedRequiredContext(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedGateOff()
	a.forge.RequireChecks("app", "main", "ci / *")
	a.forge.SetStatus("app", head, "lint", "success")
	a.pass()
	line := "#1 merge of pull app#1 waits: `ci / *` has not reported on " + head[:7] + ", and main requires it; the next pass tries again\n"
	if got := labels(a.issue(1)); a.pulls(1)[0].Merged || slices.Contains(got, forge.Blocked) || !strings.Contains(a.stdout.String(), line) {
		t.Fatalf("waiting: %v\n%s", got, a.stdout.String())
	}
	a.forge.SetStatus("app", head, "ci / test", "success")
	a.pass()
	if !a.pulls(1)[0].Merged || a.issue(1).Open {
		t.Errorf("not merged once reported:\n%s", a.stdout.String())
	}
}

// A head the mirror cannot read is blocked with a note and never merged
// unchecked (section 12.1, the maintainer's ruling 5 of 2026-11-21): the
// queue's merge of it on the mirror fails, the warning names git's error,
// the issue is blocked at review-2-approved with the reconciler's note,
// no merge check runs and nothing is merged.
func TestSwarmMergeCheckBlocksAnUnreadableHead(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.approvedHead(true)
	base := a.mainAt()
	a.tick()
	if err := a.apply(unknownHead{a.git()}, "--once"); err != nil {
		t.Fatal(err)
	}
	zero := strings.Repeat("0", 40)
	note := mark("reconciler", 1) + "The merge check of pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) cannot run: the mirror of app cannot merge its approved head 0000000 into main at " + base[:7] +
		" (at swarm:review-2-approved), so it is not merged unchecked (docs/swarm.md section 12.1, The merge queue). Push the head again, or relabel the issue; then remove swarm:blocked to resume.\n\ncc @jane"
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || !slices.Contains(got, forge.ReviewApproved) || !slices.Contains(a.bodies(1), note) ||
		a.pulls(1)[0].Merged || len(a.merges()) != 0 || !strings.Contains(a.stderr.String(), "⚠ #1: pull app#1's head "+zero[:7]+" not merged on the mirror (git merge-tree ") {
		t.Errorf("%v %q\n%s\n%s", got, a.bodies(1), a.stdout.String(), a.stderr.String())
	}
}

// A conflict the queue's merge finds is T25 at once (section 12.1, rule
// 3), the merge gate on or off: the direction naming the file on the
// pull request, the issue at review-3-changes-requested, no merge check
// started and nothing asked of the forge.
func TestSwarmMergeCheckConflictBackToTheWorker(t *testing.T) {
	t.Parallel()
	for _, gateOff := range []bool{false, true} {
		a := newApplyWS(t)
		head := a.approvedHead(gateOff)
		a.land("app", map[string]string{"main.go": "other\n"}, "another slice merged")
		a.pass()
		comments, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ChangesRequested}) || len(comments) == 0 || comments[len(comments)-1].Body != mark("reconciler", 1)+conflictIn(head, "`main.go`") ||
			len(a.merges()) != 0 || a.pulls(1)[0].Merged {
			t.Errorf("gate off %v: T25: %v %+v %d checks\n%s", gateOff, got, comments, len(a.merges()), a.stdout.String())
		}
	}
}

// A merge check that fails in the environment is run again once (section
// 7.9) and, failing in the environment again, blocks the issue at
// review-2-approved with a note naming the reason, nothing posted on the
// pull request and no checks/ file written (T17).
func TestSwarmMergeCheckEnvironmentBlocks(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.goApp()
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
	a.pass()
	a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
	a.runner.Script(1, "worker", storeWork("v1"))
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the worker
	a.pass() // the adversary approves
	a.pass() // the sympathetic reviewer: T9
	a.runner.Script(1, "merge", runner.Canned{Code: 1, Stdout: webFailed}, runner.Canned{Code: 1, Stdout: webFailed})
	a.pass()
	res := a.result("1-merge-1")
	comments, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
	got := labels(a.issue(1))
	if len(a.merges()) != 2 || !slices.Contains(got, forge.Blocked) || !slices.Contains(got, forge.ReviewApproved) || a.pulls(1)[0].Merged ||
		!strings.Contains(res, "\ncheck rerun: untouched packages example.com/app/web\n") || !strings.Contains(res, "\nfailed: check environment: untouched packages example.com/app/web\ncollected\n") {
		t.Fatalf("blocked: %v, %d checks\n%s\n%s", got, len(a.merges()), res, a.stdout.String())
	}
	for _, c := range comments {
		if strings.Contains(c.Body, "merge check") {
			t.Errorf("posted on the pull request: %q", c.Body)
		}
	}
	if note := a.bodies(1); !strings.Contains(note[len(note)-1], "failed in the check environment: untouched packages example.com/app/web") {
		t.Errorf("the issue's notes: %q", note)
	}
	a.noChecks()
	a.pass()
	if len(a.merges()) != 2 {
		t.Errorf("a blocked issue's head checked again: %d checks", len(a.merges()))
	}
}

// Each repository's queue is serial and a merge ends its repository's
// merges for the pass (section 12.1, rule 3): with the gate on, the two
// approved heads are checked one at a time, the earlier pair's first, a
// passed head holding no other; with the gate off, both passed, the
// merge step merges one head a pass.
func TestSwarmMergeQueueSerialOneMergePerPass(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxMembers = 4 })
	for _, n := range []int{1, 2} {
		a.forge.CreateIssue("Queue "+strconv.Itoa(n), "Check the queue.", []string{"swarm:proposal-accepted"})
		a.runner.Script(n, "architect", runner.Canned{Out: oneSlice})
		a.runner.Script(n, "worker", work("v"+strconv.Itoa(n)))
		a.runner.Script(n, "adversary", runner.Canned{Out: approve})
		a.runner.Script(n, "sympathetic", runner.Canned{Out: approve})
	}
	a.pass() // the architects
	for _, n := range []int{1, 2} {
		a.forge.SetLabel(a.ref(n), "swarm:plan-1-accepted")
	}
	a.pass() // the workers
	a.pass() // the adversaries, approving
	a.pass() // the sympathetic reviewers: T9
	a.pass() // the queue's first check
	if ms := a.merges(); len(ms) != 1 || ms[0].Label != "acme/swarm-app#1/merge" {
		t.Fatalf("after one pass: %+v\n%s", ms, a.stdout.String())
	}
	a.pass() // the next: the first head passed and holds nothing
	if ms := a.merges(); len(ms) != 2 || ms[1].Label != "acme/swarm-app#2/merge" {
		t.Fatalf("after two passes: %+v\n%s", ms, a.stdout.String())
	}
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.pass()
	if !a.pulls(1)[0].Merged || a.pulls(2)[0].Merged || len(a.merges()) != 2 {
		t.Fatalf("one merge a pass: %v %v, %d checks\n%s", a.pulls(1)[0].Merged, a.pulls(2)[0].Merged, len(a.merges()), a.stdout.String())
	}
	a.pass()
	if !a.pulls(2)[0].Merged {
		t.Errorf("the second not merged a pass later:\n%s", a.stdout.String())
	}
}

// A merge check that failed on a base that has since moved is checked
// again on base as it is now before the head goes back to the worker
// (section 12.1, rule 3; the maintainer's ruling of 2026-11-27 on
// R2-B's open question): main moves while the check runs, the failure
// is recorded stale, nothing is posted and the issue stays approved; the
// next pass checks it again on the moved main, and once that passes it
// merges.
func TestSwarmMergeCheckFailedOnAMovedBaseCheckedAgain(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	head := a.approvedHead(true)
	a.runner.Script(1, "merge", runner.Canned{Code: 2, Stdout: "main.go:1: undefined: other\n", Work: func(string) error {
		a.land("app", map[string]string{"other.go": "other\n"}, "another slice merged during the check")
		return nil
	}})
	a.pass()
	moved := a.mainAt()
	comments, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
	if got := labels(a.issue(1)); !slices.Contains(got, forge.ReviewApproved) || len(comments) != 0 ||
		!strings.Contains(a.result("1-merge-1"), "\nstale "+moved+"\ncollected\n") ||
		!strings.Contains(a.stdout.String(), "#1 merge run 1: merge check failed (exit 2) on main at ") {
		t.Fatalf("sent back on a moved main: %v %+v\n%s\n%s", got, comments, a.result("1-merge-1"), a.stdout.String())
	}
	a.pass()
	if !strings.HasPrefix(a.result("1-merge-2"), "merge app "+head+" into "+moved+" ") || !strings.Contains(a.result("1-merge-2"), "\npassed\n") {
		t.Fatalf("not checked again on the moved main:\n%s\n%s", a.result("1-merge-2"), a.stdout.String())
	}
	a.pass()
	if !a.pulls(1)[0].Merged || a.issue(1).Open {
		t.Errorf("not merged once checked again:\n%s", a.stdout.String())
	}
}
