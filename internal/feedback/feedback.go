// Package feedback is the deterministic half of Daisy's learning loop: the
// entries of a home's feedback/feedback.md, the prompt or skill each one's
// **Workflow:** tag names, the briefing `daisy optimize` gives the agent
// (pending entries per target, the target's rules, the rewrite flags), and
// the archive an applied proposal moves its entries to. The agent in
// session is the model; nothing here proposes text.
package feedback

import (
	"regexp"
	"strings"
)

// Entry is one feedback.md entry: its lines from the `## YYYY-MM-DD HHMM`
// heading up to the next heading, trailing newlines dropped, and the value
// of its first `**Workflow:**` line ("" when it has none).
type Entry struct {
	Text     string
	Workflow string
}

// Tag is the workflow the entry names: its header's, else daisy, the
// prompt that owns the unlabelled workflows.
func (e Entry) Tag() string {
	if e.Workflow == "" {
		return "daisy"
	}
	return e.Workflow
}

// Log is a parsed feedback.md: what comes before the first entry (trailing
// newlines dropped), and the entries in file order.
type Log struct {
	Header  string
	Entries []Entry
}

var (
	entryHeading = regexp.MustCompile(`^## [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{4}`)
	workflowLine = regexp.MustCompile(`^\*\*Workflow:\*\*\s*(.*?)\s*$`)
)

// Parse reads feedback.md as the bash loop did.
func Parse(doc []byte) Log {
	var l Log
	var header []string
	var cur []string
	flush := func() {
		if cur != nil {
			l.Entries = append(l.Entries, newEntry(cur))
		}
	}
	for _, line := range strings.Split(string(doc), "\n") {
		switch {
		case entryHeading.MatchString(line):
			flush()
			cur = []string{line}
		case cur != nil:
			cur = append(cur, line)
		default:
			header = append(header, line)
		}
	}
	flush()
	l.Header = strings.TrimRight(strings.Join(header, "\n"), "\n")
	return l
}

func newEntry(lines []string) Entry {
	e := Entry{Text: strings.TrimRight(strings.Join(lines, "\n"), "\n")}
	for _, line := range lines[1:] {
		if m := workflowLine.FindStringSubmatch(line); m != nil {
			e.Workflow = m[1]
			break
		}
	}
	return e
}

// Keep is feedback.md holding the header and the entries keep says to,
// each one blank line after what precedes it; with no header the first
// entry starts the file.
func (l Log) Keep(keep func(i int) bool) []byte {
	var b strings.Builder
	if l.Header != "" {
		b.WriteString(l.Header + "\n")
	}
	for i, e := range l.Entries {
		if keep(i) {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(e.Text + "\n")
		}
	}
	return []byte(b.String())
}

// Archive is archive.md with each entry appended, followed one blank line
// down by the provenance line naming where it went.
func Archive(archive []byte, entries []Entry, provenance string) []byte {
	var b strings.Builder
	b.Write(archive)
	if len(archive) > 0 && !strings.HasSuffix(string(archive), "\n") {
		b.WriteString("\n")
	}
	for _, e := range entries {
		b.WriteString("\n" + e.Text + "\n\n" + provenance + "\n")
	}
	return []byte(b.String())
}

// Record is feedback.md with a new entry appended: a blank line, the
// heading for stamp (`YYYY-MM-DD HHMM`), the workflow line when there is
// one, a blank line and the message.
func Record(doc []byte, stamp, workflow, message string) []byte {
	var b strings.Builder
	b.Write(doc)
	b.WriteString("\n## " + stamp + "\n")
	if workflow != "" {
		b.WriteString("**Workflow:** " + workflow + "\n")
	}
	b.WriteString("\n" + message + "\n")
	return []byte(b.String())
}
