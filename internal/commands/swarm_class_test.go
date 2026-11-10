package commands

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/swarm/runner"
)

// autoNote is the auto note's marker line (section 5).
const autoNote = "<!-- daisy-swarm role=reconciler kind=auto -->\n"

// autoNotes is the auto notes on issue n, oldest first.
func (a *applyWS) autoNotes(n int) []string {
	var out []string
	for _, c := range a.issue(n).Comments {
		if strings.HasPrefix(c.Body, autoNote) && c.Author == "swarm-bot" {
			out = append(out, c.Body)
		}
	}
	return out
}

// Gates by job class (section 5, T23 and T24), every gate on: the batch
// proposes a routine job, an architecture job and one it leaves unclassed;
// the routine one carries no class flag and the pass that finds it at
// proposal-posted passes the triage gate with an auto note and no gate
// note, the architect plans, and the pass that collects the plan passes
// the plan gate the same way, so it reaches plan-1-accepted with no
// maintainer action; the worker builds, both reviewers approve, and the
// merge gate holds it with a gate note naming its class. The other two
// carry the class flag and wait at triage with gate notes; the
// architecture one, triaged, waits again at the plan gate. status names
// each waiting issue's class and counts the two auto passes.
func TestSwarmApplyRoutineJob(t *testing.T) {
	a := newApplyWS(t)
	a.write("docs/briefs/a.md", "# Drift\n\nStatus: active\nFirst.\n\n## M1 The drift\n\nA persona sees no drift.\n")
	a.release()
	a.runner.Script(0, "owner", runner.Canned{Stdout: claudeResult, Out: "## Drift check\nBrief: docs/briefs/a.md — M1 The drift\nClass: routine\nCheck the drift.\n\n" +
		"## Rework\nBrief: docs/briefs/a.md — M1 The drift\nClass: architecture\nSplit the package.\n\n" +
		"## Loose\nBrief: docs/briefs/a.md — M1 The drift\nNo class line.\n"})
	a.pass()
	if want := "#1 swarm:proposal-posted → swarm:proposal-accepted (auto)\n#1 auto note posted\n#2 gated\n#2 gate note posted\n#3 gated\n#3 gate note posted\n"; !strings.Contains(a.stdout.String(), want) || a.stderr.Len() != 0 {
		t.Fatalf("the batch's pass:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
	}
	for n, want := range map[int][]string{1: {"swarm:proposal-accepted"}, 2: {"swarm:proposal-posted", forge.ClassArchitecture, forge.Gated}, 3: {"swarm:proposal-posted", forge.ClassArchitecture, forge.Gated}} {
		if got := labels(a.issue(n)); !reflect.DeepEqual(got, want) {
			t.Errorf("#%d: %v, want %v", n, got, want)
		}
	}
	if sys := a.file("0-owner-1", "io/system.md"); !strings.Contains(sys, `then a line "Class: routine" or "Class: architecture", then the job's body. A job is routine when it is cut from that milestone, takes at most two slices`) {
		t.Errorf("the batch's Task:\n%s", sys)
	}
	triage := autoNote + "**reconciler:** auto at proposal-posted: a routine job passes the triage gate by itself, to proposal-accepted."
	if got := a.autoNotes(1); !reflect.DeepEqual(got, []string{triage}) || len(a.gateNotes(1)) != 0 {
		t.Errorf("#1's notes: auto %q, gate %q", got, a.gateNotes(1))
	}
	for _, n := range []int{2, 3} {
		if g := a.gateNotes(n); len(g) != 1 || !strings.Contains(g[0], "\n\n- Class: architecture.\n- Brief: docs/briefs/a.md, milestone M1 The drift.\n") || len(a.autoNotes(n)) != 0 {
			t.Errorf("#%d's gate note: %q", n, g)
		}
	}

	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice})
	a.pass() // the architect plans (T5), and the pass that collects the plan passes the plan gate
	plan := autoNote + "**reconciler:** auto at plan-0-posted: a routine job passes the plan gate by itself, to plan-1-accepted."
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) || !reflect.DeepEqual(a.autoNotes(1), []string{triage, plan}) ||
		!strings.Contains(a.stdout.String(), "#1 swarm:plan-0-posted → swarm:plan-1-accepted (auto)\n#1 auto note posted\n") {
		t.Fatalf("T24: %v %q\n%s", got, a.autoNotes(1), a.stdout.String())
	}

	a.runner.Script(1, "worker", work("v1"))
	a.pass() // T8
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair: T9, and the merge gate holds a routine job
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:review-2-approved", forge.Gated}) {
		t.Fatalf("T9: %v\n%s", got, a.stdout.String())
	}
	if g := a.gateNotes(1); len(g) != 1 || !strings.Contains(g[0], "gated at review-2-approved: both reviews approve, and the merge gate leaves the merge to the maintainer.\n\n- Class: routine.\n") {
		t.Errorf("#1's merge gate note: %q", g)
	}
	a.pass()
	if a.pulls(1)[0].Merged || !reflect.DeepEqual(labels(a.issue(1)), []string{"swarm:review-2-approved", forge.Gated}) {
		t.Errorf("the merge gate let a routine job through: %v", labels(a.issue(1)))
	}
	for _, l := range a.runner.Launches() {
		if strings.HasPrefix(l.Label, "acme/app#2/") || strings.HasPrefix(l.Label, "acme/app#3/") {
			t.Errorf("a gated proposal launched %s", l.Label)
		}
	}

	a.forge.SetLabel(forge.Ref{Repo: "app", N: 2}, "swarm:proposal-accepted") // T3, as `swarm triage` makes it
	a.forge.Relabel(forge.Ref{Repo: "app", N: 2}, forge.Gated, "")
	a.runner.Script(2, "architect", runner.Canned{Out: oneSlice})
	a.pass()
	if got := labels(a.issue(2)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted", forge.ClassArchitecture, forge.Gated}) || len(a.gateNotes(2)) != 2 || len(a.autoNotes(2)) != 0 ||
		!strings.Contains(a.gateNotes(2)[1], "gated at plan-0-posted: the plan gate holds the plan until the maintainer accepts it.\n\n- Class: architecture.\n") {
		t.Errorf("an architecture job at the plan gate: %v, %d gate notes, %d auto", got, len(a.gateNotes(2)), len(a.autoNotes(2)))
	}

	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\nWaiting for you: triage #3; plan #2; merge #1 (routine)\n", "\n  auto         2 gates passed for routine jobs (triage 1, plan 1)\n"} {
		if !strings.Contains(a.stdout.String(), want) {
			t.Errorf("status lacks %q:\n%s", want, a.stdout.String())
		}
	}
}

// A routine job waits for its role: at the round limit (T11) the plan
// gate is not passed until the architect has revised the plan from the
// reviews, nor gated meanwhile; a maintainer's comment on a routine plan
// is answered by the architect before Daisy passes the plan gate on the
// revision; a paused routine job, or a paused swarm, is not passed.
func TestSwarmApplyRoutineWaitsForItsRole(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.MaxRounds = 1 })
	a.forge.CreateIssue("Drift check", mark("owner", 1)+"Brief: docs/briefs/a.md — M1\nClass: routine\nCheck it.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice}, runner.Canned{Out: oneSlice + "\nRevised.\n"})
	a.pass()
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: requestChanges})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass() // the pair at the round limit: T11 to plan-0-posted
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-0-posted"}) || len(a.autoNotes(1)) != 1 {
		t.Fatalf("T11 on a routine job: %v, %d auto notes\n%s", got, len(a.autoNotes(1)), a.stdout.String())
	}
	a.pass() // the architect revises, then the gate passes
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) || len(a.autoNotes(1)) != 2 ||
		!strings.Contains(a.stdout.String(), "#1 architect run 2: posted comment\n") {
		t.Fatalf("the revision, then T24: %v\n%s", got, a.stdout.String())
	}

	a = newApplyWS(t)
	a.forge.CreateIssue("Drift check", mark("owner", 1)+"Brief: docs/briefs/a.md — M1\nClass: routine\nCheck it.", []string{"swarm:plan-0-posted"})
	a.forge.Comment(issue1, mark("architect", 1)+oneSlice)
	a.forge.Say(issue1, "jane", "Name the test.")
	a.runner.Script(1, "architect", runner.Canned{Out: oneSlice + "\nThe test is TestV1.\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:plan-1-accepted"}) || len(a.autoNotes(1)) != 1 ||
		!strings.Contains(a.stdout.String(), "#1 architect run 2: posted comment\n") {
		t.Fatalf("the architect answers, then the gate passes: %v\n%s", got, a.stdout.String())
	}

	a.forge.CreateIssue("Held", mark("owner", 1)+"Brief: docs/briefs/a.md — M1\nClass: routine\nHold it.", []string{"swarm:proposal-posted", forge.Paused}) // #2
	a.pass()
	if got := labels(a.issue(2)); !reflect.DeepEqual(got, []string{"swarm:proposal-posted", forge.Paused}) {
		t.Errorf("a paused routine job: %v", got)
	}
	a.forge.Relabel(forge.Ref{Repo: "app", N: 2}, forge.Paused, "")
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.pass()
	if got := labels(a.issue(2)); !reflect.DeepEqual(got, []string{"swarm:proposal-posted"}) {
		t.Errorf("a paused swarm passed a gate: %v", got)
	}
}

// A maintainer-filed issue is architecture until the maintainer states
// its class: gated at triage after the owner's answer; the maintainer's
// `Class: routine` (as `swarm class` posts it) wakes no role, and the
// next pass passes the triage gate, clearing the flag.
func TestSwarmApplyMaintainerFiledClass(t *testing.T) {
	a := newApplyWS(t)
	a.forge.As(forge.Account{Login: "jane"}).CreateIssue("Fix the typo", "The README says teh.", []string{"swarm:proposal-posted"})
	a.runner.Script(1, "owner", runner.Canned{Out: "Worth doing: one line.\n"})
	a.pass()
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-posted", forge.Gated}) || len(a.autoNotes(1)) != 0 {
		t.Fatalf("unclassed, the maintainer's: %v", got)
	}
	a.forge.Say(issue1, "jane", "Class: routine")
	before := len(a.runner.Launches())
	a.pass()
	if n := len(a.runner.Launches()) - before; n != 0 {
		t.Errorf("the class statement woke %d roles", n)
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-accepted"}) || len(a.autoNotes(1)) != 1 ||
		!strings.Contains(a.stdout.String(), "#1 swarm:proposal-posted → swarm:proposal-accepted (auto)\n#1 auto note posted\n#1 gated cleared\n") {
		t.Errorf("classed routine: %v\n%s", got, a.stdout.String())
	}
}

// max_open_proposals stays the brake with the triage gate on (section 6,
// G0): the routine jobs Daisy triaged count while they wait at
// proposal-accepted, the batch proposing only the difference, and an
// architecture job the maintainer triaged, flagged or unclassed, does not
// count.
func TestSwarmBatchCountsRoutineProposals(t *testing.T) {
	a := newApplyWS(t)
	a.edit(func(c *declare.Config) { c.Owner.MaxOpenProposals = 2 })
	routine := func(title string) string {
		return mark("owner", 1) + "Brief: docs/briefs/a.md — M1\nClass: routine\n" + title + "."
	}
	for _, title := range []string{"One", "Two"} {
		a.forge.CreateIssue(title, routine(title), []string{"swarm:proposal-accepted", forge.Paused}) // held, so no architect runs
	}
	a.forge.CreateIssue("Big", mark("owner", 1)+"Split it.", []string{"swarm:proposal-accepted", forge.ClassArchitecture, forge.Paused}) // #3
	a.forge.As(forge.Account{Login: "jane"}).CreateIssue("Jane's", "Do it.", []string{"swarm:proposal-accepted", forge.Paused})          // #4: unclassed
	a.release()
	a.pass()
	if n := len(a.runner.Launches()); n != 0 {
		t.Fatalf("two routine jobs waiting, and the batch ran: %d launches", n)
	}
	a.forge.SetLabel(forge.Ref{Repo: "app", N: 2}, "swarm:plan-1-accepted")
	a.runner.Script(0, "owner", runner.Canned{Out: "## Three\nBrief: docs/briefs/a.md — M1\nClass: routine\nThree.\n"})
	a.pass()
	if r := a.result("0-owner-1"); !strings.Contains(r, "\npropose 1\n") {
		t.Errorf("one routine job waiting: %s", r)
	}
}

// `swarm class` both ways (section 11), as jane: architecture sets the
// class flag on the owner's routine proposal and posts the statement;
// routine clears it and posts the statement; a second routine changes
// nothing; a maintainer-filed issue, architecture by default with no
// flag, is given the flag, then made routine; a word other than the two
// classes is usage, and a routine
// statement by a login not in trusted is refused before any write.
func TestSwarmMaintainerClass(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("Pin the corpus", mark("owner", 1)+"Brief: docs/briefs/m0.md — First\nClass: routine\nPin it.", []string{"swarm:proposal-posted"})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmClass, head+"#1 class-architecture\n#1 commented\n", "1", "architecture")
	if is, _ := m.forge.Issue(1); !reflect.DeepEqual(is.Labels, []string{"swarm:proposal-posted", forge.ClassArchitecture}) {
		t.Errorf("architecture: %v", is.Labels)
	}
	m.ok(swarmClass, head+"#1 class-architecture cleared\n#1 commented\n", "1", "routine")
	m.ok(swarmClass, head+"#1 already routine\n", "1", "routine")
	want := []string{
		"jane labeled swarm:class-architecture #1", "jane unlabeled swarm:class-architecture #1",
		`jane comment "Class: architecture" #1`, `jane comment "Class: routine" #1`,
	}
	if got := m.writes(1); !reflect.DeepEqual(got, want) {
		t.Errorf("writes:\n%q\nwant:\n%q", got, want)
	}

	m.as["jane-456"].CreateIssue("Fix the typo", "teh", []string{"swarm:plan-0-posted"}) // #2, jane's
	m.ok(swarmClass, head+"#2 class-architecture\n", "2", "architecture")                // architecture by default, unflagged: the flag set
	m.ok(swarmClass, head+"#2 already architecture\n", "2", "architecture")
	m.ok(swarmClass, head+"#2 class-architecture cleared\n#2 commented\n", "2", "routine")
	if is, _ := m.forge.Issue(2); reconcile.ClassOf(is, "swarm-bot", []string{"jane", "swarm-bot"}) != reconcile.Routine {
		t.Errorf("#2 not routine: %+v", is)
	}

	if err := m.verb(swarmClass, "1", "small"); errText(err) != "not a class: small; swarm class takes routine or architecture" {
		t.Errorf("a bad class: %v", err)
	}
	m.edit(func(c *declare.Config) { c.Trusted = []string{"swarm-bot"} })
	m.forge.Relabel(issue1, "", forge.ClassArchitecture)
	if err := m.verb(swarmClass, "1", "routine"); errText(err) != "Error: swarm: jane is not in trusted, so Daisy would not read your class statement; add the login to trusted in swarm.json (docs/swarm.md section 3); nothing changed" {
		t.Errorf("untrusted: %v", err)
	}
	if is, _ := m.forge.Issue(1); !slices.Contains(is.Labels, forge.ClassArchitecture) || len(is.Comments) != 2 {
		t.Errorf("the refusal wrote: %+v", is)
	}
}
