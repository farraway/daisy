package cmd

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// betweenSlices is issue #1 at plan-1-accepted with twoSlices' plan, its
// architect's run 1 recorded against app's main as the pass fetched it,
// and slice 1 merged; app's protected list names docs/adr/, its rulings
// by default (section 3), unless edit says otherwise.
func (a *applyWS) betweenSlices(edit func(c *declare.Config)) {
	a.t.Helper()
	a.edit(func(c *declare.Config) {
		c.Repos[0].Protected = append(c.Repos[0].Protected, "docs/adr/")
		if edit != nil {
			edit(c)
		}
	})
	a.publish("rulings")
	a.planned(twoSlices)
	m, err := a.forge.CreatePull("app", "swarm/1-1", "main", "#1 slice 1: Drift check", mark("worker", 1)+"Did it.\n\nRefs acme/swarm-app#1")
	if err != nil {
		a.t.Fatal(err)
	}
	pr := forge.Ref{Repo: "app", N: m}
	a.forge.Push(pr, "aaa")
	if err := a.forge.Merge(pr, "aaa"); err != nil {
		a.t.Fatal(err)
	}
}

// launchedRoles is each launch's role, in order.
func (a *applyWS) launchedRoles() []string {
	var out []string
	for _, s := range a.runner.Launched {
		out = append(out, s.Label[strings.LastIndex(s.Label, "/")+1:])
	}
	return out
}

// rulingsNotes is the reconciler's notes on #1 that rulings changed.
func (a *applyWS) rulingsNotes() []string {
	var out []string
	for _, c := range a.issue(1).Comments {
		if strings.HasPrefix(c.Body, mark("reconciler", 1)+"Rulings changed") || strings.HasPrefix(c.Body, mark("reconciler", 2)+"Rulings changed") {
			out = append(out, c.Body)
		}
	}
	return out
}

const adrNote = "Rulings changed since the plan (architect run 1): `app:docs/adr/0027-roles.md`. The architect revises the remaining slices or confirms they stand."

// F33's acceptance (section 6, the rulings): an ADR merged while #1 is
// between slices posts the reconciler's note naming it and runs the
// architect, not the worker, on plan-1-accepted, once; its STANDS:
// confirms the plan, the label kept, and the next pass runs the worker
// on slice 2 with no note again and no maintainer step.
func TestSwarmApplyRulingStands(t *testing.T) {
	a := newApplyWS(t)
	a.betweenSlices(nil)
	a.land("app", map[string]string{"docs/adr/0027-roles.md": "# 27. Roles\n"}, "ADR 0027")
	a.runner.Script(1, "architect", runner.Canned{Out: "STANDS: no slice names a role.\n", Stdout: claudeResult})
	a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n"})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "architect"}) {
		t.Fatalf("launched %q\n%s", got, a.stdout.String())
	}
	if !strings.Contains(a.stdout.String(), "#1 rulings note posted\n#1 architect run 2: launched\n") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
	if notes := a.rulingsNotes(); len(notes) != 1 || notes[0] != mark("reconciler", 1)+adrNote {
		t.Errorf("notes %q", notes)
	}
	is := a.issue(1)
	if !reflect.DeepEqual(labels(is), []string{forge.PlanAccepted}) || is.Comments[len(is.Comments)-1].Body != mark("architect", 2)+"STANDS: no slice names a role.\n" {
		t.Errorf("issue: %v %q", labels(is), is.Comments[len(is.Comments)-1].Body)
	}
	head := gitIn(t, a.upstream("app"), "rev-parse", "main")
	if r := a.result("1-architect-2"); !strings.Contains(r, "start app "+head+"\n") || !strings.Contains(r, "label swarm:plan-1-accepted\n") || strings.Contains(r, "relabelled") {
		t.Errorf("run 2:\n%s", r)
	}
	sys := a.file("1-architect-2", "io/system.md")
	if !strings.Contains(sys, "Rulings changed since your plan for issue #1 was written or last confirmed: app:docs/adr/0027-roles.md; revise the remaining slices or confirm they stand.") {
		t.Errorf("system.md:\n%s", sys)
	}
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "architect", "worker"}) {
		t.Fatalf("pass 2 launched %q\n%s\n%s", got, a.stdout.String(), a.stderr.String())
	}
	if r := a.result("1-worker-1"); !strings.Contains(r, "branch swarm/1-2 in app\n") {
		t.Errorf("worker:\n%s", r)
	}
	if notes := a.rulingsNotes(); len(notes) != 1 {
		t.Errorf("notes %q", notes)
	}
}

// A revision: with the plan gate on, the architect's revised plan takes
// an architecture job to plan-0-posted (T27), gated, and nothing runs
// until the maintainer accepts it; with it off the revision is the
// accepted plan and the worker runs on slice 2 at the next pass.
func TestSwarmApplyRulingRevised(t *testing.T) {
	revised := twoSlices + "\n"
	revised = strings.Replace(revised, "2. app: add notes.md.", "2. app: add notes.md, its roles from ADR 0027.", 1)
	for _, gate := range []bool{true, false} {
		t.Run(map[bool]string{true: "gate on", false: "gate off"}[gate], func(t *testing.T) {
			a := newApplyWS(t)
			a.betweenSlices(func(c *declare.Config) {
				c.Gates.Plan = gate
				if !gate {
					c.Budget = &declare.Budget{PerDay: 10, Total: 100}
				}
			})
			a.land("app", map[string]string{"docs/adr/0027-roles.md": "# 27. Roles\n"}, "ADR 0027")
			a.runner.Script(1, "architect", runner.Canned{Out: revised, Stdout: claudeResult})
			a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n"})
			a.pass()
			is := a.issue(1)
			want := []string{forge.PlanPosted, forge.Gated}
			if !gate {
				want = []string{forge.PlanAccepted}
			}
			if !reflect.DeepEqual(labels(is), want) || !reflect.DeepEqual(a.launchedRoles(), []string{"architect", "architect"}) {
				t.Fatalf("labels %v, launched %q\n%s", labels(is), a.launchedRoles(), a.stdout.String())
			}
			if r := a.result("1-architect-2"); gate != strings.Contains(r, "relabelled swarm:plan-1-accepted swarm:plan-0-posted\n") {
				t.Errorf("run 2:\n%s", r)
			}
			a.pass()
			want = []string{"architect", "architect"}
			if !gate {
				want = append(want, "worker")
			}
			if got := a.launchedRoles(); !reflect.DeepEqual(got, want) {
				t.Errorf("pass 2 launched %q", got)
			}
			if len(a.rulingsNotes()) != 1 {
				t.Errorf("notes %q", a.rulingsNotes())
			}
		})
	}
}

// A job whose plan predates no change to its rulings is unaffected: a
// change outside them (an ADR under another directory, a README), or an
// ADR merged where rulings is [], runs the worker on slice 2 with no
// note; so does one before the plan's run.
func TestSwarmApplyRulingUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(c *declare.Config)
		files map[string]string
	}{
		{"outside the rulings", nil, map[string]string{"README.md": "# App\n", "adr/0027-roles.md": "# 27\n", "docs/adr.md": "# x\n"}},
		{"rulings off", func(c *declare.Config) { c.Repos[0].Rulings = &[]string{} }, map[string]string{"docs/adr/0027-roles.md": "# 27\n"}},
		{"in lib, which has none", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.betweenSlices(tc.edit)
			if tc.files != nil {
				a.land("app", tc.files, "elsewhere")
			} else {
				a.land("lib", map[string]string{"docs/adr/0027-roles.md": "# 27\n"}, "lib's ADR")
			}
			a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n"})
			a.pass()
			if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker"}) || len(a.rulingsNotes()) != 0 || a.stderr.Len() != 0 {
				t.Errorf("launched %q, notes %q\nstderr:\n%s", got, a.rulingsNotes(), a.stderr.String())
			}
		})
	}
	// The ADR merged before the plan's run: the plan was written against it.
	a := newApplyWS(t)
	a.land("app", map[string]string{"docs/adr/0027-roles.md": "# 27\n"}, "ADR 0027")
	a.betweenSlices(nil)
	a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n"})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker"}) || len(a.rulingsNotes()) != 0 {
		t.Errorf("before the plan: launched %q, notes %q", got, a.rulingsNotes())
	}
}

// Not between slices: with the current pull request open at
// plan-1-accepted, the worker continues it after an ADR merged, with no
// note (section 6: only a job between slices is re-planned).
func TestSwarmApplyRulingOpenSlice(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Repos[0].Protected = append(c.Repos[0].Protected, "docs/adr/") })
	a.publish("rulings")
	a.planned(twoSlices)
	gitIn(t, a.upstream("app"), "branch", "swarm/1-1", "main")
	m, err := a.forge.CreatePull("app", "swarm/1-1", "main", "#1 slice 1: Drift check", mark("worker", 1)+"Did it.\n\nRefs acme/swarm-app#1")
	if err != nil {
		t.Fatal(err)
	}
	a.forge.Push(forge.Ref{Repo: "app", N: m}, gitIn(t, a.upstream("app"), "rev-parse", "main"))
	a.land("app", map[string]string{"docs/adr/0027-roles.md": "# 27\n"}, "ADR 0027")
	a.runner.Script(1, "worker", runner.Canned{Out: "Done.\n"})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker"}) || len(a.rulingsNotes()) != 0 {
		t.Errorf("launched %q, notes %q\n%s", got, a.rulingsNotes(), a.stderr.String())
	}
	if r := a.result("1-worker-2"); !strings.Contains(r, "branch swarm/1-1 in app\n") {
		t.Errorf("worker:\n%s\n%s", r, a.stdout.String())
	}
}

// The note is posted once per basis: an architect whose launch failed
// (a swarm fault; the Fake runner lists only the containers started) is
// launched again at the next pass by the note already posted, which is
// not posted again.
func TestSwarmApplyRulingNotedOnce(t *testing.T) {
	a := newApplyWS(t)
	a.betweenSlices(nil)
	a.land("app", map[string]string{"docs/adr/0027-roles.md": "# 27. Roles\n"}, "ADR 0027")
	a.runner.StartErr = errors.New("docker: down")
	a.pass()
	a.runner.StartErr = nil
	if r := a.result("1-architect-2"); !strings.Contains(r, "fault: launch: docker: down\n") || len(a.rulingsNotes()) != 1 {
		t.Fatalf("pass 1: notes %q, run 2:\n%s", a.rulingsNotes(), r)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: "STANDS: no slice names a role.\n", Stdout: claudeResult})
	a.pass()
	if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "architect"}) || len(a.rulingsNotes()) != 1 || strings.Contains(a.stdout.String(), "rulings note posted") {
		t.Errorf("pass 2: launched %q, notes %q\n%s", got, a.rulingsNotes(), a.stdout.String())
	}
	if is := a.issue(1); is.Comments[len(is.Comments)-1].Body != mark("architect", 3)+"STANDS: no slice names a role.\n" {
		t.Errorf("last comment %q", is.Comments[len(is.Comments)-1].Body)
	}
}
