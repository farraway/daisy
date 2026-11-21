package cmd

import (
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/swarm/runner"
)

// pullsBlock is the block at the top of an issue's body listing items
// (section 5, The pull requests on the issue).
func pullsBlock(items string) string {
	return "<!-- daisy-swarm kind=pulls -->\n**Pull requests:** " + items + "\n<!-- /daisy-swarm kind=pulls -->\n\n"
}

// appPull is app#m named and linked as the block and the notes name it.
func appPull(m string) string {
	return "[acme/app#" + m + "](https://forge.example.net/acme/app/pulls/" + m + ")"
}

// edits is how many times the swarm edited issue n's body.
func (a *applyWS) edits(n int) int {
	k := 0
	for _, e := range a.forge.Events(forge.Ref{Repo: "swarm-app", N: n}) {
		if e == (forge.Event{Actor: "swarm-bot", What: "edited"}) {
			k++
		}
	}
	return k
}

// The pull requests on the issue (section 5): the owner's routine job of
// two slices gains, after its marker lines, the block listing each
// slice's pull request, linked, with its state, edited once in a pass
// that changes it and never in one that does not; with slice 1 merged and
// slice 2 open, the owner's Brief: and Class: lines are still read from
// the body beneath it (the gate note's class, ClassOf, BriefOf); the
// merge gate's note names slice 2's pull request, linked, in its first
// line; the merge step's last merge, which closes the issue, lists both
// merged in the same pass; and an edit launches no member: the owner of
// #2, a proposal with nothing to answer and a pull request, stays idle.
func TestSwarmPullsBlock(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	text := "Brief: docs/briefs/a.md — M1 The drift\nClass: routine\nCheck the drift."
	a.forge.CreateIssue("Drift check", mark("owner", 1)+text, []string{forge.ProposalAccepted})
	a.runner.Script(1, "architect", runner.Canned{Out: twoSlices})
	a.pass() // T5, then T24: a routine job passes the plan gate
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{forge.PlanAccepted}) || a.edits(1) != 0 {
		t.Fatalf("planned: %v, %d edits", got, a.edits(1))
	}

	a.runner.Script(1, "worker", work("v1"))
	a.pass() // T8: app#1 opened
	if body := a.issue(1).Body; body != mark("owner", 1)+pullsBlock("slice 1 "+appPull("1")+" open, review round 1")+text || a.edits(1) != 1 ||
		!strings.Contains(a.stdout.String(), "#1 pull requests listed\n") {
		t.Fatalf("slice 1 open, %d edits:\n%s\n%s", a.edits(1), body, a.stdout.String())
	}

	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary approves
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	a.pass() // approved: nothing runs, nothing changes
	if body := a.issue(1).Body; body != mark("owner", 1)+pullsBlock("slice 1 "+appPull("1")+" open, approved")+text || a.edits(1) != 2 {
		t.Fatalf("slice 1 approved, %d edits:\n%s", a.edits(1), body)
	}
	if err := a.forge.Merge(forge.Ref{Repo: "app", N: 1}, a.pulls(1)[0].HeadSHA); err != nil { // the maintainer merges
		t.Fatal(err)
	}
	a.forge.SetLabel(issue1, forge.PlanAccepted) // T13
	a.runner.Script(1, "worker", runner.Canned{Out: "Notes.\n\nNot established: none\n", Work: in("app", commit(map[string]string{"notes.md": "n\n"}, "Add notes"))})
	a.pass() // T8: app#2 opened
	if body := a.issue(1).Body; body != mark("owner", 1)+pullsBlock("slice 1 "+appPull("1")+" merged · slice 2 "+appPull("2")+" open, review round 1")+text || a.edits(1) != 3 {
		t.Fatalf("one merged, one open, %d edits:\n%s", a.edits(1), body)
	}
	is := a.issue(1)
	if path, milestone := reconcile.BriefOf(is, "swarm-bot"); path != "docs/briefs/a.md" || milestone != "M1 The drift" || reconcile.ClassOf(is, "swarm-bot", []string{"jane", "swarm-bot"}) != reconcile.Routine {
		t.Errorf("Brief: %q %q, Class: %s", path, milestone, reconcile.ClassOf(is, "swarm-bot", []string{"jane", "swarm-bot"}))
	}

	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary approves
	a.pass() // the sympathetic reviewer, once the adversary approved: T9, gated
	head := a.pulls(1)[1].HeadSHA
	notes := a.gateNotes(1)
	note := notes[len(notes)-1]
	first, _, _ := strings.Cut(strings.TrimPrefix(note, gateNote), "\n")
	if first != "**reconciler:** gated at review-2-approved: both reviews approve "+appPull("2")+" at "+head[:7]+", and the merge gate leaves the merge to the maintainer." ||
		!strings.Contains(note, "\n- Class: routine.\n") {
		t.Errorf("the merge gate's note:\n%s", note)
	}
	if body := a.issue(1).Body; body != mark("owner", 1)+pullsBlock("slice 1 "+appPull("1")+" merged · slice 2 "+appPull("2")+" open, approved")+text || a.edits(1) != 4 {
		t.Errorf("slice 2 approved, %d edits:\n%s", a.edits(1), body)
	}

	// #2, the owner's proposal with nothing to answer, gains a block: no
	// member runs on it, then or later.
	a.forge.CreateIssue("Second", mark("owner", 2)+"Do more.", []string{forge.ProposalPosted})
	a.forge.CreatePull("app", "swarm/2-1", "main", "acme/swarm-app#2 slice 1: Second", "Refs acme/swarm-app#2")
	launches := len(a.runner.Launches())
	a.pass()
	a.pass()
	if body := a.issue(2).Body; body != mark("owner", 2)+pullsBlock("slice 1 "+appPull("3")+" open, review round 1")+"Do more." || a.edits(2) != 1 ||
		len(a.runner.Launches()) != launches || a.edits(1) != 4 {
		t.Errorf("#2 listed and idle: %d launches, %d and %d edits:\n%s", len(a.runner.Launches())-launches, a.edits(1), a.edits(2), body)
	}

	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.pass() // T15: the last slice merged, the issue closed
	if is := a.issue(1); is.Open || is.Body != mark("owner", 1)+pullsBlock("slice 1 "+appPull("1")+" merged · slice 2 "+appPull("2")+" merged")+text || a.edits(1) != 5 ||
		!strings.Contains(a.stdout.String(), "#1 swarm:review-2-approved → swarm:done\n") || !strings.Contains(a.stdout.String(), "#1 pull requests listed\n") {
		t.Errorf("both merged in the closing pass, %d edits:\n%s\n%s", a.edits(1), is.Body, a.stdout.String())
	}
}
