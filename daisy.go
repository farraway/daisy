// Package daisy is the whole program behind the daisy binary: Run takes an
// Ops and the three streams, so main and the functional tests call the same
// thing. Exit code 0 ok, 1 failure, 2 usage.
package daisy

import (
	"io"

	"daisy/internal/cli"
	"daisy/internal/ops"
)

// Ops is every side effect the program may have; see internal/ops.
type Ops = ops.Ops

// Run runs daisy with args (without the program name).
func Run(o Ops, stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	return cli.Run(o, stdin, stdout, stderr, args)
}
