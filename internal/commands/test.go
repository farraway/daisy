package commands

import (
	"fmt"
	"path/filepath"

	"daisy/internal/env"
)

// Test builds bin/daisy from the root's source (`make build`), runs `go
// test ./...` in the root, then the acceptance suite, tests/run.sh, with
// the root's bin/ first on PATH so the cases drive the binary just built.
// A filter names the cases to run (a substring of the file name) and
// skips `go test`: it asks for those cases. Each stage streams its output;
// the first that fails is the exit 1.
func Test(c *Context) error {
	root, err := env.Root(c.Ops)
	if err != nil {
		return err
	}
	run := func(name string, args ...string) (bool, error) {
		p, err := c.Ops.Start(root, name, c.Stdin, c.Stdout, c.Stderr, args...)
		if err != nil {
			return false, fmt.Errorf("Error: cannot run %s: %v", name, err)
		}
		code, err := p.Wait()
		if err != nil {
			return false, fmt.Errorf("Error: %s did not exit normally: %v", name, err)
		}
		return code == 0, nil
	}
	if ok, err := run("make", "-C", root, "--silent", "build"); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("Error: could not build bin/daisy in %s (see above)", root)
	}
	if len(c.Args) == 0 {
		if ok, err := run("go", "test", "./..."); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("Error: go test failed in %s (see above); the acceptance suite was not run", root)
		}
	}
	path := filepath.Join(root, "bin")
	if p := c.Ops.Getenv("PATH"); p != "" {
		path += ":" + p
	}
	ok, err := run("env", append([]string{"PATH=" + path, filepath.Join(root, "tests", "run.sh")}, c.Args...)...)
	if err != nil {
		return err
	}
	if !ok {
		return ErrReported
	}
	return nil
}
