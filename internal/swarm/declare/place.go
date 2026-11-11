package declare

import (
	"fmt"
	"path/filepath"

	"daisy/internal/ops"
)

// Place is where a swarm was found (docs/swarm.md section 11, Finding
// the swarm): its root, `<swarm>`, the swarm repository's clone holding
// swarm.json and prompts/. For `init` run where none is found, the
// working folder, Outside set: init makes the clone in it (section 11,
// init).
type Place struct {
	Root    string // the clone: swarm.json and prompts/<role>.md; the unit's %f, where docker and id run
	Outside bool   // Root is the folder init makes the clone in, <Root>/<repo>, not the clone
}

// Clone is the place of a swarm repository's clone at dir.
func Clone(dir string) Place { return Place{Root: dir} }

// Folder is init's place in dir, a folder holding no swarm repository's
// clone, where it makes one.
func Folder(dir string) Place { return Place{Root: dir, Outside: true} }

// Find is the one function that finds the swarm (docs/swarm.md section
// 11, Finding the swarm): the nearest folder from cwd upward that holds
// both swarm.json and .git (a directory, or a worktree's file) is a
// swarm repository's clone. init finding none takes cwd, the folder it
// makes the clone in (Folder); every other verb gets section 3's
// not-found message.
func Find(fs ops.FS, cwd string, init bool) (Place, error) {
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if isClone(fs, dir) {
			return Clone(dir), nil
		}
		if dir == filepath.Dir(dir) || dir == "." {
			break
		}
	}
	if init {
		return Folder(cwd), nil
	}
	return Place{}, &Error{"", fmt.Sprintf("not found in %s or a folder above it; cd into the swarm repository's clone, or run 'daisy swarm init'", cwd)}
}

// isClone: dir holds swarm.json and .git, the top of a clone.
func isClone(fs ops.FS, dir string) bool {
	if _, err := fs.Lstat(filepath.Join(dir, "swarm.json")); err != nil {
		return false
	}
	fi, err := fs.Stat(filepath.Join(dir, ".git"))
	return err == nil && (fi.IsDir() || fi.Mode().IsRegular())
}
