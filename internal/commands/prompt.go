package commands

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/prompt"
	"daisy/internal/skill"
)

// PromptBuild writes AGENTS.md: with a home's name, that home's
// <home>/AGENTS.md (no workspace needed); without, the workspace's
// .daisy/AGENTS.md for its bound home. --output, which `daisy init` passes
// until it is Go, writes another file instead, named as given. The header's
// rebuild line is the command that rewrites the file. Nothing is
// committed: the home ignores its AGENTS.md, the workspace's is per-machine.
func PromptBuild(c *Context) error {
	var e env.Env
	var err error
	var output, rebuild string
	if len(c.Args) == 1 {
		e, err = env.ForHome(c.Ops, c.Args[0])
		output, rebuild = filepath.Join(e.Home, "AGENTS.md"), "daisy prompt build "+c.Args[0]
	} else {
		e, err = env.Resolve(c.Ops)
		output, rebuild = filepath.Join(e.Workspace, ".daisy", "AGENTS.md"), "daisy prompt build"
	}
	if err != nil {
		return err
	}
	path := output
	if v, ok := c.Value("--output"); ok {
		output, path, rebuild = v, v, "daisy prompt build"
		if !filepath.IsAbs(v) {
			cwd, err := c.Ops.Getwd()
			if err != nil {
				return fmt.Errorf("Error: cannot read the working directory: %v", err)
			}
			path = filepath.Join(cwd, v)
		}
	}
	b, err := prompt.Build(c.Ops.FS(), prompt.Spec{
		Root: e.Root, HomeName: e.HomeName, Home: e.Home,
		Output: output, Rebuild: rebuild, Built: c.Ops.Now(),
	})
	if err != nil {
		return err
	}
	for _, w := range b.Warnings {
		fmt.Fprintln(c.Stderr, w)
	}
	if err := c.Ops.FS().WriteFile(path, b.Text, 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	fmt.Fprintf(c.Stdout, "✅ Built AGENTS.md for home: %s\n", e.HomeName)
	if len(b.Full) > 0 {
		fmt.Fprintf(c.Stdout, "   Full: %s\n", strings.Join(b.Full, " "))
	}
	if len(b.Lazy) > 0 {
		fmt.Fprintf(c.Stdout, "   Lazy: %s\n", strings.Join(b.Lazy, " "))
	}
	fmt.Fprintf(c.Stdout, "   Output: %s\n   Size: %d lines\n", output, bytes.Count(b.Text, []byte("\n")))
	return nil
}

// List prints the home's prompts, each with its trigger's first line or
// how it loads, and the installed skills, each with its short description.
// Read-only.
var List = readOnly(list)

// ListHealth: the workspace and its home resolve, which the cli has checked.
var ListHealth = readOnly(func(*Context) error { return nil })

func list(c *Context) error {
	fs := c.Ops.FS()
	fmt.Fprintf(c.Stdout, "Prompts (%s):\n", c.Env.HomeName)
	include := filepath.Join(c.Env.Home, "include.txt")
	if fi, err := fs.Stat(include); err != nil || !fi.Mode().IsRegular() {
		fmt.Fprintln(c.Stdout, "  (no include.txt found)")
	} else {
		data, err := fs.ReadFile(include)
		if err != nil {
			return fmt.Errorf("Error: cannot read %s: %v", include, err)
		}
		entries, err := prompt.ParseInclude(data)
		if err != nil {
			return fmt.Errorf("Error: %s: %v", include, err)
		}
		for _, en := range entries {
			path, ok := prompt.Lookup(fs, c.Env.Home, c.Env.Root, en.Name)
			switch {
			case !ok:
				fmt.Fprintf(c.Stdout, "  %s (missing: %s)\n", pad(en.Name, 20), path)
			case en.Lazy:
				body, err := fs.ReadFile(path)
				if err != nil {
					return fmt.Errorf("Error: cannot read %s: %v", path, err)
				}
				fmt.Fprintf(c.Stdout, "  %s %s\n", pad(en.Name, 20), triggerSummary(body))
			default:
				fmt.Fprintf(c.Stdout, "  %s (always loaded)\n", pad(en.Name, 20))
			}
		}
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Skills:")
	skills := skill.All(fs, skill.InstalledDirs(c.Ops.Getenv("HOME"), c.Env.Workspace))
	if len(skills) == 0 {
		fmt.Fprintf(c.Stdout, "  (none installed — run: daisy install, then daisy init %s)\n", c.Env.HomeName)
	}
	for _, s := range skills {
		doc, err := fs.ReadFile(s.Path)
		if err != nil {
			return fmt.Errorf("Error: cannot read %s: %v", s.Path, err)
		}
		text, _ := skill.Field(doc, "description")
		if !c.Flag("--full") {
			if short, _ := skill.Field(doc, "short_description"); short != "" {
				text = short
			}
		}
		text = strings.TrimPrefix(strings.TrimSuffix(text, `"`), `"`)
		fmt.Fprintf(c.Stdout, "  %s %s\n", pad(s.Name, 20), text)
	}
	return nil
}

// triggerSummary is the first line of the trigger that says something: not
// the `Read the full …` pointer, a list marker dropped, not empty; cut to
// 97 characters and `...` past 100.
func triggerSummary(body []byte) string {
	for _, line := range prompt.Trigger(body) {
		if strings.HasPrefix(line, "Read the full") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		if line == "" {
			continue
		}
		if r := []rune(line); len(r) > 100 {
			line = string(r[:97]) + "..."
		}
		return line
	}
	return "(lazy, no trigger summary)"
}

// pad left-justifies s in width bytes, as the bash's printf %-Ns did.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
