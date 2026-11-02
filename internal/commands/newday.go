package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"daisy/internal/env"
	"daisy/internal/journal"
	"daisy/internal/todo"
)

// start is what differs between a new day and a new week.
type start struct {
	template string // under templates/, the home's or else the root's
	noun     string // "day" or "week"
	archive  bool   // move todo.txt's completed lines to done.txt
}

var (
	day  = start{template: "journal-day.md", noun: "day"}
	week = start{template: "journal-week.md", noun: "week", archive: true}
)

// NewDay archives today.md into journal.md, rotates the journal, drops
// cancelled tasks, writes a new today.md from the day template with the
// open tasks carried over, and commits the home once.
func NewDay(c *Context) error { return startDay(c, day) }

// NewWeek is NewDay with the week template, and todo.txt's completed tasks
// moved to done.txt first.
func NewWeek(c *Context) error { return startDay(c, week) }

// startDay reads everything and decides every write before making the
// first, so a refusal (a template or todo.txt missing, a journal rotate
// cannot split) leaves the home as it was.
func startDay(c *Context, s start) error {
	if err := preflight(c); err != nil {
		return err
	}
	fsys := c.Ops.FS()
	path, err := dayTemplate(c, s)
	if err != nil {
		return err
	}
	tpl, err := fsys.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", path, err)
	}
	b, err := fsys.ReadFile(c.todoFile())
	if err != nil {
		return errors.New("Error: todo.txt not found")
	}
	var open, cancelled, completed []string
	for _, l := range todo.Lines(string(b)) {
		switch state := todo.Parse(l).State; {
		case state == todo.Cancelled:
			cancelled = append(cancelled, l)
		case state == todo.Done && s.archive:
			completed = append(completed, l)
		default:
			open = append(open, l)
		}
	}

	// Yesterday goes to journal.md as it is, after a separator when the
	// journal has anything in it; an empty today.md is archived too.
	journalPath := c.journalFile()
	archived := false
	jb, err := fsys.ReadFile(journalPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	content := string(jb)
	if yb, err := fsys.ReadFile(c.todayFile()); err == nil {
		if content != "" {
			content += "\n---\n\n"
		}
		content += string(yb)
		archived = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	rot, err := rotation(c, content)
	if err != nil {
		return err
	}

	var done string
	if s.archive {
		db, err := fsys.ReadFile(c.doneFile())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		done = string(db)
		if len(completed) > 0 && done != "" && !strings.HasSuffix(done, "\n") {
			done += "\n"
		}
		done += todo.Content(completed)
	}
	now := c.Ops.Now()
	carried := journal.Carry(open)
	today := journal.Fill(string(tpl), now, carried)

	switch {
	case rot.Moved > 0:
		err = applyRotation(c, rot)
	case archived:
		err = fsys.WriteFile(journalPath, []byte(content), 0o644)
	}
	if err != nil {
		return err
	}
	if archived {
		fmt.Fprintln(c.Stdout, "📦 Archived yesterday to journal.md")
	}
	reportRotation(c, rot)
	if err := fsys.WriteFile(c.todoFile(), []byte(todo.Content(open)), 0o644); err != nil {
		return err
	}
	if len(cancelled) > 0 {
		fmt.Fprintf(c.Stdout, "🗑️  Deleted %d cancelled task(s)\n", len(cancelled))
	}
	if s.archive {
		if err := fsys.WriteFile(c.doneFile(), []byte(done), 0o644); err != nil {
			return err
		}
		if len(completed) > 0 {
			fmt.Fprintf(c.Stdout, "📦 Archived %d completed task(s) to done.txt\n", len(completed))
		}
	}
	if err := fsys.WriteFile(c.todayFile(), []byte(today), 0o644); err != nil {
		return err
	}

	stamp := now.Format(time.DateOnly) + " " + now.Format("Monday")
	fmt.Fprintf(c.Stdout, "✅ New %s started: %s\n", s.noun, stamp)
	fmt.Fprintf(c.Stdout, "   High priority tasks: %d\n   Next priority tasks: %d\n   Inbox tasks: %d\n   GitHub tasks: %d\n",
		len(carried.Now), len(carried.Next), len(carried.Inbox), len(carried.GitHub))
	message := fmt.Sprintf("New %s: %s", s.noun, stamp)
	hash, committed, err := commitHome(c, message)
	if err != nil {
		return err
	}
	reportCommit(c, message, hash, committed)
	return nil
}

// NewDayHealth and NewWeekHealth: the template, todo.txt, and the journal
// directory (new-day) or done.txt (new-week).
var (
	NewDayHealth = readOnly(func(c *Context) error {
		return startHealth(c, day, func() error { return requireDir(c, filepath.Dir(c.todayFile())) })
	})
	NewWeekHealth = readOnly(func(c *Context) error {
		return startHealth(c, week, func() error { return requireFiles(c, c.doneFile()) })
	})
)

func startHealth(c *Context, s start, last func() error) error {
	if _, err := dayTemplate(c, s); err != nil {
		return err
	}
	if err := requireFiles(c, c.todoFile()); err != nil {
		return err
	}
	return last()
}

// dayTemplate is the path of s's template: the home's templates/ first, then
// the root's, as a home's prompts shadow the root's. The refusal names both
// places looked.
func dayTemplate(c *Context, s start) (string, error) {
	rel := "templates/" + s.template
	path, ok := env.Lookup(c.Ops.FS(), c.Env.Home, c.Env.Root, rel)
	if !ok {
		return "", fmt.Errorf("Error: Template missing: %s\n  Looked for: %s/%s, then %s", s.template, c.Env.Home, rel, path)
	}
	return path, nil
}
