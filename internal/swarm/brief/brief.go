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

// Milestones is the text of each of the brief's `## ` headings, in order:
// one milestone each (section 4).
func (b Brief) Milestones() []string {
	var out []string
	for _, line := range strings.Split(b.Text, "\n") {
		if h, ok := strings.CutPrefix(strings.TrimRight(line, "\r "), "## "); ok {
			out = append(out, strings.TrimSpace(h))
		}
	}
	return out
}
