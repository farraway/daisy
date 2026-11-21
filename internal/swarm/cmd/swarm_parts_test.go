package cmd

import (
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// R3-13's claim (the plan implementing the 2026-11-19 swarm roadmap;
// sections 3, prompt_parts, and 7.3): a part naming the worker and the
// adversary, prompts/parts/css.md, is in both runs' system.md after the
// role's prompt, its own section, and in no other role's (the
// architect's, the sympathetic reviewer's); a second part naming the
// worker alone follows it, in the list's order.
func TestSwarmPromptParts(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.write("prompts/parts/css.md", "# CSS rules\n\nUse the grid.\n")
	a.write("prompts/parts/tokens.md", "Spend few tokens.\n")
	a.edit(func(c *declare.Config) {
		c.PromptParts = []declare.PromptPart{
			{Path: "prompts/parts/css.md", Roles: []string{"worker", "adversary"}},
			{Path: "prompts/parts/tokens.md", Roles: []string{"worker"}},
		}
	})
	a.inReview(oneSlice)
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved
	css := "\n# Prompt part: prompts/parts/css.md\n\n## CSS rules\n\nUse the grid.\n"
	tokens := "\n# Prompt part: prompts/parts/tokens.md\n\nSpend few tokens.\n"
	for role, parts := range map[string][]string{"worker": {css, tokens}, "adversary": {css}, "architect": nil, "sympathetic": nil} {
		sys := a.file("1-"+role+"-1", "io/system.md")
		if sys == "" {
			t.Fatalf("no system.md for %s\n%s", role, a.stderr.String())
		}
		at := strings.Index(sys, "\n# Persona\n")
		task := strings.Index(sys, "\n# Task\n")
		for _, p := range parts {
			i := strings.Index(sys, p)
			if i <= at || i > task {
				t.Errorf("%s: %q not between # Persona (%d) and # Task (%d), at %d:\n%s", role, p, at, task, i, sys)
			}
			at = i
		}
		if got := strings.Count(sys, "# Prompt part: "); got != len(parts) {
			t.Errorf("%s: %d parts, want %d:\n%s", role, got, len(parts), sys)
		}
	}
}
