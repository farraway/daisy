package commands

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"daisy/internal/env"
	"daisy/internal/todo"
)

// Status prints the workspace summary an agent orients itself with: the
// binding, projects, today's counts, the backlog and the active plan.
var Status = readOnly(status)

var (
	pendingLine   = regexp.MustCompile(`^- \[ \]`)
	completedLine = regexp.MustCompile(`^- \[x\]`)
	logEntry      = regexp.MustCompile(`^- [0-9]{4} `)
)

func status(c *Context) error {
	fs := c.Ops.FS()
	fmt.Fprintln(c.Stdout, header(c))
	reportDrift(c)
	fmt.Fprintf(c.Stdout, "Projects: %s\n\n", c.Env.ProjectsSummary())

	today, err := fs.ReadFile(filepath.Join(c.Env.Home, "journal", "today.md"))
	if err != nil {
		fmt.Fprintln(c.Stdout, "  No today.md — run 'daisy new-day' to start.")
		return nil
	}
	lines := strings.Split(string(today), "\n")
	pending, completed := countMatching(lines, pendingLine), countMatching(lines, completedLine)
	fmt.Fprintf(c.Stdout, "Tasks:  %d pending, %d completed today\n", pending, completed)
	fmt.Fprintf(c.Stdout, "Log:    %d entries\n", countLogEntries(lines))

	if b, err := fs.ReadFile(filepath.Join(c.Env.Home, "tasks", "todo.txt")); err == nil {
		backlog := 0
		for _, l := range todo.Lines(string(b)) {
			if l != "" && todo.Parse(l).State == todo.Open {
				backlog++
			}
		}
		fmt.Fprintf(c.Stdout, "Backlog: %d tasks in todo.txt\n", backlog)
	}

	plan := filepath.Join(c.Env.Workspace, "PLAN.md")
	if fi, err := fs.Lstat(plan); err == nil && fi.Mode()&fsModeSymlink != 0 {
		path := env.RealPath(fs, plan)
		title := ""
		if b, err := fs.ReadFile(path); err == nil {
			first, _, _ := strings.Cut(string(b), "\n")
			title = first
			if t, ok := strings.CutPrefix(first, "#"); ok {
				title = strings.TrimLeft(strings.TrimLeft(t, "#"), " ")
			}
		}
		fmt.Fprintf(c.Stdout, "\nPlan:   %s\n        %s\n", title, path)
	}
	return nil
}

func countMatching(lines []string, re *regexp.Regexp) int {
	n := 0
	for _, l := range lines {
		if re.MatchString(l) {
			n++
		}
	}
	return n
}

// countLogEntries counts `- HHMM ` lines between `#### Log` and the next heading.
func countLogEntries(lines []string) int {
	n, inLog := 0, false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "#### Log"):
			inLog = true
		case inLog && strings.HasPrefix(l, "#### "):
			return n
		case inLog && logEntry.MatchString(l):
			n++
		}
	}
	return n
}
