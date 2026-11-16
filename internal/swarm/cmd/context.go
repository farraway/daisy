// Package cmd holds the swarm's verbs, one function per `daisy swarm`
// command (docs/swarm.md section 11), the cli's table pointing at them as
// at internal/commands'. Each takes the Context the cli resolves and
// writes to its streams; the error it returns is the message for stderr.
// A verb the cli table marks ReadOnly runs under ops.ReadOnly as a whole;
// a verb narrows itself to ReadOnly here only for a form of it (`swarm
// journal` without --mark, `swarm prompt build --dry-run`).
package cmd

import (
	"fmt"

	"daisy/internal/verb"
)

// The frame a verb runs in is internal/verb's, shared with
// internal/commands; its three names every verb spells, here as they
// read there.
type (
	Context    = verb.Context
	Value      = verb.Value
	UsageError = verb.UsageError
)

// plural is n and the noun for it.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
