package reconcile

import (
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// The flag's predicate (section 5): each gated stage exactly when its gate
// is on, blocked always, every other stage never, whatever the gates.
func TestGated(t *testing.T) {
	gate := map[string]func(*declare.Gates){
		"swarm:proposed": func(g *declare.Gates) { g.Triage = false },
		"swarm:planning": func(g *declare.Gates) { g.Plan = false },
		"swarm:approved": func(g *declare.Gates) { g.Merge = false },
	}
	for _, stage := range forge.Stages {
		on := declare.Gates{Triage: true, Plan: true, Merge: true}
		want := stage == "swarm:blocked" || gate[stage] != nil
		if got := Gated(stage, on); got != want {
			t.Errorf("%s, every gate on: %v", stage, got)
		}
		if got := Gated(stage, declare.Gates{}); got != (stage == "swarm:blocked") {
			t.Errorf("%s, every gate off: %v", stage, got)
		}
		if off := gate[stage]; off != nil {
			g := on
			off(&g)
			if Gated(stage, g) {
				t.Errorf("%s with its own gate off is gated", stage)
			}
		}
	}
	for _, l := range []string{forge.Gated, forge.Done, forge.Rejected, ""} {
		if Gated(l, declare.Gates{Triage: true, Plan: true, Merge: true}) {
			t.Errorf("%q is no stage, and no gate", l)
		}
	}
}

// The batch prompt's issue lines name the stage, never the flag, and a
// closed issue's terminal.
func TestBatchLinesNameTheStage(t *testing.T) {
	f := forge.NewFake(clock(), "acme/app", []string{"app", "lib"}, forge.Account{Login: "swarm-bot"})
	f.AddLabels(forge.Labels...)
	f.CreateIssue("Open", "", []string{"swarm:proposed", forge.Gated})
	f.CreateIssue("Done", "", []string{forge.Done})
	f.CreateIssue("Dropped", "", []string{forge.Rejected})
	read := func(n int) forge.Issue { is, _ := f.Issue(n); return is }
	b := job(f, 0, "owner", "")
	b.Batch = &Batch{Open: []forge.Issue{read(1)}, Closed: []Closed{{Issue: read(2)}, {Issue: read(3)}}, Propose: 1}
	_, prompt := Render(b)
	has(t, "batch prompt.md", prompt, []string{"- #1 [proposed] Open\n", "- #2 [done] Done: closed ", "- #3 [rejected] Dropped: closed "}, "[gated]")
}
