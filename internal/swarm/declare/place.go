package declare

import (
	"fmt"
	"path/filepath"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Place is where a swarm was found (docs/swarm.md section 11, Finding
// the swarm): its root, `<swarm>`, and the folder holding swarm.json and
// prompts/. In revision 6 both are the swarm repository's clone; in the
// old layout (section 11, Moving a swarm into its repository) the root is
// the Daisy workspace and the folder its swarm/.
type Place struct {
	Root string // the unit's %f; where docker and id run; the base of repos[].path and resources until Steps 3 and 4 of revision 6's plan
	Dir  string // swarm.json and prompts/<role>.md
}

// Clone is the place of a swarm repository's clone at dir.
func Clone(dir string) Place { return Place{Root: dir, Dir: dir} }

// Workspace is the old layout's place in the workspace ws.
func Workspace(ws string) Place { return Place{Root: ws, Dir: filepath.Join(ws, "swarm")} }

// Old: the swarm is in the old layout, declared in a Daisy workspace.
func (p Place) Old() bool { return p.Dir != p.Root }

// Rel is a file of the configuration folder as the verbs name it:
// relative to the root, `swarm.json` in a clone, `swarm/swarm.json` in
// the old layout.
func (p Place) Rel(name string) string {
	if p.Old() {
		return filepath.Join(filepath.Base(p.Dir), name)
	}
	return name
}

// Find is the one function that decides the layout (docs/swarm.md
// section 11, Finding the swarm): the nearest folder from cwd upward that
// holds both swarm.json and .git (a directory, or a worktree's file) is a
// swarm repository's clone; with none, the old layout's
// <ws>/swarm/swarm.json in the workspace cwd is in. init, which writes
// the old layout's file, takes the workspace without it, and outside a
// workspace gets the workspace lookup's own refusal; every other verb
// finding neither gets section 3's not-found message. root is the Daisy
// root, which is never a workspace.
func Find(fs ops.FS, root, cwd string, init bool) (Place, error) {
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if isClone(fs, dir) {
			return Clone(dir), nil
		}
		if dir == filepath.Dir(dir) || dir == "." {
			break
		}
	}
	ws, err := env.FindWorkspace(fs, root, cwd)
	if err == nil {
		p := Workspace(ws)
		if _, serr := fs.Lstat(File(p.Dir)); serr == nil || init {
			return p, nil
		}
	} else if init {
		return Place{}, err
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
