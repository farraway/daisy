package prompt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bounded = "## Trigger\n\nRead this.\n\n## Rules\n\n1. Measure.\n\n### Exceptions\n\n2. Cut.\n\n# Document Title\n\n## Next Section\n\nBody text.\n"

// The section runs from its `## Rules` heading to the next level-1 or
// level-2 heading (case 53); a `###` is inside it, and a `#` line in a
// fence is no heading.
func TestRulesSectionBoundary(t *testing.T) {
	for _, tc := range []struct{ doc, want string }{
		{bounded, "## Rules\n\n1. Measure.\n\n### Exceptions\n\n2. Cut.\n\n"},
		{"## Rules\n1. a\n## Next\n", "## Rules\n1. a\n"},
		{"## Rules\n1. a\n#\nx\n", "## Rules\n1. a\n"},
		{"## Rules\n1. a\n#hashtag\n", "## Rules\n1. a\n#hashtag\n"},
		{"## Rules\n1. a\n```sh\n# a comment\n## not a heading\n```\n2. b\n## End\n", "## Rules\n1. a\n```sh\n# a comment\n## not a heading\n```\n2. b\n"},
		{"x\n## Rules\n1. a", "## Rules\n1. a"},
		{"## Rules of the loop\n1. a\n## End\n", "## Rules of the loop\n1. a\n"},
		{"## Rulesless\n1. a\n", ""},
		{"## Rules  \n1. a\n", "## Rules  \n1. a\n"},
		{"# T\n## Other\n", ""},
	} {
		start, end, ok := RulesSection([]byte(tc.doc))
		got := ""
		if ok {
			got = tc.doc[start:end]
		}
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("%q: got %q (ok %v), want %q", tc.doc, got, ok, tc.want)
		}
	}
}

// A rule is a list item at the left margin, numbered or bulleted, outside
// a fence.
func TestCountRules(t *testing.T) {
	for _, tc := range []struct {
		doc  string
		want int
	}{
		{bounded, 2},
		{"## Rules\n\n- a\n* b\n  - nested\n10. c\n1.no\n```\n1. fenced\n```\n", 3},
		{"## Other\n1. a\n", 0},
	} {
		if got := CountRules([]byte(tc.doc)); got != tc.want {
			t.Errorf("%q: %d rules, want %d", tc.doc, got, tc.want)
		}
	}
}

// Replacing the rules with themselves changes nothing, byte for byte,
// whatever the gap before the next heading; a proposal's own trailing
// blank lines do not change the gap; the rest of the document survives.
func TestReplaceRules(t *testing.T) {
	for _, doc := range []string{bounded, "## Rules\n1. a\n\n\n\n## Next\n", "## Rules\n1. a", "x\n## Rules\n\n1. a\n"} {
		s, e, _ := RulesSection([]byte(doc))
		section := strings.TrimRight(doc[s:e], "\n") + "\n"
		got, err := ReplaceRules([]byte(doc), []byte(section))
		if err != nil || string(got) != doc {
			t.Errorf("identity on %q: got %q, %v", doc, got, err)
		}
	}
	got, err := ReplaceRules([]byte(bounded), []byte("\n\n## Rules\n\n1. New.\n\n\n"))
	want := "## Trigger\n\nRead this.\n\n## Rules\n\n1. New.\n\n# Document Title\n\n## Next Section\n\nBody text.\n"
	if err != nil || string(got) != want {
		t.Errorf("replace: got %q, %v", got, err)
	}
}

// Without a section the rules go before the first `## ` heading other than
// the Trigger, one blank line apart, or at the end.
func TestReplaceRulesInserts(t *testing.T) {
	for _, tc := range []struct{ doc, want string }{
		{"## Trigger\n\nT.\n\n## Body\n\nB.\n", "## Trigger\n\nT.\n\n## Rules\n\n1. a\n\n## Body\n\nB.\n"},
		{"---\nname: x\n---\n\nIntro.\n", "---\nname: x\n---\n\nIntro.\n\n## Rules\n\n1. a\n"},
		{"Intro.", "Intro.\n\n## Rules\n\n1. a\n"},
		{"", "## Rules\n\n1. a\n"},
	} {
		if got, err := ReplaceRules([]byte(tc.doc), []byte("## Rules\n\n1. a\n")); err != nil || string(got) != tc.want {
			t.Errorf("%q: got %q, %v", tc.doc, got, err)
		}
	}
}

// A proposal is one `## Rules` section: it starts with the heading and
// holds no other level-1 or level-2 heading, which would end the section
// the next run reads back.
func TestReplaceRulesRefusesAProposalThatIsNotOneSection(t *testing.T) {
	for _, tc := range []struct{ proposal, want string }{
		{"1. a\n", "must begin with a '## Rules' heading"},
		{"", "must begin with a '## Rules' heading"},
		{"## Rulesless\n1. a\n", "must begin with a '## Rules' heading"},
		{"## Rules\n1. a\n\n## Another Section\n", "line 4: '## Another Section'"},
		{"## Rules\n1. a\n# Title\n", "line 3: '# Title'"},
	} {
		if _, err := ReplaceRules([]byte(bounded), []byte(tc.proposal)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: err %v, want %q", tc.proposal, err, tc.want)
		}
	}
	if _, err := ReplaceRules([]byte(bounded), []byte("## Rules\n```\n## fenced\n```\n### Sub\n")); err != nil {
		t.Errorf("a fenced heading and an H3 are inside: %v", err)
	}
}

// Markdown's rules, as CommonMark has them: a fence closes only at a run
// of its own character at least as long; a heading may be indented up to
// three spaces and may close with `#`s.
func TestRulesSectionMarkdown(t *testing.T) {
	for _, tc := range []struct{ doc, want string }{
		{"## Rules\n```\n~~~\n## in\n```\n## End\n", "## Rules\n```\n~~~\n## in\n```\n"},
		{"## Rules\n````\n```\n## in\n````\n## End\n", "## Rules\n````\n```\n## in\n````\n"},
		{"## Rules\n1. a\n   ## End\n", "## Rules\n1. a\n"},
		{"## Rules\n1. a\n    ## code\n", "## Rules\n1. a\n    ## code\n"},
		{"## Rules ##\n1. a\n", "## Rules ##\n1. a\n"},
		{"  ## Rules\n1. a\n", "  ## Rules\n1. a\n"},
		{"## Rules\n1. a\n```inline``` is code, not a fence\n2. b\n## Next\n", "## Rules\n1. a\n```inline``` is code, not a fence\n2. b\n"},
		{"## Rules\n~~~ a`b\n## in\n~~~\n## Next\n", "## Rules\n~~~ a`b\n## in\n~~~\n"},
	} {
		start, end, ok := RulesSection([]byte(tc.doc))
		if got := tc.doc[start:end]; !ok || got != tc.want {
			t.Errorf("%q: got %q (ok %v), want %q", tc.doc, got, ok, tc.want)
		}
	}
}

// A proposal leaving a fence open would swallow the rest of the file into
// the section on the next read (review pass 1, F1): refused, and whatever
// the shape, the result must read back as the proposal.
func TestReplaceRulesRefusesAnOpenFence(t *testing.T) {
	for _, p := range []string{"## Rules\n\n1. a\n\n```\nunclosed\n", "## Rules\n~~~\n"} {
		if _, err := ReplaceRules([]byte(bounded), []byte(p)); err == nil || !strings.Contains(err.Error(), "unclosed") {
			t.Errorf("%q: err %v", p, err)
		}
	}
	// A target whose own fence is open before the section cannot take one.
	if _, err := ReplaceRules([]byte("```\n"), []byte("## Rules\n1. a\n")); err == nil || !strings.Contains(err.Error(), "fence opened at line 1") {
		t.Errorf("open fence in the target: err %v", err)
	}
}

// A setext underline in the section makes the boundary ambiguous: refused,
// in the target and in a proposal, rather than read past (review pass 3,
// F12), whatever text line is above it, a list item too (F15). A `---`
// after a blank line is a thematic break.
func TestReplaceRulesRefusesASetextHeading(t *testing.T) {
	for _, target := range []string{
		"# W\n## Rules\n1. Old.\nExamples\n--------\nKeep this.\n",
		"# W\n## Rules\n1. Old.\n2. Agenda\n---\nKeep this.\n",         // after a list item (review pass 4, F15)
		"# W\r\n## Rules\r\n1. Old.\r\nNotes\r\n---\r\nKeep this.\r\n", // CRLF (F16)
	} {
		var te TargetError
		if _, err := ReplaceRules([]byte(target), []byte("## Rules\n1. New.\n")); !errors.As(err, &te) || !strings.Contains(err.Error(), "line 5 of the file (in its ## Rules section) may be a setext heading underline") {
			t.Errorf("%q: %v", target, err)
		}
	}
	if _, err := ReplaceRules([]byte(bounded), []byte("## Rules\n1. a\nTitle\n=====\n")); err == nil || !strings.Contains(err.Error(), "setext heading underline at line 4") {
		t.Errorf("proposal: %v", err)
	}
	for _, ok := range []string{"## Rules\n1. a\n\n---\n2. b\n", "## Rules\n```\nx\n---\n```\n"} {
		if _, err := ReplaceRules([]byte(bounded), []byte(ok)); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
}

// Every prompt and skill the root ships takes its own rules back,
// unchanged: the boundary and the refusals agree with the markdown Daisy
// is written in.
func TestShippedRulesRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("../../prompts/*.md")
	skills, _ := filepath.Glob("../../skills/*/SKILL.md")
	n := 0
	for _, f := range append(files, skills...) {
		doc, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s, e, ok := RulesSection(doc)
		if !ok {
			continue
		}
		n++
		if got, err := ReplaceRules(doc, doc[s:e]); err != nil || string(got) != string(doc) {
			t.Errorf("%s: %v", f, err)
		}
	}
	if n < 5 {
		t.Errorf("only %d shipped files with rules", n)
	}
}

// CRLF lines end at the CR: `##\r` is a heading (F16); a closing fence
// allows only spaces and tabs after its run, so a no-break space leaves
// the fence open (F17).
func TestRulesLineEndsAndFenceClosers(t *testing.T) {
	doc := "## Rules\r\n1. a\r\n##\r\nafter\r\n"
	if s, e, _ := RulesSection([]byte(doc)); doc[s:e] != "## Rules\r\n1. a\r\n" {
		t.Errorf("CRLF: %q", doc[s:e])
	}
	if got, err := ReplaceRules([]byte(doc), []byte("## Rules\r\n1. a\r\n")); err != nil || string(got) != doc {
		t.Errorf("CRLF identity: %q %v", got, err)
	}
	if _, err := ReplaceRules([]byte(bounded), []byte("## Rules\n```sh\nx\n```\u00a0\n")); err == nil || !strings.Contains(err.Error(), "unclosed") {
		t.Errorf("NBSP after the closer: %v", err)
	}
}

// A target whose section ends inside a fence never closed runs to the end
// of the file; replacing it would delete everything after (review pass 5,
// F20). Refused as the target's fault, and RulesProblem says so for the
// briefing.
func TestReplaceRulesRefusesATargetFenceLeftOpen(t *testing.T) {
	for _, target := range []string{
		"# Foo\n\n## Rules\n\n1. a\n\n```sh\ndaisy x\n\n## Examples\n\nlots\n\n## Notes\n\nmore\n",
		"# Foo\n\n## Rules\n\n````\nx\n```\n\n## Examples\n\nlots\n",
	} {
		var te TargetError
		_, err := ReplaceRules([]byte(target), []byte("## Rules\n\n1. b\n"))
		if !errors.As(err, &te) || !strings.Contains(err.Error(), "fence opened at line") {
			t.Errorf("%q: %v", target, err)
		}
		if p := RulesProblem([]byte(target)); p == nil || p.Error() != err.Error() {
			t.Errorf("RulesProblem %v, ReplaceRules %v", p, err)
		}
	}
	if p := RulesProblem([]byte(bounded)); p != nil {
		t.Errorf("a sound section: %v", p)
	}
}

// Without a section, an open fence in the target would take the inserted
// rules in: the target's fault, said as such (F21).
func TestInsertRulesIntoAnOpenFenceIsTheTargets(t *testing.T) {
	var te TargetError
	_, err := ReplaceRules([]byte("# Foo\n\n## Trigger\n\nx\n\n```sh\ncode\n"), []byte("## Rules\n1. a\n"))
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "fence opened at line 7") || !strings.Contains(err.Error(), "appended inside it") {
		t.Errorf("%v", err)
	}
}

// Without a section, only an open fence the rules would be appended into
// is refused, and RulesProblem says so for the briefing (review pass 6,
// F23); a heading before the fence takes the rules in safely (F24).
func TestRulesProblemWithoutASection(t *testing.T) {
	appendsIntoFence := "# Foo\n\n## Trigger\n\nx\n\n```sh\ncode\n"
	if p := RulesProblem([]byte(appendsIntoFence)); p == nil || !strings.Contains(p.Error(), "fence opened at line 7") {
		t.Errorf("appending into a fence: %v", p)
	}
	headingFirst := "# Widget\n\n## Trigger\nWhen.\n\n## Steps\nDo it.\n\n```sh\necho hi\n"
	if p := RulesProblem([]byte(headingFirst)); p != nil {
		t.Errorf("a heading before the fence: %v", p)
	}
	got, err := ReplaceRules([]byte(headingFirst), []byte("## Rules\n\n1. Be terse.\n"))
	if err != nil || !strings.HasPrefix(string(got), "# Widget\n\n## Trigger\nWhen.\n\n## Rules\n\n1. Be terse.\n\n## Steps\n") {
		t.Errorf("insert before ## Steps: %q %v", got, err)
	}
}
