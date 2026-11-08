package commands

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// The in-progress stages taken back when a run does not move the issue
// on (section 5): a worker's swarm fault leaves plan-1-accepted, unblocked;
// an abandoned worker run, whose apply died with the issue at
// plan-2-in-progress, is restored before the worker runs again; a first
// reviewer's stale run leaves review-0-ready; and a launch whose relabel
// to plan-2-in-progress was refused is moved on from the stage it was
// launched on.
func TestSwarmInProgressTakenBack(t *testing.T) {
	t.Run("a worker's fault", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", runner.Canned{Code: 125})
		a.pass()
		r := a.result("1-worker-1")
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) ||
			!strings.Contains(r, "\nrelabelled swarm:plan-1-accepted swarm:plan-2-in-progress\n") ||
			!strings.Contains(r, "\nfault: exit 125\n") || !strings.HasSuffix(r, "\nrelabelled swarm:plan-2-in-progress swarm:plan-1-accepted\noutcome fault\ncollected\n") {
			t.Errorf("%v\n%s", got, r)
		}
	})
	t.Run("an abandoned worker run", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.forge.SetLabel(issue1, "swarm:plan-2-in-progress")
		os.MkdirAll(a.state("runs/1-worker-1/work"), 0o755)
		os.WriteFile(a.state("runs/1-worker-1/result"), []byte("start aaaa\nbranch swarm/1-1 in app\nlabel swarm:plan-1-accepted\nstarted 2026-11-06T14:59:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\nrelabelled swarm:plan-1-accepted swarm:plan-2-in-progress\n"), 0o644)
		a.runner.Script(1, "worker", work("v1"))
		a.pass()
		if r := a.result("1-worker-1"); !strings.HasSuffix(r, "relabelled swarm:plan-2-in-progress swarm:plan-1-accepted\noutcome abandoned\ncollected\n") {
			t.Errorf("abandoned:\n%s", r)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) || !strings.Contains(a.result("1-worker-2"), "\nlabel swarm:plan-1-accepted\n") {
			t.Errorf("the worker did not run again from plan-1-accepted: %v\n%s", got, a.stdout.String())
		}
	})
	t.Run("an abandoned run whose restore the forge refused", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.forge.SetLabel(issue1, "swarm:plan-2-in-progress")
		os.MkdirAll(a.state("runs/1-worker-1/work"), 0o755)
		os.WriteFile(a.state("runs/1-worker-1/result"), []byte("start aaaa\nbranch swarm/1-1 in app\nlabel swarm:plan-1-accepted\nstarted 2026-11-06T14:59:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\n"), 0o644)
		a.tick()
		if err := a.apply(&flaky{gitForge: a.git(), relabels: 1}, "--once"); err == nil {
			t.Fatal("the refused restore exited 0")
		}
		if r := a.result("1-worker-1"); strings.Contains(r, "collected") || !strings.Contains(r, "\nabandoned ") {
			t.Errorf("collected without its restore:\n%s", r)
		}
		a.runner.Script(1, "worker", work("v1"))
		a.pass()
		if r := a.result("1-worker-1"); !strings.HasSuffix(r, "relabelled swarm:plan-2-in-progress swarm:plan-1-accepted\noutcome abandoned\ncollected\n") || strings.Count(r, "abandoned ") != 1 {
			t.Errorf("the retry:\n%s", r)
		}
	})
	t.Run("a first reviewer's stale run", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: approve, Work: func(string) error {
			// The maintainer pushes to the branch during the review.
			src := a.t.TempDir()
			for _, args := range [][]string{{"clone", "-q", "-b", "swarm/1-1", a.upstream("app"), src},
				{"-C", src, "-c", "user.name=T", "-c", "user.email=t@example.net", "commit", "-q", "--allow-empty", "-m", "by hand"}, {"-C", src, "push", "-q"}} {
				if err := gitRun(args...); err != nil {
					return err
				}
			}
			return nil
		}})
		a.pass()
		r := a.result("1-adversary-1")
		if !strings.Contains(r, "\nrelabelled swarm:review-0-ready swarm:review-1-in-progress\n") || !strings.Contains(r, "\nrelabelled swarm:review-1-in-progress swarm:review-0-ready\noutcome stale\n") {
			t.Errorf("stale:\n%s", r)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) {
			t.Errorf("after a stale review: %v", got)
		}
	})
	t.Run("a refused relabel at the launch", func(t *testing.T) {
		a := newApplyWS(t)
		a.planned(oneSlice)
		a.runner.Script(1, "worker", work("v1"))
		a.tick()
		if err := a.apply(&flaky{gitForge: a.git(), relabels: 1}, "--once"); err == nil {
			t.Fatal("the refused relabel exited 0")
		}
		r := a.result("1-worker-1")
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) || strings.Contains(r, "plan-2-in-progress") ||
			!strings.Contains(r, "\nrelabelled swarm:plan-1-accepted swarm:review-0-ready\noutcome posted\n") {
			t.Errorf("%v\n%s", got, r)
		}
	})
}

// A run launched before the stages' rename (its result names the old
// stage) is collected under the new name (forge.Renamed): the architect's
// plan on a ready issue moves it from proposal-accepted on.
func TestSwarmCollectsARunRecordedUnderTheOldNames(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	os.MkdirAll(a.state("runs/1-architect-1/io"), 0o755)
	os.WriteFile(a.state("runs/1-architect-1/io/out.md"), []byte(archPlan), 0o644)
	os.WriteFile(a.state("runs/1-architect-1/result"), []byte("label swarm:ready\nstarted 2026-11-06T14:00:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\nexited 0 2026-11-06T14:10:00Z\n"), 0o644)
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", forge.Gated}) ||
		!strings.Contains(a.result("1-architect-1"), "\nrelabelled swarm:proposal-accepted swarm:plan-0-posted\n") {
		t.Errorf("%v\n%s\nstderr:\n%s", got, a.result("1-architect-1"), a.stderr.String())
	}
}

// The maintainer's holds (section 6, rule 1): a paused issue launches
// nothing and, with the merge gate off, is not merged, until the flag is
// cleared; an issue whose newest maintainer comment is `Blocked by #<m>`
// launches nothing while #m is open, and its role runs once #m is closed;
// a later maintainer comment lifts the hold too. Collection is never held.
func TestSwarmApplyHolds(t *testing.T) {
	t.Run("paused", func(t *testing.T) {
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) {
			c.Gates.Plan, c.Gates.Merge = false, false
			c.Budget = &declare.Budget{PerDay: 10, Total: 100}
		})
		a.forge.CreateIssue("Held", "Check the drift.", []string{"swarm:proposal-accepted", forge.Paused}) // #1
		a.pass()
		if len(a.runner.Launches()) != 0 {
			t.Fatalf("a paused issue launched:\n%s", a.stdout.String())
		}
		a.forge.Relabel(issue1, forge.Paused, "")
		a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) {
			t.Fatalf("after resume: %v", got)
		}
		a.runner.Script(1, "worker", work("v1"))
		a.pass()
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass()
		a.forge.Relabel(issue1, "", forge.Paused) // paused between the pair's two reviews: the second waits
		a.pass()
		a.pass()
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress", forge.Paused}) {
			t.Fatalf("paused mid-pair: %v\n%s", got, a.stdout.String())
		}
		a.forge.Relabel(issue1, forge.Paused, "")
		a.pass() // T9
		a.forge.Relabel(issue1, "", forge.Paused)
		a.pass()
		if ps := a.pulls(1); ps[0].Merged || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-2-approved", forge.Paused}) {
			t.Fatalf("a paused issue merged: %v", labels(a.issue(1)))
		}
		a.forge.Relabel(issue1, forge.Paused, "")
		a.pass()
		if ps := a.pulls(1); !ps[0].Merged || a.issue(1).Open {
			t.Errorf("not merged once resumed:\n%s", a.stdout.String())
		}
	})
	t.Run("blocked by another issue", func(t *testing.T) {
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.Trusted = []string{"jane", "swarm-bot"} })
		a.forge.AddLabels("bug")
		a.forge.CreateIssue("The blocker", "", []string{"bug"})                              // #1, not the swarm's
		a.forge.CreateIssue("Held", "Check the drift.", []string{"swarm:proposal-accepted"}) // #2
		job := forge.Ref{Repo: "app", N: 2}
		a.forge.Say(job, "jane", "Blocked by #1")
		a.pass()
		if len(a.runner.Launches()) != 0 {
			t.Fatalf("a blocked job launched:\n%s", a.stdout.String())
		}
		a.forge.CloseIssue(1)
		a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 1 {
			t.Fatalf("closed blocker, %d launches", n)
		}
		a.forge.CreateIssue("Another", "", []string{"bug"})                      // #3
		a.forge.CreateIssue("Next", "Two.", []string{"swarm:proposal-accepted"}) // #4
		a.forge.Say(forge.Ref{Repo: "app", N: 4}, "jane", "Blocked by #3")
		a.forge.Say(forge.Ref{Repo: "app", N: 4}, "jane", "Go ahead anyway.")
		a.runner.Script(4, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 2 {
			t.Errorf("a later maintainer comment did not lift the hold: %d launches", n)
		}
	})
}

func gitRun(args ...string) error {
	if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
		return fmt.Errorf("git %v: %v %s", args, err, errb)
	}
	return nil
}
