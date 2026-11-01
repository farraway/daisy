// Package commands holds one function per daisy command. Each takes the
// resolved Context and writes to its streams; the error it returns is the
// message for stderr, and the command exits 1. Which Ops wrapper a command
// runs under is decided here, by the command, and nowhere else.
package commands

import (
	"io"

	"daisy/internal/env"
	"daisy/internal/ops"
)

// Context is what a command runs with. Args are the positional arguments
// left after the cli parsed the command's flags.
type Context struct {
	Ops    ops.Ops
	Env    env.Env
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Args   []string
}

// readOnly runs the command under ops.ReadOnly: the usage text's promise
// becomes a checked invariant.
func readOnly(run func(*Context) error) func(*Context) error {
	return func(c *Context) error {
		c.Ops = ops.ReadOnly(c.Ops)
		return run(c)
	}
}
