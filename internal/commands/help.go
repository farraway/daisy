package commands

import (
	"errors"
	"fmt"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/prompt"
	"daisy/internal/skill"
)

// SkillHelp prints what `daisy list` leaves out of a skill: its full
// description, its Rules section and the titles of its other sections, so
// an agent can decide whether it applies without opening SKILL.md. The
// installed copy, which is what an agent loads, is preferred over the
// source. Read-only.
var SkillHelp = readOnly(skillHelp)

func skillHelp(c *Context) error {
	name := c.Args[0]
	path, ok := findSkill(c.Ops, c.Env, name)
	if !ok {
		return errors.New("Unknown command or skill: " + name + "\n  Run 'daisy help' for commands, 'daisy list' for skills.")
	}
	doc, err := c.Ops.FS().ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	fmt.Fprintf(c.Stdout, "%s  (%s)\n\n", name, path)
	if d, ok := skill.Field(doc, "description"); ok {
		fmt.Fprintln(c.Stdout, d)
	}
	fmt.Fprintln(c.Stdout)
	for _, line := range prompt.Rules(doc) {
		fmt.Fprintln(c.Stdout, line)
	}
	if s := skill.Sections(doc); len(s) > 0 {
		fmt.Fprintf(c.Stdout, "Other sections: %s\n", strings.Join(s, ", "))
	}
	return nil
}

// ShadowNote says so when a skill shares a command's name, which `daisy
// help <name>` answers with the command: the skill is unreachable that way
// and should not look absent. Outside a workspace there is no lookup and no
// note.
func ShadowNote(c *Context, name string) {
	o := ops.ReadOnly(c.Ops)
	e, err := env.Resolve(o)
	if err != nil {
		return
	}
	if _, ok := findSkill(o, e, name); ok {
		fmt.Fprintf(c.Stdout, "\nNote: a skill named '%s' also exists; the command name takes precedence.\n  Read it with: daisy list, or open its SKILL.md directly.\n", name)
	}
}

// findSkill: an installed copy (the workspace's over the user's), else a
// source (the home's over the root's).
func findSkill(o ops.Ops, e env.Env, name string) (string, bool) {
	fs := o.FS()
	if p, ok := skill.Find(fs, skill.InstalledDirs(o.Getenv("HOME"), e.Workspace), name); ok {
		return p, true
	}
	return skill.Find(fs, skill.SourceDirs(e.Root, e.Home), name)
}
