package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

const oneSlice = "## Steps\n\n1. app: change main.go.\n\n## Test Plan\n\n1. main.go says v1.\n"

// work is a worker run that commits main.go with text and reports it.
func work(text string) runner.Canned {
	return runner.Canned{Out: text + "\n\nNot established: none\n", Work: checkout(commit(map[string]string{"main.go": text + "\n"}, text))}
}

// checkout is f run in the member's one checkout, the worker's or a
// reviewer's, under the folder of checkouts mounted at /job/work.
func checkout(f func(string) error) func(string) error {
	return func(work string) error {
		es, err := os.ReadDir(work)
		if err != nil {
			return err
		}
		if len(es) != 1 {
			return fmt.Errorf("%d checkouts in %s", len(es), work)
		}
		return f(filepath.Join(work, es[0].Name()))
	}
}

// in is f run in the member's checkout of the repository name, under the
// folder of checkouts mounted at /job/work (section 7.8).
func in(name string, f func(string) error) func(string) error {
	return func(work string) error { return f(filepath.Join(work, name)) }
}

// inReview takes a new ready issue #1 through the architect (plan) and,
// with the plan accepted, the worker's first run: #1 in-review on app#1.
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
	c, _, _ := declare.Load(ops.Real().FS(), a.place.Root)
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

// The round limit (section 5, T11) with the plan gate on and off: the
// first time to the architect's stage with the first note, the retried
// collection of that run not taken for a second time; the architect's
// revision with the open-slice sentence and the pull request in its
// prompt; the worker continuing the same pull request; the rounds counted
// again from the revised plan; the second time blocked at
// review-1-in-progress, where the pair completed, with the second note.
// Each round is completed by the adversary's request-changes alone, with
// no sympathetic run; or, the adversary approving each round and the
// sympathetic reviewer then requesting changes, the maintainer having
// moved the issue during that second reviewer's run, not blocked, the
// second note saying so.
func TestSwarmApplyRoundLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ planGate, moved bool }{{true, false}, {false, false}, {true, true}} {
		planGate := tc.planGate
		name := map[bool]string{true: "plan gate on", false: "plan gate off"}[planGate]
		if tc.moved {
			name += ", moved during the run"
		}
		t.Run(name, func(t *testing.T) {
			a := newApplyWS(t)
			a.edit(func(c *declare.Config) {
				c.MaxRounds = 1
				c.Gates.Plan = planGate
				if !planGate {
					c.Budget = &declare.Budget{PerDay: 10, Total: 100}
				}
			})
			a.inReview(oneSlice)
			completing := "1-adversary-1" // the run that completes the round
			if tc.moved {
				completing = "1-sympathetic-1"
				a.runner.Script(1, "adversary", runner.Canned{Out: approve})
				a.runner.Script(1, "sympathetic", runner.Canned{Out: requestChanges})
				a.pass() // the adversary, approving
			} else {
				a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
			}
			stage := map[bool]string{true: "swarm:plan-0-posted", false: "swarm:proposal-accepted"}[planGate]
			a.tick()
			f := &flaky{gitForge: a.git(), relabels: 1, to: stage}
			if err := a.apply(f, "--once"); err == nil { // the run completing the pair, T11's relabel refused
				t.Fatal("the interrupted collection exited 0")
			}
			// The retry finds the note it posted, takes it for its own, and
			// relabels; the architect then answers the note in the same pass,
			// the pull request's head a branch of its clone of app (7.1).
			var archHead string
			a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nRevised.\n", Work: func(work string) error {
				out, errb, err := ops.Real().Exec(filepath.Join(work, "app"), "git", "rev-parse", "--verify", "swarm/1-1")
				if err != nil {
					return &execErr{[]string{"rev-parse", "swarm/1-1"}, string(errb)}
				}
				archHead = strings.TrimSpace(string(out))
				return nil
			}})
			a.pass()
			if head := a.pulls(1)[0].HeadSHA; archHead != head {
				t.Fatalf("the architect's swarm/1-1 is %q, app#1's head %q:\n%s", archHead, head, a.result("1-architect-2"))
			}
			first := mark("reconciler", 1) + "Review round 1 of 1 on [acme/app#1](https://forge.example.net/acme/app/pulls/1) requested changes (was swarm:review-1-in-progress). The architect revises the plan from the reviews."
			if b := a.bodies(1); len(b) != 3 || b[1] != first || !strings.HasPrefix(b[2], mark("architect", 2)) {
				t.Fatalf("T11, first time: %q", b)
			}
			if !strings.Contains(a.result(completing), "relabelled swarm:review-1-in-progress "+stage+"\n") || !strings.Contains(a.result("1-architect-2"), "label "+stage+"\n") {
				t.Errorf("results:\n%s\n%s", a.result(completing), a.result("1-architect-2"))
			}
			sys, prompt := a.file("1-architect-2", "io/system.md"), a.file("1-architect-2", "io/prompt.md")
			if !strings.Contains(sys, "Pull request app#1, slice 1, is open: it reached the review limit") || !strings.Contains(prompt, "# Pull request app#1: acme/swarm-app#1 slice 1: Drift check") ||
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
			cs, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
			if len(ps) != 1 || len(cs) != 1 || !strings.HasPrefix(cs[0].Body, mark("worker", 2)+"v2") {
				t.Fatalf("the worker did not continue app#1: %+v %+v", ps, cs)
			}
			if tc.moved {
				a.runner.Script(1, "adversary", runner.Canned{Out: approve})
				a.runner.Script(1, "sympathetic", runner.Canned{Out: requestChanges})
				a.pass() // the adversary, approving
				a.tick()
				// The sympathetic reviewer, once the adversary approved, its
				// review the pass's first: the issue moved to review-0-ready
				// under it, the pair's second reviewer, launched on
				// review-1-in-progress.
				if err := a.apply(&moveOnReview{gitForge: a.git(), reviews: 1}, "--once"); err != nil {
					t.Fatal(err)
				}
				second := mark("reconciler", 2) + "Review round 1 of 1 on [acme/app#1](https://forge.example.net/acme/app/pulls/1) requested changes again (at swarm:review-1-in-progress). The issue was moved to swarm:review-0-ready during the run, so it is not blocked."
				if got := labels(a.issue(1)); slices.Contains(got, forge.Blocked) || a.bodies(1)[len(a.bodies(1))-1] != second {
					t.Fatalf("T11, second time, moved: %v %q", got, a.bodies(1))
				}
				// The note blocked nothing: status counts no block for it,
				// and with the issue blocked by hand where it was moved,
				// unblock does not take it for the round limit's.
				if err := a.status(a.forge); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(a.stdout.String(), "\n  #1           plan revisions 1, blocks 0; app#1 rounds ") {
					t.Errorf("status:\n%s", a.stdout.String())
				}
				if err := a.forge.Relabel(issue1, "", forge.Blocked); err != nil {
					t.Fatal(err)
				}
				m := &maintainerWS{statusWS: a.statusWS, as: map[string]*forge.Fake{"jane-456": a.forge.As(forge.Account{Login: "jane"})}}
				m.secrets(bothTokens)
				m.ok(swarmUnblock, "Maintainer jane on acme/swarm-app\n#1 commented on the issue\n#1 blocked cleared\n", "1", "Review", "it", "again.")
				return
			}
			a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
			a.pass() // the adversary's request-changes, completing the pair alone
			again := " requested changes again (at swarm:review-1-in-progress). Relabel the issue swarm:review-3-changes-requested for one more fix round or swarm:plan-0-posted to re-plan, then remove swarm:blocked."
			second := mark("reconciler", 2) + "Review round 1 of 1 on " + appPull("1") + again + "\n\ncc @jane"
			if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-1-in-progress", "swarm:blocked", forge.Gated}) || a.bodies(1)[len(a.bodies(1))-1] != second ||
				!strings.Contains(a.result("1-adversary-2"), "\nflagged swarm:blocked\nrecorded\noutcome posted\n") {
				t.Fatalf("T11, second time: %v %q", got, a.bodies(1))
			}
			// The maintainer's one more fix round, and the limit a third
			// time: the same note as the second's, which counts no time.
			a.forge.SetLabel(issue1, "swarm:review-3-changes-requested")
			if err := a.forge.Relabel(issue1, forge.Blocked, ""); err != nil {
				t.Fatal(err)
			}
			a.runner.Script(1, "worker", work("v3"))
			a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
			a.pass() // the worker
			a.pass() // the adversary's request-changes, completing the pair alone
			third := mark("reconciler", 3) + "Review round 2 of 1 on " + appPull("1") + again + "\n\ncc @jane"
			if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || a.bodies(1)[len(a.bodies(1))-1] != third {
				t.Fatalf("T11, third time: %v %q", got, a.bodies(1))
			}
		})
	}
}

// Whose pull requests are the issue's (section 10, Pulls): one an
// untrusted login opens on the next slice's branch with the Refs line is
// never the current pull request, so the pair reviews the swarm's.
func TestSwarmApplyPullOpeners(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.inReview(oneSlice)
	evil, _ := a.forge.As(forge.Account{Login: "mallory"}).CreatePull("app", "swarm/1-2", "main", "slice", "Refs acme/swarm-app#1")
	a.forge.Push(forge.Ref{Repo: "app", N: evil}, "e1")
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary, approving
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	ours, _ := a.forge.Reviews(forge.Ref{Repo: "app", N: 1})
	theirs, _ := a.forge.Reviews(forge.Ref{Repo: "app", N: evil})
	if sys := a.file("1-adversary-1", "io/system.md"); !strings.Contains(sys, "Review pull request app#1, slice 1 of issue #1.") || len(ours) != 2 || len(theirs) != 0 {
		t.Errorf("reviews: app#1 %d, app#%d %d\n%s", len(ours), evil, len(theirs), sys)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) {
		t.Errorf("labels %v", got)
	}
}

// The round limit with the pull request's branch gone from the forge:
// the architect's clone cannot make the head's branch, the swarm fault
// clone failed (sections 7.1 and 7.7), nothing launched.
func TestSwarmApplyRoundLimitHeadGone(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxRounds = 1 })
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.pass() // the adversary's request-changes at the round limit, completing the pair alone: T11 to plan-0-posted
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nRevised.\n"})
	if _, errb, err := ops.Real().Exec(a.upstream("app"), "git", "update-ref", "-d", "refs/heads/swarm/1-1"); err != nil {
		t.Fatal(string(errb))
	}
	a.pass()
	res := a.result("1-architect-2")
	if !strings.Contains(res, "launch-failed ") || !strings.Contains(res, "\nfault: clone: git branch swarm/1-1 origin/swarm/1-1: ") || !strings.HasSuffix(res, "outcome fault\ncollected\n") {
		t.Fatalf("the architect without the head's branch:\n%s\n%s", res, a.stdout.String())
	}
	if n := len(a.runner.Launched); a.runner.Launched[n-1].Name != "daisy-swarm.acme.swarm-app.1.adversary.1" {
		t.Errorf("launched after the fault: %+v", a.runner.Launched[n-1])
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
	t.Parallel()
	const answer = "NO CHANGE: main.go:1 says v1, as the plan's Test Plan wants.\n\nNot established: none\n"
	toChanges := func(t *testing.T) *applyWS {
		a := newApplyWS(t)
		a.inReview(oneSlice)
		a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
		a.pass() // the adversary's request-changes, completing the pair alone: T10
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
		cs, _ := a.forge.PullComments(forge.Ref{Repo: "app", N: 1})
		r := a.result("1-worker-2")
		if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-0-ready"}) || len(cs) != 1 || cs[0].Body != mark("worker", 2)+answer ||
			a.pulls(1)[0].HeadSHA != head || strings.Contains(r, "failed") || strings.Contains(r, "pushed") || !strings.HasSuffix(r, "relabelled swarm:plan-2-in-progress swarm:review-0-ready\nrecorded\noutcome posted\ncollected\n") {
			t.Fatalf("T12 without a commit: %v %+v\n%s", got, cs, r)
		}
		if !strings.Contains(a.stdout.String(), "#1 worker run 2: posted comment\n#1 worker run 2: swarm:plan-2-in-progress → swarm:review-0-ready\n") {
			t.Errorf("stdout:\n%s", a.stdout.String())
		}
		a.runner.Script(1, "adversary", runner.Canned{Out: approve})
		a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
		a.pass() // the adversary, round 2, on the same head, approving
		if sys := a.file("1-adversary-2", "io/system.md"); !strings.Contains(sys, "This is round 2, on the head you reviewed before ("+head+")") {
			t.Fatalf("round 2's Task:\n%s", sys)
		}
		if r := a.result("1-adversary-2"); !strings.HasSuffix(r, "posted review\nrecorded\noutcome posted\ncollected\n") {
			t.Fatalf("the pair completed on the adversary's review of the answer alone:\n%s", r)
		}
		a.pass() // the sympathetic reviewer, once the adversary approved: T9
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

// The reviewers' turn reads the worker's answers without a commit from
// the pull request's comments alone, as the collection does (section 6):
// a description that begins NO CHANGE: (a first report, text as any
// other, section 7.5) is no answer, so the adversary's review of the
// head, made in the same second, still stands and only the sympathetic
// is launched.
func TestSwarmApplyDescriptionIsNoAnswer(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	if _, err := a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:plan-1-accepted"}); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "worker", work("NO CHANGE: v1 is new"))
	a.pass()
	pr := forge.Ref{Repo: "app", N: 1}
	if p := a.pulls(1); len(p) != 1 || !strings.Contains(p[0].Body, "NO CHANGE: v1 is new") {
		t.Fatalf("the description: %+v\n%s", p, a.stdout.String())
	}
	a.forge.SetLabel(issue1, "swarm:review-1-in-progress")
	a.forge.Review(pr, forge.Commented, mark("adversary", 1)+approve)
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if strings.Contains(a.stdout.String(), "adversary run 2") || !strings.Contains(a.stdout.String(), "#1 sympathetic run 1: launched") {
		t.Errorf("stdout:\n%s", a.stdout.String())
	}
}

// Every gate off (section 13, Step 29's second row): the batch proposes
// #1 straight to ready (T2), the architect's plan is accepted as written
// (T6), the worker merges base first, both approvals, the merge step
// merges slice 1 and relabels planned (T14), then slice 2 and its merge
// close the issue (T15), with nothing from the maintainer; the budget is
// validated and not enforced.
func TestSwarmApplyGatesOff(t *testing.T) {
	t.Parallel()
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
		if sys := a.file("1-worker-"+string(rune('0'+s)), "io/system.md"); !strings.Contains(sys, `Before anything else, run "git merge --no-edit main"; if it conflicts, resolve each conflict, keeping what both sides meant, and commit the merge, or, if you cannot, write "BLOCKED: main conflicts in <files>" instead, naming the conflicting files.`) {
			t.Errorf("worker's Task:\n%s", sys)
		}
		a.pass() // the adversary, approving
		a.pass() // the sympathetic reviewer, once the adversary approved: T9
		a.pass() // the merge step
		want := map[int]string{1: "#1 merged pull app#1 (slice 1 of 2)\n#1 swarm:review-2-approved → swarm:plan-1-accepted\n", 2: "#1 merged pull app#2 (slice 2 of 2)\n#1 closed (last slice merged)\n#1 swarm:review-2-approved → swarm:done\n"}[s]
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
	t.Parallel()
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
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary, approving
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	a.forge.RefuseMerge(forge.Ref{Repo: "app", N: 1})
	a.pass()
	head := a.pulls(1)[0].HeadSHA
	note := mark("reconciler", 1) + "merge of pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) at " + head[:7] + " refused: 405 not allowed to merge (at swarm:review-2-approved). Resolve it, then remove swarm:blocked.\n\ncc @jane"
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", "swarm:blocked", forge.Gated}) || a.bodies(1)[len(a.bodies(1))-1] != note ||
		!strings.Contains(a.stdout.String(), "#1 merge of pull app#1 refused (405 not allowed to merge)\n#1 blocked\n") {
		t.Errorf("T16: %v %q\n%s", got, a.bodies(1), a.stdout.String())
	}
	a.pass()
	if strings.Contains(a.stdout.String(), "merge of pull") || len(a.bodies(1)) != 2 {
		t.Errorf("the merge step tried a blocked issue again:\n%s", a.stdout.String())
	}
}

// T16 with no head both reviews approve (the issue relabelled
// review-2-approved by hand): the note names no head.
func TestSwarmApplyMergeRefusedNoHead(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.forge.SetLabel(issue1, "swarm:review-2-approved")
	a.pass()
	note := mark("reconciler", 1) + "merge of pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) refused: no head both reviews approve (at swarm:review-2-approved). Resolve it, then remove swarm:blocked.\n\ncc @jane"
	if b := a.bodies(1); b[len(b)-1] != note || !strings.Contains(a.stdout.String(), "#1 merge of pull app#1 refused (no head both reviews approve)\n#1 blocked\n") {
		t.Errorf("%q\n%s", b[len(b)-1], a.stdout.String())
	}
}

// T16's note is posted unless it is already the issue's last word by the
// swarm (a retry): not when another login posted its text first, and again
// when the maintainer has answered and the merge is refused once more.
func TestSwarmApplyMergeRefusedNoteIsTheSwarms(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) {
		c.Gates.Merge = false
		c.Budget = &declare.Budget{PerDay: 10, Total: 100}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the adversary, approving
	a.pass() // the sympathetic reviewer, once the adversary approved: T9
	head := a.pulls(1)[0].HeadSHA
	note := mark("reconciler", 1) + "merge of pull request [acme/app#1](https://forge.example.net/acme/app/pulls/1) at " + head[:7] + " refused: 405 not allowed to merge (at swarm:review-2-approved). Resolve it, then remove swarm:blocked.\n\ncc @jane"
	a.forge.Say(issue1, "mallory", note)
	a.forge.RefuseMerge(forge.Ref{Repo: "app", N: 1})
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
	t.Parallel()
	a := newApplyWS(t)
	a.forge.AddLabels("needs-docs")
	a.inReview(oneSlice)
	pr := forge.Ref{Repo: "app", N: 1}
	a.forge.Relabel(pr, "", "needs-docs")
	a.forge.SetLabel(issue1, "swarm:plan-0-posted")
	a.forge.Relabel(issue1, "", forge.Blocked)
	a.pass()
	if got := a.pulls(1)[0].Labels; !reflect.DeepEqual(got, []string{"needs-docs", "swarm:plan-0-posted"}) || !strings.Contains(a.stdout.String(), "#1 pull app#1: swarm:plan-0-posted\n") {
		t.Errorf("labels %v\n%s", got, a.stdout.String())
	}
}

// uncheckedWarning is apply's warning of repos[1], lib, which declares no
// check, with the merge gate off (section 3).
const uncheckedWarning = "⚠ swarm.json: repos[1] (lib) has no check and gates.merge is false: no check of Daisy's runs on its pull requests before the merge step merges them\n"

// Step 35's claim (plan remediating the 2026-11-15 design review, talk
// A6): with the merge gate off, a repository with no check merges with no
// check of Daisy's runs on its head, the forge's own checks read as for
// any (section 12), and the maintainer sees it without
// being stopped by it: apply --once warns of lib and not of app, which
// declares one, and goes on; status lists it under Waiting for you (with
// --json under waiting.no_check), notify tells it once, and connect
// shows status's line. With the gate on, none of them names it.
func TestSwarmMergeWithoutACheck(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	notified := a.notifying()
	a.pass()
	applied := a.stderr.String() // apply's, before status resets it
	if _, line := a.waitingSet(); strings.Contains(applied, "has no check") || line != "Waiting for you: nothing" || notified() != nil {
		t.Fatalf("the gate on: %q %q %q", applied, line, notified())
	}

	a.edit(func(c *declare.Config) { c.Gates.Merge = false; c.Budget = &declare.Budget{PerDay: 10, Total: 100} })
	for range 2 {
		a.pass()
		if got := a.stderr.String(); strings.Count(got, uncheckedWarning) != 1 || strings.Contains(got, "(app)") ||
			!strings.Contains(a.stdout.String(), "Pass ") {
			t.Fatalf("apply --once's stderr:\n%s\nstdout:\n%s", got, a.stdout.String())
		}
	}
	if got := notified(); !reflect.DeepEqual(got, []string{"acme/swarm-app waits for you: no check lib"}) {
		t.Errorf("notified %q", got)
	}
	if _, line := a.waitingSet(); line != "Waiting for you: no check lib" {
		t.Errorf("status: %q", line)
	}
	a.flags = []string{"--waiting", "--json"}
	if err := a.status(a.forge); err != nil || !strings.Contains(a.stdout.String(), `"no_check": [`+"\n"+`      "lib"`+"\n") {
		t.Errorf("--waiting --json: %v\n%s", err, a.stdout.String())
	}
	a.flags = nil
	if err := a.readOnly(swarmConnect, a.forge); err != nil || !strings.Contains(a.stdout.String(), "\nWaiting for you: no check lib\n") {
		t.Errorf("connect: %v\n%s", err, a.stdout.String())
	}
	a.flags = []string{"--json"}
	if err := a.readOnly(swarmConnect, a.forge); err != nil || !strings.Contains(a.stdout.String(), `"no_check": [`) {
		t.Errorf("connect --json: %v\n%s", err, a.stdout.String())
	}
	a.flags = nil

	// The loop warns at its first pass and again only when the set
	// changes: pass 2 the same, pass 3 with app's check removed too. No
	// notify, whose timeout the waits below would end at once.
	a.edit(func(c *declare.Config) { c.Notify = nil })
	a.signals = make(chan os.Signal, 1)
	waits := 0
	var mu sync.Mutex
	a.after = func(time.Duration) <-chan time.Time {
		mu.Lock()
		defer mu.Unlock()
		waits++
		switch waits {
		case 2:
			a.edit(func(c *declare.Config) { c.Repos[0].Check = nil })
		case 3:
			a.signals <- os.Interrupt
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	if err := a.apply(a.git()); err != nil {
		t.Fatal(err)
	}
	var got string // the drift's lines aside: the edits are not pushed
	for _, l := range strings.SplitAfter(a.stderr.String(), "\n") {
		if !strings.HasPrefix(l, "⚠ Drift: ") {
			got += l
		}
	}
	both := strings.Replace(uncheckedWarning, "repos[1] (lib)", "repos[0] (app)", 1) + uncheckedWarning
	if got != uncheckedWarning+both || strings.Count(a.stdout.String(), "Pass ") != 3 {
		t.Errorf("the loop's stderr:\n%s\nwant:\n%s\nstdout:\n%s", got, uncheckedWarning+both, a.stdout.String())
	}
}

// The round-limit note in its form before the pull request was linked in
// it (`on app#1`, Step 7 of the plan for the inbox requests F27–F35) is
// the first time's (section 5, T11): with it on the issue, the round
// limit reached again blocks the issue with the second note.
func TestRoundLimitReadsTheUnlinkedNote(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxRounds = 1 })
	a.inReview(oneSlice)
	a.tick()
	a.forge.Comment(issue1, mark("reconciler", 1)+"Review round 1 of 1 on app#1 requested changes (was swarm:review-1-in-progress). The architect revises the plan from the reviews.")
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.pass()
	second := mark("reconciler", 1) + "Review round 1 of 1 on [acme/app#1](https://forge.example.net/acme/app/pulls/1) requested changes again (at swarm:review-1-in-progress)."
	if got := labels(a.issue(1)); !slices.Contains(got, forge.Blocked) || !slices.ContainsFunc(a.bodies(1), func(b string) bool { return strings.HasPrefix(b, second) }) {
		t.Errorf("labels %v, comments %q\n%s", got, a.bodies(1), a.stdout.String())
	}
}

// The claim of Step S7b of the plan implementing the 2026-11-19 swarm
// roadmap (section 5, one note per gating event): an issue flagged at a
// gate and then blocked, its newest gate note not the blocked gate's,
// gets the blocked gate's note at the next pass, once; with no gate note
// on it (the flag set by hand) it gets none, as before.
func TestSwarmBlockedWhileGatedGetsTheBlockedNote(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644) // no role runs; the flag step still does
	a.forge.CreateIssue("Planned", "Plan it.", []string{"swarm:plan-0-posted"})     // #1
	a.forge.Comment(issue1, mark("architect", 1)+gatePlan)
	a.forge.CreateIssue("Flagged by hand", "Plan it.", []string{"swarm:plan-0-posted", forge.Gated}) // #2
	a.forge.Comment(a.ref(2), mark("architect", 1)+gatePlan)
	a.pass()
	if got := a.gateNotes(1); len(got) != 1 || !strings.Contains(got[0], "gated at plan-0-posted: the plan gate holds") {
		t.Fatalf("#1, the plan gate: %q\n%s", got, a.stdout.String())
	}
	for _, n := range []int{1, 2} {
		a.tick()
		a.forge.Relabel(a.ref(n), "", forge.Blocked)
		a.forge.Comment(a.ref(n), mark("reconciler", 1)+"architect run 1 failed: x (at swarm:plan-0-posted). Remove swarm:blocked to resume.")
	}
	a.pass()
	got := a.gateNotes(1)
	if len(got) != 2 || !strings.Contains(got[1], "gated at plan-0-posted: it is blocked") || !strings.Contains(got[1], "- The note that set it, at ") {
		t.Fatalf("#1 blocked while gated: %q\n%s", got, a.stdout.String())
	}
	if got := a.gateNotes(2); len(got) != 0 {
		t.Errorf("#2, flagged by hand with no gate note, then blocked: %q", got)
	}
	a.pass()
	if got := a.gateNotes(1); len(got) != 2 {
		t.Errorf("a pass with nothing new posted again: %d notes", len(got))
	}
}
