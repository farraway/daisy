package skill

import (
	"os"
	"os/exec"
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

// Sections: the `## ` headings other than Rules, in order.
func TestSections(t *testing.T) {
	doc := []byte("## Rules\n## Worked example\n### not\n##nospace\n## Rules for x\n")
	if got, want := Sections(doc), []string{"Worked example", "Rules for x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
}

// git runs git in dir with a fixed author, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=R", "-c", "user.email=r@x"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// RootCopies: a directory in the user's skills named as a skill the root
// has or has had is a copy when every file in it is, byte for byte, a
// version of that file in the root (its working tree or any commit), so a
// copy an older root installed is one, a skill since removed from the root
// too; one holding anything else (an edit, a file of the user's) is kept,
// and a skill of another name, synced, a hidden name or a link is neither.
func TestRootCopies(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	git(t, root, "init", "-q")
	write(t, filepath.Join(root, "skills", "s", "SKILL.md"), "s v1\n")
	write(t, filepath.Join(root, "skills", "s", "ref.md"), "ref\n")
	write(t, filepath.Join(root, "skills", "gone", "SKILL.md"), "gone\n")
	write(t, filepath.Join(root, "skills", "t", "SKILL.md"), "t\n")
	write(t, filepath.Join(root, "skills", "x", "SKILL.md"), "x\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "one")
	write(t, filepath.Join(root, "skills", "s", "SKILL.md"), "s v2\n")
	write(t, filepath.Join(root, "skills", "group", "n", "SKILL.md"), "n\n")
	os.RemoveAll(filepath.Join(root, "skills", "gone"))
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "two")
	write(t, filepath.Join(root, "skills", "w", "SKILL.md"), "w, not committed\n")

	write(t, filepath.Join(dir, "s", "SKILL.md"), "s v1\n") // an older root's
	write(t, filepath.Join(dir, "s", "ref.md"), "ref\n")
	write(t, filepath.Join(dir, "gone", "SKILL.md"), "gone\n") // removed since
	write(t, filepath.Join(dir, "n", "SKILL.md"), "n\n")       // nested in the root
	write(t, filepath.Join(dir, "w", "SKILL.md"), "w, not committed\n")
	write(t, filepath.Join(dir, "t", "SKILL.md"), "t, edited\n")
	write(t, filepath.Join(dir, "x", "SKILL.md"), "x\n")
	write(t, filepath.Join(dir, "x", "notes.md"), "the user's\n")
	write(t, filepath.Join(dir, "mine", "SKILL.md"), "mine\n")
	write(t, filepath.Join(dir, "synced", "SKILL.md"), "x\n")
	write(t, filepath.Join(dir, ".s", "SKILL.md"), "s v2\n")
	write(t, filepath.Join(dir, "ref.md"), "ref\n")
	os.Symlink(filepath.Join(root, "skills", "group", "n"), filepath.Join(dir, "group"))

	copies, kept := RootCopies(&ops.Fake{}, root, dir)
	if want := []string{"gone", "n", "s", "w"}; !reflect.DeepEqual(copies, want) {
		t.Errorf("copies %q, want %q", copies, want)
	}
	if want := []string{"t", "x"}; !reflect.DeepEqual(kept, want) {
		t.Errorf("kept %q, want %q", kept, want)
	}

	// A root that is not a repository has only its working tree.
	os.RemoveAll(filepath.Join(root, ".git"))
	copies, kept = RootCopies(&ops.Fake{}, root, dir)
	if want := []string{"n", "w"}; !reflect.DeepEqual(copies, want) {
		t.Errorf("without history: copies %q, want %q", copies, want)
	}
	if want := []string{"s", "t", "x"}; !reflect.DeepEqual(kept, want) {
		t.Errorf("without history: kept %q, want %q", kept, want)
	}
}
