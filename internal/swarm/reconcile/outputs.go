package reconcile

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
)

// Outputs is the runs on issue n whose output a later member can be
// given whole when the forge holds it cut (section 7.3): each whose
// <run>/io/out.md is a regular file, but a run failed for a secret in its
// output, never posted, each `<n>-<role>-<k>`.
func Outputs(fs ops.FS, runs []Run, n int) []string {
	var out []string
	for _, r := range runs {
		if r.N != n || n == 0 {
			continue
		}
		if fi, err := fs.Lstat(filepath.Join(r.Dir, "io", "out.md")); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		lines, _ := r.Result(fs)
		if slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, "failed: secret in output") }) {
			continue
		}
		out = append(out, filepath.Base(r.Dir))
	}
	return out
}

// copyOutputs puts the output of each run the prompt shows cut (Cut) in
// the member's io, as /work/.swarm/outputs/<run>.md in its container:
// read from <state>/runs/<run>/io/out.md only as a regular file, never
// through a link (Output).
func copyOutputs(fs ops.FS, state, ioDir string, cut []string) error {
	if len(cut) == 0 {
		return nil
	}
	dir := filepath.Join(ioDir, "outputs")
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, run := range cut {
		data, err := (Run{Dir: filepath.Join(state, "runs", run)}).Output(fs)
		if err == nil {
			err = fs.WriteFile(filepath.Join(dir, run+".md"), data, 0o644)
		}
		if err != nil {
			return fmt.Errorf("output %s: %w", run, err)
		}
	}
	return nil
}
