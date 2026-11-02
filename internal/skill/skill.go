// Package skill reads skills: SKILL.md files in skill directories, the
// sources (the root's skills/ then the home's) and the installed copies
// (~/.claude/skills then the workspace's .claude/skills). Directories are
// always given lowest precedence first: a later one shadows an earlier one
// by name.
package skill

import (
	"path/filepath"
	"sort"
	"strings"

	"daisy/internal/ops"
)

// SourceDirs are where Daisy keeps skills: the root's, then the home's.
func SourceDirs(root, home string) []string {
	return []string{filepath.Join(root, "skills"), filepath.Join(home, "skills")}
}

// InstalledDirs are where agents load skills from: the user's (from
// `daisy install`), then the workspace's (from `daisy init`). userHome is
// $HOME, which may be empty: the shell made that `/.claude/skills`.
func InstalledDirs(userHome, workspace string) []string {
	return []string{filepath.Join("/"+userHome, ".claude", "skills"), filepath.Join(workspace, ".claude", "skills")}
}

// Find is the SKILL.md of the named skill in the last dir that has one.
func Find(fs ops.FS, dirs []string, name string) (path string, ok bool) {
	for _, d := range dirs {
		if p := filepath.Join(d, name, "SKILL.md"); isRegular(fs, p) {
			path, ok = p, true
		}
	}
	return path, ok
}

// Skill is a skill directory's name and its SKILL.md.
type Skill struct{ Name, Path string }

// All is every skill in dirs, a later dir's overriding an earlier one's of
// the same name, sorted by name (byte order). A skill is a directory, not
// hidden, holding a SKILL.md.
func All(fs ops.FS, dirs []string) []Skill {
	byName := map[string]string{}
	for _, d := range dirs {
		entries, _ := fs.ReadDir(d) // no directory, no skills
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if fi, err := fs.Stat(filepath.Join(d, e.Name())); err != nil || !fi.IsDir() {
				continue
			}
			if p := filepath.Join(d, e.Name(), "SKILL.md"); isRegular(fs, p) {
				byName[e.Name()] = p
			}
		}
	}
	var all []Skill
	for n, p := range byName {
		all = append(all, Skill{Name: n, Path: p})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all
}

// Field is a frontmatter value: the first line starting `<key>:`, less the
// key, the colon and one space; empty when no space follows the colon.
func Field(doc []byte, key string) (string, bool) {
	for _, line := range lines(doc) {
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			v, spaced := strings.CutPrefix(v, " ")
			if !spaced {
				v = ""
			}
			return v, true
		}
	}
	return "", false
}

// Rules is the `## Rules` section, heading included, up to the next `## `
// heading; nil when there is none.
func Rules(doc []byte) []string {
	var out []string
	found := false
	for _, line := range lines(doc) {
		switch {
		case strings.HasPrefix(line, "## Rules"):
			found = true
		case found && strings.HasPrefix(line, "## "):
			return out
		}
		if found {
			out = append(out, line)
		}
	}
	return out
}

// Sections are the titles of the `## ` headings other than Rules, in order.
func Sections(doc []byte) []string {
	var out []string
	for _, line := range lines(doc) {
		if t, ok := strings.CutPrefix(line, "## "); ok && t != "Rules" {
			out = append(out, t)
		}
	}
	return out
}

func isRegular(fs ops.FS, p string) bool {
	fi, err := fs.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// lines splits at newlines, as awk reads records.
func lines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}
