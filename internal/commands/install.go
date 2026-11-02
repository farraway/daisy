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
// `make build` writes the binary. The root is $DAISY_ROOT, else the root
// this binary was built in (bin/daisy under it), so a shell with nothing
// set can bootstrap.
func Install(c *Context) error {
	fs := c.Ops.FS()
	root, err := installRoot(c.Ops)
	if err != nil {
		return err
	}
	update := c.Flag("--update")
	home := c.Ops.Getenv("HOME")
	if home == "" {
		return errors.New("Error: HOME is not set; install writes under it")
	}
	link, target := filepath.Join(home, "bin", "daisy"), filepath.Join(root, "bin", "daisy")
	existing, linkErr := fs.Readlink(link)
	if !update && linkErr == nil && existing == target {
		return errors.New("Error: Daisy is already installed.\nRun 'daisy install --update' to re-apply symlink/skills/permissions.")
	}
	if exists(fs, link) && linkErr != nil {
		return errors.New("  ⚠ ~/bin/daisy exists but is not a symlink. Remove it and re-run.")
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	current, err := fs.ReadFile(settingsPath)
	if err != nil && exists(fs, settingsPath) {
		return fmt.Errorf("Error: cannot read %s: %v", settingsPath, err)
	}
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

// installRoot is $DAISY_ROOT, else the root holding this binary.
func installRoot(o ops.Ops) (string, error) {
	if o.Getenv("DAISY_ROOT") != "" {
		return env.Root(o)
	}
	exe, err := o.Executable()
	if err != nil {
		return "", fmt.Errorf("Error: DAISY_ROOT is not set and the binary's own path is unknown: %v", err)
	}
	root := filepath.Dir(filepath.Dir(env.RealPath(o.FS(), exe)))
	if fi, err := o.FS().Stat(filepath.Join(root, "templates")); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("Error: DAISY_ROOT is not set and %s is not in a Daisy root's bin/\n  Run it as <root>/bin/daisy install, or set DAISY_ROOT", exe)
	}
	return root, nil
}

// installSkills copies every root skill and prunes installed copies whose
// source is gone.
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
		if strings.HasPrefix(e.Name(), ".") || sources[e.Name()] != "" {
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
// and what to say: the DAISY_ROOT line updated in place or a block appended.
func planRC(o ops.Ops, home, root string) (rc string, content []byte, message string, err error) {
	shell := filepath.Base(o.Getenv("SHELL"))
	line, pattern := "export DAISY_ROOT=\""+root+"\"", regexp.MustCompile(`export DAISY_ROOT=.*`)
	switch shell {
	case "zsh":
		rc = filepath.Join(home, ".zshenv")
	case "bash":
		rc = filepath.Join(home, ".bashrc")
	case "fish":
		rc = filepath.Join(home, ".config", "fish", "config.fish")
		line, pattern = "set -gx DAISY_ROOT \""+root+"\"", regexp.MustCompile(`set -gx DAISY_ROOT .*`)
	default:
		rc = filepath.Join(home, ".profile")
	}
	data, readErr := o.FS().ReadFile(rc)
	if readErr != nil && exists(o.FS(), rc) {
		return rc, nil, "", fmt.Errorf("Error: cannot read %s: %v", rc, readErr)
	}
	text := string(data)
	if !strings.Contains(text, "DAISY_ROOT") {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return rc, []byte(text + "\n# daisy\n" + line + "\n"), "  ✓ Added DAISY_ROOT to " + rc, nil
	}
	current := ""
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, "DAISY_ROOT") && (shell == "fish" || strings.Contains(l, "DAISY_ROOT=")) {
			if q := regexp.MustCompile(`"[^"]*"`).FindString(l); q != "" {
				current = strings.Trim(q, `"`)
			}
			break
		}
	}
	if current == root {
		return rc, nil, "  ✓ DAISY_ROOT already set correctly", nil
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if loc := pattern.FindStringIndex(l); loc != nil {
			lines[i] = l[:loc[0]] + line
		}
	}
	return rc, []byte(strings.Join(lines, "\n")), "  ✓ Updated DAISY_ROOT (was: " + current + ")", nil
}
