// Package commands holds one function per daisy command. Each takes the
// resolved Context and writes to its streams; the error it returns is the
// message for stderr, and the command exits 1. Which Ops wrapper a command
// runs under is decided here, by the command, and nowhere else.
package commands

import (
	"errors"
	"io"
	"slices"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// Context is what a command runs with. Flags are the command's own flags
// the cli found leading its arguments, in order, and Values those that
// took a value; Args are the positional arguments after them.
type Context struct {
	Ops    ops.Ops
	Env    env.Env
	Swarm  declare.Place // the swarm a `swarm` verb runs on (docs/swarm.md section 11, Finding the swarm)
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Flags  []string
	Values []Value
	Args   []string
	// EmptyWords: positional words were given and some were empty, which
	// the cli drops from Args (`feedback ""` is an empty message, not
	// none).
	EmptyWords bool
	// Components are the cli table's --healthcheck hooks, in table order,
	// and Reference the Command Reference it renders: what the health
	// check runs and compares AGENTS.md with.
	Components []Component
	Reference  string
}

// Component is a command's --healthcheck hook under the command's name.
type Component struct {
	Name  string
	Check func(*Context) error
}

// ErrReported is a failure the command has already explained on its
// streams: exit 1 with nothing more said.
var ErrReported = errors.New("failure already reported")

// Value is a value flag as given: `--workflow plan`.
type Value struct{ Flag, Value string }

// Flag: the flag was given.
func (c *Context) Flag(name string) bool { return slices.Contains(c.Flags, name) }

// Value is the value of the last of the named flags given (a flag and its
// short form are one flag), and whether one was.
func (c *Context) Value(names ...string) (string, bool) {
	v, ok := "", false
	for _, f := range c.Values {
		if slices.Contains(names, f.Flag) {
			v, ok = f.Value, true
		}
	}
	return v, ok
}

// UsageError is a command's own usage error: the cli prints it with the
// command's usage and exits 2, as for one it finds itself. For a rule the
// table cannot state (`optimize apply` takes --rules or --rewrite, not
// both).
type UsageError string

func (u UsageError) Error() string { return string(u) }

// readOnly runs the command under ops.ReadOnly: the usage text's promise
// becomes a checked invariant.
func readOnly(run func(*Context) error) func(*Context) error {
	return func(c *Context) error {
		c.Ops = ops.ReadOnly(c.Ops)
		return run(c)
	}
}
