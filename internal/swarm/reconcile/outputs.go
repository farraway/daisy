package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
)

// Outputs is where Daisy finds whole the output of a run whose text the
// forge holds cut (sections 7.3 and 7.5): the swarm's <state>, read
// through FS. The zero Outputs finds none.
type Outputs struct {
	FS    ops.FS
	State string
}

// Text is run `<n>-<role>-<k>`'s output whole, its out.md as Output
// reads it (a regular file, never through a link). ok is false, with no
// error, when the output cannot be given whole: out.md missing or not a
// regular file, the run failed for a secret in its output (never
// posted), a name not a run's, the zero Outputs; a failed read is err.
func (o Outputs) Text(run string) (text string, ok bool, err error) {
	if o.FS == nil || !runName.MatchString(run) {
		return "", false, nil
	}
	r := Run{Dir: filepath.Join(o.State, "runs", run)}
	out, err := r.Output(o.FS)
	if errors.Is(err, iofs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	lines, err := r.Result(o.FS)
	if err != nil {
		return "", false, err
	}
	if slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, "failed: secret in output") }) {
		return "", false, nil
	}
	return string(out), true, nil
}

// copyOutputs puts the output of each run the prompt shows cut (Cut) in
// the member's io, as /job/io/outputs/<run>.md in its container:
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
