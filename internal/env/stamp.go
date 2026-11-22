package env

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"

	"daisy/internal/ops"
)

// StampFile is the workspace's stamp under .daisy/: the commit `daisy
// version` reported for the binary that last ran init there.
const StampFile = "version"

// Commit is the commit the running binary was built from, as `daisy
// version` prints it: vcs.revision, then ` (modified)` when vcs.modified
// is true (git status showed changes at build time, an untracked file
// included, so the binary may hold code HEAD does not). The error says
// why there is none.
func Commit(o ops.Ops) (string, error) {
	info, _ := o.BuildInfo()
	return commitLine(info)
}

func commitLine(info *debug.BuildInfo) (string, error) {
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

// StampNotice is the notice for the workspace ws whose stamp is not the
// running binary's commit, a missing stamp included (a workspace bound
// before init wrote one); "" when it is, or when the binary carries none
// to compare. `status` and `healthcheck` print it, each with its mark.
func StampNotice(o ops.Ops, ws string) string {
	commit, err := Commit(o)
	if err != nil {
		return ""
	}
	b, err := o.FS().ReadFile(filepath.Join(ws, ".daisy", StampFile))
	if err != nil {
		return fmt.Sprintf("The workspace has no .daisy/%s stamp, so it was initialized before this Daisy (%s): run 'daisy init' here to refresh it", StampFile, commit)
	}
	if stamp := strings.TrimSuffix(string(b), "\n"); stamp != commit {
		return fmt.Sprintf("The workspace was initialized by Daisy %s, not this one (%s): run 'daisy init' here to refresh it", stamp, commit)
	}
	return ""
}
