package declare

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"daisy/internal/ops"
)

// R3-13's claim, its validation (the plan implementing the 2026-11-19
// swarm roadmap; section 3, prompt_parts): each bad entry is refused,
// naming its field, byte for byte: a path outside the clone (absolute,
// or through ..), a file missing or empty, an unknown role, an empty
// roles; a valid list passes, and the file is not read when the prompts
// are skipped (init's first validation).
func TestValidatePromptParts(t *testing.T) {
	_, clone := cloneFolder(t)
	p := Clone(clone)
	write(t, filepath.Join(clone, "prompts", "parts", "css.md"), "# CSS rules\n\nUse the grid.\n")
	write(t, filepath.Join(clone, "prompts", "parts", "empty.md"), "")
	ok := PromptPart{Path: "prompts/parts/css.md", Roles: []string{"worker", "adversary"}}
	for _, tc := range []struct {
		want string
		part PromptPart
	}{
		{`swarm.json: prompt_parts[1].path: "/etc/passwd" is outside the clone`, PromptPart{Path: "/etc/passwd", Roles: []string{"worker"}}},
		{`swarm.json: prompt_parts[1].path: "prompts/../../secret.md" is outside the clone`, PromptPart{Path: "prompts/../../secret.md", Roles: []string{"worker"}}},
		{`swarm.json: prompt_parts[1].path: "" is outside the clone`, PromptPart{Path: "", Roles: []string{"worker"}}},
		{`swarm.json: prompt_parts[1].path: prompts/parts/gone.md is missing or empty`, PromptPart{Path: "prompts/parts/gone.md", Roles: []string{"worker"}}},
		{`swarm.json: prompt_parts[1].path: prompts/parts/empty.md is missing or empty`, PromptPart{Path: "prompts/parts/empty.md", Roles: []string{"worker"}}},
		{`swarm.json: prompt_parts[1].roles[1]: "tester" is not a role (owner, architect, worker, adversary, sympathetic, qa)`, PromptPart{Path: "prompts/parts/css.md", Roles: []string{"worker", "tester"}}},
		{`swarm.json: prompt_parts[1].roles: no role (at least one is required)`, PromptPart{Path: "prompts/parts/css.md", Roles: []string{}}},
	} {
		c := valid()
		c.PromptParts = []PromptPart{ok, tc.part}
		if got := errText(Validate(ops.Real(), p, c, 0)); got != tc.want {
			t.Errorf("got  %s\nwant %s", got, tc.want)
		}
	}
	c := valid()
	c.PromptParts = []PromptPart{ok, {Path: "prompts/parts/css.md", Roles: []string{"owner"}}}
	if err := Validate(ops.Real(), p, c, 0); err != nil {
		t.Errorf("a valid list: %v", err)
	}
	c.PromptParts = []PromptPart{{Path: "prompts/parts/gone.md", Roles: []string{"worker"}}}
	if err := Validate(ops.Real(), p, c, SkipPrompts|SkipAccount); err != nil {
		t.Errorf("init's first validation reads no part: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clone, "prompts", "parts", "gone.md")); err == nil {
		t.Fatal("gone.md exists")
	}
}

// prompt_parts is written last, an entry a line, only when it has one,
// and read back (section 3).
func TestFormatPromptParts(t *testing.T) {
	c := valid()
	c.PromptParts = []PromptPart{{Path: "prompts/parts/css.md", Roles: []string{"worker", "adversary"}}, {Path: "prompts/parts/tokens.md", Roles: []string{"sympathetic"}}}
	out := string(Format(c))
	if want := "  },\n  \"prompt_parts\": [\n    {\"path\": \"prompts/parts/css.md\", \"roles\": [\"worker\", \"adversary\"]},\n    {\"path\": \"prompts/parts/tokens.md\", \"roles\": [\"sympathetic\"]}\n  ]\n}\n"; !strings.HasSuffix(out, want) {
		t.Errorf("Format:\n%s\nwant ending:\n%s", out, want)
	}
	if back, err := Parse([]byte(out)); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip: %v\n%+v", err, back)
	}
	c.PromptParts = nil
	if strings.Contains(string(Format(c)), "prompt_parts") {
		t.Errorf("prompt_parts written while empty:\n%s", Format(c))
	}
}
