// Package commands holds one function per daisy command but the swarm's
// verbs (internal/swarm/cmd). Each takes the resolved Context and writes
// to its streams; the error it returns is the message for stderr, and the
// command exits 1. A command the cli table marks ReadOnly runs under
// ops.ReadOnly as a whole, the cli wrapping it; a --healthcheck hook runs
// under verb.ReadOnly.
package commands

import (
	"fmt"

	"daisy/internal/verb"
)

// The frame a command runs in is internal/verb's, shared with the swarm's
// verbs; its three names every command spells, here as they read there.
type (
	Context    = verb.Context
	Value      = verb.Value
	UsageError = verb.UsageError
)

// printNext ends a command's success output on the newcomer's path
// (install, home new, home add, init) with the next command to run, after
// a blank line: clig.dev's "suggest the next command".
func printNext(c *Context, command string) {
	fmt.Fprintf(c.Stdout, "\nNext: %s\n", command)
}
