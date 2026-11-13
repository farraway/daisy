package commands

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/gitx"
	"daisy/internal/health"
	"daisy/internal/identity"
	"daisy/internal/ops"
	"daisy/internal/prompt"
)

// HomeList prints each registered home and where it lives. Read-only.
var HomeList = readOnly(func(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	homes := env.Homes(fs, m.HomesFile())
	if len(homes) == 0 {
		fmt.Fprintf(c.Stderr, "No homes registered in %s\n  Run 'daisy home new <name>' to create one, or 'daisy home add <name> <path>' to register an existing home\n", m.HomesFile())
		return nil
	}
	for _, h := range homes {
		path, _ := env.HomePath(fs, m.HomesFile(), h.Name)
		if fi, err := fs.Stat(path); err == nil && fi.IsDir() {
			fmt.Fprintf(c.Stdout, "%-14s %s\n", h.Name, path)
		} else {
			fmt.Fprintf(c.Stdout, "%-14s %s  (missing)\n", h.Name, path)
		}
	}
	return nil
})

// HomeAdd registers an existing home directory under a name.
func HomeAdd(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	name := c.Args[0]
	if err := unregisteredName(fs, m, name); err != nil {
		return err
	}
	path, err := absolute(c, c.Args[1])
	if err != nil {
		return err
	}
	real := env.RealDir(fs, path)
	if real == "" {
		return fmt.Errorf("Error: %s is not a directory\n  Run 'daisy home new %s %s' to create a home there", c.Args[1], name, c.Args[1])
	}
	if problem := env.HomePathProblem(fs, m.Root, real); problem != "" {
		return fmt.Errorf("Error: %s %s", real, problem)
	}
	if fi, err := fs.Stat(filepath.Join(real, "include.txt")); err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("Error: %s is not a Daisy home (no include.txt)\n  A home is a copy of templates/home/; 'daisy home new' makes one", real)
	}
	if _, refusal := gitx.HomeRepo(c.Ops, m.Root, real); refusal != gitx.Accepted {
		return fmt.Errorf("Error: %s is not in its own git repository%s\n  Every Daisy mutation commits in the home's repository. Run: %s", real, refusal.Why(), refusal.Remedy(m.Root, real, name, health.ShellQuote))
	}
	// The staging call every home commit makes, as a dry run: it fails when
	// the directory is gitignored in its repository.
	if _, _, err := c.Ops.Exec(real, "git", "add", "--dry-run", "--", "."); err != nil {
		return fmt.Errorf("Error: nothing under %s can be staged in its repository (is it gitignored?)\n  Un-ignore it, or give the home its own repository", real)
	}
	if err := env.Register(fs, m.HomesFile(), name, real); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "✓ Registered home '%s' at %s\n", name, real)
	return nil
}

// HomeRemove unregisters a home; its files are left where they are.
func HomeRemove(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	name := c.Args[0]
	path, ok := env.HomePath(c.Ops.FS(), m.HomesFile(), name)
	if !ok {
		return fmt.Errorf("Error: Home '%s' is not registered\n  Run 'daisy home list' to see registered homes", name)
	}
	if err := env.Unregister(c.Ops.FS(), m.HomesFile(), name); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "✓ Unregistered home '%s' (its files at %s are untouched)\n", name, path)
	return nil
}

// HomeNew creates a home from the template at the path (asked for when not
// given; default $DAISY_LOCAL/homes/<name>), asks for its identity, makes
// it a repository with a first commit, registers it and builds its
// AGENTS.md. Every answer is collected before the first write; any failure
// from the first write to the registration removes what was written.
func HomeNew(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	fs := c.Ops.FS()
	name := c.Args[0]
	if err := unregisteredName(fs, m, name); err != nil {
		return err
	}
	template := filepath.Join(m.Root, "templates", "home")
	if fi, err := fs.Stat(template); err != nil || !fi.IsDir() {
		return fmt.Errorf("Error: Home template not found at %s", template)
	}
	var path string
	if len(c.Args) == 2 {
		path = c.Args[1]
	} else {
		def := filepath.Join(m.Local, "homes", name)
		if path, err = ask(c, fmt.Sprintf("  Location [%s]: ", def)); err != nil {
			return err
		}
		if path == "" {
			path = def
		} else if path == "~" || strings.HasPrefix(path, "~/") {
			path = c.Ops.Getenv("HOME") + path[1:]
		}
	}
	if path, err = absolute(c, path); err != nil {
		return err
	}
	if _, err := fs.Lstat(path); err == nil {
		return fmt.Errorf("Error: %s already exists\n  Run 'daisy home add %s %s' if it is already a home, or choose another location", path, name, path)
	}
	fmt.Fprintf(c.Stdout, "\nGit identity for home '%s':\n", name)
	gitName, err := ask(c, "  Name: ")
	if err != nil {
		return err
	}
	gitEmail, err := ask(c, "  Email: ")
	if err != nil {
		return err
	}

	real, err := createHome(c, m, name, template, path, gitName, gitEmail)
	if err != nil {
		switch {
		case !exists(fs, path):
			return fmt.Errorf("%v\nError: creating the home failed; nothing was written or registered", err)
		case fs.RemoveAll(path) == nil:
			return fmt.Errorf("%v\nError: creating the home failed; %s was removed and nothing is registered", err, path)
		}
		return fmt.Errorf("%v\nError: creating the home failed and %s could not be removed; remove it by hand, nothing is registered", err, path)
	}

	built, err := prompt.Build(fs, prompt.Spec{Root: m.Root, HomeName: name, Home: real, Output: filepath.Join(real, "AGENTS.md"), Rebuild: "daisy prompt build " + name, Built: c.Ops.Now()})
	if err == nil {
		for _, w := range built.Warnings {
			fmt.Fprintln(c.Stderr, w)
		}
		err = fs.WriteFile(filepath.Join(real, "AGENTS.md"), built.Text, 0o644)
	}
	if err != nil {
		fmt.Fprintf(c.Stderr, "%v\n  ⚠ AGENTS.md not built; run 'daisy prompt build %s'\n", err, name)
	}
	fmt.Fprintf(c.Stdout, "\n✅ Created home: %s\n   Location: %s\n\n📋 Next steps:\n   1. Edit %s/include.txt to choose which prompts to load\n      Common prompts: daisy, retrospective, github\n      Work-specific: add your own under %s/prompts/\n   2. To use in a workspace: daisy init %s\n", name, real, real, real, name)
	return nil
}

// createHome writes the home and registers it; the first commit is the
// identity check (git's own message says what is missing).
func createHome(c *Context, m env.Env, name, template, path, gitName, gitEmail string) (string, error) {
	fs := c.Ops.FS()
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("Error: %v", err)
	}
	if err := ops.CopyTree(fs, template, path); err != nil {
		return "", fmt.Errorf("Error: copying the template: %v", err)
	}
	real := env.RealDir(fs, path)
	if gitName != "" && gitEmail != "" {
		if err := fs.WriteFile(filepath.Join(real, "gitconfig"), []byte("name="+gitName+"\nemail="+gitEmail+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("Error: %v", err)
		}
		fmt.Fprintf(c.Stdout, "  ✓ Saved git identity to %s/gitconfig\n", real)
	} else {
		fmt.Fprintf(c.Stdout, "  ⚠ Skipped (both name and email required). Edit %s/gitconfig later.\n", real)
	}
	if err := gitx.Init(c.Ops, real); err != nil {
		return "", fmt.Errorf("Error: git init in %s failed: %v", real, err)
	}
	var steps [][]string
	if gitName != "" && gitEmail != "" {
		steps = append(steps, []string{"config", "user.name", gitName}, []string{"config", "user.email", gitEmail})
	}
	steps = append(steps, []string{"add", "-A"}, []string{"commit", "-q", "-m", "New home: " + name})
	for _, args := range steps {
		if _, errb, err := c.Ops.Exec(real, "git", args...); err != nil {
			return "", fmt.Errorf("Error: git %s in %s failed: %s", args[0], real, strings.TrimSpace(string(errb)+" "+err.Error()))
		}
	}
	fmt.Fprintln(c.Stdout, "  ✓ Initialized git repository with first commit")
	return real, env.Register(fs, m.HomesFile(), name, real)
}

// HomeNewHealth: the template `home new` copies is there.
var HomeNewHealth = readOnly(func(c *Context) error {
	root, err := env.Root(c.Ops)
	if err != nil {
		return err
	}
	if fi, err := c.Ops.FS().Stat(filepath.Join(root, "templates", "home")); err != nil || !fi.IsDir() {
		return errors.New("Error: home template not found at templates/home/")
	}
	return nil
})

// IdentityList prints every identity across the registered homes and the
// home it belongs to. Read-only.
var IdentityList = readOnly(func(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	io.WriteString(c.Stdout, identity.FormatList(identity.List(c.Ops.FS(), m), ""))
	if _, problems := env.UsableHomes(c.Ops.FS(), m.Root, m.HomesFile()); len(problems) > 0 {
		var b strings.Builder
		b.WriteString("Error: these registered homes cannot be read, so their identities are not listed:")
		for _, p := range problems {
			fmt.Fprintf(&b, "\n  home '%s' is registered at %s, which %s", p.Name, p.Path, p.Problem)
		}
		return errors.New(b.String())
	}
	return nil
})

func unregisteredName(fs ops.FS, m env.Env, name string) error {
	if !env.NameOK(name) {
		return fmt.Errorf("Error: '%s' is not a valid home name (letters, digits, '.', '_', '-'; starting with a letter or digit)", name)
	}
	if existing, ok := env.HomePath(fs, m.HomesFile(), name); ok {
		return fmt.Errorf("Error: Home '%s' is already registered at %s\n  Run 'daisy home remove %s' first to register it elsewhere", name, existing, name)
	}
	return nil
}

// ask prompts on stderr, as the bash's `read -p` did, and refuses a closed stdin.
func ask(c *Context, question string) (string, error) {
	answer, err := c.Ops.Prompt(c.Stdin, c.Stderr, question)
	if err != nil {
		return "", errors.New("\nError: no answer to the prompt (stdin closed) — pass the value as an argument, or run interactively")
	}
	return answer, nil
}

func exists(fs ops.FS, p string) bool {
	_, err := fs.Lstat(p)
	return err == nil
}
