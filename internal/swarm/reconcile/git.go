package reconcile

import (
	"fmt"
	"path/filepath"
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

// Fetch brings the repository's mirror up to date from its origin,
// creating it when absent; the origin's core.sshCommand, when it sets one,
// is passed to the fetch (section 7.1, step 1).
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
	src := filepath.Join(g.WS, r.Path)
	url, err := g.run(src, "config", "--get", "remote.origin.url")
	if err != nil {
		return fmt.Errorf("%s: no origin URL: %v", r.Name, err)
	}
	var args []string
	if ssh, _ := g.run(src, "config", "--get", "core.sshCommand"); ssh != "" {
		args = append(args, "-c", "core.sshCommand="+ssh)
	}
	_, err = g.run(m, append(args, "fetch", "--quiet", "--prune", url, "+refs/heads/*:refs/heads/*")...)
	return err
}

// Clone makes dir a clone of the repository's mirror on branch, prepared
// before any container sees it (section 7.1, step 3): a local base branch,
// no remote, the role's identity `<role> (Daisy swarm) <commit_email>`, a
// commit-msg hook stamping `Co-Authored-By: <co_author>` on every commit,
// and `/.swarm/` excluded. It returns the head commit.
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
	exclude := filepath.Join(dir, ".git", "info", "exclude")
	old, _ := fs.ReadFile(exclude)
	if len(old) > 0 && old[len(old)-1] != '\n' {
		old = append(old, '\n')
	}
	if err := fs.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return "", err
	}
	if err := fs.WriteFile(exclude, append(old, "/.swarm/\n"...), 0o644); err != nil {
		return "", err
	}
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
