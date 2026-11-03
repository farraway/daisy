package feedback

import (
	"path/filepath"
	"sort"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/prompt"
	"daisy/internal/skill"
)

// Target is a file a workflow tag names: a prompt (the home's, else the
// root's) or a skill's source SKILL.md, with its qualified label
// (`prompts/<name>`, `skills/<name>`), and the path across the root and
// the home (Src: whether the home overrides the root's, and how).
type Target struct {
	Label string
	Path  string
	Skill bool
	Src   prompt.Source
}

// daisyOwns are the workflows prompts/daisy owns, having no file of their
// own.
var daisyOwns = []string{"new-day", "new-week", "done", "log", "add", "priority", "cancel", "status", "sync", "project", "projects", "home"}

// Resolve is the file tag names. `prompts/<name>` and `skills/<name>` say
// which layer; a bare name tries a prompt, then a skill — so an alias never
// shadows a real file (`git` is the skill, `github` the prompt) — then the
// workflows daisy.md owns. A name that is empty or holds a slash names
// nothing: a target is a prompt or a skill, not a path.
func Resolve(fs ops.FS, root, home, tag string) (Target, bool) {
	if name, ok := strings.CutPrefix(tag, "prompts/"); ok {
		return promptTarget(fs, root, home, name)
	}
	if name, ok := strings.CutPrefix(tag, "skills/"); ok {
		return skillTarget(fs, root, home, name)
	}
	if t, ok := promptTarget(fs, root, home, tag); ok {
		return t, true
	}
	if t, ok := skillTarget(fs, root, home, tag); ok {
		return t, true
	}
	for _, w := range daisyOwns {
		if tag == w {
			return promptTarget(fs, root, home, "daisy") // the home's daisy.md shadows the root's, as for `daisy`
		}
	}
	return Target{}, false
}

func nameOK(name string) bool { return name != "" && !strings.ContainsRune(name, '/') }

func promptTarget(fs ops.FS, root, home, name string) (Target, bool) {
	if !nameOK(name) {
		return Target{}, false
	}
	path, ok := prompt.Lookup(fs, home, root, name)
	return Target{Label: "prompts/" + name, Path: path, Src: prompt.Locate(fs, home, root, "prompts/"+name+".md")}, ok
}

func skillTarget(fs ops.FS, root, home, name string) (Target, bool) {
	if !nameOK(name) {
		return Target{}, false
	}
	path, ok := skill.Find(fs, skill.SourceDirs(root, home), name)
	return Target{Label: "skills/" + name, Path: path, Skill: true, Src: prompt.Locate(fs, home, root, "skills/"+name+"/SKILL.md")}, ok
}

// Labels is every target a tag can name, qualified, in byte order: the
// root's and the home's prompts/*.md, and every skill source.
func Labels(fs ops.FS, root, home string) []string {
	seen := map[string]bool{}
	for _, dir := range []string{filepath.Join(root, "prompts"), filepath.Join(home, "prompts")} {
		entries, _ := fs.ReadDir(dir) // no directory, no prompts
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".md")
			if !ok || strings.HasPrefix(name, ".") {
				continue
			}
			if fi, err := fs.Stat(filepath.Join(dir, e.Name())); err == nil && fi.Mode().IsRegular() {
				seen["prompts/"+name] = true
			}
		}
	}
	for _, s := range skill.All(fs, skill.SourceDirs(root, home)) {
		seen["skills/"+s.Name] = true
	}
	var labels []string
	for l := range seen {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	return labels
}

// Vendored: the target is a skill carrying LICENSE or NOTICE.md, in the
// root's copy or the home's, which tracks an upstream source. Rewriting or
// overriding Daisy's copy from local feedback would fork it; such a skill
// is briefed, never written.
func Vendored(fs ops.FS, t Target) bool {
	if !t.Skill {
		return false
	}
	for _, dir := range []string{filepath.Dir(t.Src.Root), filepath.Dir(t.Src.Home)} {
		for _, f := range []string{"LICENSE", "NOTICE.md"} {
			if fi, err := fs.Stat(filepath.Join(dir, f)); err == nil && fi.Mode().IsRegular() {
				return true
			}
		}
	}
	return false
}
