package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"
	"time"

	"daisy/internal/swarm/forge"
)

// R3-13's claim, the prompt hash (the plan implementing the 2026-11-19
// swarm roadmap; section 7.3): a run's prompt parts follow its persona
// in system.md, each its own section in list order, its headings
// shifted, before the blocks after them (# Resources here; # Task before
// Step E1); system.md, whose sha256 is the run's
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
	want := "# not a heading\n```\n\n# Prompt part: prompts/parts/css.md\n\n## CSS rules\n\nUse the grid.\n\n# Prompt part: prompts/parts/tokens.md\n\nSpend few tokens.\n\n# Resources\n"
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

// Step E1's claim (the plan implementing the 2026-11-19 swarm roadmap;
// section 7.3): Render writes the blocks every run of a role shares
// first, the persona, its parts, praxis, testing, the repository notes
// and the resources, and the run's own header and # Task last, so two
// runs of one role on different issues share every byte of system.md
// up to the header, the prefix a provider's prompt cache keeps.
func TestRenderStableBlocksFirst(t *testing.T) {
	f, at := fixture()
	f.Comment(at, marker("architect", 1)+archPlan)
	other, _ := f.CreateIssue("Lint check", "Check the lint.", []string{"swarm:plan-1-accepted"})
	f.Comment(forge.Ref{Repo: "", N: other}, marker("architect", 1)+archPlan)
	render := func(n, k int) string {
		j := job(f, n, "worker", "swarm:plan-1-accepted")
		j.K, j.Time = k, j.Time.Add(time.Duration(k)*time.Hour)
		j.Parts = []PromptPart{{Path: "prompts/parts/css.md", Text: "Use the grid.\n"}}
		j.Praxis, j.Testing = "## Rules\n\n1. Read first.\n", "---\nname: testing\n---\n\nA test can fail.\n"
		j.Repos = []RepoNote{{Name: "lib", Text: "Run it with make.\n"}}
		sys, _ := Render(j)
		return sys
	}
	one, two := render(at.N, 1), render(other, 2)
	var headings []string
	fenced := false
	for _, l := range strings.Split(one, "\n") {
		if strings.HasPrefix(l, "```") {
			fenced = !fenced
		}
		if !fenced && (strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "<!-- daisy-swarm system: ")) {
			headings = append(headings, strings.Fields(l + " ")[0]+" "+strings.SplitN(l, " ", 3)[1])
		}
	}
	want := []string{"# Persona", "# Prompt", "# Praxis", "# Testing", "# Repository", "# Resources", "<!-- daisy-swarm", "# Task"}
	if !slices.Equal(headings, want) {
		t.Errorf("the blocks' order %q, want %q:\n%s", headings, want, one)
	}
	cut := strings.Index(one, "<!-- daisy-swarm system: ")
	if cut < 0 || cut != strings.Index(two, "<!-- daisy-swarm system: ") || one[:cut] != two[:cut] {
		t.Fatalf("the two runs' system.md differ before the header (at %d):\n%s\n----\n%s", cut, one, two)
	}
	if !strings.HasSuffix(one[:cut], "- /job/resources/app/docs\n\n") || one == two {
		t.Errorf("the shared prefix does not end with the resources, or the runs' Task is one:\n%s", one)
	}
	if head := "<!-- daisy-swarm system: acme/app#1 worker run 1, 2026-11-06T16:00:00Z -->\n\n# Task\n\nImplement slice 1"; !strings.HasPrefix(one[cut:], head) {
		t.Errorf("the per-run part does not begin %q:\n%s", head, one[cut:])
	}
}

// Step E1's hash: config_hash covers the persona and its parts, from
// `# Persona` to the first block after them, so a system.md written in
// the order before Step E1 (# Task after the parts) and one written in
// the order after it (# Praxis, # Repository notes or # Resources after
// them) hash alike for the same persona and parts: the sha256 of that
// region, given here as text.
func TestConfigHashAcrossTheOrders(t *testing.T) {
	region := "# Persona\n\n## architect\n\nYou do it.\n\n```\n# Task\n```\n\n# Prompt part: prompts/parts/css.md\n\nUse the grid.\n\n"
	sum := sha256.Sum256([]byte(region))
	want := hex.EncodeToString(sum[:])
	before := "<!-- daisy-swarm system: acme/app#1 architect run 1, 2026-11-06T15:00:00Z -->\n\n" + region + "# Task\n\nWrite the plan.\n\n# Resources\n\nNone.\n"
	for name, sys := range map[string]string{
		"before Step E1":        before,
		"after, praxis next":    region + "# Praxis\n\n## Rules\n\n# Testing\n\nA test.\n\n# Resources\n\nNone.\n\n<!-- daisy-swarm system: acme/app#1 architect run 2, 2026-11-06T16:00:00Z -->\n\n# Task\n\nRevise the plan.\n",
		"after, notes next":     region + "# Repository notes: app\n\nRun it.\n\n# Resources\n\nNone.\n\n<!-- daisy-swarm system: acme/app#2 owner run 1, 2026-11-06T16:00:00Z -->\n\n# Task\n\nAnswer.\n",
		"after, resources next": region + "# Resources\n\nNone.\n\n<!-- daisy-swarm system: acme/app#3 adversary run 1, 2026-11-06T16:00:00Z -->\n\n# Task\n\nReview.\n",
	} {
		if got := configHash([]byte(sys)); got != want {
			t.Errorf("%s: configHash %q, want %q", name, got, want)
		}
	}
}
