package reconcile

import (
	"bytes"
	"errors"
	"fmt"
	iofs "io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"daisy/internal/ops"
)

// Run is one role run's directory, `<state>/runs/<n>-<role>-<k>` (section 2).
type Run struct {
	Dir  string
	N, K int
	Role string
}

var runName = regexp.MustCompile(`^(0|[1-9][0-9]*)-(owner|architect|worker|adversary|sympathetic|qa)-([1-9][0-9]*)$`)

// Runs is every run directory under state, in no particular order.
func Runs(fs ops.FS, state string) ([]Run, error) {
	entries, err := fs.ReadDir(filepath.Join(state, "runs"))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, e := range entries {
		m := runName.FindStringSubmatch(e.Name())
		if !e.IsDir() || m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		k, _ := strconv.Atoi(m[3])
		out = append(out, Run{filepath.Join(state, "runs", e.Name()), n, k, m[2]})
	}
	return out, nil
}

// Output is the run's `io/out.md`, read only when it is a regular file
// (section 7.5): the member wrote it, and a link it left is never
// followed, so it cannot make the host read and post a file of the
// host's. A link or anything else not a regular file is
// iofs.ErrNotExist: no output.
func (r Run) Output(fs ops.FS) ([]byte, error) { return r.ioFile(fs, "out.md") }

// Builder is the architect run's builder plan, `io/builder.md` (section
// 7.8, The builder plan), read as Output reads out.md.
func (r Run) Builder(fs ops.FS) ([]byte, error) { return r.ioFile(fs, "builder.md") }

// ioFile is the member's file name in the run's io, read only when it is
// a regular file (Output).
func (r Run) ioFile(fs ops.FS, name string) ([]byte, error) {
	p := filepath.Join(r.Dir, "io", name)
	fi, err := fs.Lstat(p)
	if err != nil {
		return nil, r.unusable(path.Join("io", name), err)
	}
	if !fi.Mode().IsRegular() {
		return nil, iofs.ErrNotExist
	}
	b, err := fs.ReadFile(p)
	return b, r.unusable(path.Join("io", name), err)
}

// An unusableError is a file of a job, one of its runs' or of its
// workspace, that Daisy cannot use: there and not readable (section
// 7.7, a job's file Daisy cannot use). It is the job's
// own failure, which blocks its issue alone (blockUnusable), where any
// other error, a forge's, a mirror's or a write's, is the pass's.
type unusableError struct {
	file string // from <state>: runs/<run>/io/out.md, jobs/<n>/texts/<run>.md
	k    int    // the number of the run whose file it is, 0 for none
	err  error
}

func (e *unusableError) Error() string { return e.err.Error() }
func (e *unusableError) Unwrap() error { return e.err }

// unusable is err, the failed read of the run's file rel, as the job's
// own (unusableError); nil and a file not there (iofs.ErrNotExist) as
// they are.
func (r Run) unusable(rel string, err error) error {
	if err == nil || errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	run, _ := runOf(r.Dir)
	return &unusableError{file: path.Join("runs", filepath.Base(r.Dir), rel), k: run.K, err: err}
}

// Finished reads how a run another apply started ended, its container
// gone (section 7.7): ended, when its member finished, with the lines its
// exit status adds (a fault for 125 to 127, a failure for any other
// non-zero status, as a container of this process's); else abandoned.
// A run launched under an exit mark, attached (attachedExitScript) or
// detached (exitScript), finished when its `io/exit` is a regular file
// holding a status, whatever out.md holds; one launched before the mark
// (its argv without either) when its out.md is a regular file.
func (r Run) Finished(fs ops.FS) (ended bool, lines []string) {
	argv, _ := fs.ReadFile(filepath.Join(r.Dir, "argv"))
	if !bytes.Contains(argv, []byte(exitScript)) && !bytes.Contains(argv, []byte(attachedExitScript)) {
		_, err := r.Output(fs)
		return err == nil, nil
	}
	code, ok := r.exitStatus(fs)
	switch {
	case !ok:
		return false, nil
	case code >= 125 && code <= 127:
		return true, []string{fmt.Sprintf("fault: exit %d", code)}
	case code != 0:
		return true, []string{fmt.Sprintf("failed: exit %d", code)}
	}
	return true, nil
}

// stepFile is the step's file name in <run>/step/, read only when it is
// a regular file, as ioFile reads the member's.
func (r Run) stepFile(fs ops.FS, name string) ([]byte, error) {
	p := filepath.Join(r.Dir, "step", name)
	fi, err := fs.Lstat(p)
	if err != nil {
		return nil, r.unusable(path.Join("step", name), err)
	}
	if !fi.Mode().IsRegular() {
		return nil, iofs.ErrNotExist
	}
	b, err := fs.ReadFile(p)
	return b, r.unusable(path.Join("step", name), err)
}

// stepStatus is the status a step's sh wrote to <run>/step/exit, ok when
// it is a regular file holding a number (section 7.10, rule 2).
func (r Run) stepStatus(fs ops.FS) (code int, ok bool) {
	b, err := r.stepFile(fs, "exit")
	if err != nil || b == nil {
		return 0, false
	}
	code, err = strconv.Atoi(strings.TrimSpace(string(b)))
	return code, err == nil
}

// exitStatus is the status the member's sh wrote to `io/exit`, ok when
// it is a regular file holding a number (section 7.2): one that is not
// the script's, the member's own, is none.
func (r Run) exitStatus(fs ops.FS) (code int, ok bool) {
	b, err := r.ioFile(fs, "exit")
	if err != nil || b == nil {
		return 0, false
	}
	code, err = strconv.Atoi(strings.TrimSpace(string(b)))
	return code, err == nil
}

// Result is the run's result lines; none when it has no result yet.
func (r Run) Result(fs ops.FS) ([]string, error) {
	b, err := fs.ReadFile(filepath.Join(r.Dir, "result"))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, r.unusable("result", err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"), nil
}

// Append adds lines to the run's result.
func (r Run) Append(fs ops.FS, lines ...string) error {
	old, err := fs.ReadFile(filepath.Join(r.Dir, "result"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	// One line each, whatever a line quotes (a member's path may hold a
	// newline): the file is read line by line.
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(oneLine.Replace(l) + "\n")
	}
	return fs.WriteFile(filepath.Join(r.Dir, "result"), append(old, b.String()...), 0o644)
}

var oneLine = strings.NewReplacer("\r", `\r`, "\n", `\n`)

// endWords are the first words of a result's end line (section 2):
// `ended` a container another apply started, gone, whose member finished
// (Finished; its streams died with that apply).
var endWords = []string{"exited", "timeout", "launch-failed", "abandoned", "ended"}

// Ended is the run's end line's first word, "" when it has none.
func Ended(lines []string) string {
	for _, l := range lines {
		w, _, _ := strings.Cut(l, " ")
		for _, e := range endWords {
			if w == e {
				return w
			}
		}
	}
	return ""
}

// Has: the result holds the line, or a line beginning with prefix+" " or
// prefix+": ".
func Has(lines []string, prefix string) (string, bool) {
	for _, l := range lines {
		if l == prefix || strings.HasPrefix(l, prefix+" ") || strings.HasPrefix(l, prefix+": ") {
			return l, true
		}
	}
	return "", false
}

// NextK is a run's number (section 6): one more than the largest among
// the role's runs on n under state and the marks on the issue and its pull
// requests.
func NextK(runs []Run, n int, role string, marks []Mark) int {
	k := 0
	for _, r := range runs {
		if r.N == n && r.Role == role {
			k = max(k, r.K)
		}
	}
	for _, m := range marks {
		if m.Role == role {
			k = max(k, m.Run)
		}
	}
	return k + 1
}
