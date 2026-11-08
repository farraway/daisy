package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/env"
)

// Projects lists the home's active projects (or, with --archived, the
// archived ones), each with the real path of its file. Read-only.
var Projects = readOnly(projects)

func projects(c *Context) error {
	fs := c.Ops.FS()
	dir, label := filepath.Join(c.Env.Home, "projects"), "Active projects"
	if c.Flag("--archived") {
		dir, label = filepath.Join(dir, "_archive"), "Archived projects"
	}
	entries, _ := fs.ReadDir(dir) // no directory, no projects
	// The `*.md` glob: no dotfile, in the files' byte order (ReadDir's).
	var names []string
	for _, en := range entries {
		if n := en.Name(); strings.HasSuffix(n, ".md") && !strings.HasPrefix(n, ".") && !en.IsDir() {
			names = append(names, strings.TrimSuffix(n, ".md"))
		}
	}
	if len(names) == 0 {
		fmt.Fprintf(c.Stdout, "%s: none\n", label)
		return nil
	}
	fmt.Fprintf(c.Stdout, "%s:\n", label)
	for _, n := range names {
		fmt.Fprintf(c.Stdout, "  %-30s %s\n", n, env.RealPath(fs, filepath.Join(dir, n+".md")))
	}
	return nil
}
