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

	"daisy/internal/ops"
	"daisy/internal/swarm/fence"
)

// Brief is one brief: its path, as the batch prompt and a proposal's
// `Brief:` line name it (relative to the workspace in the old layout; in
// a swarm repository's clone its path in the swarm repository, or
// `<name>:<path>` in a declared one), its Status, its text, and where it
// is mounted under /work/.resources (its path in the old layout,
// `<repository>/<path>` in a clone).
type Brief struct{ Path, Status, Text, Mount string }

// Read is the brief at path whose text is data, mounted at mount; ok is
// false when its header has no Status: line, a resource like any other.
func Read(path, mount string, data []byte) (b Brief, ok bool) {
	status, ok := statusOf(string(data))
	return Brief{path, status, string(data), mount}, ok
}

// Find is every brief under the resources of the old layout's workspace ws, in path
// order. A resource is read as it is mounted, resolved through its links;
// a link inside it is not followed, as it resolves to nothing in a
// container.
func Find(fs ops.FS, ws string, resources []string) ([]Brief, error) {
	var out []Brief
	var walk func(real, rel string, dir bool) error
	walk = func(real, rel string, dir bool) error {
		if !dir {
			if !strings.HasSuffix(rel, ".md") || slices.ContainsFunc(out, func(b Brief) bool { return b.Path == rel }) {
				return nil
			}
			data, err := fs.ReadFile(real)
			if err != nil {
				return err
			}
			if b, ok := Read(rel, rel, data); ok {
				out = append(out, b)
			}
			return nil
		}
		entries, err := fs.ReadDir(real)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || e.Type().IsRegular() {
				if err := walk(filepath.Join(real, e.Name()), filepath.Join(rel, e.Name()), e.IsDir()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, r := range resources {
		real, err := fs.EvalSymlinks(filepath.Join(ws, r))
		if err != nil {
			return nil, err
		}
		fi, err := fs.Stat(real)
		if err != nil {
			return nil, err
		}
		if err := walk(real, filepath.Clean(r), fi.IsDir()); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(out, func(a, b Brief) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
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
