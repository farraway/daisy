package commands

import (
	"errors"
	"fmt"
	"slices"

	"daisy/internal/journal"
	"daisy/internal/ops"
	"daisy/internal/todo"
)

// Done completes the one active task matching the pattern: in todo.txt the
// line becomes `x <today> <created> <text>` after the other lines but the
// cancelled ones, before the first `z` line (docs/task-format.md), in
// today.md its `- [ ]` becomes `- [x]`, the Log section gets `Done: <text>`,
// and the home is committed once. Zero or several matches refuse without
// touching anything.
func Done(c *Context) error {
	if err := preflight(c); err != nil {
		return err
	}
	fs := c.Ops.FS()
	lines, err := readTodo(c)
	if err != nil {
		return err
	}
	i, err := resolve(lines, c.Args[0])
	if err != nil {
		return err
	}
	task, forms := todo.Parse(lines[i]), todayForms(lines[i])
	today := c.Ops.Now().Format("2006-01-02")
	lines = slices.Delete(lines, i, i+1)
	at := slices.IndexFunc(lines, func(l string) bool { return todo.Parse(l).State == todo.Cancelled })
	if at < 0 {
		at = len(lines)
	}
	lines = slices.Insert(lines, at, task.Complete(today).Format())
	if err := ops.Rewrite(fs, todoFile(c), []byte(todo.Content(lines)), 0o644); err != nil {
		return err
	}

	// Absent from today.md is a warning, not a failure: an Inbox-only item
	// is checked off by hand and never reaches todo.txt.
	ticked := false
	if b, err := fs.ReadFile(todayFile(c)); err == nil {
		for _, e := range forms {
			if out, ok := journal.Tick(string(b), e); ok {
				if err := ops.Rewrite(fs, todayFile(c), []byte(out), 0o644); err != nil {
					return err
				}
				ticked = true
				break
			}
		}
	}
	logErr := appendLog(c, c.Ops.Now().Format("1504"), "Done: "+task.Text)

	// The edits are committed whether or not the journal entry could be
	// made; a journal failure is reported after they are safe.
	hash, committed, commitErr := commitHome(c, "Completed: "+task.Text)
	if !ticked {
		fmt.Fprintln(c.Stderr, "⚠️  Task not found in today.md (may not be in active tasks)")
	}
	if logErr != nil {
		state := fmt.Sprintf("committed (%s)", hash)
		if commitErr != nil || !committed {
			state = "edited but NOT committed"
		}
		msg := fmt.Sprintf("Error: journal entry failed — todo.txt and today.md are %s, but nothing was recorded in today.md's #### Log section\n%v", state, logErr)
		if commitErr != nil {
			msg += "\n" + commitErr.Error()
		}
		return errors.New(msg)
	}
	if commitErr != nil {
		return fmt.Errorf("Error: %s is completed in todo.txt and today.md, but the commit failed\n%v", task.Text, commitErr)
	}
	if !committed {
		hash = "not committed"
	}
	fmt.Fprintf(c.Stdout, "✅ Done: %s (commit %s)\n", task.Text, hash)
	return nil
}
