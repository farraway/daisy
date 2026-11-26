package cmd

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// withByLabel gives role's persona the by_label entries (section 3,
// personas.<role>.by_label).
func (a *applyWS) withByLabel(role string, entries ...declare.ByLabel) {
	a.t.Helper()
	a.edit(func(c *declare.Config) {
		p := c.Personas[role]
		p.ByLabel = entries
		c.Personas[role] = p
	})
}

// labelled is e chosen by label.
func labelled(label string, e declare.Entry) declare.ByLabel {
	return declare.ByLabel{Label: label, Provider: e.Provider, Model: e.Model, Effort: e.Effort, CoAuthor: e.CoAuthor, Slots: e.Slots}
}

// The architect on an issue carrying swarm:class-architecture runs on
// its by_label entry for that label, the first of its entries whose
// label the issue carries, its persona line naming it, with no capacity
// note (it heads the issue's chain); on a routine issue, which carries
// no class label, on the persona
// (section 7.10, The model by label).
func TestSwarmArchitectRunsTheLabelsModel(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.forge.AddLabels("hard", "area:none")
	a.withByLabel("architect", labelled("area:none", solEntry), labelled("swarm:class-architecture", opusEntry), labelled("hard", declare.Entry{Provider: "openai", Model: "gpt-6.1-sol", Effort: "medium"}))
	a.forge.CreateIssue("Arch", "x", []string{"swarm:proposal-accepted", "swarm:class-architecture", "hard"})
	a.forge.CreateIssue("Routine", "x", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.pass()
	models := map[string]string{}
	for _, l := range a.runner.Launches() {
		models[l.Label[strings.LastIndex(l.Label, "#"):]] = model(l)
	}
	if want := map[string]string{"#1/architect": "claude-opus-5-5", "#2/architect": "claude-fable-5-1"}; !reflect.DeepEqual(models, want) {
		t.Errorf("models %v\n%s", models, a.stdout.String())
	}
	if r := a.result("1-architect-1"); !strings.Contains(r, "\npersona anthropic claude-opus-5-5 high\n") || !strings.HasSuffix(r, "outcome posted\ncollected\n") {
		t.Errorf("#1:\n%s", r)
	}
	if r := a.result("2-architect-1"); !strings.Contains(r, "\npersona anthropic claude-fable-5-1 high\n") {
		t.Errorf("#2:\n%s", r)
	}
	if n := a.capacityNotes(1); len(n) != 0 {
		t.Errorf("a capacity note on the override: %q", n)
	}
}

// The by_label entry heads the role's own chain, its fallback after it
// unchanged (section 7.10, The model by label): with the override's
// model unavailable the architect on an architecture issue runs on the
// persona, the capacity note naming the override in its place; with the
// persona's unavailable too, on the fallback.
func TestSwarmByLabelFallsBackToTheRolesChain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, capacity, model, note string
	}{
		{"to the persona", "anthropic/env claude-opus-5-5 until resume: Claude AI usage limit reached\n", "claude-fable-5-1",
			"**reconciler:** the architect runs on anthropic claude-fable-5-1 (high) in place of anthropic claude-opus-5-5 (high): anthropic/env claude-opus-5-5 until resume: Claude AI usage limit reached."},
		{"to the fallback", "anthropic/env claude-opus-5-5 until resume: Claude AI usage limit reached\nanthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached\n", "gpt-6.1-sol",
			"**reconciler:** the architect runs on openai gpt-6.1-sol (high) in place of anthropic claude-opus-5-5 (high): anthropic/env claude-opus-5-5 until resume: Claude AI usage limit reached; anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newApplyWS(t)
			a.withFallback("architect", solEntry)
			a.withByLabel("architect", labelled("swarm:class-architecture", opusEntry))
			a.forge.CreateIssue("Arch", "x", []string{"swarm:proposal-accepted", "swarm:class-architecture"})
			os.MkdirAll(a.state(""), 0o755)
			os.WriteFile(a.state("capacity"), []byte(tc.capacity), 0o644)
			a.runner.Script(1, "architect", runner.Canned{Out: archPlan})
			a.pass()
			if l := a.launched("architect"); len(l) != 1 || model(l[0]) != tc.model {
				t.Fatalf("launches %v\n%s", l, a.stdout.String())
			}
			if n := a.capacityNotes(1); !reflect.DeepEqual(n, []string{tc.note}) {
				t.Errorf("notes %q", n)
			}
		})
	}
}

// status shows each by_label entry under its persona, and in Capacity a
// line per entry beside the persona's, each with the issues and members
// whose labels choose it and what holds its chain, the bottleneck
// naming the line; JSON personas.<role>.by_label and capacity's label
// (section 7.10, The model by label; section 11).
func TestSwarmStatusShowsByLabel(t *testing.T) {
	t.Parallel()
	s := newStatusWS(t)
	s.edit(func(c *declare.Config) {
		p := c.Personas["architect"]
		p.ByLabel = []declare.ByLabel{labelled("swarm:class-architecture", opusEntry)}
		c.Personas["architect"] = p
	})
	arch := []string{"swarm:proposal-accepted", "swarm:class-architecture"}
	s.forge.CreateIssue("One", "x", arch)
	s.forge.CreateIssue("Two", "x", []string{"swarm:proposal-accepted"})
	s.forge.CreateIssue("Three", "x", arch)
	s.forge.CreateIssue("Four", "x", arch)
	s.forge.CreateIssue("Five", "x", arch)
	os.MkdirAll(s.state(""), 0o755)
	os.WriteFile(s.state("capacity"), []byte("anthropic/env claude-opus-5-5 until resume: Claude AI usage limit reached\n"+
		"anthropic/env claude-fable-5-1 until resume: Claude AI usage limit reached\n"), 0o644)
	s.docker = "daisy-acme-app-5-architect-1 acme/swarm-app#5/architect anthropic/env\n"
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	held := "anthropic/env claude-opus-5-5 unavailable until resume; anthropic/env claude-fable-5-1 unavailable until resume"
	for _, want := range []string{
		"  architect    anthropic  claude-fable-5-1 high\n    on swarm:class-architecture: anthropic claude-opus-5-5 high\n  worker ",
		"  architect    queue 1, running 0: anthropic/env claude-fable-5-1 unavailable until resume\n" +
			"  architect on swarm:class-architecture queue 3, running 1: " + held + "\n  worker ",
		"  Bottleneck: architect on swarm:class-architecture (" + held + ")\n",
	} {
		if out := s.stdout.String(); !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	s.flags = []string{"--json"}
	if err := s.status(s.forge); err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(s.stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	var personas map[string]any
	json.Unmarshal(doc["personas"], &personas)
	got, _ := json.Marshal(personas["architect"])
	if want := `{"by_label":[{"effort":"high","label":"swarm:class-architecture","model":"claude-opus-5-5","provider":"anthropic"}],"effort":"high","model":"claude-fable-5-1","provider":"anthropic"}`; string(got) != want {
		t.Errorf("personas.architect %s", got)
	}
	var v any
	json.Unmarshal(doc["capacity"], &v)
	got, _ = json.Marshal(v)
	for _, want := range []string{
		`"bottleneck":{"label":"swarm:class-architecture","role":"architect","why":"` + held + `"}`,
		`{"label":null,"queue":[2],"role":"architect","running":0,"why":"anthropic/env claude-fable-5-1 unavailable until resume"},{"label":"swarm:class-architecture","queue":[1,3,4],"role":"architect","running":1,"why":"` + held + `"}`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("capacity: no %s in\n%s", want, got)
		}
	}
}

// The worker on its by_label entry commits with the entry's co_author,
// which its collection checks the commits against: none reported
// unstamped (section 7.10, The model by label).
func TestSwarmWorkerCommitsCarryTheLabelsCoAuthor(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.withByLabel("worker", labelled("swarm:class-architecture", opusEntry))
	a.planned(oneSlice)
	if err := a.forge.Relabel(issue1, "", "swarm:class-architecture"); err != nil {
		t.Fatal(err)
	}
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	msg, _, _ := ops.Real().Exec(a.upstream("app"), "git", "log", "-1", "--format=%B", "swarm/1-1")
	r := a.result("1-worker-1")
	if !strings.Contains(string(msg), "Co-Authored-By: claude-opus-5-5 <noreply@anthropic.com>") || !strings.Contains(r, "\npersona anthropic claude-opus-5-5 high\n") ||
		strings.Contains(r, "unstamped") || strings.Contains(a.stderr.String(), "Co-Authored-By") {
		t.Errorf("the pushed commit:\n%s\nstderr %q\n%s", msg, a.stderr.String(), r)
	}
}
