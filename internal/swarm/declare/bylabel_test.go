package declare

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// byLabel gives role's persona the by_label entries.
func byLabel(c *Config, role string, entries ...ByLabel) {
	setPersona(c, role, func(p *Persona) { p.ByLabel = entries })
}

// archOpus is the architect's model on an architecture job.
var archOpus = ByLabel{"swarm:class-architecture", "anthropic", "claude-opus-5-5", "high", "claude-opus-5-5 <noreply@anthropic.com>", nil}

// A by_label entry's label is not empty and listed once, and its fields
// and slots are validated as a fallback entry's, its provider with a
// credential in keys (section 3, personas.<role>.by_label); an entry
// the same as the persona or a fallback entry is valid, and so is one
// keyed by any label.
func TestValidateByLabel(t *testing.T) {
	_, clone := cloneFolder(t)
	p := Clone(clone)
	for _, tc := range []struct {
		want string
		edit func(c *Config)
	}{
		{`swarm.json: personas.architect.by_label[0].label: empty; every by_label entry names its label`, func(c *Config) {
			byLabel(c, "architect", ByLabel{"", "anthropic", "claude-opus-5-5", "high", "", nil})
		}},
		{`swarm.json: personas.architect.by_label[1].label: "swarm:class-architecture" is listed twice`, func(c *Config) {
			byLabel(c, "architect", archOpus, ByLabel{"swarm:class-architecture", "openai", "gpt-6.1-sol", "high", "", nil})
		}},
		{`swarm.json: personas.architect.by_label[0].provider: "gemini" has no adapter`, func(c *Config) {
			byLabel(c, "architect", ByLabel{"x", "gemini", "g-1", "high", "", nil})
		}},
		{`swarm.json: personas.architect.by_label[0].model: empty; every role names its model`, func(c *Config) {
			byLabel(c, "architect", ByLabel{"x", "anthropic", "", "high", "", nil})
		}},
		{`swarm.json: personas.architect.by_label[1].effort: "Very High" is not one lowercase word`, func(c *Config) {
			byLabel(c, "architect", archOpus, ByLabel{"y", "anthropic", "claude-opus-5-5", "Very High", "", nil})
		}},
		{`swarm.json: personas.architect.by_label[0].co_author: "claude-opus-5-5" is not <name> <email>`, func(c *Config) {
			byLabel(c, "architect", ByLabel{"x", "anthropic", "claude-opus-5-5", "high", "claude-opus-5-5", nil})
		}},
		{`swarm.json: personas.worker.by_label[0].slots: anthropic's credential is env, which has no slots`, func(c *Config) {
			byLabel(c, "worker", ByLabel{"x", "anthropic", "claude-opus-5-5", "high", "", []int{1}})
		}},
		{`swarm.json: personas.worker.by_label[0].slots[0]: 2 is not between 1 and 1`, func(c *Config) {
			byLabel(c, "worker", ByLabel{"x", "openai", "gpt-6.1-sol", "high", "", []int{2}})
		}},
		{`swarm.json: keys: no credential for provider "xai"`, func(c *Config) {
			byLabel(c, "worker", ByLabel{"x", "xai", "grok-4.7", "high", "", nil})
		}},
	} {
		c := valid()
		c.Keys = map[string]Key{"anthropic": c.Keys["anthropic"], "openai": c.Keys["openai"]}
		tc.edit(&c)
		if got := errText(Validate(ops.Real(), p, c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}
	c := valid()
	fallback(&c, "architect", opus)
	byLabel(&c, "architect", archOpus, ByLabel{"area:docs", "anthropic", "claude-fable-5-1", "high", "", nil}, ByLabel{"hard", "openai", "gpt-6.1-sol", "high", "", []int{1}})
	if err := Validate(ops.Real(), p, c, 0); err != nil {
		t.Errorf("by_label: %v", err)
	}
}

// A persona's by_label is written on its line after its fallback, each
// entry's label first and then its fields in the persona's order, and
// read back; with none the line is as before (section 3,
// personas.<role>.by_label).
func TestFormatByLabel(t *testing.T) {
	c := valid()
	before := string(Format(c))
	if strings.Contains(before, `"by_label"`) {
		t.Errorf("Format wrote a by_label unset:\n%s", before)
	}
	fallback(&c, "architect", opus)
	byLabel(&c, "architect", archOpus, ByLabel{"hard", "openai", "gpt-6.1-sol", "high", "", []int{1}})
	out := string(Format(c))
	want := `"fallback": [{"provider": "anthropic", "model": "claude-opus-5-5", "effort": "high", "co_author": "claude-opus-5-5 <noreply@anthropic.com>"}], "by_label": [{"label": "swarm:class-architecture", "provider": "anthropic", "model": "claude-opus-5-5", "effort": "high", "co_author": "claude-opus-5-5 <noreply@anthropic.com>"}, {"label": "hard", "provider": "openai", "model": "gpt-6.1-sol", "effort": "high", "co_author": "", "slots": [1]}]},` + "\n"
	if !strings.Contains(out, `    "architect":   {"provider": "anthropic", "model": "claude-fable-5-1",  "effort": "high",   `) || !strings.Contains(out, want) {
		t.Errorf("Format lacks %q on the architect's line:\n%s", want, out)
	}
	if strings.Count(out, "\n") != strings.Count(before, "\n") {
		t.Errorf("by_label took a line of its own:\n%s", out)
	}
	back, err := Parse([]byte(out))
	if err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v\n%+v", err, back.Personas["architect"])
	}
	if again := string(Format(back)); again != out {
		t.Errorf("written again:\n%s", again)
	}
}

// A file's by_label is kept by the wizard, whatever the persona's
// answers: it is not asked (section 3, personas.<role>.by_label: init
// writes none, and keeps the file's), and init's file writes it back.
func TestWizardKeepsByLabel(t *testing.T) {
	_, clone := cloneFolder(t)
	answers := make([]string, 40)
	answers[6+1*4+1] = "claude-sonnet-5-5" // the architect's model
	o, _ := scripted(answers...)
	file := valid()
	byLabel(&file, "architect", archOpus)
	w := &Wizard{Ops: o, Err: io.Discard, Place: Clone(clone), File: &file}
	c, err := w.Ask()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Personas["architect"]; got.Model != "claude-sonnet-5-5" || !reflect.DeepEqual(got.ByLabel, file.Personas["architect"].ByLabel) {
		t.Errorf("architect %+v", got)
	}
	if got := c.Personas["worker"].ByLabel; got != nil {
		t.Errorf("worker's by_label %+v", got)
	}
	if want := `"by_label": [{"label": "swarm:class-architecture", "provider": "anthropic", "model": "claude-opus-5-5"`; !strings.Contains(string(Format(c)), want) {
		t.Errorf("init's file lacks %q:\n%s", want, Format(c))
	}
}
