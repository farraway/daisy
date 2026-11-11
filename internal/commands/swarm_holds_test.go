package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// The in-progress stages taken back when a run does not move the issue
// on (section 5): a worker's swarm fault leaves plan-1-accepted, unblocked;
// the pair's second reviewer failing leaves review-0-ready, blocked, as the
// first's does;
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
	t.Run("the pair's second reviewer failing", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: "Looks fine.\n"}) // no verdict line

		a.pass() // the pair, in one pass
		r := a.result("1-sympathetic-1")
		b := a.bodies(1)
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready", forge.Blocked, forge.Gated}) ||
			!strings.Contains(r, "\nlabel swarm:review-1-in-progress\n") || !strings.Contains(r, "\nrelabelled swarm:review-1-in-progress swarm:review-0-ready\n") ||
			b[len(b)-1] != mark("reconciler", 1)+"sympathetic run 1 failed: bad verdict (at swarm:review-0-ready). Remove swarm:blocked to resume." {
			t.Errorf("%v %q\n%s", got, b, r)
		}
		a.forge.Relabel(issue1, forge.Blocked, "") // T18: the second reviewer runs again, from review-0-ready
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass()
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) || !strings.Contains(a.result("1-sympathetic-2"), "\nrelabelled swarm:review-0-ready swarm:review-1-in-progress\n") {
			t.Errorf("after T18: %v\n%s", got, a.result("1-sympathetic-2"))
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
		// The adversary's slot busy: the sympathetic reviewer launches
		// alone and writes review-1-in-progress.
		release, err := ops.Real().FS().Lock(filepath.Join(a.local, "swarm", "credentials", "openai", "1.lock"), "test")
		if err != nil {
			t.Fatal(err)
		}
		a.pass()
		release()
		a.forge.Relabel(issue1, "", forge.Paused) // paused between the pair's two reviews: the second waits
		a.pass()
		a.pass()
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress", forge.Paused}) || len(a.runner.Launches()) != 4 ||
			!strings.Contains(a.result("1-sympathetic-1"), "\nrelabelled swarm:review-0-ready swarm:review-1-in-progress\n") {
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
		job := a.ref(2)
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
		a.forge.Say(a.ref(4), "jane", "Blocked by #3")
		a.forge.Say(a.ref(4), "jane", "Go ahead anyway.")
		a.runner.Script(4, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 2 {
			t.Errorf("a later maintainer comment did not lift the hold: %d launches", n)
		}
	})
	t.Run("blocked by no issue", func(t *testing.T) {
		// A `Blocked by #<m>` naming a number the forge does not hold (its
		// 404) holds nothing, with a warning, in the pass and in status:
		// one mistyped comment does not stop every launch.
		a := newApplyWS(t)
		a.forge.CreateIssue("Held", "Check the drift.", []string{"swarm:proposal-accepted"}) // #1
		a.forge.CreateIssue("Other", "Check the rest.", []string{"swarm:proposal-accepted"}) // #2
		a.forge.Say(issue1, "jane", "Blocked by #99")
		a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
		a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
		a.pass()
		if n := len(a.runner.Launches()); n != 2 || !strings.Contains(a.stderr.String(), "⚠ #1: Blocked by #99 names no issue; not held\n") {
			t.Errorf("%d launches\nstderr:\n%s", n, a.stderr.String())
		}
		a.forge.Say(issue1, "jane", "Blocked by #99")
		if err := a.status(a.forge); err != nil || !strings.Contains(a.stderr.String(), "⚠ #1: Blocked by #99 names no issue; not held\n") || strings.Contains(a.stdout.String(), "blocked by #99") {
			t.Errorf("status: %v\nstdout:\n%s\nstderr:\n%s", err, a.stdout.String(), a.stderr.String())
		}
	})
}

func gitRun(args ...string) error {
	if _, errb, err := ops.Real().Exec("", "git", args...); err != nil {
		return fmt.Errorf("git %v: %v %s", args, err, errb)
	}
	return nil
}

// The pair's second reviewer, launched on review-1-in-progress (the
// first's write), takes the issue home to review-0-ready on every path
// that does not move it on (section 5, T22; section 7.7), the first having
// approved: its swarm fault, its launch refused by Docker, its clone
// failing (the first's run took the branch from the mirror), and, its
// run collected by a later pass with launching paused, its stale,
// discarded and abandoned ends.
func TestSwarmSecondReviewerGoesHome(t *testing.T) {
	home := "\nrelabelled swarm:review-1-in-progress swarm:review-0-ready\noutcome "
	for _, tc := range []struct {
		name    string
		second  func(a *applyWS) runner.Canned
		outcome string
		line    string // a line the second's result holds
	}{
		{name: "a fault", second: func(*applyWS) runner.Canned { return runner.Canned{Code: 125} }, outcome: "fault", line: "fault: exit 125"},
		{name: "its launch refused", second: func(*applyWS) runner.Canned {
			return runner.Canned{Work: func(string) error { return fmt.Errorf("no socket") }}
		}, outcome: "fault", line: "fault: launch: no socket"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.inReview(oneSlice)
			a.runner.Script(1, "adversary", runner.Canned{Out: approve})
			a.runner.Script(1, "sympathetic", tc.second(a))
			a.pass() // the pair, in one pass
			r := a.result("1-sympathetic-1")
			if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-0-ready") || slices.Contains(got, "swarm:review-1-in-progress") || slices.Contains(got, forge.Blocked) ||
				!strings.Contains(r, "\nlabel swarm:review-1-in-progress\n") || !strings.Contains(r, "\n"+tc.line+"\n") || !strings.Contains(r, home+tc.outcome+"\ncollected\n") {
				t.Errorf("%v\n%s\nstdout:\n%s", got, r, a.stdout.String())
			}
		})
	}

	t.Run("its clone failing", func(t *testing.T) {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: approve, Work: func(string) error {
			return gitRun("-C", a.state("git/app"), "update-ref", "-d", "refs/heads/swarm/1-1")
		}})
		a.pass()
		r := a.result("1-sympathetic-1")
		if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-0-ready") || slices.Contains(got, "swarm:review-1-in-progress") ||
			!strings.HasPrefix(r, "label swarm:review-1-in-progress\nlaunch-failed ") || !strings.Contains(r, "\nfault: clone: ") || !strings.Contains(r, home+"fault\ncollected\n") {
			t.Errorf("%v\n%s\nstdout:\n%s", got, r, a.stdout.String())
		}
		if l := a.runner.Launches(); l[len(l)-1].Name != "daisy-swarm.acme.swarm-app.1.adversary.1" {
			t.Errorf("launched last: %s", l[len(l)-1].Name)
		}
		if _, err := os.Stat(a.state("runs/1-sympathetic-1/work")); err == nil {
			t.Error("the failed clone is left")
		}
	})

	// The second's run as an apply left it, collected by the next pass;
	// with an end line, its approval in out.md.
	left := func(a *applyWS, start string, more ...string) {
		run := a.state("runs/1-sympathetic-1")
		os.MkdirAll(filepath.Join(run, "io"), 0o755)
		if len(more) > 0 {
			os.WriteFile(filepath.Join(run, "io", "out.md"), []byte(approve), 0o644)
		}
		lines := append([]string{"start " + start, "label swarm:review-1-in-progress", "started 2026-11-06T14:59:00Z", "credential anthropic env CLAUDE_CODE_OAUTH_TOKEN"}, more...)
		os.WriteFile(filepath.Join(run, "result"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		a.forge.SetLabel(issue1, "swarm:review-1-in-progress")
		if err := a.verb(SwarmPause); err != nil {
			a.t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		setup   func(a *applyWS, head string)
		outcome string
	}{
		{name: "stale", setup: func(a *applyWS, head string) {
			left(a, strings.Repeat("0", 40), "exited 0 2026-11-06T15:00:00Z")
		}, outcome: "stale"},
		{name: "discarded", setup: func(a *applyWS, head string) {
			left(a, head, "exited 0 2026-11-06T15:00:00Z")
			pr := forge.Ref{Repo: "app", N: a.pulls(1)[0].Number}
			a.forge.SetMergeable(pr, forge.MergeableYes)
			if err := a.forge.Merge(pr, head); err != nil {
				a.t.Fatal(err)
			}
		}, outcome: "discarded"},
		{name: "abandoned", setup: func(a *applyWS, head string) { left(a, head) }, outcome: "abandoned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApplyWS(t)
			a.inReview(oneSlice)
			tc.setup(a, a.pulls(1)[0].HeadSHA)
			launched := len(a.runner.Launches())
			a.pass()
			r := a.result("1-sympathetic-1")
			if got := labels(a.issue(1)); !slices.Contains(got, "swarm:review-0-ready") || slices.Contains(got, "swarm:review-1-in-progress") ||
				!strings.Contains(r, home+tc.outcome+"\ncollected\n") || len(a.runner.Launches()) != launched {
				t.Errorf("%v\n%s\nstdout:\n%s\nstderr:\n%s", got, r, a.stdout.String(), a.stderr.String())
			}
		})
	}
}
