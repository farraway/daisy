// Command daisy is the Daisy CLI: cli.Run with the real Ops and the three
// streams, the same call the functional tests in this directory make with a
// Fake. Exit code 0 ok, 1 failure, 2 usage.
package main

import (
	"os"

	"daisy/internal/cli"
	"daisy/internal/ops"
)

func main() {
	os.Exit(cli.Run(ops.Real(), os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}
