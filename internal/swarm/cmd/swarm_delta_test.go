package cmd

import (
	"strings"
	"testing"

	"daisy/internal/swarm/runner"
)

// Step E2's claim, the delta review (the plan implementing the
// 2026-11-19 swarm roadmap; section 7.4; decision 168): a reviewer's
// round 1 reviews the whole change, "git diff main...HEAD", as before;
// its round 2, after the worker's revision, is given the previous
// round's findings, its file in the job's workspace, and the change
// since the head it last reviewed, its own, not the full range, which
// stays readable; a reviewer whose first review falls in round 2 (the
// sympathetic reviewer, after the adversary alone sent round 1 back)
// has no head of its own to start from, and reviews the whole change.
func TestSwarmDeltaReview(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	first := a.pulls(1)[0].HeadSHA
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // round 1: the adversary requests changes, T10
	a.runner.Script(1, "worker", work("v2"))
	a.pass() // T12
	a.pass() // round 2: the adversary
	a.pass() // round 2: the sympathetic reviewer, once the adversary approved
	full := `Review pull request app#1, slice 1 of issue #1. The change is "git diff main...HEAD" in /job/work/app, a throwaway clone`
	one := a.file("1-adversary-1", "io/system.md")
	if !strings.Contains(one, full) || strings.Contains(one, "This is round") {
		t.Errorf("round 1's Task:\n%s", one)
	}
	two := a.file("1-adversary-2", "io/system.md")
	delta := `This is round 2 of pull request app#1, slice 1 of issue #1: review what changed since ` + first + `, the head you last reviewed. The change to review is "git diff --name-only -z main...HEAD | xargs -0 -r git diff ` + first + `..HEAD --" in /job/work/app`
	findings := `First, for each blocking finding of the previous round, /job/memory/slices/1/rounds/1.md, and of your review of ` + first + `, say whether it is closed`
	if !strings.Contains(two, delta) || !strings.Contains(two, findings) || !strings.Contains(two, `The whole change, "git diff main...HEAD", is there to read`) || strings.Contains(two, full) {
		t.Errorf("round 2's Task:\n%s", two)
	}
	if _, ok := a.jobFile(1, "slices/1/rounds/1.md"); !ok {
		t.Error("the round's file the Task names is not in the job's workspace")
	}
	sym := a.file("1-sympathetic-1", "io/system.md")
	if !strings.Contains(sym, full) || strings.Contains(sym, "This is round") {
		t.Errorf("the sympathetic reviewer's first review, in round 2:\n%s", sym)
	}
}
