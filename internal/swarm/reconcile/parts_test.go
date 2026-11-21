package reconcile

import (
	"crypto/sha256"
	"strings"
	"testing"
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
