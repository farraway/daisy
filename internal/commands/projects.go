package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/env"
)

// projectsReport is the listing: which directory, and each project's name
// and the real path of its file.
type projectsReport struct {
	Archived bool        `json:"archived"`
	Projects []namedPath `json:"projects"`
	warned
}

// Projects lists the home's active projects (or, with --archived, the
// archived ones), each with the real path of its file. Read-only.
func Projects(c *Context) error {
	return printReport(c, func() (*projectsReport, error) { return readProjects(c), nil }, func(r *projectsReport) {
		label := "Active projects"
		if r.Archived {
			label = "Archived projects"
		}
		if len(r.Projects) == 0 {
			fmt.Fprintf(c.Stdout, "%s: none\n", label)
			return
		}
		fmt.Fprintf(c.Stdout, "%s:\n", label)
		for _, p := range r.Projects {
			fmt.Fprintf(c.Stdout, "  %-30s %s\n", p.Name, p.Path)
		}
	})
}

func readProjects(c *Context) *projectsReport {
	fs := c.Ops.FS()
	r := &projectsReport{Archived: c.Flag("--archived"), Projects: []namedPath{}}
	dir := filepath.Join(c.Env.Home, "projects")
	if r.Archived {
		dir = filepath.Join(dir, "_archive")
	}
	entries, _ := fs.ReadDir(dir) // no directory, no projects
	// The `*.md` glob: no dotfile, in the files' byte order (ReadDir's).
	for _, en := range entries {
		if n := en.Name(); strings.HasSuffix(n, ".md") && !strings.HasPrefix(n, ".") && !en.IsDir() {
			r.Projects = append(r.Projects, namedPath{strings.TrimSuffix(n, ".md"), env.RealPath(fs, filepath.Join(dir, n))})
		}
	}
	return r
}
