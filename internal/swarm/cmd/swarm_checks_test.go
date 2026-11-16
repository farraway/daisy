package cmd

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// approvedGateOff takes issue 1 through the pair's approval of its one
// slice with the merge gate off, and returns the approved head.
func (a *applyWS) approvedGateOff() string {
	a.t.Helper()
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	return a.pulls(1)[0].HeadSHA
}

// checksFailed is the merge step's note on app#1 whose approved head's
// checks failed (section 12, T26).
func checksFailed(head, contexts string) string {
	return "The checks on pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) failed at its approved head " + head[:7] + ": " + contexts +
		". Fix what they report on swarm/1-1 and commit; the reviewers then review the new head."
}

// A merge waits for the approved head's checks (section 12): a check
// pending, or none reported while main requires them, leaves the issue
// approved, unblocked and with nothing posted, a line saying why; the
// pass after the check succeeds merges it.
func TestSwarmMergeStepWaitsForChecks(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		report     func(a *applyWS, head string)
	}{
		{"a check pending", "its checks on %s are pending: `ci / test`", func(a *applyWS, head string) {
			a.forge.SetStatus("app", head, "ci / test", "pending")
			a.forge.SetStatus("app", head, "lint", "success")
		}},
		{"none reported while main requires them", "no check has reported on %s, and main requires them", func(*applyWS, string) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			head := a.approvedGateOff()
			a.forge.RequireChecks("app", "main")
			tc.report(a, head)
			bodies := len(a.bodies(1))
			a.pass()
			line := "#1 merge of pull app#1 waits: " + strings.Replace(tc.line, "%s", head[:7], 1) + "; the next pass tries again\n"
			if got := labels(a.issue(1)); slices.Contains(got, forge.Blocked) || !slices.Contains(got, forge.ReviewApproved) ||
				len(a.bodies(1)) != bodies || !strings.Contains(a.stdout.String(), line) || a.pulls(1)[0].Merged {
				t.Fatalf("waiting: %v %q\n%s", got, a.bodies(1), a.stdout.String())
			}
			a.forge.SetStatus("app", head, "ci / test", "success")
			a.pass()
			if is := a.issue(1); is.Open || !a.pulls(1)[0].Merged || len(a.bodies(1)) != bodies ||
				!strings.Contains(a.stdout.String(), "#1 merged pull app#1 (slice 1 of 1)\n") {
				t.Errorf("merged once the check passed: %+v %q\n%s", is.Labels, a.bodies(1), a.stdout.String())
			}
			if slices.Contains(a.forge.Events(issue1), forge.Event{Actor: "swarm-bot", What: "labeled " + forge.Blocked}) {
				t.Error("blocked on the way")
			}
		})
	}
}

// A head whose checks failed goes back to the worker (T26): the
// reconciler's note on the pull request names each context that failed
// or errored, the issue goes to review-3-changes-requested with no note
// on it and no block, and the worker's next run reads the note.
func TestSwarmMergeStepChecksFailed(t *testing.T) {
	a := newApplyWS(t)
	head := a.approvedGateOff()
	a.forge.SetStatus("app", head, "ci / test", "failure")
	a.forge.SetStatus("app", head, "lint", "success")
	a.forge.SetStatus("app", head, "build", "error")
	bodies := len(a.bodies(1))
	a.pass()
	note := mark("reconciler", 1) + checksFailed(head, "`ci / test` (failure), `build` (error)")
	comments, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ChangesRequested}) || len(a.bodies(1)) != bodies ||
		len(comments) == 0 || comments[len(comments)-1].Body != note || comments[len(comments)-1].Author != "swarm-bot" ||
		!strings.Contains(a.stdout.String(), "#1 checks failed at "+head[:7]+": back to the worker\n#1 swarm:review-2-approved → swarm:review-3-changes-requested\n") {
		t.Fatalf("T26: %v %q %+v\n%s", got, a.bodies(1), comments, a.stdout.String())
	}
	a.runner.Script(1, "worker", work("v2"))
	a.pass() // the worker: T20, then T12
	if prompt := a.file("1-worker-2", "io/prompt.md"); !strings.Contains(prompt, checksFailed(head, "`ci / test` (failure), `build` (error)")) {
		t.Errorf("the worker's prompt:\n%s", prompt)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.ReviewReady}) || a.pulls(1)[0].HeadSHA == head {
		t.Errorf("T12: %v", got)
	}
}

// Every check success and the merge refused all the same (a rule the
// swarm's reviews do not satisfy) is still T16: the issue blocked.
func TestSwarmMergeStepChecksPassedRefused(t *testing.T) {
	a := newApplyWS(t)
	head := a.approvedGateOff()
	a.forge.RequireChecks("app", "main")
	a.forge.SetStatus("app", head, "ci / test", "success")
	a.forge.RefuseMerge(forge.Ref{Repo: "app", N: 1})
	a.pass()
	note := mark("reconciler", 1) + "merge of pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) at " + head[:7] + " refused: 405 not allowed to merge (at swarm:review-2-approved). Resolve it, then remove swarm:blocked.\n\ncc @jane"
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || !slices.Contains(a.bodies(1), note) {
		t.Errorf("T16: %v %q\n%s", got, a.bodies(1), a.stdout.String())
	}
}
