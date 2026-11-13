package commands

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/env"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/reconcile"
)

// initMessage is the message of init's one commit in the swarm
// repository's clone (section 2).
const initMessage = "swarm: written by init"

// initRepository is init (docs/swarm.md section 11, init,
// its Order): the two tokens and their accounts, the swarm repository read
// or created and the swarm's account given write access with the
// maintainer's token, the clone made in a folder holding none, the swarm's
// read check, the files the clone lacks committed and pushed as the
// maintainer, <state>/env, the mirrors, the labels with the swarm's token,
// the readiness warnings and the unit. Each forge write names its token.
func initRepository(c *Context, w *declare.Wizard, cfg declare.Config, open opener) error {
	o, p := c.Ops, c.Swarm
	_, repo, _ := strings.Cut(cfg.Forge.Repo, "/")
	dir := p.Root
	if p.Outside {
		// <dir>/<repo> checked before any forge call: a clone of the
		// swarm repository holding no swarm.json is the clone; anything
		// else is refused, a clone holding one too (init runs in it).
		dir = filepath.Join(p.Root, repo)
		if _, err := o.FS().Lstat(dir); err == nil {
			if !declare.IsCloneOf(o, dir, cfg.Forge) {
				return fmt.Errorf("Error: swarm: %s exists and is not a clone of %s", dir, cfg.Forge.Repo)
			}
			if _, err := o.FS().Lstat(declare.File(dir)); err == nil {
				return fmt.Errorf("Error: swarm: %s is the swarm's clone; run 'daisy swarm init' in it, where its swarm.json gives the answers", dir)
			}
		}
	}

	tokenVar := cfg.Forge.TokenEnv
	vars, err := readSecrets(c, cfg, c.Stderr, tokenVar, MaintainerTokenEnv)
	if err != nil {
		return err
	}
	token, mtoken := vars[tokenVar], vars[MaintainerTokenEnv]
	if token == "" {
		return unsetErr(c, cfg, tokenVar)
	}
	names := repoNames(cfg)
	f, err := open(o, cfg.Forge, names, token)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	acct, err := f.Login()
	if err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read its own account: %v", tokenVar, err)
	}
	if mtoken == "" {
		return unsetErr(c, cfg, MaintainerTokenEnv)
	}
	if mtoken == token {
		return fmt.Errorf("Error: swarm: $%s holds the swarm's token ($%s); the maintainer's writes act with your own (docs/swarm.md section 8)", MaintainerTokenEnv, tokenVar)
	}
	mf, err := open(o, cfg.Forge, names, mtoken)
	if err != nil {
		return fmt.Errorf("Error: swarm: %v", err)
	}
	macct, err := mf.Login()
	if err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot read its own account: %v", MaintainerTokenEnv, err)
	}
	if strings.EqualFold(macct.Login, acct.Login) {
		return fmt.Errorf("Error: swarm: $%s is %s's token, the swarm's account ($%s's); the maintainer's writes act with your own (docs/swarm.md section 8)", MaintainerTokenEnv, macct.Login, tokenVar)
	}
	if err := w.AskAccount(&cfg, acct.Login, acct.Email); err != nil {
		return cancelledOr(c, err)
	}

	maintainer := "Maintainer " + macct.Login + ": "
	held, err := mf.Repository()
	switch {
	case errors.Is(err, forge.ErrNotFound):
		if held, err = mf.CreateRepository(); err != nil {
			return fmt.Errorf("Error: swarm: $%s cannot create %s: %v", MaintainerTokenEnv, cfg.Forge.Repo, err)
		}
		fmt.Fprintf(c.Stdout, "✓ %s%s created\n", maintainer, cfg.Forge.Repo)
	case err != nil:
		return fmt.Errorf("Error: swarm: $%s cannot read %s: %v", MaintainerTokenEnv, cfg.Forge.Repo, err)
	default:
		fmt.Fprintf(c.Stdout, "  %s%s exists\n", maintainer, cfg.Forge.Repo)
	}
	if err := mf.AddCollaborator(acct.Login); err != nil {
		return fmt.Errorf("Error: swarm: $%s cannot give %s write access to %s: %v", MaintainerTokenEnv, acct.Login, cfg.Forge.Repo, err)
	}
	fmt.Fprintf(c.Stdout, "✓ %s%s may write to %s\n", maintainer, acct.Login, cfg.Forge.Repo)

	if p.Outside {
		if _, err := o.FS().Lstat(dir); err != nil {
			if _, errb, err := o.Exec(p.Root, "git", "clone", "--quiet", held.SSHURL, dir); err != nil {
				return fmt.Errorf("Error: swarm: git clone %s %s: %v", held.SSHURL, dir, gitErr(err, errb))
			}
			fmt.Fprintf(c.Stdout, "✓ Cloned %s into %s\n", cfg.Forge.Repo, dir)
		}
		p = declare.Clone(dir)
		c.Swarm, w.Place = p, p
	}

	if err := readCheck(f, cfg); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "✓ Token $%s reads %s as %s\n", tokenVar, cfg.Forge.Repo, acct.Login)
	err = w.AskNotes(&cfg)
	if err == nil {
		err = declare.Validate(o, p, cfg, declare.SkipPrompts)
	}
	if err != nil {
		return cancelledOr(c, err)
	}

	// The declared repositories' mirrors before the files, since
	// repos/<name>.md's seed reads them, as the readiness warnings do.
	g := reconcile.Git{O: o, WS: p.Root, State: cfg.Forge.State(c.Env.Local)}
	for _, r := range cfg.Repos {
		if err := g.Fetch(r); err != nil {
			return fmt.Errorf("Error: swarm: mirror %s: %v", r.Name, err)
		}
	}
	written, err := writeDeclaration(c, p, cfg)
	if err != nil {
		return err
	}
	for _, write := range []func() ([]string, error){
		func() ([]string, error) { return writeRepoNotes(c, p, g, cfg) },
		func() ([]string, error) { return writeSeeds(c, p, cfg) },
	} {
		w, err := write()
		if err != nil {
			return err
		}
		written = append(written, w...)
	}
	if err := commitAndPush(c, p, cfg, held.DefaultBranch, written); err != nil {
		return err
	}
	if err := writeEnv(c, cfg); err != nil {
		return err
	}
	if err := writeHome(c, cfg); err != nil {
		return err
	}
	// The swarm repository's mirror after the push, so that it holds what
	// init pushed and status shows no drift.
	if err := g.FetchSwarm(repo); err != nil {
		return fmt.Errorf("Error: swarm: mirror %s: %v", repo, err)
	}
	fmt.Fprintf(c.Stdout, "✓ Mirrors: %s fetched\n", strings.Join(append([]string{repo}, names...), ", "))
	created, renamed, err := f.EnsureLabels(forge.Labels)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	fmt.Fprintf(c.Stdout, "✓ Token $%s: labels %d present (%d created, %d renamed)\n", tokenVar, len(forge.Labels), created, renamed)
	readiness(c, g, cfg, acct.Login)
	return printUnits(c, p, cfg.Interval, cfg.Timeout)
}

// gitErr is git's own message for a failed command, else the error.
func gitErr(err error, stderr []byte) string {
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		return msg
	}
	return err.Error()
}

// writeSeeds writes the swarm repository's README.md, one in each of
// briefs/, notes/ and docs/, and the .gitignore naming .env.sh, each only
// where absent (section 2).
func writeSeeds(c *Context, at declare.Place, cfg declare.Config) (written []string, err error) {
	for _, s := range declare.Seeds(cfg) {
		w, err := writeAbsent(c, at.Root, s.Path, s.Content)
		if err != nil {
			return nil, err
		}
		written = append(written, w...)
	}
	return written, nil
}

// initFiles are the clone's files of section 2's table, relative to it.
func initFiles(cfg declare.Config) []string {
	files := []string{"swarm.json"}
	for _, role := range declare.Roles {
		files = append(files, filepath.Join("prompts", role+".md"))
	}
	for _, r := range cfg.Repos {
		files = append(files, reconcile.RepoNotesFile(r.Name))
	}
	for _, s := range declare.Seeds(cfg) {
		files = append(files, s.Path)
	}
	return files
}

// commitAndPush commits the paths init wrote in the clone, with each file
// of section 2's table the clone holds and its HEAD lacks (one an earlier
// run wrote and did not commit, its commit having failed), as
// commitPaths does.
func commitAndPush(c *Context, at declare.Place, cfg declare.Config, branch string, written []string) error {
	for _, rel := range initFiles(cfg) {
		if slices.Contains(written, rel) {
			continue
		}
		if _, err := c.Ops.FS().Lstat(filepath.Join(at.Root, rel)); err != nil {
			continue
		}
		if _, _, err := c.Ops.Exec(at.Root, "git", "cat-file", "-e", "HEAD:"+rel); err != nil {
			written = append(written, rel)
		}
	}
	return commitPaths(c, at, cfg.Forge.Repo, branch, initMessage, written)
}

// commitPaths commits written, paths relative to the clone, with message,
// by the clone's own git identity, and pushes the commit to the swarm
// repository repo's default branch with the clone's own credentials;
// nothing else of the clone's index or tree is committed, and nothing to
// commit (or nothing that differs from the clone's index) commits
// nothing. A clone of an empty repository gets branch as its first
// branch.
func commitPaths(c *Context, at declare.Place, repo, branch, message string, written []string) error {
	run := func(args ...string) (string, error) {
		out, errb, err := c.Ops.Exec(at.Root, "git", args...)
		if err != nil {
			return "", fmt.Errorf("git %s: %s", args[0], gitErr(err, errb))
		}
		return strings.TrimSpace(string(out)), nil
	}
	fail := func(err error) error { return fmt.Errorf("Error: swarm: commit in %s: %v", at.Root, err) }
	if len(written) > 0 {
		if _, err := run(append([]string{"add", "--"}, written...)...); err != nil {
			return fail(err)
		}
		if _, err := run(append([]string{"diff", "--cached", "--quiet", "--"}, written...)...); err == nil {
			written = nil // what was written is what the clone holds
		}
	}
	if len(written) == 0 {
		fmt.Fprintln(c.Stdout, "  Nothing to commit")
		return nil
	}
	if _, err := run("rev-parse", "--quiet", "--verify", "HEAD"); err != nil {
		if _, err := run("symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
			return fail(err)
		}
	}
	if _, err := run(append([]string{"commit", "--quiet", "-m", message, "--"}, written...)...); err != nil {
		return fail(err)
	}
	sha, err := run("rev-parse", "--short=7", "HEAD")
	if err != nil {
		return fail(err)
	}
	author, err := run("log", "-1", "--format=%an <%ae>")
	if err != nil {
		return fail(err)
	}
	if _, err := run("push", "--quiet", "origin", "HEAD:"+branch); err != nil {
		return fmt.Errorf("Error: swarm: push %s: %v; the commit stays in the clone: pull, then push it", at.Root, err)
	}
	fmt.Fprintf(c.Stdout, "✓ Committed and pushed %s to %s's %s as %s\n", sha, repo, branch, author)
	return nil
}

// writeHome writes <state>/home when absent (section 2): the name of the
// home the Daisy workspace init runs in is bound to, to whose metrics CSV
// apply appends each run's row (section 7.6, The run's record). Outside
// a workspace it writes none, and the journal alone records the runs. An
// existing file is never rewritten: the maintainer edits it.
func writeHome(c *Context, cfg declare.Config) error {
	fs := c.Ops.FS()
	path := filepath.Join(cfg.Forge.State(c.Env.Local), "home")
	if _, err := fs.Lstat(path); err == nil {
		fmt.Fprintf(c.Stdout, "  Kept %s (exists)\n", path)
		return nil
	}
	name, err := boundHome(c)
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠ %s not written: %s\n", path, strings.TrimPrefix(strings.SplitN(err.Error(), "\n", 2)[0], "Error: "))
		return nil
	}
	if name == "" {
		fmt.Fprintf(c.Stdout, "  No Daisy workspace here: %s not written; the journal alone records each run\n", path)
		return nil
	}
	if err := fs.WriteFile(path, []byte(name+"\n"), 0o644); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", path, err)
	}
	fmt.Fprintf(c.Stdout, "✓ Wrote %s (%s): each run's metrics row goes to that home\n", path, name)
	return nil
}

// boundHome is the name of the home the workspace the working folder is
// in is bound to (its .daisy/home), "" when it is in none.
func boundHome(c *Context) (string, error) {
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return "", err
	}
	ws, err := env.FindWorkspace(c.Ops.FS(), c.Env.Root, cwd)
	if errors.Is(err, env.ErrNoWorkspace) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	b, err := c.Ops.FS().ReadFile(filepath.Join(ws, ".daisy", "home"))
	if err != nil {
		return "", fmt.Errorf("%s/.daisy has no home file to name", ws)
	}
	name := strings.TrimSpace(string(b))
	if !env.NameOK(name) {
		return "", fmt.Errorf("%s/.daisy/home holds %q, which is not a home's name", ws, name)
	}
	return name, nil
}

// writeEnv writes <state>/env when absent, mode 0600 (section 3, Secret
// values): one line per variable the configuration names and the
// maintainer's token, each the process environment's value, single-quoted
// as a shell reads it; an unset one is written commented out, `# NAME=`,
// so that it shadows no value apply's environment supplies, and is warned
// of. An existing file is never rewritten.
func writeEnv(c *Context, cfg declare.Config) error {
	fs := c.Ops.FS()
	path := secretsFile(c, cfg)
	if _, err := fs.Lstat(path); err == nil {
		fmt.Fprintf(c.Stdout, "  Kept %s (exists)\n", path)
		return nil
	}
	var b strings.Builder
	var unset []string
	for _, name := range cfg.SecretNames() {
		v := c.Ops.Getenv(name)
		if v == "" {
			unset = append(unset, name)
			fmt.Fprintf(&b, "# %s=\n", name)
			continue
		}
		fmt.Fprintf(&b, "export %s='%s'\n", name, strings.ReplaceAll(v, "'", `'\''`))
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("Error: cannot create %s: %v", filepath.Dir(path), err)
	}
	if err := fs.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", path, err)
	}
	fmt.Fprintf(c.Stdout, "✓ Wrote %s (mode 0600)\n", path)
	for _, name := range unset {
		fmt.Fprintf(c.Stderr, "⚠ %s: $%s is not set; its line is commented out: fill it in, or set $%s in apply's environment\n", path, name, name)
	}
	return nil
}
