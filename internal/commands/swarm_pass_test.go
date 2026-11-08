package commands

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// relabelOnList stands in for the maintainer relabelling issue id to the
// label to while a pass runs (section 5: "at any time"): once, when the
// pass lists the open issues of on, before the list is read or after.
type relabelOnList struct {
	*forge.Fake
	on, to string
	id     forge.Ref
	after  bool
	done   bool
}

func (r *relabelOnList) Issues(st forge.State, label string) ([]forge.Issue, error) {
	fire := !r.done && st == forge.Open && label == r.on
	if fire && !r.after {
		r.done = true
		r.Fake.SetLabel(r.id, r.to)
	}
	out, err := r.Fake.Issues(st, label)
	if fire && r.after {
		r.done = true
		r.Fake.SetLabel(r.id, r.to)
	}
	return out, err
}

// The pass acts on an issue as Issue(n) reads it, not as a list read
// earlier did (section 6, rule 1: n carries exactly one swarm:* label and
// R is its role): an issue moved from ready to planned after its stage was
// listed launches no architect.
func TestSwarmPassActsOnTheIssueAsItIs(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.tick()
	f := &relabelOnList{Fake: a.forge, on: "swarm:ready", to: "swarm:planned", id: issue1, after: true}
	if err := a.apply(f, "--once"); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); strings.Contains(out, "#1 architect run 1: launched") {
		t.Errorf("the architect launched on #1 at %v:\n%s", labels(a.issue(1)), out)
	}
}

// Rule 2 counts this pass's own launches: an issue the pass launched a
// member on, then relabelled by the maintainer to a later stage before
// that stage is listed, gets no second member in the same pass.
func TestSwarmPassLaunchesOnceAnIssue(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.tick()
	f := &relabelOnList{Fake: a.forge, on: "swarm:planning", to: "swarm:planned", id: issue1}
	if err := a.apply(f, "--once"); err != nil {
		t.Fatal(err)
	}
	if n := len(a.runner.Launches()); n != 1 {
		t.Errorf("%d members launched on #1 in one pass:\n%s", n, a.stdout.String())
	}
}

// The merge step merges only an open issue whose one swarm:* label is
// approved: one the maintainer also labelled blocked is skipped, as step
// (5) says, and neither merged nor closed.
func TestSwarmMergeStepSkipsTwoLabels(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass()
	if got := labels(a.issue(1)); len(got) != 1 || got[0] != "swarm:approved" {
		t.Fatalf("not approved: %v\n%s", got, a.stdout.String())
	}
	a.forge.As(forge.Account{Login: "jane"}).Relabel(issue1, "", "swarm:blocked")
	a.pass()
	if is, ps := a.issue(1), a.pulls(1); ps[0].Merged || !is.Open {
		t.Errorf("merged %v, open %v, labels %v\n%s", ps[0].Merged, is.Open, is.Labels, a.stdout.String())
	}
}

// failIssue fails the forge's Issue(n) at its call'th call, once.
type failIssue struct {
	gitForge
	call, calls int
}

func (f *failIssue) Issue(n int) (forge.Issue, error) {
	f.calls++
	if f.calls == f.call {
		return forge.Issue{}, errors.New("forge: 502 bad gateway")
	}
	return f.gitForge.Issue(n)
}

// max_members counts every container of the swarm running at once, a
// worker run's check included (section 6, rule 4): with max_members 1, a
// check started in step (3), collecting a run left uncollected, leaves no
// room for a launch in step (5) of the same pass.
func TestSwarmPassCountsAStartedCheck(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxMembers = 1 })
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.tick()
	if err := a.apply(&failIssue{gitForge: a.git(), call: 2}, "--once"); err == nil {
		t.Fatalf("the worker's collection did not fail:\n%s", a.stdout.String())
	}
	a.forge.CreateIssue("Second", "Another job.", []string{"swarm:ready"})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if out := a.stdout.String(); !strings.Contains(out, "#1 worker run 1: check started") || strings.Contains(out, "#2 architect run 1: launched") {
		t.Errorf("a check and a member at once under max_members 1:\n%s", out)
	}
}

// out.md is the member's: a link it leaves there is never followed (section
// 7.5), so it cannot make apply read a host file and post it; the run has no
// output.
func TestSwarmOutputLinkIsNotFollowed(t *testing.T) {
	a := newApplyWS(t)
	host := filepath.Join(a.ws, "host-secret.txt")
	if err := os.WriteFile(host, []byte("HOST-SECRET-0123456789\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"})
	a.runner.Script(1, "architect", runner.Canned{Work: func(work string) error {
		return os.Symlink(host, filepath.Join(filepath.Dir(work), "io", "out.md"))
	}})
	a.pass()
	if body := strings.Join(a.bodies(1), "\n"); strings.Contains(body, "HOST-SECRET") || !strings.Contains(body, "failed: no output") {
		t.Errorf("the issue's thread:\n%s", body)
	}
}

// apply reports a problem of .env.sh once, at its start, however often the
// pass then reads a secret (section 11, Stderr: as init).
func TestSwarmApplyWarnsOnceAboutEnvSh(t *testing.T) {
	a := newApplyWS(t)
	if err := os.WriteFile(filepath.Join(a.ws, ".env.sh"), []byte("export GITEA_TOKEN=\"tok-123\"\nsource ./other.sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if n := strings.Count(a.stderr.String(), ".env.sh line 2"); n != 1 {
		t.Errorf("warned %d times:\n%s", n, a.stderr.String())
	}
}

// A paused pass merges nothing (section 11, pause): with the merge gate
// off, an approved issue stays approved and its pull request open until
// resume.
func TestSwarmPausedPassMergesNothing(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // T9: approved
	if err := a.verb(SwarmPause); err != nil {
		t.Fatal(err)
	}
	a.pass()
	if ps := a.pulls(1); ps[0].Merged || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:approved"}) {
		t.Errorf("a paused pass merged: %+v %v\n%s", ps[0], labels(a.issue(1)), a.stdout.String())
	}
	a.resume()
	a.pass()
	if ps := a.pulls(1); !ps[0].Merged {
		t.Errorf("not merged after resume:\n%s", a.stdout.String())
	}
}
