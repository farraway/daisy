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

// Install links ~/bin/daisy to this root's binary and, unless --update,
// writes the DAISY_ROOT line to the shell's rc file. It writes nothing
// under ~/.claude: skills and permissions are init's, per workspace. It
// takes back what earlier installs put there: the copies of the root's
// skills (skill.RootCopies; a user's own skill, an edited copy and synced
// stay) and Daisy's allow rules in ~/.claude/settings.json. It does not
// build: `make build` writes the binary. The root is the one this binary
// was built in (bin/daisy under it), so a shell with nothing set can
// bootstrap; a $DAISY_ROOT naming another root is refused before any write.
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
	var newSettings []byte
	settingsRemoved, settingsEmpty := false, false
	if current, err := fs.ReadFile(settingsPath); err != nil && exists(fs, settingsPath) {
		return fmt.Errorf("Error: cannot read %s: %v", settingsPath, err)
	} else if err == nil {
		// Every form an install wrote: the current one, the deleted bash
		// scripts' grant, and the root rules' two other spellings.
		own := []string{"Read(/" + root + "/**)", "Edit(/" + root + "/**)", "Bash(daisy:*)", "Bash(" + root + "/daisy/scripts/*)", "Read(" + root + "/**)", "Edit(" + root + "/**)", "Read(//" + root + "/**)", "Edit(//" + root + "/**)"}
		if newSettings, settingsRemoved, settingsEmpty, err = settings.Disallow(current, own); err != nil {
			return fmt.Errorf("Error: %s cannot be read as settings: %v\n  Fix or remove it, then re-run 'daisy install'", settingsPath, err)
		}
	}
	rc, rcContent, rcMessage, err := planRC(c.Ops, home, root)
	if err != nil && !update {
		return err
	}
	// Installed is the link and the rc line both naming this root: a moved
	// root installed only with --update still needs its rc line written.
	if !update && linkErr == nil && existing == target && rcContent == nil {
		return errors.New("Error: Daisy is already installed.\nRun 'daisy install --update' to re-apply the symlink and take back what earlier installs put under ~/.claude.")
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

	if err := removeSkillCopies(c, root, filepath.Join(home, ".claude", "skills")); err != nil {
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

	switch {
	case settingsRemoved && settingsEmpty:
		if err := fs.Remove(settingsPath); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintln(c.Stdout, "  ✓ Removed ~/.claude/settings.json (it held only Daisy's permissions, which each workspace's 'daisy init' grants now)")
	case settingsRemoved:
		if err := ops.Rewrite(fs, settingsPath, newSettings, 0o644); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintln(c.Stdout, "  ✓ Removed Daisy's permissions from ~/.claude/settings.json (each workspace's 'daisy init' grants them now)")
	}

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

// removeSkillCopies removes the copies of the root's skills earlier
// installs left in dir and says which directories named as a root skill it
// keeps, and why.
func removeSkillCopies(c *Context, root, dir string) error {
	copies, kept := skill.RootCopies(c.Ops, root, dir)
	for _, n := range copies {
		if err := c.Ops.FS().RemoveAll(filepath.Join(dir, n)); err != nil {
			return fmt.Errorf("Error: %v", err)
		}
	}
	if len(copies) > 0 {
		fmt.Fprintf(c.Stdout, "  ✓ Removed %s from ~/.claude/skills (%s); Daisy installs skills in each workspace now: run 'daisy init' there\n", plural(len(copies), "copy of a root skill", "copies of root skills"), strings.Join(copies, ", "))
	}
	for _, n := range kept {
		fmt.Fprintf(c.Stdout, "  · Kept ~/.claude/skills/%s: named as a root skill but holding a file no version of it has (an edit, or a file of your own); remove it by hand if it is Daisy's\n", n)
	}
	return nil
}

// quoted is a value's first quoted word, double or single quotes.
var quoted = regexp.MustCompile(`"[^"]*"|'[^']*'`)

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
		if q := quoted.FindString(value); q != "" {
			value = q[1 : len(q)-1]
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
