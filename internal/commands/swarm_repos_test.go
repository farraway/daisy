package commands

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

func prRef(m int) forge.Ref { return forge.Ref{Repo: "app", N: m} }

// resume is what `swarm resume` does to <state>, until the verb is built.
func (a *applyWS) resume() {
	a.stdout.Reset()
	for name, line := range map[string]string{"paused": "Resumed: members launch at the next pass\n", "owner-held": "Owner batch released\n"} {
		if os.Remove(a.state(name)) == nil {
			a.stdout.WriteString(line)
		}
	}
}

// A job across two repositories (section 13, Step 29's third row): slice
// 1 in the second declared repository, slice 2 in the first; each built
// in its repository's mirror, clone, image, check and protected, its pull
// request opened there with the Refs form for its repository; Pulls(n)
// finds both; slice 2 after slice 1's merge, with its repository and merge
// commit under Earlier pull requests; the architect's clones at
// /work/<name> in the first repository's image.
func TestSwarmApplyTwoRepositories(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Repos[1].Image, c.Repos[1].Check, c.Repos[1].Protected = "daisy-member-lib:1", []string{"go", "test", "./..."}, []string{"go.sum"}
	})
	plan := "## Steps\n\n1. lib: add the check.\n2. app: move the pin to slice 1's merge.\n"
	a.planned(plan)
	arch := a.runner.Launches()[0]
	if arch.Workdir != "/work/app" || arch.Image != "daisy-member:1" || !strings.Contains(a.result("1-architect-1"), "start app ") || !strings.Contains(a.result("1-architect-1"), "\nstart lib ") {
		t.Errorf("architect: %+v\n%s", arch, a.result("1-architect-1"))
	}
	a.runner.Script(1, "worker", work("lib v1"))
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	ls := a.runner.Launches()
	w, check := ls[1], ls[2]
	if w.Image != "daisy-member-lib:1" || w.Workdir != "/work" || check.Image != "daisy-member-lib:1" || !reflect.DeepEqual(check.Argv, []string{"go", "test", "./..."}) {
		t.Errorf("slice 1's specs: %+v %+v", w, check)
	}
	if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, "in acme/lib, on the branch checked out in /work (swarm/1-1)") || !strings.Contains(sys, "These paths are protected: go.sum.") {
		t.Errorf("slice 1's Task:\n%s", sys)
	}
	ps := a.pulls(1)
	if len(ps) != 1 || ps[0].Repo != "lib" || ps[0].Number != 1 || ps[0].Title != "acme/app#1 slice 1: Drift check" || !strings.HasSuffix(ps[0].Body, "\n\nRefs acme/app#1") {
		t.Fatalf("slice 1's pull request: %+v", ps)
	}
	if _, _, err := ops.Real().Exec(a.upstream("lib"), "git", "rev-parse", "--verify", "refs/heads/swarm/1-1"); err != nil {
		t.Errorf("not pushed to lib: %v", err)
	}
	a.pass()
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:approved"}) {
		t.Fatalf("slice 1 not approved: %v", got)
	}
	a.forge.Merge(forge.Ref{Repo: "lib", N: 1}, a.pulls(1)[0].HeadSHA)
	a.forge.SetLabel(issue1, "swarm:planned")
	a.runner.Script(1, "worker", work("app v2"))
	a.pass()
	ps = a.pulls(1)
	if len(ps) != 2 || ps[1].Repo != "app" || ps[1].Number != 2 || ps[1].Head != "swarm/1-2" || !strings.HasSuffix(ps[1].Body, "\n\nRefs #1") {
		t.Fatalf("Pulls(1): %+v", ps)
	}
	if p := a.file("1-worker-2", "io/prompt.md"); !strings.Contains(p, "# Earlier pull requests\n\n- lib#1 swarm/1-1 merged "+ps[0].MergeSHA+": acme/app#1 slice 1: Drift check\n") {
		t.Errorf("slice 2's prompt:\n%s", p)
	}
	if sys := a.file("1-worker-2", "io/system.md"); !strings.Contains(sys, "A pin to an earlier slice's change names its merged commit") {
		t.Errorf("slice 2's Task:\n%s", sys)
	}
}

// Bad slices (section 7.5) with two repositories, and what one repository
// allows.
func TestSwarmApplyBadSlices(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		one        bool   // only app declared
		removeLib  bool   // lib removed from repos after the plan
		bad        string // "" when the worker builds in app
	}{
		{name: "an undeclared repository", plan: "## Steps\n\n1. other: x.\n", bad: "1"},
		{name: "a slice naming none", plan: "## Steps\n\n1. app: x.\n2. y.\n", bad: "2"},
		{name: "no slice lines", plan: "## Research\n\nx\n", bad: "1"},
		{name: "a slice's repository removed", plan: "## Steps\n\n1. lib: x.\n", removeLib: true, bad: "1"},
		{name: "one repository: a slice naming none", plan: "## Steps\n\n1. x.\n", one: true},
		{name: "one repository: no slice lines", plan: "## Research\n\nx\n", one: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			if tc.one {
				a.edit(func(c *declare.Config) { c.Repos = c.Repos[:1] })
			}
			a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"})
			a.runner.Script(1, "architect", runner.Canned{Out: tc.plan})
			a.pass()
			if tc.removeLib {
				a.edit(func(c *declare.Config) { c.Repos = c.Repos[:1] })
			}
			if tc.bad != "" && !tc.removeLib {
				if b := a.bodies(1); !strings.HasSuffix(b[len(b)-1], "architect run 1 failed: bad slices: "+tc.bad+" (was swarm:ready). Relabel the issue to resume.") {
					t.Errorf("architect: %q", b)
				}
				return
			}
			a.forge.SetLabel(issue1, "swarm:planned")
			a.runner.Script(1, "worker", work("v1"))
			a.pass()
			if tc.bad != "" {
				b := a.bodies(1)
				if !strings.HasSuffix(b[len(b)-1], "worker run 1 failed: bad slices: "+tc.bad+" (was swarm:planned). Relabel the issue to resume.") ||
					!strings.Contains(a.result("1-worker-1"), "launch-failed ") || len(a.runner.Launches()) != 1 {
					t.Errorf("worker: %q\n%s", b, a.result("1-worker-1"))
				}
				return
			}
			if ps := a.pulls(1); len(ps) != 1 || ps[0].Repo != "app" {
				t.Errorf("one repository: %+v\n%s", ps, a.result("1-worker-1"))
			}
		})
	}
}
