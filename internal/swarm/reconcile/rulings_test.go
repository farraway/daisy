package reconcile

import (
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
)

// The trigger rule on plan-1-accepted when rulings changed since the plan
// (section 6, The rulings): the architect runs in the worker's place, woken by
// the reconciler's note, and has nothing to answer once its own word is
// the newest; anything running holds it; Replan changes no other stage.
func TestLaunchReplan(t *testing.T) {
	trusted := []string{"jane", "swarm-bot"}
	plan := Entry{"swarm-bot", marker("architect", 1) + "## Steps\n\n1. app: x\n", t0.Add(1)}
	note := Entry{"swarm-bot", marker("reconciler", 1) + "Rulings changed since the plan (architect run 1): `app:docs/adr/1.md`. " + rulingsTail, t0.Add(2)}
	stood := Entry{"swarm-bot", marker("architect", 2) + "STANDS: none applies.", t0.Add(3)}
	opened := Entry{"jane", "please", t0}
	for _, tc := range []struct {
		name    string
		label   string
		thread  []Entry
		running []string
		want    string
	}{
		{"the note", forge.PlanAccepted, []Entry{opened, plan, note}, nil, "architect"},
		{"the note, a run in flight", forge.PlanAccepted, []Entry{opened, plan, note}, []string{"worker"}, ""},
		{"the architect's word", forge.PlanAccepted, []Entry{opened, plan, note, stood}, nil, ""},
		{"jane after it", forge.PlanAccepted, []Entry{opened, plan, note, stood, {"jane", "and?", t0.Add(4)}}, nil, "architect"},
		{"another stage", forge.ChangesRequested, []Entry{opened, plan, note}, nil, "worker"},
	} {
		got := strings.Join(Launch(Turn{Label: tc.label, Thread: tc.thread, Running: tc.running, Replan: true}, "swarm-bot", trusted), ",")
		if got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The accepted plan is the architect's newest plan, never its STANDS:
// confirmation, which is the basis the rulings are compared from; the
// note that rulings changed blocks nothing and counts as no plan.
func TestPlanAndBasis(t *testing.T) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	f.Comment(at, marker("reconciler", 1)+"Rulings changed since the plan (architect run 1): `app:docs/adr/1.md`. "+rulingsTail)
	f.Comment(at, marker("architect", 2)+"STANDS: no slice touches the roles.")
	f.Comment(at, marker("architect", 3)+"BLOCKED: which ADR?")
	j := job(f, at.N, "architect", forge.PlanAccepted)
	if _, m, ok := j.Plan(); !ok || m.Run != 1 {
		t.Errorf("plan: run %d, %v", m.Run, ok)
	}
	if _, m, ok := j.basis(); !ok || m.Run != 2 {
		t.Errorf("basis: run %d, %v", m.Run, ok)
	}
	if c := Count(j.Issue, nil, nil, "swarm-bot", j.Cfg.Trusted); c.Plans != 1 || c.Blocks != 0 {
		t.Errorf("counts %+v", c)
	}
	if _, ok := blockingNote(j.Issue, "swarm-bot"); ok {
		t.Error("the rulings note read as a block")
	}
}

// The architect on plan-1-accepted with rulings changed (section 7.4):
// the Task names the files, keeps the slices with pull requests, offers
// STANDS: and says where a revision goes; its prompt lists the earlier
// pull requests and the thread, the reconciler's note in it.
func TestRenderArchitectRulings(t *testing.T) {
	f, at, pr, _ := planned(t)
	if err := f.Merge(pr, "aaa"); err != nil {
		t.Fatal(err)
	}
	f.Comment(at, marker("reconciler", 1)+"Rulings changed since the plan (architect run 1): `app:docs/adr/1.md`. "+rulingsTail)
	j := job(f, at.N, "architect", forge.PlanAccepted)
	j.Rulings = []string{"app:docs/adr/1.md", "app:docs/adr/2.md"}
	sys, prompt := Render(j)
	has(t, "system.md", sys, []string{
		"Rulings changed since your plan for issue #1 was written or last confirmed: app:docs/adr/1.md, app:docs/adr/2.md; revise the remaining slices or confirm they stand.",
		`Slice 1 has a pull request, under "# Earlier pull requests": keep its line as it is, with its number and repository.`,
		`If the remaining slices stand as they are, write "STANDS: <why>" as the first line of your output instead, then for each changed ruling, why they agree with it. ` + wantBuilderSentence + ` A revised plan goes to the maintainer as a plan does; one that stands goes on to the next slice.`,
		`that file and the builder plan in /job/io/builder.md are the only things read back`,
		`These paths are protected:`},
		"Write the plan for issue", "Revise your plan for issue #1 in answer")
	has(t, "prompt.md", prompt, []string{"# Earlier pull requests\n\n- lib#1 swarm/1-1 merged ", "# Thread\n", "Rulings changed since the plan (architect run 1)"}, "# Pull request lib#1")
	j.Cfg.Gates.Plan = false
	sys, _ = Render(j)
	has(t, "system.md (plan gate off)", sys, []string{`A revised plan is accepted as you write it.`}, "goes to the maintainer")
}
