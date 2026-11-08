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

const oneSlice = "## Steps\n\n1. app: change main.go.\n\n## Test Plan\n\n1. main.go says v1.\n"

// work is a worker run that commits main.go with text and reports it.
func work(text string) runner.Canned {
	return runner.Canned{Out: text + "\n\nNot established: none\n", Work: commit(map[string]string{"main.go": text + "\n"}, text)}
}

// inReview takes a new ready issue #1 through the architect (plan) and,
// with the plan accepted, the worker's first run: #1 in-review on app#2.
func (a *applyWS) inReview(plan string) {
	a.t.Helper()
	if _, err := a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:ready"}); err != nil {
		a.t.Fatal(err)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: plan})
	a.pass()
	if a.Cfg().Gates.Plan {
		a.forge.SetLabel(issue1, "swarm:planned")
	}
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:in-review"}) {
		a.t.Fatalf("not in review: %v\n%s", got, a.stdout.String())
	}
}

func (a *applyWS) Cfg() declare.Config {
	c, _, _ := declare.Load(ops.Real().FS(), a.ws)
	return c
}

func (a *applyWS) bodies(n int) []string {
	var out []string
	for _, c := range a.issue(n).Comments {
		out = append(out, c.Body)
	}
	return out
}

// flaky is the git-following forge whose next Relabels fail, as a network
// error interrupting a collection does.
type flaky struct {
	gitForge
	relabels int
}

func (f *flaky) Relabel(at forge.Ref, remove, add string) error {
	if f.relabels > 0 {
		f.relabels--
		return &execErr{[]string{"forge: POST /repos/acme/app/issues/1/labels"}, "502 Bad Gateway"}
	}
	return f.gitForge.Relabel(at, remove, add)
}

// The round limit (section 6, T11) with the plan gate on and off: the
// first time to the architect's stage with the first note, the retried
// collection of that run not taken for a second time; the architect's
// revision with the open-slice sentence and the pull request in its
// prompt; the worker continuing the same pull request; the rounds counted
// again from the revised plan; the second time to blocked with the second
// note.
func TestSwarmApplyRoundLimit(t *testing.T) {
	for _, planGate := range []bool{true, false} {
		t.Run(map[bool]string{true: "plan gate on", false: "plan gate off"}[planGate], func(t *testing.T) {
			a := newApplyWS(t)
			a.edit(func(c *declare.Config) {
				c.MaxRounds = 1
				c.Gates.Plan = planGate
				if !planGate {
					c.Budget = &declare.Budget{PerDay: 10, Total: 100}
				}
			})
			a.inReview(oneSlice)
			a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges}, runner.Canned{Out: requestChanges})
			a.runner.Script(1, "sympathetic", runner.Canned{Out: approve}, runner.Canned{Out: approve})
			a.pass()
			a.tick()
			f := &flaky{gitForge: a.git(), relabels: 1}
			if err := a.apply(f, "--once"); err == nil {
				t.Fatal("the interrupted collection exited 0")
			}
			// The retry finds the note it posted, takes it for its own, and
			// relabels; the architect then answers the note in the same pass.
			a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nRevised.\n"})
			a.pass()
			first := mark("reconciler", 1) + "Review round 1 of 1 on app#2 requested changes (was swarm:in-review). The architect revises the plan from the reviews."
			stage := map[bool]string{true: "swarm:planning", false: "swarm:ready"}[planGate]
			if b := a.bodies(1); len(b) != 3 || b[1] != first || !strings.HasPrefix(b[2], mark("architect", 2)) {
				t.Fatalf("T11, first time: %q", b)
			}
			if !strings.Contains(a.result("1-sympathetic-1"), "relabelled swarm:in-review "+stage+"\n") || !strings.Contains(a.result("1-architect-2"), "label "+stage+"\n") {
				t.Errorf("results:\n%s\n%s", a.result("1-sympathetic-1"), a.result("1-architect-2"))
			}
			sys, prompt := a.file("1-architect-2", "io/system.md"), a.file("1-architect-2", "io/prompt.md")
			if !strings.Contains(sys, "Pull request app#2, slice 1, is open: it reached the review limit") || !strings.Contains(prompt, "# Pull request app#2: #1 slice 1: Drift check") ||
				!strings.Contains(prompt, "# Check\n\nmake check on ") {
				t.Errorf("the architect's revision:\n%s\n%s", sys, prompt)
			}
			if planGate {
				if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:planning", forge.Gated}) {
					t.Fatalf("the revision moved the label: %v", got)
				}
				a.forge.SetLabel(issue1, "swarm:planned") // T7
			} else if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:planned"}) {
				t.Fatalf("T6 after T11: %v", got)
			}
			a.runner.Script(1, "worker", work("v2"))
			a.pass()
			ps := a.pulls(1)
			cs, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 2})
			if len(ps) != 1 || len(cs) != 1 || !strings.HasPrefix(cs[0].Body, mark("worker", 2)+"v2") {
				t.Fatalf("the worker did not continue app#2: %+v %+v", ps, cs)
			}
			a.pass()
			a.pass()
			second := mark("reconciler", 2) + "Review round 1 of 1 on app#2 requested changes a second time (was swarm:in-review). Relabel the issue to resume."
			if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:blocked", forge.Gated}) || a.bodies(1)[len(a.bodies(1))-1] != second {
				t.Fatalf("T11, second time: %v %q", got, a.bodies(1))
			}
		})
	}
}

// Every gate off (section 13, Step 29's second row): the batch proposes
// #1 straight to ready (T2), the architect's plan is accepted as written
// (T6), the worker merges base first, both approvals, the merge step
// merges slice 1 and relabels planned (T14), then slice 2 and its merge
// close the issue (T15), with nothing from the maintainer; the budget is
// validated and not enforced.
func TestSwarmApplyGatesOff(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates = declare.Gates{Triage: false, Plan: false, Merge: false}
		c.Budget = &declare.Budget{PerDay: 0.01, Total: 0.01}
	})
	a.release()
	a.runner.Script(0, "owner", runner.Canned{Out: "## Drift check\nCheck the drift.\n", Stdout: claudeResult})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:ready"}) {
		t.Fatalf("T2: %v", got)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: twoSlices, Stdout: claudeResult})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:planned"}) {
		t.Fatalf("T6: %v", got)
	}
	if sys := a.file("1-architect-1", "io/system.md"); !strings.Contains(sys, "The plan is accepted as you write it: there is no revision round.") {
		t.Errorf("architect's Task:\n%s", sys)
	}
	for s := 1; s <= 2; s++ {
		a.runner.Script(1, "worker", work("v"+string(rune('0'+s))))
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass()
		if sys := a.file("1-worker-"+string(rune('0'+s)), "io/system.md"); !strings.Contains(sys, `Before anything else, run "git merge --no-edit main"; if it conflicts, write "BLOCKED: main conflicts" instead.`) {
			t.Errorf("worker's Task:\n%s", sys)
		}
		a.pass()
		a.pass() // T9
		a.pass() // the merge step
		want := map[int]string{1: "#1 merged pull app#2 (slice 1 of 2)\n#1 swarm:approved → swarm:planned\n", 2: "#1 merged pull app#3 (slice 2 of 2)\n#1 closed (last slice merged)\n#1 swarm:approved → swarm:done\n"}[s]
		if !strings.Contains(a.stdout.String(), want) {
			t.Fatalf("slice %d's merge step:\n%s", s, a.stdout.String())
		}
	}
	is := a.issue(1)
	ps := a.pulls(1)
	if is.Open || len(ps) != 2 || !ps[0].Merged || !ps[1].Merged || !reflect.DeepEqual(is.Labels, []string{forge.Done}) {
		t.Errorf("T15: issue %+v, pulls %+v", is, ps)
	}
	if _, err := os.Stat(a.state("paused")); err == nil {
		t.Error("the budget was enforced")
	}
}

// T4: with the triage gate off, the owner's answer on a proposed issue
// moves it to ready.
func TestSwarmApplyOwnerAnswersWithTriageOff(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Triage = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposed"})
	a.forge.Say(issue1, "jane", "Is this worth it?")
	a.runner.Script(1, "owner", runner.Canned{Out: "Yes: the whole job is the drift check.\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:ready"}) || !strings.HasPrefix(a.bodies(1)[1], mark("owner", 1)+"Yes") {
		t.Errorf("T4: %v %q", got, a.bodies(1))
	}
	if sys := a.file("1-owner-1", "io/system.md"); !strings.Contains(sys, "Then restate the whole job as it now stands: the architect plans from your answer.") {
		t.Errorf("owner's Task:\n%s", sys)
	}
}

// T16: with the merge gate off, a refused merge blocks the issue with the
// reconciler's note naming the pull request and the forge's message.
func TestSwarmApplyMergeRefused(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // T9
	a.forge.Unmergeable(forge.Ref{Repo: "app", N: 2})
	a.pass()
	head := a.pulls(1)[0].HeadSHA
	note := mark("reconciler", 1) + "merge of pull request acme/app#2 at " + head[:7] + " refused: 405 Please try again later (was swarm:approved). Resolve it and relabel the issue."
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:blocked", forge.Gated}) || a.bodies(1)[len(a.bodies(1))-1] != note ||
		!strings.Contains(a.stdout.String(), "#1 merge of pull app#2 refused (405 Please try again later)\n") {
		t.Errorf("T16: %v %q\n%s", got, a.bodies(1), a.stdout.String())
	}
}

// T16's note is posted unless it is already the issue's last word by the
// swarm (a retry): not when another login posted its text first, and again
// when the maintainer has answered and the merge is refused once more.
func TestSwarmApplyMergeRefusedNoteIsTheSwarms(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // T9
	head := a.pulls(1)[0].HeadSHA
	note := mark("reconciler", 1) + "merge of pull request acme/app#2 at " + head[:7] + " refused: 405 Please try again later (was swarm:approved). Resolve it and relabel the issue."
	a.forge.Say(issue1, "mallory", note)
	a.forge.Unmergeable(forge.Ref{Repo: "app", N: 2})
	a.pass()
	is := a.issue(1)
	if last := is.Comments[len(is.Comments)-1]; last.Author != "swarm-bot" || last.Body != note {
		t.Errorf("after another login's copy, the last word is %s's", last.Author)
	}
	a.forge.Say(issue1, "jane", "Try once more.")
	a.forge.SetLabel(issue1, "swarm:approved")
	a.pass()
	is = a.issue(1)
	if last := is.Comments[len(is.Comments)-1]; last.Author != "swarm-bot" || last.Body != note || !reflect.DeepEqual(labels(is), []string{"swarm:blocked", forge.Gated}) {
		t.Errorf("refused again: the last word is %s's, labels %v", last.Author, labels(is))
	}
}

// The pull request carries the issue's stage (section 5): copied by the
// next pass when the maintainer moves the issue, the maintainer's own
// labels on it kept.
func TestSwarmApplyCopiesTheLabel(t *testing.T) {
	a := newApplyWS(t)
	a.forge.AddLabels("needs-docs")
	a.inReview(oneSlice)
	pr := forge.Ref{Repo: "app", N: 2}
	a.forge.Relabel(pr, "", "needs-docs")
	a.forge.SetLabel(issue1, "swarm:blocked")
	a.pass()
	if got := a.pulls(1)[0].Labels; !reflect.DeepEqual(got, []string{"needs-docs", "swarm:blocked"}) || !strings.Contains(a.stdout.String(), "#1 pull app#2: swarm:blocked\n") {
		t.Errorf("labels %v\n%s", got, a.stdout.String())
	}
}
