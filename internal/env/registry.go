package env

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"strings"

	"daisy/internal/ops"
)

const fsModeSymlink = iofs.ModeSymlink

// HomesFile is the registry: one `name=/abs/path` per line.
func (e Env) HomesFile() string { return filepath.Join(e.Config, "homes") }

// Home is one registry line: a name and the path it is registered at.
type Home struct{ Name, Path string }

// Homes is every line of the registry in order, names and paths trimmed of
// surrounding whitespace and paths of trailing slashes; comments, blank
// lines and lines without a path are skipped. A name may appear twice: the
// first line is the one that counts.
func Homes(fs ops.FS, homesFile string) []Home {
	b, err := fs.ReadFile(homesFile)
	if err != nil {
		return nil
	}
	var homes []Home
	for _, line := range strings.Split(string(b), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if key == "" || strings.HasPrefix(key, "#") || value == "" {
			continue
		}
		homes = append(homes, Home{Name: key, Path: value})
	}
	return homes
}

// HomePath is the registered path of a home; ok is false for an
// unregistered name.
func HomePath(fs ops.FS, homesFile, name string) (path string, ok bool) {
	for _, h := range Homes(fs, homesFile) {
		if h.Name == name {
			return h.Path, true
		}
	}
	return "", false
}

// UsableHomes are the registered homes that can be used as homes, each
// name once (the line HomePath reads), in registry order; Problems are the
// others, each with what is wrong with it.
func UsableHomes(fs ops.FS, root, homesFile string) (usable []Home, problems []HomeProblem) {
	seen := map[string]bool{}
	for _, h := range Homes(fs, homesFile) {
		if seen[h.Name] {
			continue
		}
		seen[h.Name] = true
		if p := HomePathProblem(fs, root, filepath.Clean(h.Path)); p != "" {
			problems = append(problems, HomeProblem{h, p})
			continue
		}
		usable = append(usable, Home{Name: h.Name, Path: filepath.Clean(h.Path)})
	}
	return usable, problems
}

// HomeProblem is a registered home that cannot be used, and why.
type HomeProblem struct {
	Home
	Problem string
}

// Register appends `name=path` to the registry on a line of its own,
// creating the file. Callers validate the name and the path.
func Register(fs ops.FS, homesFile, name, path string) error {
	b, err := fs.ReadFile(homesFile)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return fmt.Errorf("Error: cannot read %s: %v", homesFile, err)
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	b = append(b, name+"="+path+"\n"...)
	if err := fs.MkdirAll(filepath.Dir(homesFile), 0o755); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	if err := ops.Rewrite(fs, homesFile, b, 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	return nil
}

// Unregister drops every line whose name is name; the other lines are kept
// byte for byte.
func Unregister(fs ops.FS, homesFile, name string) error {
	b, err := fs.ReadFile(homesFile)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", homesFile, err)
	}
	lines := strings.SplitAfter(string(b), "\n")
	var kept strings.Builder
	for _, line := range lines {
		key, _, _ := strings.Cut(line, "=")
		if strings.TrimSpace(key) == name {
			continue
		}
		kept.WriteString(line)
	}
	if err := ops.Rewrite(fs, homesFile, []byte(kept.String()), 0o644); err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	return nil
}

// HomePathProblem says why a path cannot be a home, or "" when it can: it
// must be absolute, present, and neither the root nor a directory holding
// the root, where a home commit would commit the root.
func HomePathProblem(fs ops.FS, root, path string) string {
	if !filepath.IsAbs(path) {
		return "is not an absolute path"
	}
	real := RealDir(fs, path)
	if real == "" {
		return "does not exist"
	}
	rootReal := RealDir(fs, root)
	if real == rootReal {
		return "is the Daisy root; the root cannot be a home"
	}
	if strings.HasPrefix(rootReal, strings.TrimSuffix(real, "/")+"/") {
		return "contains the Daisy root; the root cannot be inside a home"
	}
	return ""
}

// HomeDir is the directory of e.HomeName when it is registered and can be
// a home; otherwise the error names the fix.
func HomeDir(o ops.Ops, e Env) (string, error) {
	fs := o.FS()
	path, ok := HomePath(fs, e.HomesFile(), e.HomeName)
	if !ok {
		return "", fmt.Errorf("Error: Home '%s' is not registered in %s\n  Run 'daisy home list' to see registered homes, or 'daisy home add %s /path/to/home'", e.HomeName, e.HomesFile(), e.HomeName)
	}
	// Checked as it will be used (cleaned: `..` lexical, as `cd` takes it),
	// reported as written.
	if problem := HomePathProblem(fs, e.Root, filepath.Clean(path)); problem != "" {
		return "", fmt.Errorf("Error: Home '%s' is registered at %s, which %s\n  Run 'daisy home remove %s', then 'daisy home add %s /path/to/home' with its location", e.HomeName, path, problem, e.HomeName, e.HomeName)
	}
	return path, nil
}

// RealDir is the resolved path of an existing directory, "" when there is
// none: the one spelling of a directory for every comparison.
func RealDir(fs ops.FS, p string) string {
	if p == "" {
		return ""
	}
	real, err := fs.EvalSymlinks(p)
	if err != nil {
		return ""
	}
	if fi, err := fs.Stat(real); err != nil || !fi.IsDir() {
		return ""
	}
	return real
}

// RealPath is `readlink -f`: every link resolved, the components that
// exist made real, a missing tail appended as written. A dangling link is
// followed to where it points.
func RealPath(fs ops.FS, p string) string { return realPath(fs, filepath.Clean(p), 0) }

func realPath(fs ops.FS, p string, depth int) string {
	if real, err := fs.EvalSymlinks(p); err == nil {
		return real
	}
	dir, base := filepath.Split(p)
	if dir == "" || base == "" || depth > 40 {
		return p
	}
	full := p
	if dir != "/" {
		full = filepath.Join(realPath(fs, filepath.Clean(dir), depth+1), base)
	}
	if fi, err := fs.Lstat(full); err == nil && fi.Mode()&fsModeSymlink != 0 {
		if target, err := fs.Readlink(full); err == nil {
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(full), target)
			}
			return realPath(fs, filepath.Clean(target), depth+1)
		}
	}
	return full
}

// RootExclusions are the usable registered homes whose directory lies
// inside the root, root-relative, in registry order: what a root commit
// leaves out and the publication scan does not read — one list, so the
// two cannot disagree. Compared resolved, so a symlinked root or home
// still matches.
func RootExclusions(fs ops.FS, e Env) []string {
	root := RealDir(fs, e.Root)
	if root == "" {
		return nil
	}
	usable, _ := UsableHomes(fs, e.Root, e.HomesFile())
	var rels []string
	for _, h := range usable {
		if rel, ok := strings.CutPrefix(RealDir(fs, h.Path), root+"/"); ok {
			rels = append(rels, rel)
		}
	}
	return rels
}
