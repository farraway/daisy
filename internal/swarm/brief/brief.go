// Package brief finds the briefs under a swarm's resources (docs/swarm.md
// section 4, The brief): Markdown files whose header, the lines before the
// first `## ` heading outside a fenced block, holds a `Status:` line. Field
// reads such a header's lines, a research note's among them (section 7.3).
package brief

import (
	"iter"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/swarm/fence"
)

// Brief is one brief: its path, as the batch prompt and a proposal's
// `Brief:` line name it (its path in the swarm repository, or
// `<name>:<path>` in a declared one), its Status, its text, and where it
// is mounted under /work/.resources (`<repository>/<path>`).
type Brief struct{ Path, Status, Text, Mount string }

// Read is the brief at path whose text is data, mounted at mount; ok is
// false when its header has no Status: line, a resource like any other.
func Read(path, mount string, data []byte) (b Brief, ok bool) {
	status, ok := statusOf(string(data))
	return Brief{path, status, string(data), mount}, ok
}

// statusOf is the value of the header's `Status:` line (section 4).
func statusOf(text string) (string, bool) {
	return Field(text, "Status")
}

// Field is the value of the header's first `<name>:` line, trimmed: the
// header is the lines before the first `## ` heading, and a line inside a
// fenced block is neither (section 4; a research note's header is read by
// the same rule, section 7.3).
func Field(text, name string) (string, bool) {
	for line := range unfenced(text) {
		if isHeading(line) {
			break
		}
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), name+":"); ok {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// Milestones is the text of each of the brief's `## ` headings, in order,
// once each: one milestone each (section 4). A `## ` line inside a fenced
// block is no heading, and a heading with no text no milestone.
func (b Brief) Milestones() []string {
	var out []string
	for line := range unfenced(b.Text) {
		if isHeading(line) {
			if h := strings.TrimSpace(strings.TrimPrefix(line, "## ")); !slices.Contains(out, h) {
				out = append(out, h)
			}
		}
	}
	return out
}

// isHeading: line is a `## ` heading with text, at the left margin, the
// one rule for where the header ends and what is a milestone (section 4:
// a heading without text is none).
func isHeading(line string) bool {
	h, ok := strings.CutPrefix(line, "## ")
	return ok && strings.TrimSpace(h) != ""
}

// unfenced yields the lines of text outside its fenced blocks, the fence
// lines themselves left out.
func unfenced(text string) iter.Seq[string] {
	return func(yield func(string) bool) {
		var fences fence.Scan
		for _, line := range strings.Split(text, "\n") {
			if !fences.Line(line) && !yield(line) {
				return
			}
		}
	}
}

// Slug is the brief's file name without `.md`.
func (b Brief) Slug() string { return strings.TrimSuffix(filepath.Base(b.Path), ".md") }

// MilestoneTitle is the title of the forge milestone that mirrors the
// brief's milestone m (section 10): `<slug>: <m>`.
func (b Brief) MilestoneTitle(m string) string { return b.Slug() + ": " + m }
