package commands

import (
	"errors"
	"fmt"
	"strings"

	"daisy/internal/journal"
	"daisy/internal/ops"
	"daisy/internal/todo"
)

// Done completes the one active task matching the pattern: in todo.txt the
// line becomes `x <today> <created> <text>` at the end of the file, in
// today.md its `- [ ]` becomes `- [x]`, the Log section gets `Done: <text>`,
// and the home is committed once. Zero or several matches refuse without
// touching anything.
func Done(c *Context) error {
	if err := preflight(c); err != nil {
		return err
	}
	pattern := c.Args[0]
	fs := c.Ops.FS()
	b, err := fs.ReadFile(c.todoFile())
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", c.todoFile(), err)
	}
	lines := todo.Lines(string(b))

	// Only active lines are candidates: an `x`/`z` record that also matches
	// is neither offered nor touched.
	var matches []int
	for i, l := range lines {
		if todo.Parse(l).State == todo.Open && todo.ContainsFold(l, pattern) {
			matches = append(matches, i)
		}
	}
	switch {
	case len(matches) == 0:
		return fmt.Errorf("No task found matching: %s", pattern)
	case len(matches) > 1:
		var msg strings.Builder
		fmt.Fprintf(&msg, "Multiple tasks match \"%s\":", pattern)
		for n, i := range matches {
			fmt.Fprintf(&msg, "\n  %d) %s", n+1, lines[i])
		}
		return errors.New(msg.String())
	}

	task := todo.Parse(lines[matches[0]])
	// today.md holds the line as new-day carried it (an undated (A)
	// whole), or as the description, as the agent writes a task in.
	entries := []string{task.Text}
	if e, carried := journal.Entry(lines[matches[0]]); carried && e != task.Text {
		entries = append([]string{e}, entries...)
	}
	today := c.Ops.Now().Format("2006-01-02")
	lines = append(append(lines[:matches[0]:matches[0]], lines[matches[0]+1:]...), task.Complete(today).Format())
	if err := ops.Rewrite(fs, c.todoFile(), []byte(todo.Content(lines)), 0o644); err != nil {
		return err
	}

	// Absent from today.md is a warning, not a failure: an Inbox-only item
	// is checked off by hand and never reaches todo.txt.
	ticked := false
	if b, err := fs.ReadFile(c.todayFile()); err == nil {
		for _, e := range entries {
			if out, ok := journal.Tick(string(b), e); ok {
				if err := ops.Rewrite(fs, c.todayFile(), []byte(out), 0o644); err != nil {
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
