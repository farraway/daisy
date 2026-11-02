// Package health is the system health check behind `daisy healthcheck`
// and the pre-flight of every day command: the root and the home resolve
// and are repositories, the required directories exist, today.md keeps its
// format, every command's own --healthcheck hook passes, the publishable
// surface of the root holds no identifying term, and AGENTS.md's Command
// Reference is the cli table's. A pass is cached in the home's
// .healthcheck.ttl; the cache fails toward re-running.
package health

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"daisy/internal/env"
	"daisy/internal/gitx"
	"daisy/internal/ops"
	"daisy/internal/skill"
)

// Component is one `Component:` line: a command's --healthcheck hook, run
// in-process against the resolved workspace.
type Component struct {
	Name  string
	Check func(env.Env) error
}

// Options are a run's inputs from the command line and the cli table.
type Options struct {
	Force      bool
	Components []Component
	// Reference is the Command Reference block AGENTS.md must hold, markers
	// included (`daisy help --markdown`).
	Reference string
}

// DefaultTTL is how long a pass is cached, in seconds, when
// DAISY_HEALTHCHECK_TTL is unset or not a number; 0 disables caching.
const DefaultTTL = 300

// StampName is the cache file in the home: the unix time the pass expires.
const StampName = ".healthcheck.ttl"

// The marks the bash printed, colours included, whatever the stream is.
const (
	okMark   = "\033[0;32m✓\033[0m "
	warnMark = "\033[1;33m⚠\033[0m "
	failMark = "\033[0;31m✗\033[0m "
)

type run struct {
	o         ops.Ops
	out, errw io.Writer
	issues    int
	stamp     string
	fs        ops.FS
	root      string
	e         env.Env
}

func (r *run) ok(format string, a ...any)   { fmt.Fprintf(r.out, okMark+format+"\n", a...) }
func (r *run) warn(format string, a ...any) { fmt.Fprintf(r.errw, warnMark+format+"\n", a...) }
func (r *run) fail(format string, a ...any) {
	fmt.Fprintf(r.errw, failMark+format+"\n", a...)
	r.issues++
}

// Run runs the health check and says whether the system is healthy. A pass
// writes the stamp (best effort: a read-only home only means the next run
// re-checks); a failure, wherever it stops the run, removes it, so no
// cached pass outlives a failure. DAISY_HEALTHCHECK_PASSED=1 from outside
// is a pass without a look, unless Force.
func Run(o ops.Ops, stdout, stderr io.Writer, opt Options) bool {
	r := &run{o: o, out: stdout, errw: stderr, fs: o.FS()}
	ttl := TTL(o.Getenv("DAISY_HEALTHCHECK_TTL"))
	if !opt.Force && o.Getenv("DAISY_HEALTHCHECK_PASSED") == "1" {
		return true
	}

	// Checks 1–2 always run: a cache must never mask a broken root or an
	// unresolvable home, and the stamp lives in the home.
	root, err := env.Root(o)
	if err != nil {
		r.fail("DAISY_ROOT not set or invalid")
		fmt.Fprintln(stderr, "  Add to ~/.zshenv: export DAISY_ROOT=/path/to/daisy")
		return false
	}
	r.root = root
	r.ok("DAISY_ROOT: %s", root)
	e, err := env.Resolve(o)
	if err != nil {
		fmt.Fprintln(stderr, err)
		r.fail("Cannot resolve home for this workspace")
		return false
	}
	r.e = e
	r.ok("DAISY_HOME: %s (home: %s)", e.Home, e.HomeName)
	r.stamp = filepath.Join(e.Home, StampName)
	now := o.Now().Unix()
	if !opt.Force {
		if left, ok := Cached(r.fs, r.stamp, ttl, now); ok {
			r.ok("Checks cached — valid for %ds (--force to re-run)", left)
			return true
		}
	}

	// Check 3: root code commits in the root's repository, every data
	// mutation in the home's; without either the commit fails after the
	// files are written.
	if !gitx.IsRepoRoot(o, root) {
		r.fail("DAISY_ROOT is not a git repository")
		return r.failed()
	}
	if _, refusal := gitx.HomeRepo(o, root, e.Home); refusal != gitx.Accepted {
		r.fail("Home '%s' at %s is not in its own git repository%s", e.HomeName, e.Home, refusal.Why())
		fmt.Fprintf(stderr, "  Run: %s\n", refusal.Remedy(root, e.Home, e.HomeName, ShellQuote))
		return r.failed()
	}
	head, _, _ := o.Exec(root, "git", "rev-parse", "--short", "HEAD")
	r.ok("Git repository: %s", strings.TrimRight(string(head), "\n"))

	// Check 4: the directories the commands read from the root.
	for _, d := range []string{"prompts", "templates", "skills"} {
		if !isDir(r.fs, filepath.Join(root, d)) {
			r.fail("Missing directory: %s", d)
		}
	}
	r.skillDrift()

	// Check 5: today.md's format; 5b the journal's rotation (warnings).
	if r.today() {
		r.ok("today.md format valid")
	}
	r.rotation()

	// Check 6: every command's own hook.
	for _, c := range opt.Components {
		if err := c.Check(e); err != nil {
			r.fail("Component: %s failed health check", c.Name)
			for _, line := range strings.Split(err.Error(), "\n") {
				fmt.Fprintf(stderr, "  %s\n", line)
			}
			continue
		}
		r.ok("Component: %s", c.Name)
	}

	// Check 6c: publication hygiene; 6b the Command Reference (a warning).
	if r.hygiene() {
		r.ok("Publication hygiene: no identifying terms outside the homes")
	}
	if problem := ReferenceDrift(r.fs, root, opt.Reference); problem != "" {
		r.warn("%s", problem)
	}

	r.feedback()

	fmt.Fprintln(stdout)
	if r.issues > 0 {
		r.fail("%d issue(s) found", r.issues)
		return r.failed()
	}
	r.ok("System healthy")
	if ttl > 0 {
		_ = r.fs.WriteFile(r.stamp, []byte(strconv.FormatInt(now+ttl, 10)+"\n"), 0o644)
	}
	return true
}

// failed removes the stamp and reports the failure.
func (r *run) failed() bool {
	_ = r.fs.Remove(r.stamp)
	return false
}

// TTL is DAISY_HEALTHCHECK_TTL in seconds: DefaultTTL unless it is a
// decimal number.
func TTL(v string) int64 {
	if v == "" || strings.Trim(v, "0123456789") != "" {
		return DefaultTTL
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return DefaultTTL
	}
	return n
}

// Cached is the seconds left on an unexpired pass. Anything unparseable,
// or dated further ahead than ttl could produce (a corrupt write, a clock
// jump, a hand edit), is no cache at all.
func Cached(fs ops.FS, stamp string, ttl, now int64) (left int64, ok bool) {
	if ttl <= 0 {
		return 0, false
	}
	if fi, err := fs.Stat(stamp); err != nil || !fi.Mode().IsRegular() {
		return 0, false
	}
	b, err := fs.ReadFile(stamp)
	if err != nil {
		return 0, false
	}
	s := strings.TrimRight(string(b), "\n")
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	expiry, err := strconv.ParseInt(s, 10, 64)
	if err != nil || now >= expiry || expiry > now+ttl {
		return 0, false
	}
	return expiry - now, true
}

// skillDrift: Check 4b, a workspace skill with no source in the root or
// the home; 4c, a skill `daisy install` put in ~/.claude/skills whose
// source is gone, Claude Code's own directory aside (its sync writes only
// there, never in a workspace, so 4b has no exemption). Warnings: the next
// init or install --update prunes them.
func (r *run) skillDrift() {
	if r.e.Workspace != "" {
		known := skill.Discover(r.fs, filepath.Join(r.root, "skills"))
		for n, d := range skill.Discover(r.fs, filepath.Join(r.e.Home, "skills")) {
			known[n] = d
		}
		for _, n := range skillDirs(r.fs, filepath.Join(r.e.Workspace, ".claude", "skills")) {
			if known[n] == "" {
				r.warn("Installed skill '%s' has no matching source in skills/ — stale from a removed/renamed skill; re-run 'daisy init' or remove .claude/skills/%s manually", n, n)
			}
		}
	}
	dir := filepath.Join("/"+r.o.Getenv("HOME"), ".claude", "skills")
	known := skill.Discover(r.fs, filepath.Join(r.root, "skills"))
	for _, n := range skillDirs(r.fs, dir) {
		if known[n] == "" && n != skill.ClaudeCodeSynced {
			r.warn("Root-installed skill '%s' has no matching source in $DAISY_ROOT/skills — stale from a removed skill; re-run 'daisy install --update' or remove %s manually", n, filepath.Join(dir, n))
		}
	}
}

// skillDirs are the directories (or links to one) in dir, as `dir/*/`
// globs them: hidden names left out.
func skillDirs(fs ops.FS, dir string) []string {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, en := range entries {
		if !strings.HasPrefix(en.Name(), ".") && isDir(fs, filepath.Join(dir, en.Name())) {
			names = append(names, en.Name())
		}
	}
	return names
}

var (
	dayBlock      = regexp.MustCompile(`(?m)^### [0-9]{4}-[0-9]{2}-[0-9]{2}`)
	feedbackEntry = regexp.MustCompile(`(?m)^## [0-9]{4}-[0-9]{2}-[0-9]{2}`)
	windowFile    = regexp.MustCompile(`^journal-[0-9].*\.md$`)
	tocHeading    = regexp.MustCompile(`(?m)^## Table of Contents`)
	summaryHead   = regexp.MustCompile(`(?m)^## Summary`)
)

// today is Check 5; it reports each fault and says whether there were none.
// A missing or empty today.md (before the first new-day) passes; one
// that cannot be read fails.
func (r *run) today() bool {
	path := filepath.Join(r.e.Home, "journal", "today.md")
	if fi, err := r.fs.Stat(path); errors.Is(err, iofs.ErrNotExist) || err == nil && !fi.Mode().IsRegular() {
		return true // as `[ -f ]`: a FIFO is never opened
	}
	b, err := r.fs.ReadFile(path)
	if err == nil && len(b) == 0 {
		return true
	}
	if err != nil {
		r.fail("today.md cannot be read: %v", err)
		return false
	}
	faults := TodayFaults(string(b))
	for _, f := range faults {
		r.fail("%s", f)
	}
	return len(faults) == 0
}

// TodayFaults are today.md's format faults, each as the health check
// reports it: the four headings, a blank line after `#### Log` and before
// `#### Retrospective`, and no blank line between Log entries.
func TodayFaults(text string) []string {
	lines := records(text)
	var faults []string
	for _, h := range []string{"Agenda", "Tasks", "Log", "Retrospective"} {
		found := false
		for _, l := range lines {
			found = found || strings.HasPrefix(l, "#### "+h)
		}
		if !found {
			faults = append(faults, "today.md missing required heading: #### "+h)
		}
	}
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#### Log") {
			// awk's getline: the line after is consumed; at the end of
			// the file the heading itself is what is tested.
			if i+1 >= len(lines) || lines[i+1] != "" {
				faults = append(faults, "today.md missing blank line after #### Log")
				break
			}
			i++
		}
	}
	prev := ""
	for _, l := range lines {
		if strings.HasPrefix(l, "#### Retrospective") && prev != "" {
			faults = append(faults, "today.md missing blank line before #### Retrospective")
			break
		}
		prev = l
	}
	if n := logGaps(lines); n > 0 {
		faults = append(faults, fmt.Sprintf("today.md has %d whitespace chunk(s) within log entries", n))
	}
	return faults
}

// logGaps counts the runs of blank lines inside the Log section after the
// leading one, less a trailing one before the next heading.
func logGaps(lines []string) int {
	inLog, inBlank, entries, gaps := false, false, 0, 0
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "#### Log"):
			inLog, inBlank = true, true
		case !inLog:
		case strings.HasPrefix(l, "#### "):
			if entries > 0 && inBlank {
				gaps--
			}
			return gaps
		case l == "":
			if !inBlank {
				inBlank = true
				gaps++
			}
		case l[0] != '$':
			inBlank = false
			entries++
		}
	}
	return gaps
}

// records splits text into lines as awk reads them: a final newline ends
// the last line rather than starting an empty one.
func records(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// rotation is Check 5b: journal.md over five day-blocks (rotate is not
// being run), and window files without their front matter.
func (r *run) rotation() {
	dir := filepath.Join(r.e.Home, "journal")
	if b, err := readFile(r.fs, filepath.Join(dir, "journal.md")); err == nil {
		if n := len(dayBlock.FindAllIndex(b, -1)); n > 5 {
			r.warn("journal.md has %d day-blocks (expected ≤ 5) — run 'daisy rotate', or check that new-day/new-week are invoking it", n)
		}
	}
	entries, _ := r.fs.ReadDir(dir)
	for _, en := range entries {
		p := filepath.Join(dir, en.Name())
		if !windowFile.MatchString(en.Name()) || !isFile(r.fs, p) {
			continue
		}
		b, _ := r.fs.ReadFile(p)
		if !tocHeading.Match(b) || !summaryHead.Match(b) {
			r.warn("%s is missing TOC/Summary front matter", en.Name())
		}
	}
}

// feedback is Check 7: the pending entries and when the prompts were last
// optimized, with a nudge at twenty.
func (r *run) feedback() {
	dir := filepath.Join(r.e.Home, "feedback")
	if !isFile(r.fs, filepath.Join(dir, "feedback.md")) {
		return
	}
	b, _ := r.fs.ReadFile(filepath.Join(dir, "feedback.md"))
	n := len(feedbackEntry.FindAllIndex(b, -1))
	if last, err := readFile(r.fs, filepath.Join(dir, ".last-optimized")); err == nil {
		r.ok("Feedback: %d entries (last optimized: %s)", n, strings.TrimRight(string(last), "\n"))
	} else {
		r.ok("Feedback: %d entries (never optimized)", n)
	}
	if n >= 20 {
		r.warn("Feedback has %d entries — consider running 'daisy optimize'", n)
	}
}

func isDir(fs ops.FS, p string) bool {
	fi, err := fs.Stat(p)
	return err == nil && fi.IsDir()
}

// readFile reads a regular file only, as `[ -f ]` guards: a FIFO or a
// device is never opened (an open would wait for a writer).
func readFile(fs ops.FS, p string) ([]byte, error) {
	if !isFile(fs, p) {
		return nil, iofs.ErrNotExist
	}
	return fs.ReadFile(p)
}

func isFile(fs ops.FS, p string) bool {
	fi, err := fs.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// ShellQuote spells a path as one shell word for a command to paste: as
// it is when every character is safe (letters, digits — non-ASCII
// letters included — and `/._-+:@%=`), which is also what `printf %q`
// prints for such a path; otherwise in single quotes, `'` written `'\”`;
// and when a byte is not printable UTF-8, as `$'…'` with `\xHH` for it.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe, printable := true, true
	for i, r := range s {
		if r == utf8.RuneError && !strings.HasPrefix(s[i:], "\uFFFD") || !unicode.IsPrint(r) {
			printable = false
		}
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-+:@%=", r) || r > 127 && unicode.IsLetter(r)) {
			safe = false
		}
	}
	switch {
	case safe && printable:
		return s
	case printable:
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	var b strings.Builder
	b.WriteString("$'")
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n <= 1 || !unicode.IsPrint(r):
			for _, c := range []byte(s[i : i+n]) {
				fmt.Fprintf(&b, "\\x%02x", c)
			}
		case r == '\'' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String() + "'"
}
