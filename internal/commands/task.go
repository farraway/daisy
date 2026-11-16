package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"daisy/internal/journal"
	"daisy/internal/ops"
	"daisy/internal/todo"
)

// Add adds a task: `(P) <today> <description>` (no priority: `<today>
// <description>`) in todo.txt after the last open line that sorts at or
// before it, and an (A) or (B) task in today.md's section as a new day
// would carry it; the home is committed once. today.md absent, or without
// that section, is a warning: todo.txt takes the task alone.
func Add(c *Context) error {
	text := c.Args[0]
	p, err := priorityArg(c.Args[1])
	if err != nil {
		return err
	}
	switch t := todo.Parse(text); {
	case strings.ContainsAny(text, "\r\n"):
		return errors.New("Error: a task is one line; the description holds a line break")
	case strings.TrimSpace(text) == "":
		return errors.New("Error: the description is blank")
	case t.Priority != 0:
		return fmt.Errorf("Error: the description starts with a priority, (%c); give it as the priority argument: daisy add %q %c", t.Priority, text[4:], t.Priority)
	}
	if err := preflight(c); err != nil {
		return err
	}
	lines, err := readTodo(c)
	if err != nil {
		return err
	}
	line := todo.Task{Priority: p, Created: c.Ops.Now().Format("2006-01-02"), Text: text}.Format()
	day, err := readToday(c)
	if err != nil {
		return err
	}
	var outcome string
	if p == 'A' || p == 'B' {
		outcome = day.add(c, line, "added to ")
	} else {
		outcome = "not added, " + carried(line)
	}
	if err := writeTask(c, todo.Insert(lines, line), day); err != nil {
		return err
	}
	message := "Added: " + text
	if p != 0 {
		message = "Added: " + priorityLabel(p) + " " + text
	}
	return commitTask(c, message, "✅ Added: %s; today.md: %s (commit %s)\n", line, outcome)
}

// Cancel cancels the one open task matching the pattern, resolved as
// `done` resolves it: in todo.txt the line becomes `z <today> <created>
// <text>` at the end of the file, where docs/task-format.md keeps the
// cancelled lines until the next start deletes them; in today.md its
// `- [ ]` becomes `- [z]`; the home is committed once.
func Cancel(c *Context) error {
	if err := preflight(c); err != nil {
		return err
	}
	lines, err := readTodo(c)
	if err != nil {
		return err
	}
	i, err := resolve(lines, c.Args[0])
	if err != nil {
		return err
	}
	task, forms := todo.Parse(lines[i]), todayForms(lines[i])
	lines = append(slices.Delete(lines, i, i+1), task.Cancel(c.Ops.Now().Format("2006-01-02")).Format())
	day, err := readToday(c)
	if err != nil {
		return err
	}
	for _, f := range forms {
		if out, ok := journal.Cancel(day.content, f); ok {
			day.content, day.changed = out, true
			break
		}
	}
	if err := writeTask(c, lines, day); err != nil {
		return err
	}
	if !day.changed {
		fmt.Fprintln(c.Stderr, "⚠️  Task not found in today.md (may not be in active tasks)")
	}
	return commitTask(c, "Cancelled: "+task.Text, "✅ Cancelled: %s (commit %s)\n", task.Text)
}

// Priority gives the one open task matching the pattern, resolved as
// `done` resolves it, a new priority: in todo.txt the line, its creation
// date and text kept, moves to the new priority's place (Add's); in
// today.md its line moves to the section a new day would carry it to, or
// out when it would carry it nowhere, and one absent from today.md is
// added when it becomes (A) or (B). The home is committed once; the
// priority the task already has changes nothing.
func Priority(c *Context) error {
	p, err := priorityArg(c.Args[1])
	if err != nil {
		return err
	}
	if err := preflight(c); err != nil {
		return err
	}
	lines, err := readTodo(c)
	if err != nil {
		return err
	}
	i, err := resolve(lines, c.Args[0])
	if err != nil {
		return err
	}
	old := todo.Parse(lines[i])
	if old.Priority == p {
		fmt.Fprintf(c.Stdout, "✅ %s is already %s; nothing changed\n", old.Text, priorityLabel(p))
		return nil
	}
	moved := old
	moved.Priority = p
	line := moved.Format()
	day, err := readToday(c)
	if err != nil {
		return err
	}
	removed := false
	for _, f := range todayForms(lines[i]) {
		if out, ok := journal.Remove(day.content, f); ok {
			day.content, removed = out, true
			break
		}
	}
	// The removal is written only with the line's new place, or as the
	// whole change when it has none: add leaves today.md unchanged when
	// the section is missing.
	var outcome string
	switch _, _, placed := journal.Place(line); {
	case removed && placed:
		outcome = day.add(c, line, "moved to ")
	case removed:
		day.changed, outcome = true, "removed, "+carried(line)
	case p == 'A' || p == 'B':
		outcome = day.add(c, line, "added to ")
	default:
		outcome = "not in it, " + carried(line)
	}
	if err := writeTask(c, todo.Insert(slices.Delete(lines, i, i+1), line), day); err != nil {
		return err
	}
	change := priorityLabel(old.Priority) + " → " + priorityLabel(p)
	return commitTask(c, "Priority: "+old.Text+" "+change, "✅ Priority: %s %s; today.md: %s (commit %s)\n", old.Text, change, outcome)
}

// priorityArg reads a priority argument: A, B, C or D, or none for a task
// without one (the inbox), as 0.
func priorityArg(word string) (byte, error) {
	switch word {
	case "A", "B", "C", "D":
		return word[0], nil
	case "none":
		return 0, nil
	}
	return 0, UsageError("not a priority: " + word + "; give A, B, C, D or none")
}

func priorityLabel(p byte) string {
	if p == 0 {
		return "none"
	}
	return "(" + string(p) + ")"
}

// resolve is the index of the one open line of todo.txt that holds the
// pattern, a literal substring compared without case; a closed `x`/`z`
// line that also holds it is neither offered nor touched. No match, or
// several (each listed), is the refusal.
func resolve(lines []string, pattern string) (int, error) {
	var matches []int
	for i, l := range lines {
		if todo.Parse(l).State == todo.Open && todo.ContainsFold(l, pattern) {
			matches = append(matches, i)
		}
	}
	switch {
	case len(matches) == 0:
		return 0, fmt.Errorf("No task found matching: %s", pattern)
	case len(matches) > 1:
		var msg strings.Builder
		fmt.Fprintf(&msg, "Multiple tasks match \"%s\":", pattern)
		for n, i := range matches {
			fmt.Fprintf(&msg, "\n  %d) %s", n+1, lines[i])
		}
		return 0, errors.New(msg.String())
	}
	return matches[0], nil
}

// todayForms are the texts today.md holds an open todo.txt line under,
// after `- [ ] `: as new-day carried it (an undated (A) whole), then as
// the description, as an agent writes a task in.
func todayForms(line string) []string {
	text := todo.Parse(line).Text
	if e, carried := journal.Entry(line); carried && e != text {
		return []string{e, text}
	}
	return []string{text}
}

// carried says what the next new day does with a line Daisy did not put
// in today.md.
func carried(line string) string {
	if s, _, ok := journal.Place(line); ok {
		return "the next new day carries it to " + string(s)
	}
	return "the next new day does not carry it"
}

func readTodo(c *Context) ([]string, error) {
	b, err := c.Ops.FS().ReadFile(todoFile(c))
	if err != nil {
		return nil, fmt.Errorf("Error: cannot read %s: %v", todoFile(c), err)
	}
	return todo.Lines(string(b)), nil
}

// todayEdit is today.md as read, and as a command means to write it.
type todayEdit struct {
	content        string
	found, changed bool
}

// readToday reads today.md; its absence is no error.
func readToday(c *Context) (todayEdit, error) {
	b, err := c.Ops.FS().ReadFile(todayFile(c))
	if errors.Is(err, fs.ErrNotExist) {
		return todayEdit{}, nil
	} else if err != nil {
		return todayEdit{}, fmt.Errorf("Error: cannot read %s: %v", todayFile(c), err)
	}
	return todayEdit{content: string(b), found: true}, nil
}

// add writes the line's task into its section of today.md, where a new
// day would carry it, and says so after verb; when today.md is absent or
// has no such section, it warns that only todo.txt changes, and says
// today.md is unchanged.
func (d *todayEdit) add(c *Context, line, verb string) string {
	s, text, _ := journal.Place(line)
	if !d.found {
		fmt.Fprintln(c.Stderr, "⚠️  today.md not found; only todo.txt changed")
		d.changed = false
		return "unchanged"
	}
	out, ok := journal.Add(d.content, s, text)
	if !ok {
		fmt.Fprintf(c.Stderr, "⚠️  today.md has no **%s:** section; only todo.txt changed\n", s)
		d.changed = false
		return "unchanged"
	}
	d.content, d.changed = out, true
	return verb + string(s)
}

// writeTask writes todo.txt and, when changed, today.md, both decided
// before the first is written.
func writeTask(c *Context, lines []string, d todayEdit) error {
	if err := ops.Rewrite(c.Ops.FS(), todoFile(c), []byte(todo.Content(lines)), 0o644); err != nil {
		return err
	}
	if d.changed {
		return ops.Rewrite(c.Ops.FS(), todayFile(c), []byte(d.content), 0o644)
	}
	return nil
}

// commitTask commits the home once and prints the summary, format's last
// verb the commit's hash.
func commitTask(c *Context, message, format string, args ...any) error {
	hash, committed, err := commitHome(c, message)
	if err != nil {
		return fmt.Errorf("Error: todo.txt and today.md are written, but the commit failed\n%v", err)
	}
	if !committed {
		hash = "not committed"
	}
	fmt.Fprintf(c.Stdout, format, append(args, hash)...)
	return nil
}
