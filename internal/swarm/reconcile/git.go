package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// Git is the swarm's host-side git (section 7.1): a bare mirror of each
// declared repository under <state>/git, in a swarm repository's clone
// the swarm repository's there too and each reference's under
// <state>/references, and the clones members work in.
// WS is the swarm's root, the swarm repository's clone.
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

// remote is where the repository is reached (section 7.1, step 1): the
// swarm repository's clone's origin URL, read at each call, with its
// last path element replaced by <name>.git, and the entry's ssh_command.
func (g Git) remote(r declare.Repo) (url string, ssh *string, err error) {
	origin, err := g.run(g.WS, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", nil, fmt.Errorf("%s: the swarm repository's clone %s has no origin URL: %v", r.Name, g.WS, err)
	}
	return RepoURL(origin, r.Name), r.SSHCommand, nil
}

// record makes the repository's mirror, a bare repository, when it is
// absent, and writes where the repository is reached into its config
// whenever that differs from the configuration: remote.origin.url, and
// core.sshCommand, unset when none is configured (section 7.1, step 1).
// Every fetch and push then reads them there, so a mirror made before
// its config recorded them (an empty config) is brought up to date by
// its next fetch or push.
func (g Git) record(r declare.Repo) (string, error) {
	m := g.Mirror(r.Name)
	if err := g.bare(m); err != nil {
		return "", err
	}
	url, ssh, err := g.remote(r)
	if err != nil {
		return "", err
	}
	return m, g.recordAt(m, url, ssh)
}

// bare makes m a bare repository when it is absent.
func (g Git) bare(m string) error {
	if _, err := g.O.FS().Stat(m); err == nil {
		return nil
	}
	if err := g.O.FS().MkdirAll(m, 0o755); err != nil {
		return err
	}
	_, err := g.run(m, "init", "--bare", "--quiet")
	return err
}

// recordAt writes url and ssh into the mirror m's config where they
// differ from what it records (record).
func (g Git) recordAt(m, url string, ssh *string) error {
	if cur, err := g.run(m, "config", "--local", "--get", "remote.origin.url"); err != nil || cur != url {
		if _, err := g.run(m, "config", "remote.origin.url", url); err != nil {
			return err
		}
	}
	cur, err := g.run(m, "config", "--local", "--get", "core.sshCommand")
	switch set := err == nil; {
	case ssh == nil && set:
		if _, err := g.run(m, "config", "--unset", "core.sshCommand"); err != nil {
			return err
		}
	case ssh != nil && (!set || cur != *ssh):
		if _, err := g.run(m, "config", "core.sshCommand", *ssh); err != nil {
			return err
		}
	}
	return nil
}

// Fetch brings the repository's mirror up to date from its origin as the
// mirror's config records it, creating the mirror when absent (section
// 7.1, step 1).
func (g Git) Fetch(r declare.Repo) error {
	m, err := g.record(r)
	if err != nil {
		return err
	}
	_, err = g.run(m, "fetch", "--quiet", "--prune", "origin", "+refs/heads/*:refs/heads/*")
	return err
}

// FetchSwarm brings the swarm repository's mirror, <state>/git/<repo>,
// up to date as a declared repository's is, reached at its clone's
// origin URL with the clone's core.sshCommand, and points the mirror's
// HEAD at the forge's default branch, read by `ls-remote --symref`
// (section 7.1, step 1).
func (g Git) FetchSwarm(repo string) error {
	url, err := g.run(g.WS, "config", "--get", "remote.origin.url")
	if err != nil {
		return fmt.Errorf("the swarm repository's clone %s has no origin URL: %v", g.WS, err)
	}
	var ssh *string
	if v, _ := g.run(g.WS, "config", "--get", "core.sshCommand"); v != "" {
		ssh = &v
	}
	m := g.Mirror(repo)
	if err := g.bare(m); err != nil {
		return err
	}
	if err := g.recordAt(m, url, ssh); err != nil {
		return err
	}
	if _, err := g.run(m, "fetch", "--quiet", "--prune", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
		return err
	}
	out, err := g.run(m, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return err
	}
	for _, l := range strings.Split(out, "\n") {
		if target, ok := strings.CutPrefix(l, "ref: "); ok {
			target, _, _ = strings.Cut(target, "\t")
			if cur, _ := g.run(m, "symbolic-ref", "HEAD"); cur != target {
				if _, err := g.run(m, "symbolic-ref", "HEAD", target); err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

// Reference is a reference's bare mirror, <state>/references/<name>.
func (g Git) Reference(name string) string { return filepath.Join(g.State, "references", name) }

// FetchPin makes the reference's mirror, recording its url, and fetches
// its pin when the mirror does not hold it: a commit by its id, a tag
// into refs/tags/<pin>; once there, never again (section 7.1, step 1).
// It returns the pin's commit.
func (g Git) FetchPin(r declare.Reference) (string, error) {
	m := g.Reference(r.Name)
	if err := g.bare(m); err != nil {
		return "", err
	}
	if err := g.recordAt(m, r.URL, nil); err != nil {
		return "", err
	}
	if sha, err := g.Pin(r); err == nil {
		return sha, nil
	}
	refspec := r.Pin
	if !declare.PinIsCommit(r.Pin) {
		refspec = "+refs/tags/" + r.Pin + ":refs/tags/" + r.Pin
	}
	if _, err := g.run(m, "fetch", "--quiet", "--no-tags", "origin", refspec); err != nil {
		return "", err
	}
	return g.Pin(r)
}

// Pin is the commit the reference's pin names in its mirror, an error
// when the mirror does not hold it.
func (g Git) Pin(r declare.Reference) (string, error) {
	rev := r.Pin
	if !declare.PinIsCommit(r.Pin) {
		rev = "refs/tags/" + r.Pin
	}
	return g.run(g.Reference(r.Name), "rev-parse", "--verify", "--quiet", rev+"^{commit}")
}

// Head is the commit of rev ("HEAD", the swarm repository's default
// branch, or refs/heads/<base>) in the mirror m, "" when it has none (no
// mirror yet, or nothing fetched).
func (g Git) Head(m, rev string) string {
	if _, err := g.O.FS().Stat(m); err != nil {
		return ""
	}
	sha, err := g.run(m, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil {
		return ""
	}
	return sha
}

// TreeEntry is one entry of a commit's tree: its mode, its type (blob,
// tree or commit), its object and its path in the repository.
type TreeEntry struct{ Mode, Type, Object, Path string }

// Tree is every entry at and under p in the mirror m's commit, trees
// included (`ls-tree -r -t`), each path as it is (-z); none when the
// commit holds no p.
func (g Git) Tree(m, commit, p string) ([]TreeEntry, error) {
	args := []string{"ls-tree", "-r", "-t", "-z", commit, "--", p}
	out, errb, err := g.O.Exec(m, "git", args...)
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(errb)))
	}
	var entries []TreeEntry
	for _, rec := range strings.Split(string(out), "\x00") {
		meta, name, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			continue
		}
		entries = append(entries, TreeEntry{f[0], f[1], f[2], name})
	}
	return entries, nil
}

// Blob is the content of the object in the mirror m.
func (g Git) Blob(m, object string) ([]byte, error) {
	out, errb, err := g.O.Exec(m, "git", "cat-file", "blob", object)
	if err != nil {
		return nil, fmt.Errorf("git cat-file blob %s: %v: %s", object, err, strings.TrimSpace(string(errb)))
	}
	return out, nil
}

// Archive is `git archive --format=tar <commit> -- <p>` in the mirror m:
// p at the commit, as a tar of its regular files, directories and links.
func (g Git) Archive(m, commit, p string) ([]byte, error) {
	args := []string{"archive", "--format=tar", commit, "--", p}
	out, errb, err := g.O.Exec(m, "git", args...)
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(errb)))
	}
	return out, nil
}

// HasFile: path is a file in the repository's mirror on branch, as the
// last fetch left it (`init`'s readiness, section 11).
func (g Git) HasFile(name, branch, path string) bool {
	_, err := g.run(g.Mirror(name), "cat-file", "-e", "refs/heads/"+branch+":"+path)
	return err == nil
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
// command that reads the member's clone. A clone that plainly holds no
// branch is ErrNoBranch, nothing run.
func (g Git) FetchRun(name, work, branch, ref string) (string, error) {
	dotgit := filepath.Join(work, ".git")
	if err := selfContained(g.O.FS(), dotgit); err != nil {
		return "", err
	}
	if noBranch(g.O.FS(), dotgit, branch) {
		return "", fmt.Errorf("%w %s", ErrNoBranch, branch)
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

// ErrNoBranch: a member's clone without the run's branch (a member
// killed mid-run, say), which no retry of the fetch brings back.
var ErrNoBranch = errors.New("the clone has no branch")

// noBranch: the files of dotgit, a self-contained .git, hold no ref
// refs/heads/<branch>, loose or packed; a clone whose refs are not files
// (git's reftable) is never taken to lack it, the fetch telling.
func noBranch(fs ops.FS, dotgit, branch string) bool {
	if _, err := fs.Lstat(filepath.Join(dotgit, "reftable")); err == nil {
		return false
	}
	if _, err := fs.Lstat(filepath.Join(dotgit, "refs", "heads", filepath.FromSlash(branch))); err == nil {
		return false
	}
	packed, err := fs.ReadFile(filepath.Join(dotgit, "packed-refs"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return false
	}
	for _, line := range strings.Split(string(packed), "\n") {
		if strings.HasSuffix(strings.TrimRight(line, "\r"), " refs/heads/"+branch) {
			return false
		}
	}
	return true
}

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
	// A path gone between its directory's listing and the walk's look at
	// it (a lock a git process released) leads nowhere, and is skipped.
	gone := func(p string, err error) bool { return p != dir && errors.Is(err, iofs.ErrNotExist) }
	var walk func(string) error
	walk = func(d string) error {
		// The reason names no path: a name in the member's .git is its
		// text, past no secret check (nothing was fetched to check).
		entries, err := fs.ReadDir(d)
		if gone(d, err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: a directory in .git cannot be read", ErrForeignClone)
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			fi, err := fs.Lstat(p)
			if gone(p, err) {
				continue
			}
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

// Conflicts is whether sha merges into base on the repository's mirror,
// as `git merge-tree` merges them without a work tree (section 12, the
// merge step): clean, or the paths that conflict, each once.
func (g Git) Conflicts(name, base, sha string) (paths []string, clean bool, err error) {
	args := []string{"merge-tree", "--write-tree", "--name-only", "--no-messages", "-z", "refs/heads/" + base, sha}
	out, errb, err := g.O.Exec(g.Mirror(name), "git", args...)
	// Exit 1 is a conflict when git wrote the merged tree's id first; an
	// object it cannot read exits 1 as well, with nothing written.
	var exit *exec.ExitError
	tree, rest, _ := strings.Cut(string(out), "\x00")
	switch {
	case err == nil:
		return nil, true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1 && objectID.MatchString(tree):
		for _, p := range strings.Split(rest, "\x00") {
			if p != "" {
				paths = append(paths, p)
			}
		}
		return paths, false, nil
	}
	return nil, false, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(errb)))
}

var objectID = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

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

// Push sends ref to the branch on the repository's origin, as the
// mirror's config records it (record), never forced, then moves the mirror's branch to sha and drops ref (section 7.6, step
// 5). A push the remote refuses is ErrPushRejected, with git's message;
// one that does not reach it is an error.
func (g Git) Push(r declare.Repo, ref, branch, sha string) error {
	m, err := g.record(r)
	if err != nil {
		return err
	}
	if err := g.pushTo(m, nil, "origin", ref+":refs/heads/"+branch); err != nil {
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
// and a local branch for each of also (the revising architect's pull
// request head) from the mirror's, which the clone fetched; no remote, the role's identity `<role> (Daisy swarm) <commit_email>`, a
// commit-msg hook stamping `Co-Authored-By: <co_author>` on every commit
// (none for an empty co_author: no trailer), and `/.swarm/` and the mount
// points of the notes, `/.notes/`, and of the resources, `/.resources/`,
// excluded. It returns the head commit.
func (g Git) Clone(r declare.Repo, branch, base, dir, role, email, coAuthor string, also ...string) (string, error) {
	if _, err := g.run("", "clone", "--no-local", "--quiet", "--branch", branch, g.Mirror(r.Name), dir); err != nil {
		return "", err
	}
	for _, b := range append([]string{base}, also...) {
		if _, err := g.run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+b); err != nil {
			if _, err := g.run(dir, "branch", b, "origin/"+b); err != nil {
				return "", err
			}
		}
	}
	// maintenance.auto false: a member's commit starts no detached `git
	// maintenance run --auto`, a writer still at work in .git when the
	// run is collected (7.6, step 1).
	for _, args := range [][]string{{"remote", "remove", "origin"}, {"config", "user.name", role + " (Daisy swarm)"}, {"config", "user.email", email}, {"config", "maintenance.auto", "false"}} {
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
	if err := fs.WriteFile(exclude, append(old, "/.swarm/\n/.notes/\n/.resources/\n"...), 0o644); err != nil {
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
