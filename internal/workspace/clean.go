package workspace

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/identity"
	"daisy/internal/ops"
	"daisy/internal/settings"
	"daisy/internal/skill"
)

// Binding is what clean reads of a workspace before it asks: the home and
// identity it names (either may be unknown) and the claim to unset against.
type Binding struct {
	Workspace, HomeName, Identity string
	claim                         claim
}

// ReadBinding reads the workspace's binding leniently: the home may be
// unregistered and .daisy/home missing; clean still removes what it can.
func ReadBinding(o ops.Ops, m env.Env, ws string) Binding {
	fs := o.FS()
	daisyDir := filepath.Join(ws, ".daisy")
	b := Binding{Workspace: ws}
	if isFile(fs, filepath.Join(daisyDir, homeFile)) {
		if data, err := fs.ReadFile(filepath.Join(daisyDir, homeFile)); err == nil {
			b.HomeName = strings.TrimRight(string(data), "\n")
			b.Identity = env.WorkspaceIdentity(fs, daisyDir, b.HomeName)
		}
	}
	b.claim = readClaim(fs, daisyDir)
	if !b.claim.ok {
		if f, ok := boundIdentityFile(o, m, b.HomeName, b.Identity); ok {
			if id, err := identity.Read(fs, f); err == nil {
				b.claim.id, b.claim.file, b.claim.ok = id, f, true
			}
		}
	}
	return b
}

// Clean takes the binding out of the workspace: the identity init applied
// goes back to what the keys held before, .daisy/ and init's leftovers go,
// and the agent files, skills, ignore blocks and allow rules init added
// come out. Settings are read before anything is removed.
func Clean(o ops.Ops, m env.Env, b Binding, out io.Writer) error {
	fs := o.FS()
	ws := b.Workspace
	at := func(rel ...string) string { return filepath.Join(append([]string{ws}, rel...)...) }

	settingsPath := at(settingsFile)
	var newSettings []byte
	settingsRemoved, settingsEmpty := false, false
	if data, err := fs.ReadFile(settingsPath); err == nil {
		rules := append(allowRules(m.Root), legacyRules(m.Root)...)
		if newSettings, settingsRemoved, settingsEmpty, err = settings.Disallow(data, rules); err != nil {
			return fmt.Errorf("Error: %s cannot be read as settings: %v\n  Fix or remove it, then re-run 'daisy clean'", settingsPath, err)
		}
	}
	// The skills init installed are those whose stub in .cursor/rules/ is
	// in init's form, whatever the home holds now.
	skills := stubs(fs, ws)

	// 1. The identity, while its record is still there; a failed unset
	//    stops here so the record survives for the retry.
	if isDir(fs, at(".daisy")) {
		switch {
		case !identity.IsRepoRoot(o, ws):
			fmt.Fprintln(out, "  · Local git identity not checked: this workspace is not a repository")
		case identity.IsLinkedWorktree(o, ws):
			fmt.Fprintln(out, "  · Local git identity not checked: this is a linked worktree and its config is the main checkout's")
		default:
			if !b.claim.ok {
				fmt.Fprintf(out, "  · No readable .daisy/gitconfig record, and no identity file resolves for home '%s' identity '%s'; only a key of Daisy's form can be recognised as its\n", orQ(b.HomeName), orQ(b.Identity))
			}
			cleared, err := identity.Unset(o, ws, b.claim.id, b.claim.before)
			if err != nil {
				return err
			}
			// A stage an interrupted init left holds the record of what that
			// init applied: reclaim it here, identity and all.
			for _, l := range env.InitLeftovers(fs, ws) {
				if !strings.Contains(filepath.Base(l), ".stage.") {
					continue
				}
				c := readClaim(fs, l)
				more, err := identity.Unset(o, ws, c.id, c.before)
				if err != nil {
					return err
				}
				cleared = append(cleared, more...)
				if fs.RemoveAll(l) != nil {
					return fmt.Errorf("Error: could not remove %s; remove it by hand", l)
				}
			}
			if len(cleared) > 0 {
				fmt.Fprintf(out, "  ✓ Cleared local git identity (%s)\n", strings.Join(cleared, ", "))
			} else {
				what := b.claim.file
				if what == "" {
					what = "what Daisy writes"
				}
				fmt.Fprintf(out, "  · Local git identity left as is (nothing matched %s)\n", what)
			}
		}
	}

	// 2. .daisy/ and what an interrupted swap left beside it.
	if isDir(fs, at(".daisy")) {
		if err := fs.RemoveAll(at(".daisy")); err != nil {
			return fmt.Errorf("Error: could not remove %s: %v", at(".daisy"), err)
		}
		fmt.Fprintln(out, "  ✓ Removed .daisy/")
	}
	for _, l := range env.InitLeftovers(fs, ws) {
		if fs.RemoveAll(l) != nil {
			return fmt.Errorf("Error: could not remove %s; remove it by hand", l)
		}
		fmt.Fprintf(out, "  ✓ Removed %s (left by an interrupted init)\n", l)
	}

	// 3. The agent files and skills.
	removed := false
	for _, r := range cursorRules {
		if isFile(fs, at(".cursor", "rules", r.Name)) && fs.Remove(at(".cursor", "rules", r.Name)) == nil {
			removed = true
		}
	}
	if removed {
		fmt.Fprintln(out, "  ✓ Removed Cursor rules")
	}
	if isFile(fs, at(claudeCommand)) && fs.Remove(at(claudeCommand)) == nil {
		fmt.Fprintln(out, "  ✓ Removed Claude command")
	}
	if isFile(fs, at(skill.AgentsTree, skill.AgentsDaisy, "SKILL.md")) && fs.Remove(at(skill.AgentsTree, skill.AgentsDaisy, "SKILL.md")) == nil {
		fmt.Fprintln(out, "  ✓ Removed Codex skill: daisy")
	}
	if len(skills) > 0 {
		for _, n := range skills {
			for _, tree := range skill.WorkspaceTrees {
				fs.RemoveAll(at(tree, n))
			}
			fs.Remove(at(".cursor", "rules", n+".mdc"))
		}
		fmt.Fprintf(out, "  ✓ Removed %d skill(s) from .claude/skills/, .agents/skills/ and .cursor/rules/\n", len(skills))
	}

	// 4. What init added to the ignore files and the settings.
	if err := cleanBlock(fs, at(".gitignore"), gitignoreHeader, isGitignoreEntry, out); err != nil {
		return err
	}
	if settingsRemoved {
		if settingsEmpty {
			if err := fs.Remove(settingsPath); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			fmt.Fprintln(out, "  ✓ Removed .claude/settings.local.json")
		} else {
			if err := ops.Rewrite(fs, settingsPath, newSettings, 0o644); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			fmt.Fprintln(out, "  ✓ Removed Daisy permissions from .claude/settings.local.json")
		}
	}
	if err := cleanBlock(fs, at(".cursorignore"), cursorignoreHeader, func(l string) bool { return hasLine(cursorignoreLines, l) }, out); err != nil {
		return err
	}
	removeEmptyDirs(fs, at(".cursor", "rules"), at(".cursor"), at(".claude", "commands"), at(".claude", "skills"), at(".claude"), at(skill.AgentsTree, skill.AgentsDaisy), at(skill.AgentsTree), at(".agents"))

	fmt.Fprint(out, "\nDone. Daisy has been removed from this workspace.\n")
	return nil
}

func cleanBlock(fs ops.FS, path, header string, isEntry func(string) bool, out io.Writer) error {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil
	}
	name := filepath.Base(path)
	cleaned, changed := removeBlock(data, header, isEntry)
	if !changed {
		return nil
	}
	if len(cleaned) == 0 {
		if err := fs.Remove(path); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintf(out, "  ✓ Cleaned %s\n  ✓ Removed empty %s\n", name, name)
		return nil
	}
	if err := ops.Rewrite(fs, path, cleaned, 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	fmt.Fprintf(out, "  ✓ Cleaned %s\n", name)
	return nil
}

func orQ(s string) string {
	if s == "" {
		return "?"
	}
	return s
}
