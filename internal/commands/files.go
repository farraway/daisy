package commands

import (
	"errors"
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
// Given a name, it prints that one path and nothing else.
var Files = readOnly(files)

var (
	rollingWindow = regexp.MustCompile(`^journal-.*-(last14|mtd|ytd)\.md$`)
	closedWindow  = regexp.MustCompile(`^journal-[0-9]{4}([0-9]{2})?\.md$`)
)

// named is a name `daisy files <name>` takes and its path, unresolved.
type named struct{ name, path string }

// fileNames are the names `daisy files <name>` takes, in the listing's
// order: one per row the listing always prints, with that row's path, and
// `plan`, the workspace's PLAN.md link. A name is its file's stem or its
// directory's, `-archive` after the directory an archive sits in.
func fileNames(e env.Env) []named {
	h := e.Home
	return []named{
		{"identity", e.IdentityFile},
		{"today", filepath.Join(h, "journal", "today.md")},
		{"journal", filepath.Join(h, "journal", "journal.md")},
		{"todo", filepath.Join(h, "tasks", "todo.txt")},
		{"done", filepath.Join(h, "tasks", "done.txt")},
		{"alias", filepath.Join(h, "tasks", "alias.txt")},
		{"projects", filepath.Join(h, "projects")},
		{"projects-archive", filepath.Join(h, "projects", "_archive")},
		{"feedback", filepath.Join(h, "feedback", "feedback.md")},
		{"plans", filepath.Join(h, "plans")},
		{"plans-archive", filepath.Join(h, "plans", "archive")},
		{"plan", filepath.Join(e.Workspace, "PLAN.md")},
		{"metrics", env.MetricsFile(h)},
		{"prompts", filepath.Join(h, "prompts")},
		{"agents", filepath.Join(h, "AGENTS.md")},
	}
}

func files(c *Context) error {
	if len(c.Args) == 1 {
		return fileByName(c, c.Args[0])
	}
	fs := c.Ops.FS()
	home := c.Env.Home
	paths := map[string]string{}
	for _, n := range fileNames(c.Env) {
		paths[n.name] = n.path
	}
	row := func(name, path string) { fmt.Fprintf(c.Stdout, "  %-12s %s\n", name, env.RealPath(fs, path)) }
	exists := func(path string) bool { fi, err := fs.Stat(path); return err == nil && fi.Mode().IsRegular() }

	fmt.Fprintln(c.Stdout, header(c))
	fmt.Fprintln(c.Stdout)
	reportDrift(c)

	fmt.Fprintln(c.Stdout, "Identity:")
	if exists(paths["identity"]) {
		row(c.Env.Identity, paths["identity"])
	} else {
		fmt.Fprintf(c.Stdout, "  %-12s %s  (missing)\n", c.Env.Identity, paths["identity"])
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Journal:")
	row("today.md", paths["today"])
	row("journal.md", paths["journal"])
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
	row("todo.txt", paths["todo"])
	row("done.txt", paths["done"])
	row("alias.txt", paths["alias"])
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Projects:")
	row("active", paths["projects"])
	row("archived", paths["projects-archive"])
	fmt.Fprintf(c.Stdout, "  %-12s %s\n", "workspace", c.Env.ProjectsSummary())
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Feedback:")
	row("feedback.md", paths["feedback"])
	if exists(filepath.Join(home, "feedback", "archive.md")) {
		row("archive.md", filepath.Join(home, "feedback", "archive.md"))
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Plans:")
	row("plans/", paths["plans"])
	row("archive/", paths["plans-archive"])
	if fi, err := fs.Lstat(paths["plan"]); err == nil && fi.Mode()&fsModeSymlink != 0 {
		row("current plan", paths["plan"])
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Metrics:")
	row("metrics.csv", paths["metrics"])
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Prompts:")
	row("home/", paths["prompts"])
	fmt.Fprintln(c.Stdout)

	fmt.Fprintf(c.Stdout, "%-14s %s\n", "AGENTS.md:", env.RealPath(fs, paths["agents"]))
	return nil
}

// fileByName prints the real path name stands for, alone on its line,
// whether or not the file exists yet. Drift is the listing's to report:
// every name's path but plan's comes from the home, not through the
// .daisy/ links. `plan` alone can have no path to give, when the workspace has no
// plan's link: a failure naming the repair.
func fileByName(c *Context, name string) error {
	fsys := c.Ops.FS()
	var names []string
	for _, n := range fileNames(c.Env) {
		names = append(names, n.name)
		if n.name == name && name == "plan" {
			return printPlan(c, n.path)
		}
		if n.name == name {
			fmt.Fprintln(c.Stdout, env.RealPath(fsys, n.path))
			return nil
		}
	}
	return UsageError(fmt.Sprintf("unknown name: %s; one of %s", name, strings.Join(names, ", ")))
}

// printPlan prints the plan the link resolves to; no link, a file that is
// not one or a link that resolves to nothing is no current plan. PLAN.md
// reaches the home through .daisy/plans, so a broken link under a drifted
// binding names init's repair first.
func printPlan(c *Context, link string) error {
	fsys := c.Ops.FS()
	fi, err := fsys.Lstat(link)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return errors.New("Error: no current plan: this workspace has no PLAN.md\n  Start one: daisy plan new <description> [+project]")
	case err != nil:
		return fmt.Errorf("Error: cannot read %s: %w", link, err)
	case fi.Mode()&fsModeSymlink == 0:
		return fmt.Errorf("Error: no current plan: %s is not a link (a plan's PLAN.md links into the home's plans/)\n  Move it aside, then start one: daisy plan new <description> [+project]", link)
	}
	real, err := fsys.EvalSymlinks(link)
	if err != nil {
		if drift := c.Env.LinkDrift(fsys); len(drift) > 0 {
			return fmt.Errorf("Error: no current plan: %s is a broken link, and %s\n  Re-run 'daisy init %s'; if the link stays broken, remove it, then start one: daisy plan new <description> [+project]", link, strings.Join(drift, "; "), c.Env.Identity)
		}
		return fmt.Errorf("Error: no current plan: %s is a broken link\n  Remove it, then start one: daisy plan new <description> [+project]", link)
	}
	fmt.Fprintln(c.Stdout, real)
	return nil
}
