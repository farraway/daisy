package commands

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"

	"daisy/internal/gitx"
	"daisy/internal/health"
)

// Paths into the home every day command shares.
func (c *Context) todayFile() string { return filepath.Join(c.Env.Home, "journal", "today.md") }
func (c *Context) todoFile() string  { return filepath.Join(c.Env.Home, "tasks", "todo.txt") }
func (c *Context) doneFile() string  { return filepath.Join(c.Env.Home, "tasks", "done.txt") }
func (c *Context) journalFile() string {
	return filepath.Join(c.Env.Home, "journal", "journal.md")
}

// preflight runs the system health check, in-process and silently,
// before a mutation; a cached pass or DAISY_HEALTHCHECK_PASSED=1 makes it
// a no-op.
func preflight(c *Context) error {
	if !checkHealth(c, io.Discard, io.Discard, false) {
		return errors.New("Error: System health check failed\n  Run: daisy healthcheck")
	}
	// A cached pass or the override skips the check, never this: nothing
	// is written into a home that cannot commit.
	_, err := homeRepo(c)
	return err
}

// commitHome commits the home's changes once. committed is false, without
// an error, when there was nothing to commit.
func commitHome(c *Context, message string) (hash string, committed bool, err error) {
	repo, err := homeRepo(c)
	if err != nil {
		return "", false, err
	}
	return repo.Commit(c.Ops, message, coAuthor(c))
}

// CoAuthorFlag is the value flag every command that commits takes.
const CoAuthorFlag = "--co-author"

// coAuthor is the Co-Authored-By trailer every commit Daisy makes ends
// with: the command's --co-author, which an agent running Daisy gives
// its model's line, `Name <email>` (the cli refuses another shape), or
// else DAISY_CO_AUTHOR; with neither, none (a person at the keyboard).
// An environment value of another shape is warned about and left out,
// and the commit goes on.
func coAuthor(c *Context) string {
	if v, ok := c.Value(CoAuthorFlag); ok {
		return v
	}
	v := c.Ops.Getenv("DAISY_CO_AUTHOR")
	if v == "" || IsCoAuthor(v) {
		return v
	}
	fmt.Fprintf(c.Stderr, "⚠ DAISY_CO_AUTHOR %q is not one line 'Name <email>': committing without Co-Authored-By\n", v)
	return ""
}

// IsCoAuthor is whether v is one line `Name <email>`.
func IsCoAuthor(v string) bool { return coAuthorLine.MatchString(v) }

var coAuthorLine = regexp.MustCompile(`^[^<>\s][^<>\r\n]* <[^<>\s]+@[^<>\s]+>$`)

// homeRepo is where the home commits; a command that writes before it
// commits checks it first.
func homeRepo(c *Context) (gitx.Repo, error) {
	repo, refusal := gitx.HomeRepo(c.Ops, c.Env.Root, c.Env.Home)
	if refusal != gitx.Accepted {
		return gitx.Repo{}, fmt.Errorf("Error: Home '%s' at %s is not in its own git repository%s\n  Run: %s", c.Env.HomeName, c.Env.Home, refusal.Why(), refusal.Remedy(c.Env.Root, c.Env.Home, c.Env.HomeName, health.ShellQuote))
	}
	return repo, nil
}

// The --healthcheck hooks: each command's own preconditions, nothing on
// success, an Error line otherwise (the day templates' hooks add where they
// looked). `daisy healthcheck` runs them.
var (
	LogHealth      = readOnly(func(c *Context) error { return requireFiles(c, c.todayFile()) })
	DoneHealth     = readOnly(func(c *Context) error { return requireFiles(c, c.todayFile(), c.todoFile()) })
	TasksHealth    = readOnly(func(c *Context) error { return requireDir(c, filepath.Join(c.Env.Home, "tasks")) })
	ProjectsHealth = readOnly(func(c *Context) error { return requireDir(c, filepath.Join(c.Env.Home, "projects")) })
)

func requireFiles(c *Context, paths ...string) error {
	for _, p := range paths {
		if fi, err := c.Ops.FS().Stat(p); err != nil || !fi.Mode().IsRegular() {
			return fmt.Errorf("Error: %s not found", filepath.Base(p))
		}
	}
	return nil
}

func requireDir(c *Context, dir string) error {
	if fi, err := c.Ops.FS().Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("Error: %s directory not found", filepath.Base(dir))
	}
	return nil
}
