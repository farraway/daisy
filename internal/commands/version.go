package commands

import (
	"errors"
	"fmt"
	"runtime/debug"
)

// Version prints the commit this binary was built from: the stamp the Go
// toolchain writes into a binary built inside a git checkout (`make build`,
// or any `go build` there), so no build step has to pass it in. `make
// install-dev` compares it with the root's HEAD.
var Version = readOnly(func(c *Context) error {
	info, _ := debug.ReadBuildInfo()
	line, err := versionLine(info)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, line)
	return nil
})

// versionLine is vcs.revision, then ` (modified)` when vcs.modified is
// true: git status showed changes at build time, an untracked file
// included, so the binary may hold code HEAD does not.
func versionLine(info *debug.BuildInfo) (string, error) {
	rev, modified := "", false
	if info != nil {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
	}
	if rev == "" {
		return "", errors.New("Error: this binary carries no commit: it was built outside a git checkout, with -buildvcs=false, or by go test or go run\n  Build it with 'make build' in the root")
	}
	if modified {
		return rev + " (modified)", nil
	}
	return rev, nil
}
