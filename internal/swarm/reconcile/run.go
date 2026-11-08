package reconcile

import (
	"errors"
	iofs "io/fs"
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

var runName = regexp.MustCompile(`^(0|[1-9][0-9]*)-(owner|architect|worker|adversary|sympathetic)-([1-9][0-9]*)$`)

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
func (r Run) Output(fs ops.FS) ([]byte, error) {
	p := filepath.Join(r.Dir, "io", "out.md")
	fi, err := fs.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, iofs.ErrNotExist
	}
	return fs.ReadFile(p)
}

// Result is the run's result lines; none when it has no result yet.
func (r Run) Result(fs ops.FS) ([]string, error) {
	b, err := fs.ReadFile(filepath.Join(r.Dir, "result"))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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

// endWords are the first words of a result's end line (section 2).
var endWords = []string{"exited", "timeout", "launch-failed", "abandoned"}

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
