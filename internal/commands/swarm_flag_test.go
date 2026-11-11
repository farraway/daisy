package commands

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// gatedSet is the open issues carrying the maintainer's flag, by number.
func (a *applyWS) gatedSet() []int {
	a.t.Helper()
	var out []int
	for _, l := range append(forge.Stages, forge.Gated, forge.Done, forge.Rejected) {
		is, err := a.forge.Issues(forge.Open, l)
		if err != nil {
			a.t.Fatal(err)
		}
		for _, i := range is {
			if slices.Contains(i.Labels, forge.Gated) && !slices.Contains(out, i.Number) {
				out = append(out, i.Number)
			}
		}
	}
	slices.Sort(out)
	return out
}

var issueNumber = regexp.MustCompile(`#([0-9]+)`)

// waitingSet is the issues `swarm status`'s Waiting line names, by number,
// and the line.
func (a *applyWS) waitingSet() ([]int, string) {
	a.t.Helper()
	if err := a.status(a.forge); err != nil {
		a.t.Fatal(err)
	}
	var line string
	for _, l := range strings.Split(a.stdout.String(), "\n") {
		if strings.HasPrefix(l, "Waiting for you: ") {
			line = l
		}
	}
	var out []int
	for _, m := range issueNumber.FindAllStringSubmatch(line, -1) {
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	slices.Sort(out)
	return out, line
}

// The maintainer's flag (section 5) over a paused pass, which still sets
// it: an issue at each gated stage carries it exactly when its gate is
// on, a blocked one at any stage always, every other stage never; an
// issue carrying the flag and no stage, or two stages, is cleared of it;
// a stage moved by hand, or a block lifted by hand, loses it at the next
// pass; and status's Waiting line names exactly the open issues that
// carry it.
func TestSwarmApplyGatedFlag(t *testing.T) {
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	for _, l := range forge.Stages { // #1 proposal-posted … #9 review-3-changes-requested
		a.forge.CreateIssue("At "+l, "", []string{l})
	}
	a.forge.CreateIssue("Blocked at its stage", "", []string{"swarm:plan-1-accepted", forge.Blocked})                // #10
	a.forge.CreateIssue("The flag alone", "", []string{forge.Gated})                                                 // #11
	a.forge.CreateIssue("Two stages", "", []string{"swarm:proposal-posted", "swarm:proposal-accepted", forge.Gated}) // #12
	agree := func(when string, want []int) {
		t.Helper()
		got, line := a.waitingSet()
		if flagged := a.gatedSet(); !reflect.DeepEqual(flagged, want) || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: flagged %v, %q; want %v", when, flagged, line, want)
		}
	}

	a.pass()
	if want := "#1 gated\n#1 gate note posted\n#3 gated\n#3 gate note posted\n#8 gated\n#8 gate note posted\n#10 gated\n#10 gate note posted\n#11 gated cleared\n#12 gated cleared\n"; !strings.Contains(a.stdout.String(), want) {
		t.Errorf("every gate on:\n%s\nwant:\n%s", a.stdout.String(), want)
	}
	agree("every gate on", []int{1, 3, 8, 10})
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-posted", forge.Gated}) {
		t.Errorf("#1: %v", got)
	}
	a.pass()
	if strings.Contains(a.stdout.String(), "gated") {
		t.Errorf("a second pass relabelled:\n%s", a.stdout.String())
	}

	for _, gate := range []struct {
		name string
		off  func(*declare.Gates)
		n    int
	}{{"triage", func(g *declare.Gates) { g.Triage = false }, 1}, {"plan", func(g *declare.Gates) { g.Plan = false }, 3}, {"merge", func(g *declare.Gates) { g.Merge = false }, 8}} {
		a.edit(func(c *declare.Config) {
			c.Gates = declare.Gates{Triage: true, Plan: true, Merge: true}
			gate.off(&c.Gates)
			c.Budget = &declare.Budget{PerDay: 10, Total: 100}
		})
		a.pass()
		want := []int{1, 3, 8, 10}
		want = slices.DeleteFunc(want, func(n int) bool { return n == gate.n })
		agree(gate.name+" gate off", want)
		a.edit(func(c *declare.Config) { c.Gates = declare.Gates{Triage: true, Plan: true, Merge: true} })
		a.pass()
		agree(gate.name+" gate on again", []int{1, 3, 8, 10})
	}

	a.forge.SetLabel(a.ref(3), "swarm:plan-1-accepted") // T7 on the forge: the flag stays until the pass
	a.forge.Relabel(a.ref(10), forge.Blocked, "")       // T18 on the forge
	a.pass()
	if !strings.Contains(a.stdout.String(), "#3 gated cleared\n#10 gated cleared\n") {
		t.Errorf("moved by hand:\n%s", a.stdout.String())
	}
	agree("moved by hand", []int{1, 8})
}

// gateNotes is the gate notes on issue n, oldest first.
func (a *applyWS) gateNotes(n int) []string {
	var out []string
	for _, c := range a.issue(n).Comments {
		if strings.HasPrefix(c.Body, gateNote) && c.Author == "swarm-bot" {
			out = append(out, c.Body)
		}
	}
	return out
}

const gatePlan = `## Research

- /work/app/main.go: the entry point.

## Steps

1. app: change main.go to v1; the test proves it.
2. lib: add notes.md.

## Risks

- **The corpus drifts.** It moves under us.
  Watch it.
- A second risk.

## Open Questions

1. Should main.go say v1 or v2?
`

// The gate note (section 5): when the flag step adds swarm:gated it posts
// one note on the issue, at each of the four gates naming what the
// maintainer decides from: the brief, the milestone's other open issues
// and the issues the proposal names at triage; the plan's slices, its Open
// Questions as written and its Risks at the plan gate; the pull request,
// its slice, rounds and the approving reviews' non-blocking findings at
// the merge gate; the note that blocked the issue. A second pass posts
// none, the flag being on; a re-gating after a transition posts a new
// one; no note wakes a role, and status's Waiting line still names
// exactly the flagged issues.
func TestSwarmApplyGateNotes(t *testing.T) {
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.forge.CreateIssue("Drift check", mark("owner", 1)+"Brief: docs/briefs/a.md — M1 The drift\nAfter #2 lands; not acme/lib#9 nor #1.\n", []string{"swarm:proposal-posted"}) // #1
	a.forge.CreateIssue("Other half", mark("owner", 1)+"Brief: docs/briefs/a.md — M1 The drift\nThe rest.\n", []string{"swarm:proposal-posted"})                               // #2
	a.forge.CreateIssue("Planned", "Plan it.", []string{"swarm:plan-0-posted"})                                                                                                // #3
	a.forge.Comment(a.ref(3), mark("architect", 1)+gatePlan)
	a.forge.CreateIssue("Merge me", "Build it.", []string{"swarm:review-2-approved"}) // #4
	a.forge.Comment(a.ref(4), mark("architect", 1)+twoSlices)
	a.forge.CreateIssue("Stuck", "Build it.", []string{"swarm:plan-1-accepted", forge.Blocked}) // #5
	a.forge.Comment(a.ref(5), mark("reconciler", 1)+"worker run 1 failed: no commits (at swarm:plan-1-accepted). Remove swarm:blocked to resume.")
	a.forge.CreateIssue("Later", mark("owner", 1)+"Brief: docs/briefs/a.md — M2 Later\nAfter #1.\n", []string{"swarm:proposal-posted"})  // #6
	m, _ := a.forge.CreatePull("app", "swarm/4-1", "main", "#4 slice 1: Merge me", mark("worker", 1)+"Did it.\n\nRefs acme/swarm-app#4") // app#1
	pr := forge.Ref{Repo: "app", N: m}
	a.forge.Push(pr, "c0ffee0")
	a.forge.Review(pr, forge.RequestChanges, mark("adversary", 1)+"VERDICT: request-changes\n\n[blocking] main.go:1 says v0.\n[non-blocking] main.go:2 an old nit.\n")
	a.forge.Review(pr, forge.Approve, mark("sympathetic", 1)+approve)
	a.forge.Push(pr, "c1c1c1c")
	a.forge.Review(pr, forge.Approve, mark("adversary", 2)+"VERDICT: approve\n\n[non-blocking] main.go:2 could name the version.\n")
	a.forge.Review(pr, forge.Approve, mark("sympathetic", 2)+"VERDICT: approve\n\n[non-blocking] notes.md:1 a typo.\n")

	a.pass()
	if want := "#1 gated\n#1 gate note posted\n#2 gated\n#2 gate note posted\n#3 gated\n#3 gate note posted\n#4 gated\n#4 gate note posted\n#5 gated\n#5 gate note posted\n#6 gated\n#6 gate note posted\n"; !strings.Contains(a.stdout.String(), want) || a.stderr.Len() != 0 {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", a.stdout.String(), a.stderr.String())
	}
	want := map[int]string{
		1: "**reconciler:** gated at proposal-posted: the triage gate holds a proposal until the maintainer triages it.\n\n" +
			"- Class: architecture.\n- Brief: docs/briefs/a.md, milestone M1 The drift.\n- Open issues on the same milestone: #2 Other half.\n- Issues it names, its dependencies among them: #2 (open, proposal-posted).\n\n" +
			"Decide: `daisy swarm triage 1`, `daisy swarm close 1`, `daisy swarm class 1 routine`, after which Daisy passes this gate and the plan's, or a comment here, which the owner answers.",
		3: "**reconciler:** gated at plan-0-posted: the plan gate holds the plan until the maintainer accepts it.\n\n" +
			"- Class: architecture.\n- Slices: 2.\n  1. app: change main.go to v1; the test proves it.\n  2. lib: add notes.md.\n" +
			"- Open questions, as the plan has them:\n  1. Should main.go say v1 or v2?\n" +
			"- Risks:\n  - **The corpus drifts.** It moves under us.\n  - A second risk.\n\n" +
			"Decide: `daisy swarm accept 3`, `daisy swarm class 3 routine`, after which Daisy accepts it, or a comment here, from which the architect revises the plan.",
		4: "**reconciler:** gated at review-2-approved: both reviews approve, and the merge gate leaves the merge to the maintainer.\n\n" +
			"- Class: architecture.\n- Pull request acme/app#1, #4 slice 1: Merge me (open, head c1c1c1c).\n- Slice 1 of 2.\n- Review rounds: 2.\n" +
			"- Non-blocking findings in the approving reviews:\n  - adversary: [non-blocking] main.go:2 could name the version.\n  - sympathetic: [non-blocking] notes.md:1 a typo.\n\n" +
			"Decide: `daisy swarm merge 4`, `daisy swarm approve 4` first when the forge wants an approving review, or a comment.",
		5: "**reconciler:** gated at plan-1-accepted: it is blocked, and only the maintainer unblocks it.\n\n" +
			"- Class: architecture.\n- The note that set it, at 2026-11-06T15:00:00Z: worker run 1 failed: no commits (at swarm:plan-1-accepted). Remove swarm:blocked to resume.\n\n" +
			"Decide: make the relabel the note names, if it names one, then `daisy swarm unblock 5 <comment>`; or `daisy swarm close 5`.",
	}
	for n, text := range want {
		if got := a.gateNotes(n); len(got) != 1 || got[0] != gateNote+text {
			t.Errorf("#%d's gate notes:\n%q\nwant:\n%q", n, got, gateNote+text)
		}
	}
	if got := a.gateNotes(2); len(got) != 1 || !strings.Contains(got[0], "- Open issues on the same milestone: #1 Drift check.\n- Issues it names, its dependencies among them: none.\n") {
		t.Errorf("#2's gate note: %q", got)
	}
	if got := a.gateNotes(6); len(got) != 1 || !strings.Contains(got[0], "- Open issues on the same milestone: none.\n- Issues it names, its dependencies among them: #1 (open, proposal-posted).\n") {
		t.Errorf("#6's gate note: %q", got)
	}

	a.pass()
	if strings.Contains(a.stdout.String(), "gate note") {
		t.Errorf("a second pass posted a note with the flag on:\n%s", a.stdout.String())
	}
	os.Remove(a.state("paused"))
	a.pass()
	if l := a.runner.Launches(); len(l) != 0 {
		t.Errorf("a gate note woke a role: %+v", l)
	}
	if got, line := a.waitingSet(); !reflect.DeepEqual(got, a.gatedSet()) || !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6}) {
		t.Errorf("Waiting %q, flagged %v", line, a.gatedSet())
	}

	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.forge.SetLabel(a.ref(3), "swarm:plan-1-accepted") // T7 on the forge
	a.pass()
	a.forge.SetLabel(a.ref(3), "swarm:plan-0-posted") // and back: gated again
	a.pass()
	if got := a.gateNotes(3); len(got) != 2 || got[1] != gateNote+want[3] || !strings.Contains(a.stdout.String(), "#3 gated\n#3 gate note posted\n") {
		t.Errorf("re-gated: %d notes\n%s", len(got), a.stdout.String())
	}
}

// A gate note per gating event, the flag left on between them (section
// 5): the architect's revised plan under the plan gate gets a note
// quoting it (after a comment or the round limit, T11); an issue moved
// to another gated stage with the flag left on (a hand relabel, or two
// transitions within one --once pass) gets one at its new stage; a pass
// that finds nothing new posts none; a blocked issue gets none, the note
// that blocked it being the word.
func TestSwarmApplyGateNoteOnEachGating(t *testing.T) {
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	a.forge.CreateIssue("Planned", "Plan it.", []string{"swarm:plan-0-posted"}) // #1
	a.forge.Comment(issue1, mark("architect", 1)+gatePlan)
	a.forge.CreateIssue("Proposed", mark("owner", 1)+"Brief: docs/briefs/a.md — M1 The drift\n", []string{"swarm:proposal-posted"}) // #2
	a.forge.CreateIssue("Planned, then blocked", "Plan it.", []string{"swarm:plan-0-posted"})                                       // #3
	a.forge.Comment(a.ref(3), mark("architect", 1)+gatePlan)
	a.pass()
	for _, n := range []int{1, 2, 3} {
		if got := a.gateNotes(n); len(got) != 1 {
			t.Fatalf("#%d: %d gate notes\n%s", n, len(got), a.stdout.String())
		}
	}

	revised := strings.Replace(gatePlan, "2. lib: add notes.md.", "2. lib: add notes.md.\n3. lib: add a test.", 1)
	a.tick()
	a.forge.Comment(issue1, mark("architect", 2)+revised)
	a.forge.SetLabel(a.ref(2), "swarm:plan-0-posted")
	a.forge.Comment(a.ref(2), mark("architect", 1)+gatePlan)
	a.forge.Relabel(a.ref(3), "", forge.Blocked)
	a.forge.Comment(a.ref(3), mark("architect", 2)+revised)
	a.pass()
	if got := a.gateNotes(1); len(got) != 2 || !strings.Contains(got[1], "- Slices: 3.\n") || !strings.Contains(a.stdout.String(), "#1 gate note posted\n") {
		t.Errorf("#1, the plan revised: %q\n%s", got, a.stdout.String())
	}
	if got := a.gateNotes(2); len(got) != 2 || !strings.HasPrefix(got[1], gateNote+"**reconciler:** gated at plan-0-posted: the plan gate holds") {
		t.Errorf("#2, moved with the flag on: %q", got)
	}
	if got := a.gateNotes(3); len(got) != 1 {
		t.Errorf("#3, blocked: %d gate notes", len(got))
	}
	a.pass()
	if strings.Contains(a.stdout.String(), "gate note") {
		t.Errorf("a pass with nothing new posted a note:\n%s", a.stdout.String())
	}
}

// The blocked gate's note quotes the newest reconciler note that blocked
// the issue, its first line cut at 120 characters, unless the block was
// lifted since (a comment of the maintainer's after it, or a gate note of
// another gate posted after it), when swarm:blocked was set again by
// hand; the blocked gate's own earlier note tells nothing (a verb that
// clears the flag of an issue still blocked).
func TestSwarmApplyBlockedGateNote(t *testing.T) {
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	long := "worker run 1 failed: " + strings.Repeat("x", 130)
	block := func(n int, why string) {
		a.forge.Comment(a.ref(n), mark("reconciler", 1)+why+" (at swarm:plan-1-accepted). Remove swarm:blocked to resume.")
	}
	a.forge.CreateIssue("Long", "Build it.", []string{"swarm:plan-1-accepted", forge.Blocked}) // #1
	block(1, long)
	a.forge.CreateIssue("Unblocked, then blocked by hand", "Build it.", []string{"swarm:plan-1-accepted"}) // #2
	block(2, "worker run 1 failed: no commits")
	a.tick()
	a.forge.Say(a.ref(2), "jane", "Carry on.")
	a.forge.Relabel(a.ref(2), "", forge.Blocked)
	a.forge.CreateIssue("Lifted by hand", "Build it.", []string{"swarm:plan-1-accepted", forge.Blocked}) // #3
	block(3, "worker run 1 failed: no commits")
	a.pass()
	head := func(n int) string {
		got := a.gateNotes(n)
		if len(got) == 0 {
			return ""
		}
		_, items, _ := strings.Cut(got[len(got)-1], "\n- Class: architecture.\n")
		l, _, _ := strings.Cut(items, "\n")
		return l
	}
	if got, want := head(1), "- The note that set it, at 2026-11-06T15:00:00Z: "+long[:119]+"…"; got != want {
		t.Errorf("#1:\n%q\nwant\n%q", got, want)
	}
	if got, want := head(2), "- The note that set it: none since the block of 2026-11-06T15:00:00Z was lifted; swarm:blocked was set by hand."; got != want {
		t.Errorf("#2:\n%q\nwant\n%q", got, want)
	}
	if got := head(3); !strings.HasPrefix(got, "- The note that set it, at ") {
		t.Errorf("#3 first: %q", got)
	}
	at3 := a.ref(3)
	a.forge.SetLabel(at3, "swarm:plan-0-posted") // lifted on the forge and re-planned: the plan gate's note
	a.forge.Relabel(at3, forge.Blocked, "")
	a.forge.Comment(at3, mark("architect", 1)+gatePlan)
	a.pass()
	a.forge.SetLabel(at3, "swarm:plan-1-accepted") // accepted on the forge: the flag cleared
	a.pass()
	a.forge.Relabel(at3, "", forge.Blocked) // and blocked again by hand
	a.pass()
	if got, want := head(3), "- The note that set it: none since the block of 2026-11-06T15:01:00Z was lifted; swarm:blocked was set by hand."; len(a.gateNotes(3)) != 3 || got != want {
		t.Errorf("#3 again: %d notes\n%q\nwant\n%q", len(a.gateNotes(3)), got, want)
	}
	at1 := a.ref(1)
	a.forge.SetLabel(at1, "swarm:plan-2-in-progress") // #1 moved with its flag cleared, as a verb does, still blocked
	a.forge.Relabel(at1, forge.Gated, "")
	a.pass()
	if got, want := head(1), "- The note that set it, at 2026-11-06T15:00:00Z: "+long[:119]+"…"; len(a.gateNotes(1)) != 2 || got != want {
		t.Errorf("#1 flagged again, still blocked: %d notes\n%q\nwant\n%q", len(a.gateNotes(1)), got, want)
	}
}

// The flags and the terminals are no stage (section 6, rule 1): a
// proposal-posted issue carrying the gated flag is one stage, and the
// owner answers the maintainer there; an open issue carrying only a flag
// or only a terminal is no swarm issue: nothing launches on it, no warning
// names it, and status counts it under no stage.
func TestSwarmFlagAndTerminalsAreNoStage(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Idea", mark("owner", 1)+"Do it.", []string{"swarm:proposal-posted", forge.Gated}) // #1
	a.forge.Say(issue1, "jane", "Which file?")
	a.forge.CreateIssue("Flag only", "", []string{forge.Gated})               // #2
	a.forge.CreateIssue("Done, open", "", []string{forge.Done})               // #3
	a.forge.CreateIssue("Rejected, open", "", []string{forge.Rejected})       // #4
	for _, f := range []string{forge.Blocked, forge.Paused, forge.Blocking} { // #5, #6, #7
		a.forge.CreateIssue(f+" only", "", []string{f})
	}
	a.runner.Script(1, "owner", runner.Canned{Out: "main.go.\n"})
	a.pass()
	var on []string
	for _, l := range a.runner.Launches() {
		on = append(on, l.Label)
	}
	if !reflect.DeepEqual(on, []string{"acme/swarm-app#1/owner"}) || a.stderr.Len() != 0 {
		t.Errorf("launched %q\nstderr:\n%s", on, a.stderr.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposal-posted", forge.Gated}) {
		t.Errorf("#1: %v", got)
	}
	if got := labels(a.issue(3)); !reflect.DeepEqual(got, []string{forge.Done}) {
		t.Errorf("#3: %v", got)
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "\n  proposal-posted            1  #1\n") || !strings.Contains(out, "\nWaiting for you: triage #1\nHeld: nothing\n") ||
		regexp.MustCompile(`#[2-7]\b`).MatchString(out) {
		t.Errorf("status:\n%s", out)
	}
}

// The maintainer's verbs clear the gated flag with their relabel, and
// their closes leave a terminal in the stage's place and no flag: close is
// swarm:rejected, merge of the last slice swarm:done.
func TestSwarmMaintainerFlagAndTerminals(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("Pin the corpus", mark("owner", 1)+"a", []string{"swarm:proposal-posted", forge.Gated})
	const head = "Maintainer jane on acme/swarm-app\n"
	m.ok(swarmTriage, head+"#1 swarm:proposal-posted → swarm:proposal-accepted\n#1 gated cleared\n", "1")
	if is, _ := m.forge.Issue(1); !reflect.DeepEqual(is.Labels, []string{"swarm:proposal-accepted"}) {
		t.Errorf("triage: %v", is.Labels)
	}

	m.forge.CreateIssue("Drop it", mark("owner", 1)+"b", []string{"swarm:plan-1-accepted", forge.Blocked, forge.Paused, forge.Gated}) // #2
	m.ok(swarmClose, head+"#2 closed\n#2 swarm:plan-1-accepted → swarm:rejected\n#2 gated cleared\n#2 blocked cleared\n#2 paused cleared\n", "2")
	if is, _ := m.forge.Issue(2); is.Open || !reflect.DeepEqual(is.Labels, []string{forge.Rejected}) {
		t.Errorf("close: open %v, %v", is.Open, is.Labels)
	}

	m.forge.Comment(issue1, mark("architect", 1)+oneSlice)
	m.approvedPull(1, "c1")
	m.forge.Relabel(issue1, "", forge.Gated)
	m.ok(swarmMerge, head+"#1 merged pull app#1 (slice 1 of 1)\n#1 closed (last slice merged)\n#1 swarm:review-2-approved → swarm:done\n#1 gated cleared\n", "1")
	if is, _ := m.forge.Issue(1); is.Open || !reflect.DeepEqual(is.Labels, []string{forge.Done}) {
		t.Errorf("merge of the last slice: open %v, %v", is.Open, is.Labels)
	}
	if err := m.verb(swarmClose, "1"); errText(err) != "Error: swarm: #1 is closed; nothing changed" {
		t.Errorf("a closed issue: %v", err)
	}
}

// A closed issue carrying its terminal is still a swarm issue for the
// lists of closed ones: the batch prompt's Recently closed names it with
// its terminal.
func TestSwarmBatchReadsTheTerminals(t *testing.T) {
	a := newApplyWS(t)
	for _, l := range []string{forge.Done, forge.Rejected} {
		n, _ := a.forge.CreateIssue("Closed "+l, "", []string{l})
		a.forge.CloseIssue(n)
	}
	a.release()
	a.runner.Script(0, "owner", runner.Canned{Out: "## Next\nNext.\n"})
	a.pass()
	if p := a.file("0-owner-1", "io/prompt.md"); !strings.Contains(p, "- #2 [rejected] Closed swarm:rejected: closed ") || !strings.Contains(p, "- #1 [done] Closed swarm:done: closed ") {
		t.Errorf("batch prompt:\n%s", p)
	}
}
