package prompt

import (
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
)

const (
	overRoot    = "## Trigger\n\nT.\n\n## Rules\n\n1. a\n2. b\n\n## Body\n\nB.\n"
	overLayered = Marker + "\n\n## Rules\n\n1. c\n2. d\n"
)

// The merge rule: the built prompt is the root file with the home's rules
// appended to the root's `## Rules` section, renumbered after them; a root
// without the section gets one where apply --rules would put it. Which
// text a name stands for is the form of its home file.
func TestOverrideText(t *testing.T) {
	for _, tc := range []struct {
		name, root, home, want string
		form                   Form
	}{
		{"root only", overRoot, "", overRoot, RootOnly},
		{"home only", "", "## Trigger\n\nH.\n", "## Trigger\n\nH.\n", HomeOwn},
		{"root + full", overRoot, "## Trigger\n\nMine.\n", "## Trigger\n\nMine.\n", Full},
		{"root + layered", overRoot, overLayered,
			"## Trigger\n\nT.\n\n## Rules\n\n1. a\n2. b\n\n3. c\n4. d\n\n## Body\n\nB.\n", Layered},
		{"layered over no rules, a heading after", "## Trigger\n\nT.\n\n## Body\n\nB.\n", overLayered,
			"## Trigger\n\nT.\n\n## Rules\n\n1. c\n2. d\n\n## Body\n\nB.\n", Layered},
		{"layered over no rules, none after", "## Trigger\n\nT.\n", overLayered,
			"## Trigger\n\nT.\n\n## Rules\n\n1. c\n2. d\n", Layered},
		{"renumbered after the root's, bullets and fences kept", "## Rules\n\n1. a\n2. b",
			Marker + "\n## Rules\n7. x\n- y\n```\n1. not a rule\n```\n9. z\n",
			"## Rules\n\n1. a\n2. b\n\n3. x\n- y\n```\n1. not a rule\n```\n4. z", Layered},
		{"a layered file with no rules is the root", overRoot, Marker + "\n\n## Rules\n", overRoot, Layered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, home := t.TempDir(), t.TempDir()
			if tc.root != "" {
				write(t, filepath.Join(root, "prompts", "x.md"), tc.root)
			}
			if tc.home != "" {
				write(t, filepath.Join(home, "prompts", "x.md"), tc.home)
			}
			s := Locate(ops.Real().FS(), home, root, "prompts/x.md")
			if s.Form != tc.form {
				t.Errorf("form %v, want %v", s.Form, tc.form)
			}
			got, err := s.Text(ops.Real().FS())
			if err != nil || string(got) != tc.want {
				t.Errorf("text %q (%v)\nwant %q", got, err, tc.want)
			}
		})
	}
}

// A layered file is the marker and one `## Rules` section, nothing else:
// anything more is refused with its line, never merged.
func TestMergeRefusesAMalformedLayer(t *testing.T) {
	for _, tc := range []struct{ layered, line string }{
		{"intro\n" + Marker + "\n## Rules\n1. x\n", "line 1"},
		{Marker + "\n\nstray text\n## Rules\n1. x\n", "line 3"},
		{Marker + "\n## Rules\n1. x\n## Another\n", "line 4"},
		{Marker + "\n# Title\n", "line 2"},
		{Marker + "\n## Rules\n```\n1. x\n", "line 3"},
	} {
		_, err := Merge([]byte(overRoot), []byte(tc.layered))
		if err == nil || !strings.Contains(err.Error(), tc.line) {
			t.Errorf("%q: err %v, want one naming %s", tc.layered, err, tc.line)
		}
	}
	if _, err := Merge([]byte("## Rules\n```\n1. a\n"), []byte(overLayered)); err == nil {
		t.Error("a root whose section runs into an open fence merged")
	}
}

// apply --rules on a root target: the proposal is the section the agent
// saw (the merged one); the root's rules must stand first, unchanged, and
// what follows them is the home's layer, renumbered from 1.
func TestLayeredFrom(t *testing.T) {
	root := []byte(overRoot)
	for _, tc := range []struct{ name, root, proposal, want, err string }{
		{"rules after the root's", overRoot, "## Rules\n\n1. a\n2. b\n\n3. c\n", Marker + "\n\n## Rules\n\n1. c\n", ""},
		{"tight after the root's", overRoot, "## Rules\n\n1. a\n2. b\n3. c\n4. d\n", Marker + "\n\n## Rules\n\n1. c\n2. d\n", ""},
		{"a root without rules", "## Trigger\n\nT.\n", "## Rules\n\n1. c\n", Marker + "\n\n## Rules\n\n1. c\n", ""},
		{"nothing added", overRoot, "## Rules\n\n1. a\n2. b\n", Marker + "\n\n## Rules\n", ""},
		{"a root rule changed", overRoot, "## Rules\n\n1. a!\n2. b\n3. c\n", "", "line 3"},
		{"a root rule dropped", overRoot, "## Rules\n\n1. a\n3. c\n", "", "line 4"},
		{"the root's last rule extended", overRoot, "## Rules\n\n1. a\n2. bb\n", "", "line 4"},
		{"the root's last rule continued", overRoot, "## Rules\n\n1. a\n2. b\n   - only on Fridays\n3. c\n", "", "line 5"},
		{"the root's last rule continued after a blank", overRoot, "## Rules\n\n1. a\n2. b\n\n   More on b.\n", "", "line 6"},
		{"the root's last rule continued lazily", overRoot, "## Rules\n\n1. a\n2. b\nonly on Fridays\n3. c\n", "", "line 5"},
		{"a bullet after the root's rules", overRoot, "## Rules\n\n1. a\n2. b\n- c\n", Marker + "\n\n## Rules\n\n- c\n", ""},
		{"a heading after the root's rules", overRoot, "## Rules\n\n1. a\n2. b\n### Home\n\n- c\n", Marker + "\n\n## Rules\n\n### Home\n\n- c\n", ""},
		{"a break after the root's rules", overRoot, "## Rules\n\n1. a\n2. b\n***\n\n- c\n", Marker + "\n\n## Rules\n\n***\n\n- c\n", ""},
		{"inline HTML continuing the root's last rule", overRoot, "## Rules\n\n1. a\n2. b\n<em>never</em> on Fridays\n", "", "line 5"},
		{"a fence after the root's rules", overRoot, "## Rules\n\n1. a\n2. b\n```\nx\n```\n", Marker + "\n\n## Rules\n\n```\nx\n```\n", ""},
		{"not a rules section", overRoot, "# Title\n", "", "must begin with"},
	} {
		got, err := LayeredFrom([]byte(tc.root), []byte(tc.proposal))
		switch {
		case tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err) || !strings.Contains(err.Error(), "--rewrite")):
			t.Errorf("%s: err %v, want one naming %s and --rewrite", tc.name, err, tc.err)
		case tc.err == "" && (err != nil || string(got) != tc.want):
			t.Errorf("%s: %q (%v), want %q", tc.name, got, err, tc.want)
		}
	}
	// Proposing the merged section as it stands is the layer as it stands.
	merged, _ := Merge(root, []byte(overLayered))
	start, end, _ := RulesSection(merged)
	got, err := LayeredFrom(root, merged[start:end])
	if err != nil || string(got) != Marker+"\n\n## Rules\n\n1. c\n2. d\n" {
		t.Errorf("round trip: %q (%v)", got, err)
	}
}

func TestHash(t *testing.T) {
	if got := Hash([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("Hash = %q", got)
	}
}

// A ledger line splits at its last space: a prompt's name may hold one.
func TestLedgerNameWithASpace(t *testing.T) {
	home := t.TempDir()
	data := SetLedger([]byte("prompts/my plan.md old\n"), "prompts/my plan.md", "new")
	if string(data) != "prompts/my plan.md new\n" {
		t.Errorf("SetLedger %q", data)
	}
	write(t, filepath.Join(home, LedgerRel), string(data))
	if l, err := ReadLedger(ops.Real().FS(), home); err != nil || l["prompts/my plan.md"] != "new" || len(l) != 1 {
		t.Errorf("ReadLedger %v %v", l, err)
	}
}
