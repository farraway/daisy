package skill

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"daisy/internal/ops"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Dirs are given lowest precedence first; the last that holds the skill
// wins, so a home skill shadows a root one and a workspace copy a user one.
func TestFindLastDirWins(t *testing.T) {
	low, high := t.TempDir(), t.TempDir()
	write(t, filepath.Join(low, "a", "SKILL.md"), "")
	write(t, filepath.Join(high, "a", "SKILL.md"), "")
	write(t, filepath.Join(low, "b", "SKILL.md"), "")
	os.MkdirAll(filepath.Join(high, "b", "SKILL.md"), 0o755) // not a file
	fs := ops.Real().FS()
	for name, want := range map[string]string{"a": high, "b": low} {
		if got, ok := Find(fs, []string{low, high, filepath.Join(low, "absent")}, name); !ok || got != filepath.Join(want, name, "SKILL.md") {
			t.Errorf("%s: %q %v", name, got, ok)
		}
	}
	if _, ok := Find(fs, []string{low, high}, "c"); ok {
		t.Error("c found")
	}
}

// All lists every skill directory in the dirs, later dirs overriding by
// name, sorted; hidden directories and directories without SKILL.md are not
// skills.
func TestAll(t *testing.T) {
	low, high := t.TempDir(), t.TempDir()
	write(t, filepath.Join(low, "zeta", "SKILL.md"), "")
	write(t, filepath.Join(low, "alpha", "SKILL.md"), "")
	write(t, filepath.Join(high, "alpha", "SKILL.md"), "")
	write(t, filepath.Join(low, ".hidden", "SKILL.md"), "")
	write(t, filepath.Join(low, "empty", "README.md"), "")
	write(t, filepath.Join(low, "file"), "")
	got := All(ops.Real().FS(), []string{low, high})
	want := []Skill{
		{Name: "alpha", Path: filepath.Join(high, "alpha", "SKILL.md")},
		{Name: "zeta", Path: filepath.Join(low, "zeta", "SKILL.md")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

// Frontmatter: the first line starting `<key>:`, the value after it less
// one leading space (none without the space); a value holding `: ` is kept
// whole.
func TestField(t *testing.T) {
	doc := []byte("---\nname: x\ndescription: Use when: things happen.\nshort_description:\nother:no space\n---\ndescription: body line\n")
	for key, want := range map[string]string{
		"description":       "Use when: things happen.",
		"short_description": "",
		"other":             "",
		"name":              "x",
	} {
		if got, ok := Field(doc, key); !ok || got != want {
			t.Errorf("%s: %q %v", key, got, ok)
		}
	}
	if _, ok := Field(doc, "absent"); ok {
		t.Error("absent found")
	}
}

// The Rules section runs from its heading to the next `## ` heading; `# `
// and `### ` lines are inside it, as the bash's help printed it.
func TestRules(t *testing.T) {
	doc := []byte("# T\n## Rules\n\n1. a\n### sub\n# top\n## Next\nx\n")
	if got, want := Rules(doc), []string{"## Rules", "", "1. a", "### sub", "# top"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	if got := Rules([]byte("# T\n## Other\n")); got != nil {
		t.Errorf("got %q", got)
	}
}

// Sections: the `## ` headings other than Rules, in order.
func TestSections(t *testing.T) {
	doc := []byte("## Rules\n## Worked example\n### not\n##nospace\n## Rules for x\n")
	if got, want := Sections(doc), []string{"Worked example", "Rules for x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
}
