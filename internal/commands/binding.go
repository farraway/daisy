package commands

import (
	"fmt"
	"path/filepath"

	"daisy/internal/identity"
)

// header is the first line status and files share.
func header(c *Context) string {
	return fmt.Sprintf("Daisy | home: %s | identity: %s | workspace: %s", c.Env.HomeName, c.Env.Identity, c.Env.Workspace)
}

// reportDrift prints, on stderr, each way the binding has drifted from
// what init wrote, with the fix.
func reportDrift(c *Context) {
	e := c.Env
	drift := identity.Drift(c.Ops, e.Workspace, filepath.Join(e.Workspace, ".daisy"), e.IdentityFile)
	drift = append(drift, e.LinkDrift(c.Ops.FS())...)
	for _, line := range drift {
		fmt.Fprintf(c.Stderr, "⚠️  %s; re-run 'daisy init %s'\n", line, e.Identity)
	}
	for _, w := range e.Warnings {
		fmt.Fprintln(c.Stderr, w)
	}
}
