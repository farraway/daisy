// Command daisy is the Daisy CLI.
package main

import (
	"os"

	"daisy"
	"daisy/internal/ops"
)

func main() {
	os.Exit(daisy.Run(ops.Real(), os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}
