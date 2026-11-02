package commands

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/settings"
	"daisy/internal/skill"
)

// Install links ~/bin/daisy to this root's binary, copies the root's skills
// into ~/.claude/skills (pruning copies whose source is gone), grants
// Daisy's permissions in ~/.claude/settings.json and, unless --update,
// writes the DAISY_ROOT line to the shell's rc file. It does not build:
// `make build` writes the binary. The root is the one this binary was
// built in (bin/daisy under it), so a shell with nothing set can bootstrap;
// a $DAISY_ROOT naming another root is refused before any write.
func Install(c *Context) error {
	fs := c.Ops.FS()
	update := c.Flag("--update")
	root, err := installRoot(c.Ops)
	if err != nil {
		return err
	}
	home := c.Ops.Getenv("HOME")
	if home == "" {
		return errors.New("Error: HOME is not set; install writes under it")
	}
	link, target := filepath.Join(home, "bin", "daisy"), filepath.Join(root, "bin", "daisy")
	existing, linkErr := fs.Readlink(link)
	if exists(fs, link) && linkErr != nil {
		return errors.New("  ⚠ ~/bin/daisy exists but is not a symlink. Remove it and re-run.")
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	current, err := fs.ReadFile(settingsPath)
	if err != nil && exists(fs, settingsPath) {
		return fmt.Errorf("Error: cannot read %s: %v", settingsPath, err)
	}
	// The removed rules are legacy: older installs granted the deleted
	// bash scripts and wrote the root rules in two other spellings.
	newSettings, err := settings.Allow(current,
		[]string{"Read(/" + root + "/**)", "Edit(/" + root + "/**)", "Bash(daisy:*)"},
		[]string{"Bash(" + root + "/daisy/scripts/*)", "Read(" + root + "/**)", "Edit(" + root + "/**)", "Read(//" + root + "/**)", "Edit(//" + root + "/**)"})
	if err != nil {
		return fmt.Errorf("Error: %s cannot be read as settings: %v\n  Fix or remove it, then re-run 'daisy install'", settingsPath, err)
	}
	rc, rcContent, rcMessage, err := planRC(c.Ops, home, root)
	if err != nil && !update {
		return err
	}
	// Installed is the link and the rc line both naming this root: a moved
	// root installed only with --update still needs its rc line written.
	if !update && linkErr == nil && existing == target && rcContent == nil {
		return errors.New("Error: Daisy is already installed.\nRun 'daisy install --update' to re-apply symlink/skills/permissions.")
	}

	if update {
		fmt.Fprintln(c.Stdout, "Updating Daisy...")
	} else {
		fmt.Fprintln(c.Stdout, "Installing Daisy...")
	}
	fmt.Fprintf(c.Stdout, "  DAISY_ROOT: %s\n\n", root)

	if !exists(fs, filepath.Join(home, "bin")) {
		if err := fs.MkdirAll(filepath.Join(home, "bin"), 0o755); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintln(c.Stdout, "  ✓ Created ~/bin")
	}
	if fi, err := fs.Stat(target); err != nil || fi.Mode().Perm()&0o111 == 0 {
		fmt.Fprintf(c.Stderr, "  ⚠ %s is not built yet; run 'make build' in %s\n", target, root)
	}
	switch {
	case linkErr == nil && existing == target:
		fmt.Fprintln(c.Stdout, "  ✓ ~/bin/daisy symlink already correct")
	case linkErr == nil:
		if err := fs.Remove(link); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := fs.Symlink(target, link); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintf(c.Stdout, "  ✓ Updated ~/bin/daisy symlink (was: %s)\n", existing)
	default:
		if err := fs.Symlink(target, link); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintf(c.Stdout, "  ✓ Created ~/bin/daisy → %s\n", target)
	}

	if err := installSkills(c, root, filepath.Join(home, ".claude", "skills")); err != nil {
		return err
	}

	if !update {
		fmt.Fprintf(c.Stdout, "\n  Shell config: %s\n", rc)
		if rcContent != nil {
			if err := fs.MkdirAll(filepath.Dir(rc), 0o755); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			if err := ops.Rewrite(fs, rc, rcContent, 0o644); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
		}
		fmt.Fprintln(c.Stdout, rcMessage)
	}

	if err := fs.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	if err := ops.Rewrite(fs, settingsPath, newSettings, 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	fmt.Fprintln(c.Stdout, "  ✓ Configured ~/.claude/settings.json with Daisy permissions")

	if update {
		fmt.Fprint(c.Stdout, "\nDone.\n")
		return nil
	}
	fmt.Fprintf(c.Stdout, "\nDone. To activate, run:\n  source %s\n\n", rc)
	m := env.Env{Config: env.ConfigDir(c.Ops)}
	if homes := env.Homes(fs, m.HomesFile()); len(homes) == 0 {
		fmt.Fprint(c.Stdout, "Then create a home and initialize Daisy in a workspace:\n  daisy home new <name>\n  cd /path/to/project\n  daisy init <name>\n")
	} else {
		fmt.Fprintf(c.Stdout, "Then initialize Daisy in a workspace:\n  cd /path/to/project\n  daisy init %s\n", homes[0].Name)
	}
	return nil
}

// installRoot is the root holding this binary, <exe>/../.. resolved through
// links. $DAISY_ROOT, when set, must name that root: a stale export would
// otherwise send the install, and ~/bin/daisy, to a root other than the
// binary that was run. The fix printed is plain install, which also writes
// the rc line the stale export came from.
func installRoot(o ops.Ops) (string, error) {
	set := o.Getenv("DAISY_ROOT")
	exe, err := o.Executable()
	if err != nil {
		if set != "" {
			return "", fmt.Errorf("Error: DAISY_ROOT is %s and the binary's own path is unknown (%v), so install cannot check that it is this binary's root", set, err)
		}
		return "", fmt.Errorf("Error: DAISY_ROOT is not set and the binary's own path is unknown: %v", err)
	}
	root := filepath.Dir(filepath.Dir(env.RealPath(o.FS(), exe)))
	if set != "" {
		if env.RealPath(o.FS(), set) != root {
			return "", fmt.Errorf("Error: DAISY_ROOT is %s, but this binary (%s) is in %s\n  Install the binary's root:  DAISY_ROOT=%s %s/bin/daisy install  (or unset DAISY_ROOT; install writes the rc line)\n  Install DAISY_ROOT's root:  %s/bin/daisy install\n  Refused again in a new shell: another file exports it (grep -n DAISY_ROOT ~/.zshenv ~/.zshrc ~/.zprofile ~/.bashrc ~/.bash_profile ~/.profile ~/.config/fish/config.fish)", set, exe, root, root, root, set)
		}
		return env.Root(o)
	}
	if fi, err := o.FS().Stat(filepath.Join(root, "templates")); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("Error: DAISY_ROOT is not set and %s is not in a Daisy root's bin/\n  Run it as <root>/bin/daisy install", exe)
	}
	return root, nil
}

// installSkills copies every root skill and prunes installed copies whose
// source is gone, sparing Claude Code's own directory.
func installSkills(c *Context, root, dir string) error {
	fs := c.Ops.FS()
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	sources := skill.Discover(fs, filepath.Join(root, "skills"))
	for n, src := range sources {
		if err := fs.RemoveAll(filepath.Join(dir, n)); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		if err := ops.CopyTree(fs, src, filepath.Join(dir, n)); err != nil {
			return fmt.Errorf("Error: copying the skill %s: %v", n, err)
		}
	}
	fmt.Fprintf(c.Stdout, "  ✓ Installed %d skill(s) into %s\n", len(sources), dir)
	pruned := 0
	entries, _ := fs.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || sources[e.Name()] != "" || e.Name() == skill.ClaudeCodeSynced {
			continue
		}
		if fi, err := fs.Stat(filepath.Join(dir, e.Name())); err == nil && fi.IsDir() {
			if err := fs.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			pruned++
		}
	}
	if pruned > 0 {
		fmt.Fprintf(c.Stdout, "  ✓ Pruned %d stale skill(s) from %s\n", pruned, dir)
	}
	return nil
}

// planRC is the shell's rc file and, when it must change, its new content
// and what to say: the DAISY_ROOT lines updated in place or a block
// appended. One pattern finds the lines both to check and to rewrite (an
// export at the start of a line, not a comment), so
// what is called stale is rewritten and a second run finds it right; a
// later stale line is the one the shell keeps, so every line must be right.
func planRC(o ops.Ops, home, root string) (rc string, content []byte, message string, err error) {
	shell := filepath.Base(o.Getenv("SHELL"))
	line, pattern := "export DAISY_ROOT=\""+root+"\"", regexp.MustCompile(`^(\s*)export DAISY_ROOT=(.*)`)
	switch shell {
	case "zsh":
		rc = filepath.Join(home, ".zshenv")
	case "bash":
		rc = filepath.Join(home, ".bashrc")
	case "fish":
		rc = filepath.Join(home, ".config", "fish", "config.fish")
		line, pattern = "set -gx DAISY_ROOT \""+root+"\"", regexp.MustCompile(`^(\s*)set -gx DAISY_ROOT (.*)`)
	default:
		rc = filepath.Join(home, ".profile")
	}
	data, readErr := o.FS().ReadFile(rc)
	if readErr != nil && exists(o.FS(), rc) {
		return rc, nil, "", fmt.Errorf("Error: cannot read %s: %v", rc, readErr)
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	found, stale, current := false, false, ""
	for i, l := range lines {
		loc := pattern.FindStringSubmatchIndex(l)
		if loc == nil {
			continue
		}
		found = true
		value := strings.TrimSpace(l[loc[4]:loc[5]])
		if q := regexp.MustCompile(`"[^"]*"`).FindString(value); q != "" {
			value = strings.Trim(q, `"`)
		}
		if value != root && !stale {
			stale, current = true, value
		}
		lines[i] = l[loc[2]:loc[3]] + line
	}
	if !found {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return rc, []byte(text + "\n# daisy\n" + line + "\n"), "  ✓ Added DAISY_ROOT to " + rc, nil
	}
	if !stale {
		return rc, nil, "  ✓ DAISY_ROOT already set correctly", nil
	}
	return rc, []byte(strings.Join(lines, "\n")), "  ✓ Updated DAISY_ROOT (was: " + current + ")", nil
}
