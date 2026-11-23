package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// interventions is the home's intervention rows, each by its column
// names.
func interventions(t *testing.T, home string) []map[string]string {
	t.Helper()
	var out []map[string]string
	for _, r := range rows(t, home) {
		if r["kind"] == "intervention" {
			out = append(out, r)
		}
	}
	return out
}

// interventionWS is the applyWS's swarm with a home taking its rows and
// issues #1 and #2 at proposal-posted, and jane as its maintainer.
func interventionWS(t *testing.T) (*applyWS, *maintainerWS, string) {
	a := newApplyWS(t)
	home := a.metricsHome()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalPosted})
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{forge.ProposalPosted})
	m := &maintainerWS{statusWS: a.statusWS}
	m.secrets(bothTokens)
	m.as = map[string]*forge.Fake{"tok-123": a.forge, "jane-456": a.forge.As(forge.Account{Login: "jane"})}
	return a, m, home
}

// A maintainer's verb run with --type (docs/swarm.md section 11, The
// interventions' record) makes its writes, then records the
// intervention: a row of kind intervention in the home's metrics CSV,
// its type in cause and the verb in outcome, and its line in the
// journal, which the next pass pushes; --type after <n> among the verb's
// own value flags, or leading as the cli reads a value flag; the issue's
// interventions numbered in turn.
func TestSwarmVerbRecordsItsIntervention(t *testing.T) {
	t.Parallel()
	a, m, home := interventionWS(t)
	if err := m.verb(maintainerVerb("block"), "2", "--type", "I7", "--by", "1"); err != nil {
		t.Fatal(err)
	}
	if want := "Maintainer jane on acme/swarm-app\n#1 blocking\n#2 commented on the issue\nIntervention 2-intervention-1 recorded: I7\n"; m.stdout.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", m.stdout.String(), want)
	}
	if b := a.bodies(2); b[len(b)-1] != "Blocked by #1" {
		t.Errorf("comment %q", b[len(b)-1])
	}
	m.values = []Value{{Flag: "--type", Value: "I7"}}
	if err := m.verb(maintainerVerb("pause"), "2"); err != nil {
		t.Fatal(err)
	}
	m.values = nil
	if want := "Maintainer jane on acme/swarm-app\n#2 paused\nIntervention 2-intervention-2 recorded: I7\n"; m.stdout.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", m.stdout.String(), want)
	}
	got := interventions(t, home)
	if len(got) != 2 {
		t.Fatalf("rows %v", got)
	}
	for i, want := range []map[string]string{
		{"when": "2026-11-06T15:00:00Z", "plan_or_swarm": "acme/swarm-app", "step_or_run": "2-intervention-1", "role": "maintainer", "outcome": "block", "cause": "I7", "notes": "#2", "model": "", "tokens_in": ""},
		{"step_or_run": "2-intervention-2", "outcome": "pause", "cause": "I7", "notes": "#2"},
	} {
		for c, v := range want {
			if got[i][c] != v {
				t.Errorf("row %d %s: %q, want %q", i+1, c, got[i][c], v)
			}
		}
		if len(got[i]["root_sha"]) != 40 || len(got[i]["home_sha"]) != 40 {
			t.Errorf("row %d shas %q %q", i+1, got[i]["root_sha"], got[i]["home_sha"])
		}
	}
	queue, _ := os.ReadFile(a.state("journal"))
	if want := "2026-11-06 - 1500 2-intervention-1: I7; block; #2\n2026-11-06 - 1500 2-intervention-2: I7; pause; #2\n"; string(queue) != want {
		t.Errorf("queue:\n%s\nwant:\n%s", queue, want)
	}
	a.pass()
	if journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md"); !strings.Contains(journal, "\n- 1500 2-intervention-1: I7; block; #2\n- 1500 2-intervention-2: I7; pause; #2\n") {
		t.Errorf("journal:\n%s", journal)
	}
}

// swarm intervene records an intervention no verb makes, on no issue,
// with the maintainer's note, and writes nothing else: no token read, no
// forge call.
func TestSwarmIntervene(t *testing.T) {
	t.Parallel()
	a, m, home := interventionWS(t)
	before := m.writes(1, 2)
	intervene := func(c *Context, _ opener) error { return swarmIntervene(c) }
	m.values = []Value{{Flag: "--type", Value: "I13"}}
	if err := m.verb(intervene, "Published", "58d0dbe", "as\na", "patch."); err != nil {
		t.Fatal(err)
	}
	if want := "Intervention 0-intervention-1 recorded: I13\n"; m.stdout.String() != want {
		t.Errorf("stdout %q", m.stdout.String())
	}
	if m.opened != nil || !slices.Equal(m.writes(1, 2), before) {
		t.Errorf("forge: opened %q, writes %q", m.opened, m.writes(1, 2)[len(before):])
	}
	got := interventions(t, home)
	if len(got) != 1 || got[0]["step_or_run"] != "0-intervention-1" || got[0]["outcome"] != "intervene" || got[0]["cause"] != "I13" || got[0]["notes"] != "Published 58d0dbe as a patch." {
		t.Errorf("rows %v", got)
	}
	if queue, _ := os.ReadFile(a.state("journal")); string(queue) != "2026-11-06 - 1500 0-intervention-1: I13; intervene; Published 58d0dbe as a patch.\n" {
		t.Errorf("queue %q", queue)
	}
}

// The claim of Step R0-7 of the plan implementing the 2026-11-19 swarm
// roadmap (section 11, The interventions' record): swarm intervene with
// --issue <n>, leading in either order with --type, records on issue n,
// `<n>-intervention-<k>` numbered after the issue's verbs' records, its
// notes and journal line ending `#<n>`, with no forge call (an issue
// the forge does not hold recorded as well); --issue that is not a
// positive issue number, or given twice, is usage, nothing written.
func TestSwarmInterveneIssue(t *testing.T) {
	t.Parallel()
	a, m, home := interventionWS(t)
	intervene := func(c *Context, _ opener) error { return swarmIntervene(c) }
	if err := m.verb(maintainerVerb("block"), "2", "--type", "I7", "--by", "1"); err != nil {
		t.Fatal(err)
	}
	m.opened = nil
	before := m.writes(1, 2)
	var u UsageError
	for _, issue := range [][]string{{"0"}, {"x"}, {"02"}, {"-3"}, {"2", "2"}} {
		m.values = []Value{{Flag: "--type", Value: "I13"}}
		for _, n := range issue {
			m.values = append(m.values, Value{Flag: "--issue", Value: n})
		}
		if err := m.verb(intervene, "Published", "a", "patch."); !errors.As(err, &u) {
			t.Errorf("--issue %q: %v, want usage", issue, err)
		}
	}
	if got := interventions(t, home); len(got) != 1 {
		t.Fatalf("a usage error wrote a row: %v", got)
	}

	m.values = []Value{{Flag: "--issue", Value: "2"}, {Flag: "--type", Value: "I13"}}
	if err := m.verb(intervene, "Published", "58d0dbe", "as", "a", "patch."); err != nil {
		t.Fatal(err)
	}
	if want := "Intervention 2-intervention-2 recorded: I13\n"; m.stdout.String() != want {
		t.Errorf("stdout %q", m.stdout.String())
	}
	m.values = []Value{{Flag: "--type", Value: "I9"}, {Flag: "--issue", Value: "9"}}
	if err := m.verb(intervene, "Merged", "the", "notes."); err != nil {
		t.Fatal(err)
	}
	m.values = nil
	if m.opened != nil || !slices.Equal(m.writes(1, 2), before) {
		t.Errorf("forge: opened %q, writes %q", m.opened, m.writes(1, 2)[len(before):])
	}
	got := interventions(t, home)
	if len(got) != 3 {
		t.Fatalf("rows %v", got)
	}
	for i, want := range []map[string]string{
		{"step_or_run": "2-intervention-2", "outcome": "intervene", "cause": "I13", "notes": "Published 58d0dbe as a patch., #2"},
		{"step_or_run": "9-intervention-1", "outcome": "intervene", "cause": "I9", "notes": "Merged the notes., #9"},
	} {
		for c, v := range want {
			if got[i+1][c] != v {
				t.Errorf("row %d %s: %q, want %q", i+2, c, got[i+1][c], v)
			}
		}
	}
	queue, _ := os.ReadFile(a.state("journal"))
	if want := "2026-11-06 - 1500 2-intervention-1: I7; block; #2\n" +
		"2026-11-06 - 1500 2-intervention-2: I13; intervene; Published 58d0dbe as a patch.; #2\n" +
		"2026-11-06 - 1500 9-intervention-1: I9; intervene; Merged the notes.; #9\n"; string(queue) != want {
		t.Errorf("queue:\n%s\nwant:\n%s", queue, want)
	}
}

// A type outside the catalogue's, --type without one or twice, words
// after <n> the verb does not take once --type is read out, and intervene
// without --type or a note are usage, refused before anything
// is read or written; a type with no home to record it in is refused
// before the verb's writes.
func TestSwarmInterventionRefused(t *testing.T) {
	t.Parallel()
	a, m, home := interventionWS(t)
	before := m.writes(1, 2)
	intervene := func(c *Context, _ opener) error { return swarmIntervene(c) }
	lead := func(typ string) []Value { return []Value{{Flag: "--type", Value: typ}} }
	var u UsageError
	for _, tc := range []struct {
		run    func(*Context, opener) error
		values []Value
		args   []string
	}{
		{maintainerVerb("block"), nil, []string{"2", "--by", "1", "--type", "I99"}},
		{maintainerVerb("block"), nil, []string{"2", "--type", "X", "--by", "1"}},
		{maintainerVerb("unblock"), nil, []string{"2", "--type", "I0", "Go", "on."}},
		{maintainerVerb("unblock"), nil, []string{"2", "--type", "I07", "Go", "on."}},
		{maintainerVerb("unblock"), nil, []string{"2", "--cause", "plan", "--type"}},
		{maintainerVerb("triage"), lead("I15"), []string{"2"}},
		{maintainerVerb("triage"), lead("I7"), []string{"2", "--type", "I7"}},
		{maintainerVerb("pause"), lead("i7"), nil},
		{maintainerVerb("class"), nil, []string{"2", "--type", "I6"}},
		{maintainerVerb("triage"), nil, []string{"2", "3"}},
		{maintainerVerb("pause"), nil, []string{"2", "--type", "I6", "junk"}},
		{maintainerVerb("block"), []Value{{Flag: "--by", Value: "1"}}, []string{"2", "--type", "I7", "junk"}},
		{intervene, lead("I99"), []string{"Edited", "swarm.json."}},
		{intervene, lead("X"), []string{"Edited", "swarm.json."}},
		{intervene, nil, []string{"Edited", "swarm.json."}},
		{intervene, lead("I5"), []string{" "}},
	} {
		m.values = tc.values
		if err := m.verb(tc.run, tc.args...); !errors.As(err, &u) {
			t.Errorf("%v %q: %v, want usage", tc.values, tc.args, err)
		}
	}
	m.values = nil
	if _, err := os.Stat(filepath.Join(home, "metrics", "metrics.csv")); err == nil {
		t.Error("a usage error wrote a row")
	}
	if _, err := os.Stat(a.state("journal")); err == nil {
		t.Error("a usage error queued a journal line")
	}
	if m.opened != nil || !slices.Equal(m.writes(1, 2), before) {
		t.Errorf("a usage error reached the forge: opened %q, writes %q", m.opened, m.writes(1, 2)[len(before):])
	}

	os.Remove(a.state("home"))
	err := m.verb(maintainerVerb("triage"), "2", "--type", "I12")
	if want := "Error: swarm: --type: " + a.state("home") + " names no Daisy home, whose metrics CSV would take the rows; write a registered home's name in it; nothing changed"; errText(err) != want {
		t.Errorf("%v\nwant %s", err, want)
	}
	if m.opened != nil || !slices.Equal(m.writes(1, 2), before) {
		t.Errorf("a refusal reached the forge: opened %q, writes %q", m.opened, m.writes(1, 2)[len(before):])
	}
}

// A comment command takes --type as the verb does (section 6, The
// comment commands): its answer says the intervention is recorded, and
// the journal holds the command's line and the intervention's.
func TestSwarmApplyCommandRecordsItsIntervention(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.maintainer()
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalPosted, forge.Blocked})
	id := a.say(issue1, "jane", "/swarm unblock --type I2 Main is green again.")
	a.pass()
	want := answer(id, "/swarm unblock --type I2 Main is green again.", "Done:\n\n    Maintainer jane on acme/swarm-app\n    #1 commented on the issue\n    #1 blocked cleared\n    Intervention 1-intervention-1 recorded: I2\n")
	if got := a.answers(issue1); !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("answers %q\nwant %q\n%s", got, want, a.stdout.String())
	}
	if b := a.bodies(1); !slices.Contains(b, "Main is green again.") {
		t.Errorf("comments %q", b)
	}
	if got := interventions(t, home); len(got) != 1 || got[0]["step_or_run"] != "1-intervention-1" || got[0]["outcome"] != "unblock" || got[0]["cause"] != "I2" {
		t.Errorf("rows %v", got)
	}
	journal := gitIn(t, a.upstreams[swarmURL], "show", "main:journal/2026-11-06.md")
	for _, line := range []string{fmt.Sprintf(" 1-command-%d: /swarm unblock --type I2 Main is green again. by jane; done\n", id), " 1-intervention-1: I2; unblock; #1"} {
		if !strings.Contains(journal, line) {
			t.Errorf("journal lacks %q:\n%s", line, journal)
		}
	}

	id = a.say(issue1, "jane", "/swarm pause --type I99")
	a.pass()
	if got := a.answers(issue1); len(got) != 2 || got[1] != answer(id, "/swarm pause --type I99", "Refused:\n\n    Error: swarm: --type takes one of I1 to I14, not \"I99\"\n") {
		t.Errorf("answers %q", got)
	}
	if got := interventions(t, home); len(got) != 1 {
		t.Errorf("rows %v", got)
	}
}

// A line queued while the pass pushes the journal (a maintainer's verb
// recording an intervention beside a running apply) stays queued for the
// next pass; the lines pushed are dropped (section 7.6, The run's
// record).
func TestSwarmJournalKeepsALineQueuedDuringItsPush(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	up := a.upstreams[swarmURL]
	queue := a.state("journal")
	once := filepath.Join(t.TempDir(), "once")
	late := "2026-11-06 - 1501 0-intervention-1: I13; intervene; Published a patch."
	os.WriteFile(filepath.Join(up, "hooks", "pre-receive"), []byte("#!/bin/sh\n[ -e "+once+" ] || { printf '%s\\n' '"+late+"' >> "+queue+"; touch "+once+"; }\nexit 0\n"), 0o755)
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{forge.ProposalAccepted})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
	a.pass()
	if !strings.Contains(gitIn(t, up, "show", "main:journal/2026-11-06.md"), " 1-architect-1: posted;") {
		t.Fatal("the run's line not pushed")
	}
	if b, _ := os.ReadFile(queue); string(b) != late+"\n" {
		t.Fatalf("queue %q", b)
	}
	a.pass()
	if journal := gitIn(t, up, "show", "main:journal/2026-11-06.md"); !strings.Contains(journal, "\n- 1501 0-intervention-1: I13; intervene; Published a patch.") {
		t.Errorf("the line queued during the push not pushed at the next pass:\n%s\n%s", journal, a.stderr.String())
	}
	if _, err := os.Stat(queue); err == nil {
		t.Error("the queue kept after its push")
	}
}
