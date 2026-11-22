package commands

import (
	"fmt"

	"daisy/internal/env"
)

// Version prints the commit this binary was built from: the stamp the Go
// toolchain writes into a binary built inside a git checkout (`make build`,
// or any `go build` there), so no build step has to pass it in. `make
// install-dev` compares it with the root's HEAD; init stamps a workspace
// with it (env.Commit).
func Version(c *Context) error {
	line, err := env.Commit(c.Ops)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, line)
	return nil
}
