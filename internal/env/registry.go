package env

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"daisy/internal/ops"
)

const fsModeSymlink = fs.ModeSymlink

// HomesFile is the registry: one `name=/abs/path` per line.
func (e Env) HomesFile() string { return filepath.Join(e.Config, "homes") }

// HomePath is the registered path of a home, trimmed of surrounding
// whitespace and trailing slashes; ok is false for an unregistered name.
// Comment and blank lines are skipped, the first match wins.
func HomePath(fs ops.FS, homesFile, name string) (path string, ok bool) {
	b, err := fs.ReadFile(homesFile)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.HasPrefix(key, "#") || key != name {
			continue
		}
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			continue
		}
		return value, true
	}
	return "", false
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
