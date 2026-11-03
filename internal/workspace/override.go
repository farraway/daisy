package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/prompt"
)

// promptsDir is where in .daisy/ a layered prompt is materialised: the
// merged text its stub in AGENTS.md names.
const promptsDir = "prompts"

// merged is an overriding skill as init installs it: the root's skill
// directory, the home's other files over it, its SKILL.md the home's text
// (a layered one merged).
type merged struct {
	rootDir string
	skill   []byte
}

// mergeSkills are the home's skills that override the root's (by their
// path under the home, as prompt.Locate reads it), layered or full: either
// stands on the root's directory, whose other files its SKILL.md may
// name. A layer that does not merge, or has no root skill to layer on,
// refuses init before any write.
func mergeSkills(fs ops.FS, root, home string, skills map[string]string) (map[string]merged, error) {
	out := map[string]merged{}
	for n, dir := range skills {
		rel, err := filepath.Rel(home, filepath.Join(dir, "SKILL.md"))
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		src := prompt.Locate(fs, home, root, rel)
		if !src.Overridden() {
			continue
		}
		text, err := src.Text(fs)
		if err != nil {
			return nil, fmt.Errorf("Error: %v", err)
		}
		out[n] = merged{rootDir: filepath.Dir(src.Root), skill: text}
	}
	return out, nil
}

// copySkill copies the home's skill dir to dst; an overriding one is the
// root's skill directory, the home's other files over it, and m's
// SKILL.md.
func copySkill(fs ops.FS, dir, dst string, m merged) error {
	if m.skill == nil {
		return ops.CopyTree(fs, dir, dst)
	}
	if err := ops.CopyTree(fs, m.rootDir, dst); err != nil {
		return err
	}
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "SKILL.md" {
			continue
		}
		if err := fs.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return err
		}
		if err := ops.CopyTree(fs, filepath.Join(dir, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	// The copy's SKILL.md may be the root's link (CopyTree keeps links):
	// replaced, not written through, or the merge would land in the root.
	if err := removeFile(fs, filepath.Join(dst, "SKILL.md")); err != nil {
		return err
	}
	return fs.WriteFile(filepath.Join(dst, "SKILL.md"), m.skill, 0o644)
}

// writePrompts replaces the stage's prompts/ with the materialised
// prompts, each named by its path under daisyDir.
func writePrompts(fs ops.FS, daisyDir, stage string, prompts map[string][]byte) error {
	if err := fs.RemoveAll(filepath.Join(stage, promptsDir)); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	for path, text := range prompts {
		rel, err := filepath.Rel(daisyDir, path)
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		dst := filepath.Join(stage, rel)
		if err := fs.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := fs.WriteFile(dst, text, 0o644); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	return nil
}
