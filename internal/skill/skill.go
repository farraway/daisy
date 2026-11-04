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

// ClaudeCodeSynced is the directory in ~/.claude/skills where Claude Code's
// own skill sync writes, recreating it minutes after it is removed. It is
// not Daisy's: install never prunes it and the health check never calls it
// stale.
const ClaudeCodeSynced = "synced"

// WorkspaceTrees are where init copies each of the home's skills in a
// workspace, one per agent reading the Agent Skills layout: Claude Code's,
// then Codex's. They are twins: init, clean and the health check go
// through this list, never through one tree alone.
var WorkspaceTrees = []string{".claude/skills", AgentsTree}

// AgentsTree is Codex's project skill directory; init also writes there
// AgentsDaisy, the skill that is Codex's way into Daisy (as the /daisy
// command is Claude Code's). No home skill takes the name: it is a core
// Cursor rule's.
const (
	AgentsTree  = ".agents/skills"
	AgentsDaisy = "daisy"
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

// Discover is every skill under dir by name: each directory, at any depth,
// holding a SKILL.md, named by that directory (a later one in walk order
// wins a name). What `daisy init` and `daisy install` copy.
func Discover(fs ops.FS, dir string) map[string]string {
	skills := map[string]string{}
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return skills
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			for n, d := range Discover(fs, p) {
				skills[n] = d
			}
		} else if e.Name() == "SKILL.md" {
			skills[filepath.Base(dir)] = dir
		}
	}
	return skills
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
