package commands

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// The maintainer's word lifts a block (docs/swarm.md section 6, The
// maintainer's word): the reconciler's note and the gate note after a
// worker's BLOCKED: wake no one, nor does an untrusted login's comment;
// the maintainer's plain comment on the issue clears swarm:blocked and
// the worker runs in the same pass, reading it under # Direction.
func TestSwarmApplyBlockAnswered(t *testing.T) {
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", runner.Canned{Out: "BLOCKED: v1 or v2?\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanAccepted, forge.Blocked, forge.Gated}) {
		t.Fatalf("not blocked: %v\n%s", got, a.stdout.String())
	}
	a.pass()
	a.say(issue1, "mallory", "Use v1.")
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker"}) || !reflect.DeepEqual(labels(a.issue(1)), []string{forge.PlanAccepted, forge.Blocked, forge.Gated}) {
		t.Fatalf("the swarm's notes or mallory woke %q: %v\n%s", got, labels(a.issue(1)), a.stdout.String())
	}

	a.say(issue1, "jane", "Use v1: the plan's Test Plan says so.")
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "worker", "check"}) {
		t.Fatalf("after jane's answer launched %q\n%s", got, a.stdout.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady}) {
		t.Errorf("labels %v", got)
	}
	if out := a.stdout.String(); !strings.Contains(out, "#1 block answered by jane at 2026-11-06T") || !strings.Contains(out, "\n#1 blocked cleared\n") {
		t.Errorf("stdout:\n%s", out)
	}
	if _, direction, _ := strings.Cut(a.file("1-worker-2", "io/prompt.md"), "\n# Direction\n\n"); !strings.Contains(direction, "Use v1: the plan's Test Plan says so.") || strings.Contains(direction, "mallory") && !strings.Contains(direction, "content omitted") {
		t.Errorf("worker run 2's # Direction:\n%s", direction)
	}
}

// The maintainer's word in review (section 6, The maintainer's word): a
// reviewer's failure blocks the issue at review-0-ready, the
// sympathetic's approval standing; the maintainer's comment on the issue,
// which neither reviewer reads, lifts nothing; one on the pull request
// lifts the block and runs both reviewers again, the standing approval
// counting no more, so the pair completes on the two new reviews (T10),
// not on the old approval and the adversary's new one (T9); the
// worker's earlier rounds then say the maintainer's comment reopened
// the head.
func TestSwarmApplyReviewComment(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	pull := forge.Ref{Repo: "app", N: 1}
	a.runner.Script(1, "adversary", runner.Canned{Out: "VERDICT: approve\n\n[blocking] none.\n"})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady, forge.Blocked, forge.Gated}) {
		t.Fatalf("not blocked: %v\n%s", got, a.stdout.String())
	}
	a.say(issue1, "jane", "Retry it.")
	a.pass()
	if got := a.launchedRoles(); len(got) != 5 || !slices.Contains(labels(a.issue(1)), forge.Blocked) {
		t.Fatalf("the issue's comment lifted the block: launched %q, %v\n%s", got, labels(a.issue(1)), a.stdout.String())
	}

	a.say(pull, "jane", "The plan wants v1; review it against that.")
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: requestChanges})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary", "sympathetic", "adversary", "sympathetic"}) {
		t.Fatalf("after jane's pull request comment launched %q\n%s", got, a.stdout.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ChangesRequested}) {
		t.Errorf("labels %v: the pair completed on a review from before jane's comment\n%s", got, a.stdout.String())
	}
	for _, run := range []string{"1-adversary-2", "1-sympathetic-2"} {
		if _, thread, _ := strings.Cut(a.file(run, "io/prompt.md"), "\n## Thread\n\n"); !strings.Contains(thread, "The plan wants v1; review it against that.") {
			t.Errorf("%s's ## Thread:\n%s", run, thread)
		}
	}
	// The worker's earlier rounds name what reopened the head.
	head := a.pulls(1)[0].HeadSHA
	a.runner.Script(1, "worker", work("v2"))
	a.pass()
	if line := "- round on " + head + ": 0 blocking, reviewed again after the maintainer's comment\n"; !strings.Contains(a.file("1-worker-2", "io/prompt.md"), line) {
		t.Errorf("worker run 2's prompt lacks %q:\n%s", line, a.file("1-worker-2", "io/prompt.md"))
	}
}

// A question passed to the maintainer at a review stage (section 6, The
// question): the asker reads the issue's comments under # Questions, so
// the maintainer's answer on the issue lifts the block, and only the
// asker runs again, the sympathetic's approval standing, as unblock's
// comment would.
func TestSwarmApplyQuestionAnsweredInReview(t *testing.T) {
	a := newApplyWS(t)
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: "QUESTION: Is v1 the release's name or the file's?\n"})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.runner.Script(1, "architect", runner.Canned{Out: "QUESTION: Does the brief name v1?\n", Stdout: claudeResult})
	a.pass()
	a.runner.Script(1, "owner", runner.Canned{Out: "QUESTION: Release or file?\n", Stdout: claudeResult})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady, forge.Blocked, forge.Gated}) {
		t.Fatalf("not at the maintainer: %v\n%s", got, a.stdout.String())
	}
	a.say(issue1, "jane", "The file's.")
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary", "sympathetic", "architect", "owner", "adversary"}) {
		t.Fatalf("after jane's answer launched %q\n%s", got, a.stdout.String())
	}
	if _, questions, _ := strings.Cut(a.file("1-adversary-2", "io/prompt.md"), "\n# Questions\n\n"); !strings.Contains(questions, "The file's.") {
		t.Errorf("adversary run 2's # Questions:\n%s", questions)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewApproved, forge.Gated}) {
		t.Errorf("labels %v", got)
	}
}
