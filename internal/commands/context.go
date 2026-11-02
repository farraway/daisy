// Package commands holds one function per daisy command. Each takes the
// resolved Context and writes to its streams; the error it returns is the
// message for stderr, and the command exits 1. Which Ops wrapper a command
// runs under is decided here, by the command, and nowhere else.
package commands

import (
	"io"
	"slices"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Context is what a command runs with. Flags are the command's own flags
// the cli found leading its arguments, in order; Args are the positional
// arguments after them. Values are the flags that take a value, by name.
type Context struct {
	Ops    ops.Ops
	Env    env.Env
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Flags  []string
	Args   []string
	Values map[string]string
}

// Flag: the flag was given.
func (c *Context) Flag(name string) bool { return slices.Contains(c.Flags, name) }

// Value: the value given for a flag that takes one, and whether it was.
func (c *Context) Value(name string) (string, bool) {
	v, ok := c.Values[name]
	return v, ok
}

// readOnly runs the command under ops.ReadOnly: the usage text's promise
// becomes a checked invariant.
func readOnly(run func(*Context) error) func(*Context) error {
	return func(c *Context) error {
		c.Ops = ops.ReadOnly(c.Ops)
		return run(c)
	}
}
