// Package todo is the todo.txt line model: a line is a task, and every line
// parses to a Task whose Format is the line again, byte for byte, whether
// or not it follows the format. Parse reads only what the format puts at
// the front (the x/z marker, the dates, the priority); the rest is Text.
package todo

import (
	"regexp"
	"strings"
)

// State is a task's completion marker.
type State int

const (
	Open      State = iota
	Done            // `x `
	Cancelled       // `z `, Daisy's addition
)

// Task is one line. Closed is the completion or cancellation date, Created
// the creation date, both as written (YYYY-MM-DD) or ""; Priority is the
// letter or 0; Text is the description, verbatim.
type Task struct {
	State    State
	Closed   string
	Priority byte
	Created  string
	Text     string
}

var date = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2} `)

// Parse reads a line. An open task is `(P) `, then a date, then text; a
// closed one is `x ` or `z `, the closing date, the creation date, text.
// Each part is optional and read only in that order, so anything out of
// place stays in Text.
func Parse(line string) Task {
	var t Task
	rest := line
	switch {
	case strings.HasPrefix(rest, "x "):
		t.State, rest = Done, rest[2:]
	case strings.HasPrefix(rest, "z "):
		t.State, rest = Cancelled, rest[2:]
	}
	if t.State != Open {
		t.Closed, rest = cutDate(rest)
	} else if len(rest) >= 4 && rest[0] == '(' && rest[1] >= 'A' && rest[1] <= 'Z' && rest[2] == ')' && rest[3] == ' ' {
		t.Priority, rest = rest[1], rest[4:]
	}
	t.Created, rest = cutDate(rest)
	t.Text = rest
	return t
}

func cutDate(s string) (string, string) {
	if m := date.FindString(s); m != "" {
		return m[:len(m)-1], s[len(m):]
	}
	return "", s
}

// Format is the line for the task.
func (t Task) Format() string {
	var b strings.Builder
	switch t.State {
	case Done:
		b.WriteString("x ")
	case Cancelled:
		b.WriteString("z ")
	}
	if t.Closed != "" {
		b.WriteString(t.Closed + " ")
	}
	if t.Priority != 0 {
		b.WriteString("(" + string(t.Priority) + ") ")
	}
	if t.Created != "" {
		b.WriteString(t.Created + " ")
	}
	b.WriteString(t.Text)
	return b.String()
}

// Complete is the task done on date: `x <date> <created> <text>`, the
// priority dropped, as the format has it.
func (t Task) Complete(date string) Task {
	return Task{State: Done, Closed: date, Created: t.Created, Text: t.Text}
}

// Projects are the `+tag` words of the text, in order.
func (t Task) Projects() []string {
	var out []string
	for _, w := range strings.Fields(t.Text) {
		if len(w) > 1 && w[0] == '+' {
			out = append(out, w[1:])
		}
	}
	return out
}

// HasProject: the task carries `+name`, compared without case.
func (t Task) HasProject(name string) bool {
	for _, p := range t.Projects() {
		if strings.EqualFold(p, name) {
			return true
		}
	}
	return false
}

// Lines splits a file into its lines, without the trailing newline's empty
// line; Content joins them back, every line newline-terminated.
func Lines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func Content(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// ContainsFold: pattern occurs in line as a literal, case-insensitive
// substring. The one matching rule for `done` and `tasks`.
func ContainsFold(line, pattern string) bool {
	return strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
}
