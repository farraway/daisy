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

// workspaceReport is status's report: the binding, the workspace's
// projects, today's counts (nil without today.md), the backlog (nil
// without todo.txt) and the current plan (nil unless PLAN.md is a link).
// The text stops at a missing today.md; the report reads on.
type workspaceReport struct {
	Home      string            `json:"home"`
	Identity  string            `json:"identity"`
	Workspace string            `json:"workspace"`
	Projects  workspaceProjects `json:"workspace_projects"`
	Today     *todayCounts      `json:"today"`
	Backlog   *int              `json:"backlog"`
	Plan      *currentPlan      `json:"plan"`
	warned
}

type todayCounts struct {
	Pending    int `json:"pending"`
	Completed  int `json:"completed"`
	LogEntries int `json:"log_entries"`
}

// currentPlan is the plan PLAN.md links to: its first line, less a
// heading's marks, and its real path.
type currentPlan struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

func status(c *Context) error {
	if c.Flag(JSONFlag) {
		return writeJSON(c, func() (jsonDoc, error) {
			reportDrift(c)
			return readWorkspace(c), nil
		})
	}
	r := readWorkspace(c)
	fmt.Fprintln(c.Stdout, header(c))
	reportDrift(c)
	fmt.Fprintf(c.Stdout, "Projects: %s\n\n", c.Env.ProjectsSummary())
	if r.Today == nil {
		fmt.Fprintln(c.Stdout, "  No today.md — run 'daisy new-day' to start.")
		return nil
	}
	fmt.Fprintf(c.Stdout, "Tasks:  %d pending, %d completed today\n", r.Today.Pending, r.Today.Completed)
	fmt.Fprintf(c.Stdout, "Log:    %d entries\n", r.Today.LogEntries)
	if r.Backlog != nil {
		fmt.Fprintf(c.Stdout, "Backlog: %d tasks in todo.txt\n", *r.Backlog)
	}
	if r.Plan != nil {
		fmt.Fprintf(c.Stdout, "\nPlan:   %s\n        %s\n", r.Plan.Title, r.Plan.Path)
	}
	return nil
}

// readWorkspace reads status's report.
func readWorkspace(c *Context) *workspaceReport {
	fs := c.Ops.FS()
	r := &workspaceReport{Home: c.Env.HomeName, Identity: c.Env.Identity, Workspace: c.Env.Workspace, Projects: projectsOf(c.Env)}
	if today, err := fs.ReadFile(filepath.Join(c.Env.Home, "journal", "today.md")); err == nil {
		lines := strings.Split(string(today), "\n")
		r.Today = &todayCounts{countMatching(lines, pendingLine), countMatching(lines, completedLine), countLogEntries(lines)}
	}
	if b, err := fs.ReadFile(filepath.Join(c.Env.Home, "tasks", "todo.txt")); err == nil {
		backlog := 0
		for _, l := range todo.Lines(string(b)) {
			if l != "" && todo.Parse(l).State == todo.Open {
				backlog++
			}
		}
		r.Backlog = &backlog
	}
	plan := filepath.Join(c.Env.Workspace, "PLAN.md")
	if fi, err := fs.Lstat(plan); err == nil && fi.Mode()&fsModeSymlink != 0 {
		r.Plan = &currentPlan{Path: env.RealPath(fs, plan)}
		if b, err := fs.ReadFile(r.Plan.Path); err == nil {
			first, _, _ := strings.Cut(string(b), "\n")
			r.Plan.Title = first
			if t, ok := strings.CutPrefix(first, "#"); ok {
				r.Plan.Title = strings.TrimLeft(strings.TrimLeft(t, "#"), " ")
			}
		}
	}
	return r
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
