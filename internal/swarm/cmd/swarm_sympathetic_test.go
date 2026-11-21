package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// The sympathetic reviewer after the adversary approves (decision
// [163](docs/swarm/decisions/0163-the-sympathetic-reviewer-after-the-adversary-approves.md);
// section 5's stages and T9–T11, section 6's role rule and rounds): on
// review-0-ready the adversary alone launches; the sympathetic reviewer
// launches on the head once the adversary's approval of it is
// collected, in the pass that collects it, and its review completes the
// pair; an adversary's request for changes completes the pair with its
// one review (T10), and the round limit counts such a round as any
// other (T11 at max_rounds).
func TestSwarmSympatheticAfterTheAdversaryApproves(t *testing.T) {
	t.Parallel()
	t.Run("the adversary first, then the sympathetic reviewer", func(t *testing.T) {
		t.Parallel()
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass() // the adversary alone; its approval collected by the drain
		if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary"}) {
			t.Fatalf("launched %q", got)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress"}) {
			t.Errorf("the adversary's approval: %v", got)
		}
		a.pass() // with --once, the next pass launches the sympathetic reviewer
		if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "adversary", "sympathetic"}) {
			t.Fatalf("launched %q", got)
		}
		if r := a.result("1-sympathetic-1"); !strings.Contains(r, "\nlabel swarm:review-1-in-progress\n") ||
			!strings.Contains(r, "\nrelabelled swarm:review-1-in-progress swarm:review-2-approved\n") {
			t.Errorf("the sympathetic reviewer's run:\n%s", r)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) {
			t.Errorf("T9: %v", got)
		}
	})
	t.Run("launched in the pass that collects the approval", func(t *testing.T) {
		t.Parallel()
		a := newApplyWS(t)
		a.inReview(oneSlice)
		// The adversary's run whose member finished after its apply
		// died: the next pass collects it, then launches.
		head := a.pulls(1)[0].HeadSHA
		run := a.state("runs/1-adversary-1")
		os.MkdirAll(filepath.Join(run, "io"), 0o755)
		os.WriteFile(filepath.Join(run, "io", "out.md"), []byte(approve), 0o644)
		os.WriteFile(filepath.Join(run, "result"), []byte("start "+head+"\nlabel swarm:review-0-ready\nstarted 2026-11-06T15:02:00Z\ncredential anthropic env CLAUDE_CODE_OAUTH_TOKEN\nround 1\nrelabelled swarm:review-0-ready swarm:review-1-in-progress\n"), 0o644)
		a.forge.SetLabel(issue1, "swarm:review-1-in-progress")
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass()
		out := a.stdout.String()
		posted, launched := strings.Index(out, "#1 adversary run 1: posted review (approve)\n"), strings.Index(out, "#1 sympathetic run 1: launched\n")
		if posted < 0 || launched < posted || strings.Count(out, "Pass ") != 1 {
			t.Errorf("not collected then launched in one pass:\n%s", out)
		}
		if got := a.launchedRoles(); !reflect.DeepEqual(got, []string{"architect", "worker", "check", "sympathetic"}) {
			t.Errorf("launched %q", got)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) {
			t.Errorf("T9: %v", got)
		}
	})
	t.Run("an adversary's request for changes, then the round limit", func(t *testing.T) {
		t.Parallel()
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.MaxRounds = 2 })
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: requestChanges})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass() // the adversary alone: T10 with its one review
		reviews, _ := a.forge.Reviews(forge.Ref{Repo: "app", N: 1})
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested"}) || len(reviews) != 1 ||
			!strings.Contains(a.result("1-adversary-1"), "\nrelabelled swarm:review-1-in-progress swarm:review-3-changes-requested\n") {
			t.Fatalf("T10: %v, %d reviews\n%s", got, len(reviews), a.result("1-adversary-1"))
		}
		a.runner.Script(1, "worker", work("v2"))
		a.pass() // T12
		a.pass() // round 2: the adversary alone again, at max_rounds: T11
		if sys := a.file("1-adversary-2", "io/system.md"); !strings.Contains(sys, "This is round 2.") {
			t.Errorf("round 2's Task:\n%s", sys)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", forge.Gated}) {
			t.Errorf("T11: %v", got)
		}
		if b := a.bodies(1); len(b) == 0 || !strings.HasPrefix(b[len(b)-1], mark("reconciler", 2)+"Review round 2 of 2 on "+appPull("1")+" requested changes") {
			t.Errorf("the round limit's note: %q", b)
		}
		for _, r := range a.launchedRoles() {
			if r == "sympathetic" {
				t.Errorf("the sympathetic reviewer launched on a head the adversary sent back: %q", a.launchedRoles())
			}
		}
	})
	t.Run("the round limit's second time, reached by the other reviewer's first run", func(t *testing.T) {
		// The first note is adversary run 1's, the second sympathetic run
		// 1's, both marked run=1 and both of round 1 of 1: the second is
		// not taken for the run's own first.
		t.Parallel()
		a := newApplyWS(t)
		a.edit(func(c *declare.Config) { c.MaxRounds = 1 })
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
		a.pass() // the adversary alone: T11, the first time
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", forge.Gated}) {
			t.Fatalf("T11, first time: %v", got)
		}
		a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nRevised.\n"})
		a.pass()
		a.forge.SetLabel(issue1, "swarm:plan-1-accepted") // T7
		a.runner.Script(1, "worker", work("v2"))
		a.pass()
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: requestChanges})
		a.pass() // the adversary approves
		a.pass() // the sympathetic reviewer's request for changes: T11, the second time
		again := " requested changes again (at swarm:review-1-in-progress). Relabel the issue swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then remove swarm:blocked."
		second := mark("reconciler", 1) + "Review round 1 of 1 on " + appPull("1") + again + "\n\ncc @jane"
		if got, b := labels(a.issue(1)), a.bodies(1); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress", "swarm:blocked", forge.Gated}) || b[len(b)-1] != second {
			t.Errorf("T11, second time: %v %q\n%s", got, b, a.result("1-sympathetic-1"))
		}
	})
}
