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
	if _, err := a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"}); err != nil {
		a.t.Fatal(err)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: plan})
	a.pass()
	if a.Cfg().Gates.Plan {
		a.forge.SetLabel(issue1, "swarm:plan-1-accepted")
	}
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) {
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
		if !strings.HasPrefix(c.Body, gateNote) {
			out = append(out, c.Body)
		}
	}
	return out
}

// gateNote is the gate note's marker line (section 5), which
// TestSwarmApplyGateNotes pins; bodies leaves the notes out.
const gateNote = "<!-- daisy-swarm role=reconciler kind=gate -->\n"

// flaky is the git-following forge whose next Relabels fail (with to,
// only those adding to), as a network error interrupting a collection
// does.
type flaky struct {
	gitForge
	relabels int
	to       string
}

func (f *flaky) Relabel(at forge.Ref, remove, add string) error {
	if f.relabels > 0 && (f.to == "" || add == f.to) {
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
// again from the revised plan; the second time blocked at
// review-1-in-progress, where the pair completed, with the second note.
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
			stage := map[bool]string{true: "swarm:plan-0-posted", false: "swarm:proposal-accepted"}[planGate]
			a.tick()
			f := &flaky{gitForge: a.git(), relabels: 1, to: stage}
			if err := a.apply(f, "--once"); err == nil { // the pair in one pass, T11's relabel refused
				t.Fatal("the interrupted collection exited 0")
			}
			// The retry finds the note it posted, takes it for its own, and
			// relabels; the architect then answers the note in the same pass.
			a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nRevised.\n"})
			a.pass()
			first := mark("reconciler", 1) + "Review round 1 of 1 on app#2 requested changes (was swarm:review-1-in-progress). The architect revises the plan from the reviews."
			if b := a.bodies(1); len(b) != 3 || b[1] != first || !strings.HasPrefix(b[2], mark("architect", 2)) {
				t.Fatalf("T11, first time: %q", b)
			}
			if !strings.Contains(a.result("1-sympathetic-1"), "relabelled swarm:review-1-in-progress "+stage+"\n") || !strings.Contains(a.result("1-architect-2"), "label "+stage+"\n") {
				t.Errorf("results:\n%s\n%s", a.result("1-sympathetic-1"), a.result("1-architect-2"))
			}
			sys, prompt := a.file("1-architect-2", "io/system.md"), a.file("1-architect-2", "io/prompt.md")
			if !strings.Contains(sys, "Pull request app#2, slice 1, is open: it reached the review limit") || !strings.Contains(prompt, "# Pull request app#2: #1 slice 1: Drift check") ||
				!strings.Contains(prompt, "# Check\n\nmake check on ") {
				t.Errorf("the architect's revision:\n%s\n%s", sys, prompt)
			}
			if planGate {
				if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", forge.Gated}) {
					t.Fatalf("the revision moved the label: %v", got)
				}
				a.forge.SetLabel(issue1, "swarm:plan-1-accepted") // T7
			} else if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) {
				t.Fatalf("T6 after T11: %v", got)
			}
			a.runner.Script(1, "worker", work("v2"))
			a.pass()
			ps := a.pulls(1)
			cs, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 2})
			if len(ps) != 1 || len(cs) != 1 || !strings.HasPrefix(cs[0].Body, mark("worker", 2)+"v2") {
				t.Fatalf("the worker did not continue app#2: %+v %+v", ps, cs)
			}
			a.pass() // the pair
			second := mark("reconciler", 2) + "Review round 1 of 1 on app#2 requested changes a second time (at swarm:review-1-in-progress). Relabel the issue swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then remove swarm:blocked."
			if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress", "swarm:blocked", forge.Gated}) || a.bodies(1)[len(a.bodies(1))-1] != second ||
				!strings.Contains(a.result("1-sympathetic-2"), "\nflagged swarm:blocked\noutcome posted\n") {
				t.Fatalf("T11, second time: %v %q", got, a.bodies(1))
			}
		})
	}
}

// A revision that needs no commit (section 7.6): the worker on
// review-3-changes-requested ending with no commit and NO CHANGE: as its
// first line is collected as posted: its report is the pull request's
// comment, T12 takes the issue to review-0-ready, and the pair reviews
// the unchanged head again, as round 2, through to T9. The same run
// without the line, or the line on plan-1-accepted, is the failure no
// commits.
func TestSwarmApplyNoChange(t *testing.T) {
	const answer = "NO CHANGE: main.go:1 says v1, as the plan's Test Plan wants.\n\nNot established: none\n"
	toChanges := func(t *testing.T) *applyWS {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass() // the pair, then T10
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested"}) {
			t.Fatalf("T10: %v", got)
		}
		return a
	}
	t.Run("answered", func(t *testing.T) {
		a := toChanges(t)
		head := a.pulls(1)[0].HeadSHA
		a.runner.Script(1, "worker", runner.Canned{Out: answer})
		a.pass()
		cs, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 2})
		r := a.result("1-worker-2")
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) || len(cs) != 1 || cs[0].Body != mark("worker", 2)+answer ||
			a.pulls(1)[0].HeadSHA != head || strings.Contains(r, "failed") || strings.Contains(r, "pushed") || !strings.HasSuffix(r, "relabelled swarm:plan-2-in-progress swarm:review-0-ready\noutcome posted\ncollected\n") {
			t.Fatalf("T12 without a commit: %v %q\n%s", got, cs, r)
		}
		if !strings.Contains(a.stdout.String(), "#1 worker run 2: posted comment\n#1 worker run 2: swarm:plan-2-in-progress → swarm:review-0-ready\n") {
			t.Errorf("stdout:\n%s", a.stdout.String())
		}
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass() // the pair, round 2, on the same head: T9
		if sys := a.file("1-adversary-2", "io/system.md"); !strings.Contains(sys, "This is round 2, on the head you reviewed before ("+head+")") {
			t.Fatalf("round 2's Task:\n%s", sys)
		}
		if r := a.result("1-adversary-2"); !strings.HasSuffix(r, "posted review\noutcome posted\ncollected\n") {
			t.Fatalf("the pair completed on the adversary's review of the answer alone:\n%s", r)
		}
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) {
			t.Errorf("T9 on the answer: %v\n%s", got, a.stdout.String())
		}
	})
	t.Run("not answered", func(t *testing.T) {
		a := toChanges(t)
		a.runner.Script(1, "worker", runner.Canned{Out: "Nothing to change.\n" + answer})
		a.pass()
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-3-changes-requested", forge.Blocked, forge.Gated}) || !strings.Contains(a.result("1-worker-2"), "\nfailed: no commits\n") {
			t.Errorf("no commit and no NO CHANGE: line: %v\n%s", got, a.result("1-worker-2"))
		}
	})
	t.Run("on plan-1-accepted", func(t *testing.T) {
		a := newApplyWS(t)
		a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:plan-1-accepted"})
		a.runner.Script(1, "worker", runner.Canned{Out: answer})
		a.pass()
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted", forge.Blocked, forge.Gated}) || !strings.Contains(a.result("1-worker-1"), "\nfailed: no commits\n") {
			t.Errorf("NO CHANGE: on a first build: %v\n%s", got, a.result("1-worker-1"))
		}
	})
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
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted", forge.ClassArchitecture}) { // unclassed
		t.Fatalf("T2: %v", got)
	}
	a.runner.Script(1, "architect", runner.Canned{Out: twoSlices, Stdout: claudeResult})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted", forge.ClassArchitecture}) {
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
		a.pass() // the pair: T9
		a.pass() // the merge step
		want := map[int]string{1: "#1 merged pull app#2 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n", 2: "#1 merged pull app#3 (slice 2 of 2)\n#1 closed (last slice merged)\n#1 swarm:review-2-approved → swarm:done\n"}[s]
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
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-posted"})
	a.forge.Say(issue1, "jane", "Is this worth it?")
	a.runner.Script(1, "owner", runner.Canned{Out: "Yes: the whole job is the drift check.\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted"}) || !strings.HasPrefix(a.bodies(1)[1], mark("owner", 1)+"Yes") {
		t.Errorf("T4: %v %q", got, a.bodies(1))
	}
	if sys := a.file("1-owner-1", "io/system.md"); !strings.Contains(sys, "Then restate the whole job as it now stands: the architect plans from your answer.") {
		t.Errorf("owner's Task:\n%s", sys)
	}
}

// T16: with the merge gate off, a refused merge blocks the issue at
// review-2-approved with the reconciler's note naming the pull request and
// the forge's message; the merge step skips it while it is blocked.
func TestSwarmApplyMergeRefused(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9
	a.forge.Unmergeable(forge.Ref{Repo: "app", N: 2})
	a.pass()
	head := a.pulls(1)[0].HeadSHA
	note := mark("reconciler", 1) + "merge of pull request acme/app#2 at " + head[:7] + " refused: 405 Please try again later (at swarm:review-2-approved). Resolve it, then remove swarm:blocked."
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", "swarm:blocked", forge.Gated}) || a.bodies(1)[len(a.bodies(1))-1] != note ||
		!strings.Contains(a.stdout.String(), "#1 merge of pull app#2 refused (405 Please try again later)\n#1 blocked\n") {
		t.Errorf("T16: %v %q\n%s", got, a.bodies(1), a.stdout.String())
	}
	a.pass()
	if strings.Contains(a.stdout.String(), "merge of pull") || len(a.bodies(1)) != 2 {
		t.Errorf("the merge step tried a blocked issue again:\n%s", a.stdout.String())
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
	a.pass() // the pair: T9
	head := a.pulls(1)[0].HeadSHA
	note := mark("reconciler", 1) + "merge of pull request acme/app#2 at " + head[:7] + " refused: 405 Please try again later (at swarm:review-2-approved). Resolve it, then remove swarm:blocked."
	a.forge.Say(issue1, "mallory", note)
	a.forge.Unmergeable(forge.Ref{Repo: "app", N: 2})
	a.pass()
	is := a.issue(1)
	if last := is.Comments[len(is.Comments)-2]; last.Author != "swarm-bot" || last.Body != note || !strings.HasPrefix(is.Comments[len(is.Comments)-1].Body, gateNote) {
		t.Errorf("after another login's copy, the last word before the gate note is %s's", last.Author)
	}
	a.forge.Say(issue1, "jane", "Try once more.")
	a.forge.Relabel(issue1, forge.Blocked, "")
	a.pass()
	is = a.issue(1)
	if last := is.Comments[len(is.Comments)-1]; last.Author != "swarm-bot" || last.Body != note || !reflect.DeepEqual(labels(is), []string{"swarm:review-2-approved", "swarm:blocked", forge.Gated}) {
		t.Errorf("refused again: the last word is %s's, labels %v", last.Author, labels(is))
	}
}

// The pull request carries the issue's stage (section 5): copied by the
// next pass when the maintainer moves the issue, the maintainer's own
// labels on it kept, the issue's flags not copied.
func TestSwarmApplyCopiesTheLabel(t *testing.T) {
	a := newApplyWS(t)
	a.forge.AddLabels("needs-docs")
	a.inReview(oneSlice)
	pr := forge.Ref{Repo: "app", N: 2}
	a.forge.Relabel(pr, "", "needs-docs")
	a.forge.SetLabel(issue1, "swarm:plan-0-posted")
	a.forge.Relabel(issue1, "", forge.Blocked)
	a.pass()
	if got := a.pulls(1)[0].Labels; !reflect.DeepEqual(got, []string{"needs-docs", "swarm:plan-0-posted"}) || !strings.Contains(a.stdout.String(), "#1 pull app#2: swarm:plan-0-posted\n") {
		t.Errorf("labels %v\n%s", got, a.stdout.String())
	}
}
