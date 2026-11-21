package reconcile

import (
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

// R3-13's claim, the prompt hash (the plan implementing the 2026-11-19
// swarm roadmap; section 7.3): a run's prompt parts follow its persona
// in system.md, each its own section in list order, its headings
// shifted, before # Task; system.md, whose sha256 is the run's
// prompt_hash (7.6), changes with a part's text and with nothing else
// of the job changed.
func TestRenderPromptParts(t *testing.T) {
	f, at := fixture()
	render := func(css string) string {
		j := job(f, at.N, "architect", "swarm:proposal-accepted")
		j.Parts = []PromptPart{{Path: "prompts/parts/css.md", Text: css}, {Path: "prompts/parts/tokens.md", Text: "Spend few tokens.\n"}}
		sys, _ := Render(j)
		return sys
	}
	sys := render("# CSS rules\n\nUse the grid.\n")
	want := "# not a heading\n```\n\n# Prompt part: prompts/parts/css.md\n\n## CSS rules\n\nUse the grid.\n\n# Prompt part: prompts/parts/tokens.md\n\nSpend few tokens.\n\n# Task\n"
	if !strings.Contains(sys, want) {
		t.Errorf("system.md:\n%s\nwant within:\n%s", sys, want)
	}
	if sha256.Sum256([]byte(render("# CSS rules\n\nUse the grid.\n"))) != sha256.Sum256([]byte(sys)) {
		t.Error("the same job rendered twice hashes otherwise")
	}
	if sha256.Sum256([]byte(render("# CSS rules\n\nUse flex.\n"))) == sha256.Sum256([]byte(sys)) {
		t.Error("a part's text changed, the hash did not")
	}
}

// R3-12b's hash (the plan implementing the 2026-11-19 swarm roadmap;
// section 7.6, The run's record): configHash reads system.md from its
// `# Persona` line up to its `# Task` line, so two runs of one role on
// other issues and times, their Task and header otherwise, hash alike;
// a part's text changed, or a persona line after a fenced `# Task`
// (a line Render does not shift), hashes otherwise; a file without
// both headings has none.
func TestConfigHash(t *testing.T) {
	f, at := fixture()
	other, _ := f.CreateIssue("Lint check", "Check the lint.", []string{"swarm:plan-1-accepted"})
	render := func(n int, label, persona, part string, k int) []byte {
		j := job(f, n, "architect", label)
		j.K, j.Time = k, j.Time.Add(time.Duration(k)*time.Hour)
		if persona != "" {
			j.Persona = persona
		}
		j.Parts = []PromptPart{{Path: "prompts/parts/css.md", Text: part}}
		sys, _ := Render(j)
		return []byte(sys)
	}
	one := render(at.N, "swarm:proposal-accepted", "", "Use the grid.\n", 1)
	two := render(other, "swarm:plan-1-accepted", "", "Use the grid.\n", 2)
	if string(one) == string(two) {
		t.Fatal("the two runs' system.md are the same")
	}
	if configHash(one) == "" || configHash(one) != configHash(two) {
		t.Errorf("configHash %q and %q, want one, not empty", configHash(one), configHash(two))
	}
	if configHash(render(at.N, "swarm:proposal-accepted", "", "Use flex.\n", 1)) == configHash(one) {
		t.Error("a part's text changed, the hash did not")
	}
	fenced := func(last string) string { return "# architect\n\n```\n# Task\n```\n\n" + last + "\n" }
	if configHash(render(at.N, "swarm:proposal-accepted", fenced("Plan."), "Use the grid.\n", 1)) ==
		configHash(render(at.N, "swarm:proposal-accepted", fenced("Plan more."), "Use the grid.\n", 1)) {
		t.Error("a persona line after a fenced # Task left out of the hash")
	}
	for _, sys := range []string{"", "# Persona\n\nYou do it.\n", "<!-- head -->\n\n# Task\n\nDo it.\n"} {
		if h := configHash([]byte(sys)); h != "" {
			t.Errorf("configHash(%q) = %q, want none", sys, h)
		}
	}
}
