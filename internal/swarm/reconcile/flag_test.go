package reconcile

import (
	"testing"
	"time"

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

// The gate note `swarm connect` shows (section 11, swarm connect): the
// newest gate note while the issue carries swarm:gated and the note is of
// its gating now; none for an issue without the flag, a note of another
// stage, a plan posted after the note, or a blocked issue whose newest
// note is not the blocked gate's, and the blocked gate's for one.
func TestCurrentGateNote(t *testing.T) {
	at := func(m int) time.Time { return time.Date(2026, 11, 6, 15, m, 0, 0, time.UTC) }
	note := func(m int, text string) forge.Comment {
		return forge.Comment{Author: "swarm-bot", Body: gateMarker + "\n" + text, Created: at(m)}
	}
	plan := forge.Comment{Author: "swarm-bot", Body: "<!-- daisy-swarm role=architect run=2 -->\n**architect:**\n\n## Steps\n\n1. app: x.\n", Created: at(5)}
	atPlan := "**reconciler:** gated at plan-0-posted: the plan gate holds the plan until the maintainer accepts it.\n\nDecide: accept."
	atTriage := "**reconciler:** gated at proposal-posted: the triage gate holds a proposal until the maintainer triages it.\n\nDecide: triage."
	blocked := "**reconciler:** gated at plan-0-posted: " + blockedWhy + ".\n\nDecide: unblock."
	for _, c := range []struct {
		name     string
		labels   []string
		comments []forge.Comment
		want     string // "" for none
	}{
		{"the newest, of this gating", []string{forge.PlanPosted, forge.Gated}, []forge.Comment{note(1, atTriage), note(3, atPlan)}, atPlan},
		{"not flagged", []string{forge.PlanPosted}, []forge.Comment{note(3, atPlan)}, ""},
		{"no note", []string{forge.PlanPosted, forge.Gated}, nil, ""},
		{"a note of another stage", []string{forge.PlanPosted, forge.Gated}, []forge.Comment{note(1, atTriage)}, ""},
		{"a plan after the note", []string{forge.PlanPosted, forge.Gated}, []forge.Comment{note(3, atPlan), plan}, ""},
		{"blocked, the blocked gate's", []string{forge.PlanPosted, forge.Gated, forge.Blocked}, []forge.Comment{note(3, atPlan), note(4, blocked)}, blocked},
		{"blocked, another gate's", []string{forge.PlanPosted, forge.Gated, forge.Blocked}, []forge.Comment{note(3, atPlan)}, ""},
		{"not blocked, the blocked gate's", []string{forge.PlanPosted, forge.Gated}, []forge.Comment{note(4, blocked)}, ""},
		{"another's note", []string{forge.PlanPosted, forge.Gated}, []forge.Comment{{Author: "jane", Body: gateMarker + "\n" + atPlan, Created: at(3)}}, ""},
	} {
		text, ok := CurrentGateNote(forge.Issue{Number: 4, Open: true, Labels: c.labels, Comments: c.comments}, "swarm-bot")
		if ok != (c.want != "") || ok && text != c.want {
			t.Errorf("%s: %v %q, want %q", c.name, ok, text, c.want)
		}
	}
}
