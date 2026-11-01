package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"daisy/internal/todo"
)

// Tasks searches todo.txt and done.txt and prints the matching lines, bare.
// Two axes: scope, this workspace's projects (`+tag`) unless --all; files,
// both unless --todo or --done. The pattern is a literal, case-insensitive
// substring. Read-only.
var Tasks = readOnly(tasks)

func tasks(c *Context) error {
	fs := c.Ops.FS()
	files := []string{c.todoFile(), c.doneFile()}
	for _, f := range c.Flags {
		switch f {
		case "--todo":
			files = []string{c.todoFile()}
		case "--done":
			files = []string{c.doneFile()}
		}
	}
	var lines []string
	found := 0
	for _, f := range files {
		b, err := fs.ReadFile(f)
		if err != nil {
			continue // a missing done.txt is not an error
		}
		found++
		lines = append(lines, todo.Lines(string(b))...)
	}
	if found == 0 {
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = filepath.Base(f)
		}
		return fmt.Errorf("Error: no task file to search: %s in %s", strings.Join(names, ", "), filepath.Dir(c.todoFile()))
	}

	var scope []string
	if !c.Flag("--all") {
		for _, w := range c.Env.Warnings {
			fmt.Fprintln(c.Stderr, w)
		}
		scope = c.Env.Projects
		if len(scope) == 0 {
			// Returning nothing would be a silent lie: the tasks exist, this
			// workspace just does not say which are its own.
			fmt.Fprintln(c.Stderr, "⚠️  This workspace resolves to no project; listing every project's tasks.")
			fmt.Fprintln(c.Stderr, "  Bind it by creating .daisy/project, or pass --all to ask for this deliberately.")
		}
	}
	pattern := ""
	if len(c.Args) == 1 {
		pattern = c.Args[0]
	}

	printed := 0
	for _, l := range lines {
		if l == "" || !inScope(l, scope) || !todo.ContainsFold(l, pattern) {
			continue
		}
		fmt.Fprintln(c.Stdout, l)
		printed++
	}
	if pattern != "" && printed == 0 {
		return fmt.Errorf("No tasks match: %s", pattern)
	}
	return nil
}

// inScope: the line carries one of the projects; an empty scope is every line.
func inScope(line string, projects []string) bool {
	if len(projects) == 0 {
		return true
	}
	t := todo.Parse(line)
	for _, p := range projects {
		if t.HasProject(p) {
			return true
		}
	}
	return false
}
