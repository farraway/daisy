package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// Git is the swarm's host-side git (section 7.1): a bare mirror of each
// declared repository under <state>/git, and the clones members work in.
type Git struct {
	O         ops.Ops
	WS, State string
}

func (g Git) run(dir string, args ...string) (string, error) {
	out, errb, err := g.O.Exec(dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(errb)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Mirror is the repository's bare mirror.
func (g Git) Mirror(name string) string { return filepath.Join(g.State, "git", name) }

// origin is the repository's origin URL and the git options that reach
// it: `-c core.sshCommand=<value>` when its clone sets one (section 7.1,
// step 1).
func (g Git) origin(r declare.Repo) (url string, opts []string, err error) {
	src := filepath.Join(g.WS, r.Path)
	url, err = g.run(src, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", nil, fmt.Errorf("%s: no origin URL: %v", r.Name, err)
	}
	if ssh, _ := g.run(src, "config", "--get", "core.sshCommand"); ssh != "" {
		opts = []string{"-c", "core.sshCommand=" + ssh}
	}
	return url, opts, nil
}

// Fetch brings the repository's mirror up to date from its origin,
// creating it when absent (section 7.1, step 1).
func (g Git) Fetch(r declare.Repo) error {
	m := g.Mirror(r.Name)
	if _, err := g.O.FS().Stat(m); err != nil {
		if err := g.O.FS().MkdirAll(m, 0o755); err != nil {
			return err
		}
		if _, err := g.run(m, "init", "--bare", "--quiet"); err != nil {
			return err
		}
	}
	url, opts, err := g.origin(r)
	if err != nil {
		return err
	}
	_, err = g.run(m, append(opts, "fetch", "--quiet", "--prune", url, "+refs/heads/*:refs/heads/*")...)
	return err
}

// Branch starts branch at the clone's head (section 7.1, step 3: a worker
// starting a slice).
func (g Git) Branch(dir, branch string) error {
	if _, err := g.run(dir, "checkout", "--quiet", "-b", branch); err != nil {
		return err
	}
	return forgetHost(g.O.FS(), dir) // the checkout's reflog is host git's too
}

// FetchRun fetches a worker run's branch from its clone into the mirror as
// ref, and returns the fetched commit (section 7.6, step 1): the only git
// command that reads the member's clone.
func (g Git) FetchRun(name, work, branch, ref string) (string, error) {
	if err := selfContained(g.O.FS(), filepath.Join(work, ".git")); err != nil {
		return "", err
	}
	m := g.Mirror(name)
	if _, err := g.run(m, "fetch", "--quiet", "--no-tags", work, "+refs/heads/"+branch+":"+ref); err != nil {
		return "", err
	}
	return g.run(m, "rev-parse", ref)
}

// ErrForeignClone: a member's clone that reaches outside itself, which
// host git, reading it with the host's rights, would follow (section 8:
// a member must not reach the host's state).
var ErrForeignClone = errors.New("the clone reaches outside itself")

// selfContained: the member's .git is a directory of its own, with no
// link anywhere in it (a ref or an objects directory led to a host
// repository's), no alternates and no commondir (another repository's
// objects or refs read as its own).
func selfContained(fs ops.FS, dir string) error {
	fi, err := fs.Lstat(dir)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("%w: .git is not a directory", ErrForeignClone)
	}
	for _, rel := range []string{"commondir", filepath.Join("objects", "info", "alternates"), filepath.Join("objects", "info", "http-alternates")} {
		if _, err := fs.Lstat(filepath.Join(dir, rel)); err == nil {
			return fmt.Errorf("%w: it holds .git/%s", ErrForeignClone, rel)
		}
	}
	var walk func(string) error
	walk = func(d string) error {
		// The reason names no path: a name in the member's .git is its
		// text, past no secret check (nothing was fetched to check).
		entries, err := fs.ReadDir(d)
		if err != nil {
			return fmt.Errorf("%w: a directory in .git cannot be read", ErrForeignClone)
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			fi, err := fs.Lstat(p)
			if err != nil {
				return fmt.Errorf("%w: a file in .git cannot be read", ErrForeignClone)
			}
			switch {
			case fi.Mode()&iofs.ModeSymlink != 0:
				return fmt.Errorf("%w: .git holds a link", ErrForeignClone)
			case fi.IsDir():
				if err := walk(p); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(dir)
}

// Unstamped is each commit of base..sha whose message lacks the line
// `Co-Authored-By: <coAuthor>` (section 7.6, The trailer).
func (g Git) Unstamped(name, base, sha, coAuthor string) ([]string, error) {
	out, err := g.run(g.Mirror(name), "log", "--format=%H%x00%B%x01", "refs/heads/"+base+".."+sha)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, c := range strings.Split(out, "\x01") {
		h, msg, ok := strings.Cut(strings.TrimLeft(c, "\n"), "\x00")
		if !ok {
			continue
		}
		stamped := false
		for _, l := range strings.Split(msg, "\n") {
			stamped = stamped || strings.TrimRight(l, "\r") == "Co-Authored-By: "+coAuthor
		}
		if !stamped {
			bad = append(bad, h)
		}
	}
	return bad, nil
}

// Changed is every path base...sha changes (section 7.6, step 3), a
// renamed file by both its paths.
func (g Git) Changed(name, base, sha string) ([]string, error) {
	// -z: each path as it is, NUL-ended, never quoted (core.quotePath
	// would quote a path with a byte outside ASCII, which no pattern
	// matches).
	args := []string{"diff", "--name-only", "-z", "--no-renames", "refs/heads/" + base + "..." + sha}
	out, errb, err := g.O.Exec(g.Mirror(name), "git", args...)
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(errb)))
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// Objects is every object the push of sha would send past base, as
// stored: commits with their headers, trees and blobs (section 7.6, step
// 3, the secret check).
func (g Git) Objects(name, base, sha string) ([]byte, error) {
	m := g.Mirror(name)
	list, err := g.run(m, "rev-list", "--objects", "--no-object-names", "refs/heads/"+base+".."+sha)
	if err != nil {
		return nil, err
	}
	var out, errb bytes.Buffer
	p, err := g.O.Start(m, "git", strings.NewReader(list+"\n"), &out, &errb, "cat-file", "--batch")
	if err == nil {
		var code int
		if code, err = p.Wait(); err == nil && code != 0 {
			err = fmt.Errorf("exit %d", code)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %v: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// CheckClone makes dir a clone of the mirror at sha, fetched by its run's
// ref, with no remote (section 7.6, step 4).
func (g Git) CheckClone(name, dir, ref, sha string) error {
	if _, err := g.run("", "clone", "--no-local", "--quiet", "--no-checkout", g.Mirror(name), dir); err != nil {
		return err
	}
	for _, args := range [][]string{{"fetch", "--quiet", "--no-tags", "origin", ref}, {"checkout", "--quiet", "--detach", sha}, {"remote", "remove", "origin"}} {
		if _, err := g.run(dir, args...); err != nil {
			return err
		}
	}
	return forgetHost(g.O.FS(), dir)
}

// forgetHost removes what host git wrote into a clone a container will
// see beside the objects (section 8, rule 3): the reflogs, which name the
// host's git identity and the mirror's path (`clone: from <state>/git/…`),
// and FETCH_HEAD, which names the mirror too.
func forgetHost(fs ops.FS, dir string) error {
	for _, p := range []string{filepath.Join(dir, ".git", "logs"), filepath.Join(dir, ".git", "FETCH_HEAD")} {
		if err := fs.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

// ErrPushRejected is the forge refusing a worker's push (section 7.7).
var ErrPushRejected = errors.New("push rejected")

// Push sends ref to the branch on the repository's origin, never forced,
// then moves the mirror's branch to sha and drops ref (section 7.6, step
// 5). A push the remote refuses is ErrPushRejected, with git's message;
// one that does not reach it is an error.
func (g Git) Push(r declare.Repo, ref, branch, sha string) error {
	m := g.Mirror(r.Name)
	url, opts, err := g.origin(r)
	if err != nil {
		return err
	}
	if err := g.pushTo(m, opts, url, ref+":refs/heads/"+branch); err != nil {
		return err
	}
	if _, err := g.run(m, "update-ref", "refs/heads/"+branch, sha); err != nil {
		return err
	}
	return g.DropRef(r.Name, ref)
}

// pushTo is `git push --porcelain <url> <refspec>` from dir, never
// forced. A ref the remote refused, which git's porcelain marks with `!`,
// is ErrPushRejected with git's message; any other failure (the forge
// unreachable) is an error to retry.
func (g Git) pushTo(dir string, opts []string, url, refspec string) error {
	args := append(slices.Clone(opts), "push", "--quiet", "--porcelain", url, refspec)
	out, errb, err := g.O.Exec(dir, "git", args...)
	if err == nil {
		return nil
	}
	err = fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(errb)))
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "!") {
			return fmt.Errorf("%w: %v", ErrPushRejected, err)
		}
	}
	return err
}

func isRejected(err error) bool { return errors.Is(err, ErrPushRejected) }

// DropRef deletes a ref from the mirror.
func (g Git) DropRef(name, ref string) error {
	_, err := g.run(g.Mirror(name), "update-ref", "-d", ref)
	return err
}

// Protected is the paths that match one of the patterns (section 3,
// repos[].protected): an entry ending `/` every path under that
// directory; one without `/` a path whose last element it matches, at any
// depth; any other the whole path.
func Protected(paths, patterns []string) []string {
	var hit []string
	for _, p := range paths {
		for _, pat := range patterns {
			var ok bool
			switch {
			case strings.HasSuffix(pat, "/"):
				ok = strings.HasPrefix(p, pat)
			case !strings.Contains(pat, "/"):
				ok, _ = path.Match(pat, path.Base(p))
			default:
				ok, _ = path.Match(pat, p)
			}
			if ok {
				hit = append(hit, p)
				break
			}
		}
	}
	return hit
}

// Clone makes dir a clone of the repository's mirror on branch, prepared
// before any container sees it (section 7.1, step 3): a local base branch,
// no remote, the role's identity `<role> (Daisy swarm) <commit_email>`, a
// commit-msg hook stamping `Co-Authored-By: <co_author>` on every commit
// (none for an empty co_author: no trailer), and `/.swarm/` and the notes'
// mount point `/.notes/` excluded. It
// returns the head commit.
func (g Git) Clone(r declare.Repo, branch, base, dir, role, email, coAuthor string) (string, error) {
	if _, err := g.run("", "clone", "--no-local", "--quiet", "--branch", branch, g.Mirror(r.Name), dir); err != nil {
		return "", err
	}
	if _, err := g.run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+base); err != nil {
		if _, err := g.run(dir, "branch", base, "origin/"+base); err != nil {
			return "", err
		}
	}
	for _, args := range [][]string{{"remote", "remove", "origin"}, {"config", "user.name", role + " (Daisy swarm)"}, {"config", "user.email", email}} {
		if _, err := g.run(dir, args...); err != nil {
			return "", err
		}
	}
	fs := g.O.FS()
	if err := forgetHost(fs, dir); err != nil {
		return "", err
	}
	exclude := filepath.Join(dir, ".git", "info", "exclude")
	old, _ := fs.ReadFile(exclude)
	if len(old) > 0 && old[len(old)-1] != '\n' {
		old = append(old, '\n')
	}
	if err := fs.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return "", err
	}
	if err := fs.WriteFile(exclude, append(old, "/.swarm/\n/.notes/\n"...), 0o644); err != nil {
		return "", err
	}
	if coAuthor != "" {
		hooks := filepath.Join(dir, ".git", "hooks")
		if err := fs.MkdirAll(hooks, 0o755); err != nil {
			return "", err
		}
		if err := fs.WriteFile(filepath.Join(hooks, "commit-msg"), []byte(CommitMsgHook(coAuthor)), 0o755); err != nil {
			return "", err
		}
		if err := fs.Chmod(filepath.Join(hooks, "commit-msg"), 0o755); err != nil {
			return "", err
		}
	}
	return g.run(dir, "rev-parse", "HEAD")
}

// CommitMsgHook is the clone's commit-msg hook: the role's Co-Authored-By
// trailer added to each commit message unless the message already carries
// that exact trailer (section 7.1).
func CommitMsgHook(coAuthor string) string {
	trailer := "Co-Authored-By: " + coAuthor
	return "#!/bin/sh\n# Daisy swarm (docs/swarm.md section 7.1): the role's co-author on every commit.\n" +
		"exec git interpret-trailers --in-place --if-exists addIfDifferent --trailer '" + strings.ReplaceAll(trailer, "'", `'\''`) + "' \"$1\"\n"
}
