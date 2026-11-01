// Package gitx is git over Ops.Exec: which repository a home's data commits
// to, and the one commit a mutation makes.
package gitx

import (
	"fmt"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Repo is where a home commits: the home directory, and whether the commit
// must be scoped to it because the repository is larger than the home (a
// home still inside the root's repository), where a bare commit would sweep
// in whatever else is staged.
type Repo struct {
	Dir    string
	Scoped bool
}

// Toplevel is the top of the repository holding dir, "" when there is none.
func Toplevel(o ops.Ops, dir string) string {
	out, _, err := o.Exec(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// HomeRepo is the repository a home's data commits to: the home's own, or
// the root's while the home lies inside it; ok is false when neither holds.
func HomeRepo(o ops.Ops, root, home string) (Repo, bool) {
	fs := o.FS()
	top := Toplevel(o, home)
	if top == "" {
		return Repo{}, false
	}
	if top == env.RealDir(fs, home) {
		return Repo{Dir: home}, true
	}
	if rootTop := Toplevel(o, root); rootTop != "" && rootTop == env.RealDir(fs, root) && top == rootTop {
		return Repo{Dir: home, Scoped: true}, true
	}
	return Repo{}, false
}

// Commit stages everything under the repo's directory and commits it once.
// committed is false, with no error, when there was nothing to commit; hash
// is the new commit's short hash.
func (r Repo) Commit(o ops.Ops, message string) (hash string, committed bool, err error) {
	git := func(args ...string) (string, error) {
		out, errb, err := o.Exec(r.Dir, "git", args...)
		if err != nil {
			return "", fmt.Errorf("git %s in %s: %v\n%s", args[0], r.Dir, err, strings.TrimSpace(string(errb)))
		}
		return string(out), nil
	}
	// `status --porcelain` rather than `diff`: a new, untracked file counts.
	status, err := git("status", "--porcelain", "--", ".")
	if err != nil {
		return "", false, err
	}
	if status == "" {
		return "", false, nil
	}
	if _, err := git("add", "--", "."); err != nil {
		return "", false, fmt.Errorf("Error: Failed to stage changes: %v", err)
	}
	args := []string{"commit", "-m", message}
	if r.Scoped {
		args = append(args, "--", ".")
	}
	if _, err := git(args...); err != nil {
		return "", false, fmt.Errorf("Error: Failed to commit changes: %v", err)
	}
	out, err := git("rev-parse", "--short", "HEAD")
	if err != nil {
		return "", true, err
	}
	return strings.TrimRight(out, "\n"), true, nil
}
