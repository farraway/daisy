package commands

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"daisy/internal/env"
	"daisy/internal/health"
)

// Healthcheck runs the system health check (internal/health) and exits 1
// when it finds an issue, which it has already reported. Real: a pass
// writes the home's TTL stamp.
func Healthcheck(c *Context) error {
	if !checkHealth(c, c.Stdout, c.Stderr, c.Flag("--force")) {
		return ErrReported
	}
	return nil
}

// checkHealth runs the health check with the table's hooks as its
// components: each hook is called in-process with the resolved
// workspace, as `daisy <command> --healthcheck` would run it.
func checkHealth(c *Context, stdout, stderr io.Writer, force bool) bool {
	var components []health.Component
	for _, comp := range c.Components {
		components = append(components, health.Component{Name: comp.Name, Check: func(e env.Env) error {
			var out bytes.Buffer
			sub := &Context{Ops: c.Ops, Env: e, Stdin: c.Stdin, Stdout: &out, Stderr: &out}
			if err := comp.Check(sub); err != nil {
				if s := strings.TrimRight(out.String(), "\n"); s != "" {
					return errors.New(s + "\n" + err.Error())
				}
				return err
			}
			return nil
		}})
	}
	return health.Run(c.Ops, stdout, stderr, health.Options{Force: force, Components: components, Reference: c.Reference})
}

// TestHealth: the acceptance suite's runner is there and executable.
var TestHealth = readOnly(func(c *Context) error {
	root, err := env.Root(c.Ops)
	if err != nil {
		return err
	}
	if fi, err := c.Ops.FS().Stat(filepath.Join(root, "tests", "run.sh")); err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return errors.New("Error: test runner not found at tests/run.sh")
	}
	return nil
})
