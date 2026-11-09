package reconcile

import (
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// The flag's predicate (section 5): for an architecture job, each gated
// stage exactly when its gate is on, any stage carrying swarm:blocked,
// every other stage never, whatever the gates; no stage, or two, never;
// for a routine job, the same but for the triage and plan gates, which
// never hold it, and the merge gate, which does.
func TestGated(t *testing.T) {
	gate := map[string]func(*declare.Gates){
		forge.ProposalPosted: func(g *declare.Gates) { g.Triage = false },
		forge.PlanPosted:     func(g *declare.Gates) { g.Plan = false },
		forge.ReviewApproved: func(g *declare.Gates) { g.Merge = false },
	}
	on := declare.Gates{Triage: true, Plan: true, Merge: true}
	for _, stage := range forge.Stages {
		if got := Gated([]string{stage}, on, Architecture); got != (gate[stage] != nil) {
			t.Errorf("%s, every gate on: %v", stage, got)
		}
		if Gated([]string{stage}, declare.Gates{}, Architecture) {
			t.Errorf("%s, every gate off: gated", stage)
		}
		for _, g := range []declare.Gates{on, {}} {
			if !Gated([]string{forge.Paused, stage, forge.Blocked}, g, Architecture) || !Gated([]string{forge.Paused, stage, forge.Blocked}, g, Routine) {
				t.Errorf("%s blocked, gates %+v: not gated", stage, g)
			}
		}
		if got := Gated([]string{stage}, on, Routine); got != (stage == forge.ReviewApproved) {
			t.Errorf("%s, a routine job, every gate on: %v", stage, got)
		}
		if off := gate[stage]; off != nil {
			g := on
			off(&g)
			if Gated([]string{stage}, g, Architecture) {
				t.Errorf("%s with its own gate off is gated", stage)
			}
		}
	}
	for _, ls := range [][]string{{forge.Gated}, {forge.Blocked}, {forge.Done}, {forge.Rejected}, nil, {forge.ProposalPosted, forge.PlanPosted, forge.Blocked}} {
		if Gated(ls, on, Architecture) {
			t.Errorf("%q: not one stage, and gated", ls)
		}
	}
}

// The batch prompt's issue lines name the stage, never the flag, and a
// closed issue's terminal.
func TestBatchLinesNameTheStage(t *testing.T) {
	f := forge.NewFake(clock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	f.AddLabels(forge.Labels...)
	f.CreateIssue("Open", "", []string{"swarm:proposal-posted", forge.Gated})
	f.CreateIssue("Done", "", []string{forge.Done})
	f.CreateIssue("Dropped", "", []string{forge.Rejected})
	read := func(n int) forge.Issue { is, _ := f.Issue(n); return is }
	b := job(f, 0, "owner", "")
	b.Batch = &Batch{Open: []forge.Issue{read(1)}, Closed: []Closed{{Issue: read(2)}, {Issue: read(3)}}, Propose: 1}
	_, prompt := Render(b)
	has(t, "batch prompt.md", prompt, []string{"- #1 [proposal-posted] Open\n", "- #2 [done] Done: closed ", "- #3 [rejected] Dropped: closed "}, "[gated]")
}
