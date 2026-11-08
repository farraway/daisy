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
// on, blocked always, every other stage never; an issue carrying the flag
// and no stage, or two stages, is cleared of it; a stage moved by hand
// loses it at the next pass; and status's Waiting line names exactly the
// open issues that carry it.
func TestSwarmApplyGatedFlag(t *testing.T) {
	a := newApplyWS(t)
	os.WriteFile(a.state("paused"), []byte("paused 2026-11-06T14:00:00Z\n"), 0o644)
	for _, l := range forge.Stages { // #1 proposed … #8 blocked
		a.forge.CreateIssue("At "+l, "", []string{l})
	}
	a.forge.CreateIssue("The flag alone", "", []string{forge.Gated})                              // #9
	a.forge.CreateIssue("Two stages", "", []string{"swarm:proposed", "swarm:ready", forge.Gated}) // #10
	agree := func(when string, want []int) {
		t.Helper()
		got, line := a.waitingSet()
		if flagged := a.gatedSet(); !reflect.DeepEqual(flagged, want) || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: flagged %v, %q; want %v", when, flagged, line, want)
		}
	}

	a.pass()
	if want := "#1 gated\n#3 gated\n#7 gated\n#8 gated\n#9 gated cleared\n#10 gated cleared\n"; !strings.Contains(a.stdout.String(), want) {
		t.Errorf("every gate on:\n%s\nwant:\n%s", a.stdout.String(), want)
	}
	agree("every gate on", []int{1, 3, 7, 8})
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposed", forge.Gated}) {
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
	}{{"triage", func(g *declare.Gates) { g.Triage = false }, 1}, {"plan", func(g *declare.Gates) { g.Plan = false }, 3}, {"merge", func(g *declare.Gates) { g.Merge = false }, 7}} {
		a.edit(func(c *declare.Config) {
			c.Gates = declare.Gates{Triage: true, Plan: true, Merge: true}
			gate.off(&c.Gates)
			c.Budget = &declare.Budget{PerDay: 10, Total: 100}
		})
		a.pass()
		want := []int{1, 3, 7, 8}
		want = slices.DeleteFunc(want, func(n int) bool { return n == gate.n })
		agree(gate.name+" gate off", want)
		a.edit(func(c *declare.Config) { c.Gates = declare.Gates{Triage: true, Plan: true, Merge: true} })
		a.pass()
		agree(gate.name+" gate on again", []int{1, 3, 7, 8})
	}

	a.forge.SetLabel(forge.Ref{Repo: "app", N: 3}, "swarm:planned") // T7 on the forge: the flag stays until the pass
	a.forge.SetLabel(forge.Ref{Repo: "app", N: 8}, "swarm:ready")   // T18 on the forge
	a.pass()
	if !strings.Contains(a.stdout.String(), "#3 gated cleared\n#8 gated cleared\n") {
		t.Errorf("moved by hand:\n%s", a.stdout.String())
	}
	agree("moved by hand", []int{1, 7})
}

// The flag and the terminals are no stage (section 6, rule 1): a proposed
// issue carrying the flag is one stage, and the owner answers the
// maintainer there; an open issue carrying only the flag or only a
// terminal is no swarm issue: nothing launches on it, no warning names it,
// and status counts it under no stage.
func TestSwarmFlagAndTerminalsAreNoStage(t *testing.T) {
	a := newApplyWS(t)
	a.forge.CreateIssue("Idea", mark("owner", 1)+"Do it.", []string{"swarm:proposed", forge.Gated}) // #1
	a.forge.Say(issue1, "jane", "Which file?")
	a.forge.CreateIssue("Flag only", "", []string{forge.Gated})         // #2
	a.forge.CreateIssue("Done, open", "", []string{forge.Done})         // #3
	a.forge.CreateIssue("Rejected, open", "", []string{forge.Rejected}) // #4
	a.runner.Script(1, "owner", runner.Canned{Out: "main.go.\n"})
	a.pass()
	var on []string
	for _, l := range a.runner.Launches() {
		on = append(on, l.Label)
	}
	if !reflect.DeepEqual(on, []string{"acme/app#1/owner"}) || a.stderr.Len() != 0 {
		t.Errorf("launched %q\nstderr:\n%s", on, a.stderr.String())
	}
	if got := labels(a.issue(1)); !reflect.DeepEqual(got, []string{"swarm:proposed", forge.Gated}) {
		t.Errorf("#1: %v", got)
	}
	if got := labels(a.issue(3)); !reflect.DeepEqual(got, []string{forge.Done}) {
		t.Errorf("#3: %v", got)
	}
	if err := a.status(a.forge); err != nil {
		t.Fatal(err)
	}
	if out := a.stdout.String(); !strings.Contains(out, "\n  proposed     1  #1\n") || !strings.Contains(out, "\nWaiting for you: triage #1\n") || strings.Contains(out, "#2") || strings.Contains(out, "#3") || strings.Contains(out, "#4") {
		t.Errorf("status:\n%s", out)
	}
}

// The maintainer's verbs clear the flag with their relabel, and their
// closes leave a terminal in the stage's place and no flag: close is
// swarm:rejected, merge of the last slice swarm:done.
func TestSwarmMaintainerFlagAndTerminals(t *testing.T) {
	m := newMaintainerWS(t)
	m.forge.CreateIssue("Pin the corpus", mark("owner", 1)+"a", []string{"swarm:proposed", forge.Gated})
	const head = "Maintainer jane on acme/app\n"
	m.ok(swarmTriage, head+"#1 swarm:proposed → swarm:ready\n#1 gated cleared\n", "1")
	if is, _ := m.forge.Issue(1); !reflect.DeepEqual(is.Labels, []string{"swarm:ready"}) {
		t.Errorf("triage: %v", is.Labels)
	}

	m.forge.CreateIssue("Drop it", mark("owner", 1)+"b", []string{"swarm:blocked", forge.Gated}) // #2
	m.ok(swarmClose, head+"#2 closed\n#2 swarm:blocked → swarm:rejected\n#2 gated cleared\n", "2")
	if is, _ := m.forge.Issue(2); is.Open || !reflect.DeepEqual(is.Labels, []string{forge.Rejected}) {
		t.Errorf("close: open %v, %v", is.Open, is.Labels)
	}

	m.forge.Comment(issue1, mark("architect", 1)+oneSlice)
	m.approvedPull(1, "c1")
	m.forge.Relabel(issue1, "", forge.Gated)
	m.ok(swarmMerge, head+"#1 merged pull app#3 (slice 1 of 1)\n#1 closed (last slice merged)\n#1 swarm:approved → swarm:done\n#1 gated cleared\n", "1")
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
