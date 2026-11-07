// Package brief finds the briefs under a swarm's resources (docs/swarm.md
// section 4, The brief): Markdown files whose header, the lines before the
// first `## ` heading, holds a `Status:` line.
package brief

import (
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
)

// Brief is one brief: its path relative to the workspace, as the batch
// prompt and a proposal's `Brief:` line name it, its Status and its text.
type Brief struct{ Path, Status, Text string }

// Find is every brief under the resources of the workspace ws, in path
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
			if status, ok := statusOf(string(data)); ok {
				out = append(out, Brief{rel, status, string(data)})
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

// statusOf is the value of the header's `Status:` line.
func statusOf(text string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			break
		}
		if v, ok := strings.CutPrefix(line, "Status:"); ok {
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
	fence := ""
	for _, line := range strings.Split(b.Text, "\n") {
		line = strings.TrimRight(line, "\r ")
		if f := fenceOf(line); f != "" && (fence == "" || strings.HasPrefix(f, fence)) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if h, ok := strings.CutPrefix(line, "## "); ok {
			if h = strings.TrimSpace(h); h != "" && !slices.Contains(out, h) {
				out = append(out, h)
			}
		}
	}
	return out
}

// fenceOf is the run of backticks or tildes that opens or closes a fenced
// block on line, "" when line is no fence.
func fenceOf(line string) string {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return ""
	}
	for _, c := range []string{"`", "~"} {
		if n := len(t) - len(strings.TrimLeft(t, c)); n >= 3 {
			return strings.Repeat(c, n)
		}
	}
	return ""
}

// Slug is the brief's file name without `.md`.
func (b Brief) Slug() string { return strings.TrimSuffix(filepath.Base(b.Path), ".md") }

// MilestoneTitle is the title of the forge milestone that mirrors the
// brief's milestone m (section 10): `<slug>: <m>`.
func (b Brief) MilestoneTitle(m string) string { return b.Slug() + ": " + m }
