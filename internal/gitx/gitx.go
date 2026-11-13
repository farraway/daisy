// Package gitx is git over Ops.Exec: which repository a home's data commits
// to, the one commit a mutation makes, and which of Daisy's paths a
// repository's .gitignore already ignores.
package gitx

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Repo is where a commit lands: the repository's top directory, and the
// paths under it a commit leaves out (the homes inside the root, for a
// root commit). While there are any the commit takes the pathspec too, or
// a bare commit would sweep in whatever else is staged; otherwise it is
// bare, since a partial commit is refused during a merge.
type Repo struct {
	Dir     string
	Exclude []string
}

// Toplevel is the top of the repository holding dir, "" when there is none.
func Toplevel(o ops.Ops, dir string) string {
	out, _, err := o.Exec(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// IsRepoRoot: dir is the top level of its own repository, not merely inside
// one.
func IsRepoRoot(o ops.Ops, dir string) bool {
	top := Toplevel(o, dir)
	return top != "" && top == env.RealDir(o.FS(), dir)
}

// HomeRepo is the repository a home's data commits to: the home's own,
// and only that. A home inside the root's repository (the layout before
// the split), or one whose git directory is the root's (a `gitdir:` file
// or a linked worktree of the root), is refused: its every commit would
// land in the published root's history. The Refusal says which.
func HomeRepo(o ops.Ops, root, home string) (Repo, Refusal) {
	if !IsRepoRoot(o, home) {
		if fi, err := o.FS().Lstat(filepath.Join(home, ".git")); err == nil && !fi.IsDir() {
			return Repo{}, DanglingGitFile
		}
		return Repo{}, NotARepo
	}
	if hc := commonDir(o, home); hc == "" {
		return Repo{}, NotARepo
	} else if hc == commonDir(o, root) {
		return Repo{}, RootsGitDir
	}
	return Repo{Dir: home}, Accepted
}

// Refusal is why HomeRepo refused a home, Accepted when it did not.
type Refusal int

const (
	Accepted        Refusal = iota
	NotARepo                // not the top of a repository of its own
	RootsGitDir             // its .git points into the root's: a gitdir file, a link or a worktree of the root
	DanglingGitFile         // its .git is a file or link pointing at no repository (a pruned worktree)
)

// Why is what to add to "not in its own git repository": nothing, or that
// the git directory is the root's.
func (r Refusal) Why() string {
	switch r {
	case RootsGitDir:
		return " (its .git points into the root's)"
	case DanglingGitFile:
		return " (its .git points at no repository)"
	}
	return ""
}

// Remedy is the commands that make home its own repository, for a
// refusal: `git init` and a first commit — after removing a borrowed .git,
// a file that only points into the root's (`worktree prune` then drops a
// worktree's record), since `git init` there would only reinitialise the
// root's and the commit would land in the root. quote spells a path for the shell.
func (r Refusal) Remedy(root, home, name string, quote func(string) string) string {
	q := quote(home)
	fix := fmt.Sprintf("git -C %s init && git -C %s add -A && git -C %s commit -m 'New home: %s'", q, q, q, name)
	switch r {
	case RootsGitDir:
		return fmt.Sprintf("rm %s && git -C %s worktree prune && %s", quote(home+"/.git"), quote(root), fix)
	case DanglingGitFile:
		return fmt.Sprintf("rm %s && %s", quote(home+"/.git"), fix)
	}
	return fix
}

// commonDir is the resolved git directory shared by every worktree of the
// repository holding dir, "" when there is none.
func commonDir(o ops.Ops, dir string) string {
	out, _, err := o.Exec(dir, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return ""
	}
	return env.RealDir(o.FS(), strings.TrimRight(string(out), "\n"))
}

// Commit stages everything under the repo's directory but the exclusions
// and commits it once.
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
	spec := []string{"--", "."}
	for _, rel := range r.Exclude {
		spec = append(spec, ":(exclude,literal)"+rel)
	}
	// `status --porcelain` rather than `diff`: a new, untracked file counts.
	// --untracked-files=all: whatever status.showUntrackedFiles says.
	status, err := git(append([]string{"status", "--porcelain", "--untracked-files=all"}, spec...)...)
	if err != nil {
		return "", false, err
	}
	if status == "" {
		return "", false, nil
	}
	if _, err := git(append([]string{"add"}, spec...)...); err != nil {
		return "", false, fmt.Errorf("Error: Failed to stage changes: %v", err)
	}
	args := []string{"commit", "-m", message}
	if len(r.Exclude) > 0 {
		args = append(args, spec...)
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

// Ignored is the paths (relative to dir, in a repository) that a
// .gitignore of the repository already ignores, as git reads it: rules
// every clone shares. A rule only this machine has, in its
// core.excludesFile or .git/info/exclude, does not count: a clone
// elsewhere lacks it. git names the rule that decides each path, a
// .gitignore's before the machine's, and a negated rule ignores nothing.
// A path under a symbolic link is outside the repository's tree, which
// git refuses to be asked about; nothing ignores it there.
func Ignored(o ops.Ops, dir string, paths []string) (map[string]bool, error) {
	ignored := map[string]bool{}
	var ask []string
	for _, p := range paths {
		if !underLink(o.FS(), dir, p) {
			ask = append(ask, p)
		}
	}
	if len(ask) == 0 {
		return ignored, nil
	}
	out, errb, err := o.Exec(dir, "git", append([]string{"check-ignore", "--no-index", "--verbose", "--"}, ask...)...)
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) { // 1: none ignored
		msg := strings.TrimSpace(string(errb))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("Error: cannot ask git which of Daisy's paths %s ignores: %s", dir, msg)
	}
	for _, l := range strings.Split(string(out), "\n") {
		// <source>:<line>:<pattern> TAB <path>
		rule, path, _ := strings.Cut(l, "\t")
		f := strings.SplitN(rule, ":", 3)
		if len(f) == 3 && !filepath.IsAbs(f[0]) && filepath.Base(f[0]) == ".gitignore" && !strings.HasPrefix(f[2], "!") {
			ignored[path] = true
		}
	}
	return ignored, nil
}

// underLink: a directory on the way from dir to rel, or rel itself when
// it names a directory (a trailing /), is a symbolic link. A linked file
// is a path in the tree, and git answers for it.
func underLink(fsys ops.FS, dir, rel string) bool {
	parts := strings.Split(strings.TrimSuffix(rel, "/"), "/")
	if !strings.HasSuffix(rel, "/") {
		parts = parts[:len(parts)-1]
	}
	p := dir
	for _, part := range parts {
		p = filepath.Join(p, part)
		if fi, err := fsys.Lstat(p); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			return true
		}
	}
	return false
}
