// Package fence follows a Markdown text's fenced code blocks line by
// line (CommonMark 0.31, 4.5): the one reading of a fence that the
// swarm's readers of plans, briefs, notes and prompts share
// (docs/swarm.md sections 4, 7.3 and 7.5).
package fence

import "strings"

// Scan is one pass over a text's lines, Line called on each in order.
type Scan struct{ open string }

// Line reads the next line and reports whether it is fenced: a line that
// opens or closes a fenced block, or one inside it. A block opens on a
// line indented at most three spaces that begins with three or more
// backticks or tildes; it closes on a line indented at most three spaces
// that holds a run of the same character at least as long and nothing
// after it but spaces, tabs or a `\r`. A fence line with an info string
// (```go) closes nothing.
func (s *Scan) Line(line string) bool {
	run, rest := marker(line)
	switch {
	case s.open == "":
		s.open = run
		return run != ""
	case run != "" && strings.HasPrefix(run, s.open) && strings.Trim(rest, " \t\r") == "":
		s.open = ""
	}
	return true
}

// Open is the run of the block the lines so far leave open, a line that
// closes it; "" when none is open.
func (s *Scan) Open() string { return s.open }

// marker is line's run of three or more backticks or tildes after at
// most three spaces, and the text after the run; "" when it has none.
func marker(line string) (run, rest string) {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return "", ""
	}
	for _, c := range []string{"`", "~"} {
		if n := len(t) - len(strings.TrimLeft(t, c)); n >= 3 {
			return t[:n], t[n:]
		}
	}
	return "", ""
}
