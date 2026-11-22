package commands

import (
	"bytes"
	"fmt"
	iofs "io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/prompt"
	"daisy/internal/skill"
	"daisy/internal/verb"
)

// PromptBuild writes AGENTS.md: with a home's name, that home's
// <home>/AGENTS.md (no workspace needed); without, the workspace's
// .daisy/AGENTS.md for its bound home. The header's rebuild line is the
// command that rewrites the file. Nothing is
// committed: the home ignores its AGENTS.md, the workspace's is per-machine.
func PromptBuild(c *Context) error {
	var e env.Env
	var err error
	var output, rebuild, prompts string
	if len(c.Args) == 1 {
		e, err = env.ForHome(c.Ops, c.Args[0])
		output, rebuild = filepath.Join(e.Home, "AGENTS.md"), "daisy prompt build "+c.Args[0]
	} else {
		e, err = env.Resolve(c.Ops)
		output, rebuild = filepath.Join(e.Workspace, ".daisy", "AGENTS.md"), prompt.WorkspaceRebuild
		prompts = filepath.Join(e.Workspace, ".daisy", "prompts")
	}
	if err != nil {
		return err
	}
	b, err := buildAgents(c, e, output, rebuild, prompts)
	if err != nil {
		return err
	}
	writes := []fileWrite{{output, b.Text}}
	if prompts != "" {
		writes = append(writes, promptWrites(c.Ops.FS(), prompts, b)...)
	}
	var undo undoStack
	if err := writeAll(c.Ops.FS(), e, &undo, writes); err != nil {
		return err
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

// buildAgents builds e's AGENTS.md for output, saying its warnings on
// stderr; rebuild is the command the header names, prompts the directory
// layered prompts are materialised in ("" for a home's own build).
func buildAgents(c *Context, e env.Env, output, rebuild, prompts string) (prompt.Built, error) {
	b, err := prompt.Build(c.Ops.FS(), prompt.Spec{
		Root: e.Root, HomeName: e.HomeName, Home: e.Home,
		Output: output, Rebuild: rebuild, Built: c.Ops.Now(), Prompts: prompts,
	})
	if err != nil {
		return b, err
	}
	for _, w := range b.Warnings {
		fmt.Fprintln(c.Stderr, w)
	}
	return b, nil
}

// promptWrites leave dir holding b's materialised prompts and no other:
// each written, any other *.md there (a prompt no longer layered) removed.
func promptWrites(fs ops.FS, dir string, b prompt.Built) []fileWrite {
	var writes []fileWrite
	for _, p := range slices.Sorted(maps.Keys(b.Materialised)) {
		// The file is Daisy's own: a link standing there is replaced, never
		// written through (it may lead into the root).
		if fi, err := fs.Lstat(p); err == nil && fi.Mode()&iofs.ModeSymlink != 0 {
			writes = append(writes, fileWrite{p, nil})
		}
		writes = append(writes, fileWrite{p, append([]byte{}, b.Materialised[p]...)})
	}
	for _, rel := range prompt.MDFiles(fs, dir) {
		if p := filepath.Join(dir, rel); b.Materialised[p] == nil {
			writes = append(writes, fileWrite{p, nil})
		}
	}
	return writes
}

// ListHealth: the workspace and its home resolve, which the cli has checked.
var ListHealth = verb.ReadOnly(func(*Context) error { return nil })

// listReport is list's report: the home's prompts in include.txt's order
// (nil without include.txt) and the installed skills. read is how far
// the reading got: the text prints what was read before a failure, as
// the listing always has, and --json prints only a whole report.
type listReport struct {
	Home    string       `json:"home"`
	Prompts []listPrompt `json:"prompts"`
	Skills  []listSkill  `json:"skills"`
	read    listRead
	verb.Warned
}

// listRead is how far readList got: the ledger (the header printed), the
// prompts (the Skills section begun), or the whole listing.
type listRead int

const (
	readLedger listRead = iota + 1
	readPrompts
	readAll
)

// listPrompt is an include.txt entry: lazy or included in full; a lazy
// one's trigger summary, nil when it has none; the path looked up when
// the prompt is missing, else nil; and the home's override.
type listPrompt struct {
	Name    string  `json:"name"`
	Lazy    bool    `json:"lazy"`
	Trigger *string `json:"trigger"`
	Missing *string `json:"missing"`
	override
}

// listSkill is an installed skill: its short_description (its
// description with --full, or without one) and the home's override.
type listSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	override
}

// override is a home's override of a root file: "layered", "full" or nil,
// and behind, a full one the root has changed since.
type override struct {
	Form   *string `json:"override"`
	Behind bool    `json:"behind"`
}

// List prints the home's prompts, each with its trigger's first line or
// how it loads, and the installed skills, each with its short description.
// Read-only.
func List(c *Context) error {
	if c.Flag(verb.JSONFlag) {
		return verb.WriteJSON(c, func() (verb.Doc, error) { return readList(c) })
	}
	r, err := readList(c)
	if r.read >= readLedger {
		fmt.Fprintf(c.Stdout, "Prompts (%s):\n", r.Home)
	}
	if r.read >= readPrompts && r.Prompts == nil {
		fmt.Fprintln(c.Stdout, "  (no include.txt found)")
	}
	for _, p := range r.Prompts {
		switch {
		case p.Missing != nil:
			fmt.Fprintf(c.Stdout, "  %s (missing: %s)\n", pad(p.Name, 20), *p.Missing)
		case p.Lazy && p.Trigger == nil:
			fmt.Fprintf(c.Stdout, "  %s (lazy, no trigger summary)%s\n", pad(p.Name, 20), p.tag())
		case p.Lazy:
			fmt.Fprintf(c.Stdout, "  %s %s%s\n", pad(p.Name, 20), *p.Trigger, p.tag())
		default:
			fmt.Fprintf(c.Stdout, "  %s (loaded with .daisy/AGENTS.md)%s\n", pad(p.Name, 20), p.tag())
		}
	}
	if r.read >= readPrompts {
		fmt.Fprintln(c.Stdout)
		fmt.Fprintln(c.Stdout, "Skills:")
	}
	if r.read == readAll && len(r.Skills) == 0 {
		fmt.Fprintf(c.Stdout, "  (none installed — run: daisy init %s)\n", r.Home)
	}
	for _, s := range r.Skills {
		fmt.Fprintf(c.Stdout, "  %s %s%s\n", pad(s.Name, 20), s.Description, s.tag())
	}
	return err
}

// readList reads list's report.
func readList(c *Context) (*listReport, error) {
	fs := c.Ops.FS()
	r := &listReport{Home: c.Env.HomeName, Skills: []listSkill{}}
	ledger, err := prompt.ReadLedger(fs, c.Env.Home)
	if err != nil {
		return r, fmt.Errorf("Error: cannot read %s: %v", filepath.Join(c.Env.Home, prompt.LedgerRel), err)
	}
	r.read = readLedger
	include := filepath.Join(c.Env.Home, "include.txt")
	if fi, err := fs.Stat(include); err == nil && fi.Mode().IsRegular() {
		data, err := fs.ReadFile(include)
		if err != nil {
			return r, fmt.Errorf("Error: cannot read %s: %v", include, err)
		}
		entries, err := prompt.ParseInclude(data)
		if err != nil {
			return r, fmt.Errorf("Error: %s: %v", include, err)
		}
		r.Prompts = []listPrompt{}
		for _, en := range entries {
			p := listPrompt{Name: en.Name, Lazy: en.Lazy}
			path, ok := prompt.Lookup(fs, c.Env.Home, c.Env.Root, en.Name)
			src := prompt.Locate(fs, c.Env.Home, c.Env.Root, "prompts/"+en.Name+".md")
			switch {
			case !ok:
				p.Missing = &path
			case en.Lazy:
				body, err := src.Text(fs)
				if err != nil {
					return r, fmt.Errorf("Error: %v", err)
				}
				if t := triggerSummary(body); t != "" {
					p.Trigger = &t
				}
				p.override = overrideOf(fs, src, ledger)
			default:
				p.override = overrideOf(fs, src, ledger)
			}
			r.Prompts = append(r.Prompts, p)
		}
	}
	r.read = readPrompts

	for _, s := range skill.All(fs, skill.InstalledDirs(c.Ops.Getenv("HOME"), c.Env.Workspace)) {
		doc, err := fs.ReadFile(s.Path)
		if err != nil {
			return r, fmt.Errorf("Error: cannot read %s: %v", s.Path, err)
		}
		text, _ := skill.Field(doc, "description")
		if !c.Flag("--full") {
			if short, _ := skill.Field(doc, "short_description"); short != "" {
				text = short
			}
		}
		text = strings.TrimPrefix(strings.TrimSuffix(text, `"`), `"`)
		src := prompt.Locate(fs, c.Env.Home, c.Env.Root, "skills/"+s.Name+"/SKILL.md")
		r.Skills = append(r.Skills, listSkill{s.Name, text, overrideOf(fs, src, ledger)})
	}
	r.read = readAll
	return r, nil
}

// overrideOf is src's override: layered, full, or full and behind, the
// root changed since the copy; none for the root's own.
func overrideOf(fs ops.FS, src prompt.Source, ledger map[string]string) override {
	var form string
	switch src.Form {
	case prompt.Layered:
		form = "layered"
	case prompt.Full:
		form = "full"
	default:
		return override{}
	}
	return override{&form, src.Behind(fs, ledger)}
}

// tag is the text's mark of the override: ` [layered]`, ` [full]` or
// ` [full, behind]`.
func (o override) tag() string {
	switch {
	case o.Form == nil:
		return ""
	case o.Behind:
		return " [" + *o.Form + ", behind]"
	}
	return " [" + *o.Form + "]"
}

// triggerSummary is the first line of the trigger that says something: not
// the `Read the full …` pointer, a list marker dropped, not empty; cut to
// 97 characters and `...` past 100; "" when none does.
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
	return ""
}

// pad left-justifies s in width bytes, as the bash's printf %-Ns did.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
