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

// filesReport is the listing: the binding, every name's real path (plan's
// nil without a current plan, PLAN.md a link), the rows printed only when
// their file exists and the workspace's projects. The listing and
// `files --json` both print it.
type filesReport struct {
	Home            string             `json:"home"`
	Identity        string             `json:"identity"`
	Workspace       string             `json:"workspace"`
	Paths           map[string]*string `json:"paths"`
	IdentityMissing bool               `json:"identity_missing"` // its path then as named, unresolved
	PreRotation     *string            `json:"pre_rotation"`
	JournalWindows  []namedPath        `json:"journal_windows"` // the rolling windows, then the closed ones
	FeedbackArchive *string            `json:"feedback_archive"`
	Projects        workspaceProjects  `json:"workspace_projects"`
	warned
}

// workspaceProjects is the projects the workspace resolves to and where
// from: what the text's `workspace` and `Projects:` rows summarise.
type workspaceProjects struct {
	Names  []string           `json:"names"`
	Source env.ProjectsSource `json:"source"`
}

// projectsOf is the workspace's projects, [] for none.
func projectsOf(e env.Env) workspaceProjects {
	p := workspaceProjects{e.Projects, e.ProjectsSource}
	if p.Names == nil {
		p.Names = []string{}
	}
	return p
}

// namedPath is a row whose name varies: its file's name and real path.
type namedPath struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Files prints the resolved real path of every per-home file and
// directory: where things actually live, whatever the .daisy/ links say.
// Given a name, it prints that one path and nothing else.
func Files(c *Context) error {
	switch {
	case c.Flag(JSONFlag) && len(c.Args) == 1:
		return UsageError("--json lists every path; with <name>, its path alone is printed")
	case len(c.Args) == 1:
		return fileByName(c, c.Args[0])
	case c.Flag(JSONFlag):
		return writeJSON(c, func() (jsonDoc, error) {
			reportDrift(c)
			return listFiles(c), nil
		})
	}
	r := listFiles(c)
	row := func(name string, path *string) { fmt.Fprintf(c.Stdout, "  %-12s %s\n", name, *path) }

	fmt.Fprintln(c.Stdout, header(c))
	fmt.Fprintln(c.Stdout)
	reportDrift(c)

	fmt.Fprintln(c.Stdout, "Identity:")
	if r.IdentityMissing {
		fmt.Fprintf(c.Stdout, "  %-12s %s  (missing)\n", r.Identity, *r.Paths["identity"])
	} else {
		row(r.Identity, r.Paths["identity"])
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Journal:")
	row("today.md", r.Paths["today"])
	row("journal.md", r.Paths["journal"])
	if r.PreRotation != nil {
		row("pre-rotation", r.PreRotation)
	}
	for _, w := range r.JournalWindows {
		row(w.Name, &w.Path)
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Tasks:")
	row("todo.txt", r.Paths["todo"])
	row("done.txt", r.Paths["done"])
	row("alias.txt", r.Paths["alias"])
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Projects:")
	row("active", r.Paths["projects"])
	row("archived", r.Paths["projects-archive"])
	fmt.Fprintf(c.Stdout, "  %-12s %s\n", "workspace", c.Env.ProjectsSummary())
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Feedback:")
	row("feedback.md", r.Paths["feedback"])
	if r.FeedbackArchive != nil {
		row("archive.md", r.FeedbackArchive)
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Plans:")
	row("plans/", r.Paths["plans"])
	row("archive/", r.Paths["plans-archive"])
	if r.Paths["plan"] != nil {
		row("current plan", r.Paths["plan"])
	}
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Metrics:")
	row("metrics.csv", r.Paths["metrics"])
	fmt.Fprintln(c.Stdout)

	fmt.Fprintln(c.Stdout, "Prompts:")
	row("home/", r.Paths["prompts"])
	fmt.Fprintln(c.Stdout)

	fmt.Fprintf(c.Stdout, "%-14s %s\n", "AGENTS.md:", *r.Paths["agents"])
	return nil
}

// listFiles reads the listing.
func listFiles(c *Context) *filesReport {
	fs := c.Ops.FS()
	home := c.Env.Home
	real := func(path string) *string { p := env.RealPath(fs, path); return &p }
	exists := func(path string) bool { fi, err := fs.Stat(path); return err == nil && fi.Mode().IsRegular() }
	r := &filesReport{Home: c.Env.HomeName, Identity: c.Env.Identity, Workspace: c.Env.Workspace, Paths: map[string]*string{}, JournalWindows: []namedPath{}}
	for _, n := range fileNames(c.Env) {
		r.Paths[n.name] = real(n.path)
	}
	if id := c.Env.IdentityFile; !exists(id) {
		r.Paths["identity"], r.IdentityMissing = &id, true
	}
	if fi, err := fs.Lstat(filepath.Join(c.Env.Workspace, "PLAN.md")); err != nil || fi.Mode()&fsModeSymlink == 0 {
		r.Paths["plan"] = nil
	}
	if p := filepath.Join(home, "journal", "journal-pre-rotation.md"); exists(p) {
		r.PreRotation = real(p)
	}
	journal, _ := fs.ReadDir(filepath.Join(home, "journal"))
	window := func(name string) {
		r.JournalWindows = append(r.JournalWindows, namedPath{name, *real(filepath.Join(home, "journal", name))})
	}
	for _, suffix := range []string{"last14", "mtd", "ytd"} {
		for _, en := range journal {
			if rollingWindow.MatchString(en.Name()) && strings.HasSuffix(en.Name(), "-"+suffix+".md") && exists(filepath.Join(home, "journal", en.Name())) {
				window(en.Name())
			}
		}
	}
	for _, en := range journal {
		if closedWindow.MatchString(en.Name()) && !en.IsDir() {
			window(en.Name())
		}
	}
	if p := filepath.Join(home, "feedback", "archive.md"); exists(p) {
		r.FeedbackArchive = real(p)
	}
	r.Projects = projectsOf(c.Env)
	return r
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
