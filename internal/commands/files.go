package commands

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"daisy/internal/env"
)

const fsModeSymlink = fs.ModeSymlink

// Files prints the resolved real path of every per-home file and
// directory: where things actually live, whatever the .daisy/ links say.
var Files = readOnly(files)

var (
	rollingWindow = regexp.MustCompile(`^journal-.*-(last14|mtd|ytd)\.md$`)
	closedWindow  = regexp.MustCompile(`^journal-[0-9]{4}([0-9]{2})?\.md$`)
)

func files(c *Context) error {
	fs := c.Ops.FS()
	home := c.Env.Home
	row := func(name, path string) { fmt.Fprintf(c.Stdout, "  %-12s %s\n", name, env.RealPath(fs, path)) }
	exists := func(path string) bool { fi, err := fs.Stat(path); return err == nil && fi.Mode().IsRegular() }

	fmt.Fprintln(c.Stdout, header(c))
	fmt.Fprintln(c.Stdout)
	reportDrift(c)

	fmt.Fprintln(c.Stdout, "Identity:")
	if exists(c.Env.IdentityFile) {
		row(c.Env.Identity, c.Env.IdentityFile)
	} else {
		fmt.Fprintf(c.Stdout, "  %-12s %s  (missing)\n", c.Env.Identity, c.Env.IdentityFile)
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Journal:")
	row("today.md", filepath.Join(home, "journal", "today.md"))
	row("journal.md", filepath.Join(home, "journal", "journal.md"))
	if exists(filepath.Join(home, "journal", "journal-pre-rotation.md")) {
		row("pre-rotation", filepath.Join(home, "journal", "journal-pre-rotation.md"))
	}
	journal, _ := fs.ReadDir(filepath.Join(home, "journal"))
	for _, suffix := range []string{"last14", "mtd", "ytd"} {
		for _, en := range journal {
			if rollingWindow.MatchString(en.Name()) && strings.HasSuffix(en.Name(), "-"+suffix+".md") && exists(filepath.Join(home, "journal", en.Name())) {
				row(en.Name(), filepath.Join(home, "journal", en.Name()))
			}
		}
	}
	for _, en := range journal {
		if closedWindow.MatchString(en.Name()) && !en.IsDir() {
			row(en.Name(), filepath.Join(home, "journal", en.Name()))
		}
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Tasks:")
	row("todo.txt", filepath.Join(home, "tasks", "todo.txt"))
	row("done.txt", filepath.Join(home, "tasks", "done.txt"))
	row("alias.txt", filepath.Join(home, "tasks", "alias.txt"))
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Projects:")
	row("active", filepath.Join(home, "projects"))
	row("archived", filepath.Join(home, "projects", "_archive"))
	fmt.Fprintf(c.Stdout, "  %-12s %s\n", "workspace", c.Env.ProjectsSummary())
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Feedback:")
	row("feedback.md", filepath.Join(home, "feedback", "feedback.md"))
	if exists(filepath.Join(home, "feedback", "archive.md")) {
		row("archive.md", filepath.Join(home, "feedback", "archive.md"))
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Plans:")
	row("plans/", filepath.Join(home, "plans"))
	row("archive/", filepath.Join(home, "plans", "archive"))
	plan := filepath.Join(c.Env.Workspace, "PLAN.md")
	if fi, err := fs.Lstat(plan); err == nil && fi.Mode()&fsModeSymlink != 0 {
		row("current plan", plan)
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Prompts:")
	row("home/", filepath.Join(home, "prompts"))
	fmt.Fprintln(c.Stdout)

	fmt.Fprintf(c.Stdout, "%-14s %s\n", "AGENTS.md:", env.RealPath(fs, filepath.Join(home, "AGENTS.md")))
	return nil
}
