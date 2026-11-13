package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	iofs "io/fs"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/env"
	"daisy/internal/metrics"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/provider"
)

// The run's record (section 7.6): each collected run's line in the
// swarm's journal, journal/<YYYY-MM-DD>.md in the swarm repository, which
// apply commits and pushes once a pass, and, when <state>/home names a
// Daisy home, its row in that home's metrics CSV.

// record is the run's record, made once at its collection's end, before
// its outcome: the journal line queued in <state>/journal for the pass's
// push, then the home's row; `recorded` says both are done, so a
// collection retried does not record the run again. A row the home
// cannot take is a warning: the run is still collected.
func (c *collection) record(outcome string) error {
	if _, done := Has(c.lines, "recorded"); done {
		return nil
	}
	s := c.s
	now := s.O.Now().UTC()
	row, turns := c.row(outcome, now)
	if err := s.queue(now.Format("2006-01-02"), journalLine(row, turns, now)); err != nil {
		return err
	}
	s.central(c.head, row)
	return c.step("recorded")
}

// row is the run as a metrics row (section 7.6, The run's record) and
// its turns ("" when not reported), which the row's notes carry.
func (c *collection) row(outcome string, now time.Time) (metrics.Row, string) {
	s := c.s
	r := metrics.Row{When: now.Format(time.RFC3339), Kind: metrics.SwarmRun, Source: s.Cfg.Forge.Repo,
		Unit: filepath.Base(c.r.Dir), Role: c.r.Role, Outcome: outcome}
	// The persona the launch recorded; a run without the line (launched
	// by an older Daisy, or refused before its credential) takes the one
	// swarm.json declares now.
	p := s.Cfg.Personas[c.r.Role]
	r.Provider, r.Model, r.Effort = p.Provider, p.Model, p.Effort
	if l, ok := Has(c.lines, "persona"); ok {
		if f := strings.Fields(l); len(f) == 4 {
			r.Provider, r.Model, r.Effort = f[1], f[2], f[3]
		}
	}
	if l, ok := Has(c.lines, "round"); ok {
		r.ReviewRounds = strings.TrimPrefix(l, "round ")
	}
	var turns string
	if l, ok := Has(c.lines, "cost"); ok {
		// cost <usd|-> tokens <in>/<out> turns <t> (section 2)
		if f := strings.Fields(l); len(f) == 6 && f[2] == "tokens" && f[4] == "turns" {
			if f[1] != "-" {
				r.CostReported = f[1]
			}
			in, out, _ := strings.Cut(f[3], "/")
			r.TokensIn, r.TokensOut, turns = in, out, f[5]
			i, err1 := strconv.Atoi(in)
			o, err2 := strconv.Atoi(out)
			if est, ok := provider.Estimate(r.Model, i, o); ok && err1 == nil && err2 == nil {
				r.CostEstimated = strconv.FormatFloat(math.Round(est*1e6)/1e6, 'f', -1, 64)
			}
		}
	}
	if b, err := s.O.FS().ReadFile(filepath.Join(c.r.Dir, "io", "system.md")); err == nil {
		sum := sha256.Sum256(b)
		r.PromptHash = hex.EncodeToString(sum[:])
	}
	// A worker's lines are its pushed commits', start to pushed, counted
	// in its repository's mirror.
	if start, ok := Has(c.lines, "start"); ok && c.r.Role == "worker" {
		if pushed, ok := Has(c.lines, "pushed"); ok {
			rng := strings.TrimPrefix(start, "start ") + ".." + strings.TrimPrefix(pushed, "pushed ")
			if lines, err := metrics.Measure(s.O, s.git().Mirror(branchRepo(c.lines)), rng); err != nil {
				fmt.Fprintf(s.Err, "⚠ %slines not counted: %v\n", c.head, err)
			} else {
				r.Lines, r.Range = &lines, rng
			}
		}
	}
	r.Notes = fmt.Sprintf("#%d", c.r.N)
	if c.r.N == 0 {
		r.Notes = "batch"
	}
	if turns != "" {
		r.Notes += ", " + turns + " turns"
	}
	return r, turns
}

// journalLine is the run's line in its day's journal file, as `daisy
// log` writes a day's log: `- HHMM <run>: <outcome>; <persona>; <usage>;
// <cost>`, then a worker's lines and the issue.
func journalLine(r metrics.Row, turns string, now time.Time) string {
	parts := []string{r.Outcome, strings.Join(slices.DeleteFunc([]string{r.Provider, r.Model, r.Effort}, func(s string) bool { return s == "" }), " ")}
	if r.ReviewRounds != "" {
		parts = append(parts, "round "+r.ReviewRounds)
	}
	switch {
	case r.TokensIn == "":
		parts = append(parts, "no usage reported")
	default:
		parts = append(parts, fmt.Sprintf("tokens %s/%s, %s turns", r.TokensIn, r.TokensOut, turns))
	}
	switch {
	case r.CostReported != "":
		parts = append(parts, "cost $"+r.CostReported)
	case r.CostEstimated != "":
		parts = append(parts, "cost not reported (~$"+r.CostEstimated+" at list price)")
	case r.TokensIn != "":
		parts = append(parts, "cost not reported")
	}
	if l := r.Lines; l != nil {
		parts = append(parts, fmt.Sprintf("code %s, tests %s, prose %s (%s)", l.Code(), l.Tests(), l.Prose(), r.Range))
	}
	issue, _, _ := strings.Cut(r.Notes, ",")
	parts = append(parts, issue)
	return fmt.Sprintf("- %s %s: %s", now.Format("1504"), r.Unit, strings.Join(parts, "; "))
}

// journalQueue is <state>/journal: the journal lines not yet pushed, one
// per run, each `<YYYY-MM-DD> <line>`.
func (s *Swarm) journalQueue() string { return filepath.Join(s.state(), "journal") }

// queue adds the line to <state>/journal under day, unless the run's line
// is there already (a collection retried before its `recorded` line).
func (s *Swarm) queue(day, line string) error {
	fs := s.O.FS()
	old, err := fs.ReadFile(s.journalQueue())
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	run := strings.Fields(line)[2] // `- HHMM <run>: …`
	for _, q := range strings.Split(string(old), "\n") {
		if f := strings.Fields(q); len(f) > 3 && f[3] == run {
			return nil
		}
	}
	return fs.WriteFile(s.journalQueue(), append(old, day+" "+line+"\n"...), 0o644)
}

// homeName is the Daisy home <state>/home names, "" for none.
func (s *Swarm) homeName() string {
	b, err := s.O.FS().ReadFile(filepath.Join(s.state(), "home"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// central appends the row to the metrics CSV of the home <state>/home
// names, under the machine's metrics lock, with the root's and the
// home's commits; no home named, no row. A home that cannot be used, or
// a write refused, is a warning.
func (s *Swarm) central(head string, row metrics.Row) {
	name := s.homeName()
	if name == "" {
		return
	}
	warn := func(format string, a ...any) {
		fmt.Fprintf(s.Err, "⚠ %smetrics row not written: "+format+"\n", append([]any{head}, a...)...)
	}
	home, err := env.HomeDir(s.O, env.Env{Root: s.Root, Config: s.Config, HomeName: name})
	if err != nil {
		first, _, _ := strings.Cut(strings.TrimPrefix(err.Error(), "Error: "), "\n")
		warn("%s names %s: %s", filepath.Join(s.state(), "home"), name, first)
		return
	}
	row.RootSHA, row.HomeSHA = s.headOf(head, s.Root, "root_sha"), s.headOf(head, home, "home_sha")
	lock := env.MetricsLock(s.Local)
	release, err := metrics.Lock(s.O, lock, "swarm apply "+s.Cfg.Forge.Repo+" since "+s.O.Now().UTC().Format(time.RFC3339))
	if err != nil {
		warn("cannot take %s: %v", lock, err)
		return
	}
	defer release()
	if err := metrics.Append(s.O.FS(), env.MetricsFile(home), row); err != nil {
		warn("%v", err)
	}
}

// headOf is the commit dir's HEAD is at; "" with a warning when git cannot
// say, the cell then unknown.
func (s *Swarm) headOf(head, dir, column string) string {
	out, stderr, err := s.O.Exec(dir, "git", "rev-parse", "--verify", "HEAD")
	if err != nil {
		fmt.Fprintf(s.Err, "⚠ %s%s left empty: git rev-parse HEAD in %s failed: %s\n", head, column, dir, firstOf(stderr, err))
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstOf(stderr []byte, err error) string {
	if l, _, _ := strings.Cut(strings.TrimSpace(string(stderr)), "\n"); l != "" {
		return l
	}
	return err.Error()
}

// Journal pushes the journal lines <state>/journal holds (section 7.6,
// The run's record): once a pass, in the host's clone of the swarm
// repository, each appended to its day's file on the default branch as
// fetched now, one commit by the reconciler's identity, pushed; the queue
// is emptied once the push lands. A push the forge refuses, or a
// journal/ that is not a directory, keeps the lines for the next pass
// with a warning; an error that did not reach the forge is returned, the
// lines kept.
func (s *Swarm) Journal() error {
	fs := s.O.FS()
	b, err := fs.ReadFile(s.journalQueue())
	if errors.Is(err, iofs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var queued []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			queued = append(queued, l)
		}
	}
	if len(queued) == 0 {
		return fs.Remove(s.journalQueue())
	}
	w, err := s.notesRepo()
	if err != nil {
		return err
	}
	runs := "runs"
	if len(queued) == 1 {
		runs = "run"
	}
	line, err := w.Journal(queued, fmt.Sprintf("swarm: journal of %d %s", len(queued), runs), "reconciler (Daisy swarm)", s.Cfg.Forge.CommitEmail)
	if err != nil {
		return fmt.Errorf("journal: %v", err)
	}
	if reason, kept := strings.CutPrefix(line, "kept: "); kept {
		fmt.Fprintf(s.Err, "⚠ journal not pushed (%s); its %d %s kept in %s for the next pass\n", reason, len(queued), map[bool]string{true: "line is", false: "lines are"}[len(queued) == 1], s.journalQueue())
		return nil
	}
	return fs.Remove(s.journalQueue())
}

// Journal appends the queued lines (`<YYYY-MM-DD> <line>`) to their days'
// files, journal/<YYYY-MM-DD>.md (a new one opening `# <YYYY-MM-DD>` and a
// blank line), on the branch's head as fetched now, and pushes one
// commit by name and email with msg: `pushed <sha>`; `kept: <reason>`
// when journal/ or a day's file is not what it should be (never replaced
// or followed) or the forge refuses the push, the clone then left at the
// head.
func (w NotesRepo) Journal(queued []string, msg, name, email string) (string, error) {
	if err := w.ensure(); err != nil {
		return "", err
	}
	if err := w.fetch(); err != nil {
		return "", err
	}
	b, err := w.branch()
	if err != nil {
		return "", err
	}
	head, err := w.run(w.Dir(), "rev-parse", "refs/remotes/origin/"+b)
	if err != nil {
		return "", err
	}
	if err := w.reset(head); err != nil {
		return "", err
	}
	line, err := w.journal(queued, msg, name, email, b)
	if err == nil && strings.HasPrefix(line, "kept: ") {
		err = w.reset(head)
	}
	return line, err
}

func (w NotesRepo) journal(queued []string, msg, name, email, branch string) (string, error) {
	fs := w.O.FS()
	dir := filepath.Join(w.Dir(), declare.JournalDir)
	if fi, err := fs.Lstat(dir); err == nil && !fi.IsDir() {
		return "kept: journal/ is not a directory", nil
	}
	var days []string
	adds := map[string]string{}
	for _, q := range queued {
		day, line, _ := strings.Cut(q, " ")
		if _, ok := adds[day]; !ok {
			days = append(days, day)
		}
		adds[day] += line + "\n"
	}
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	var paths []string
	for _, day := range days {
		rel := declare.JournalDir + "/" + day + ".md"
		p := filepath.Join(w.Dir(), rel)
		old, err := fs.ReadFile(p)
		switch fi, lerr := fs.Lstat(p); {
		case lerr == nil && !fi.Mode().IsRegular():
			return "kept: " + rel + " is not a regular file", nil
		case errors.Is(err, iofs.ErrNotExist):
			old = []byte("# " + day + "\n\n")
		case err != nil:
			return "", err
		case len(old) > 0 && old[len(old)-1] != '\n':
			old = append(old, '\n')
		}
		if err := fs.WriteFile(p, append(old, adds[day]...), 0o644); err != nil {
			return "", err
		}
		paths = append(paths, rel)
	}
	if _, err := w.run(w.Dir(), append([]string{"add", "--"}, paths...)...); err != nil {
		return "", err
	}
	ident := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "-c", "commit.gpgsign=false"}
	if _, err := w.run(w.Dir(), append(ident, "commit", "--quiet", "--no-verify", "-m", msg)...); err != nil {
		return "", err
	}
	commit, err := w.run(w.Dir(), "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	err = w.pushTo(w.Dir(), w.Opts, w.URL, commit+":refs/heads/"+branch)
	switch {
	case err == nil:
		return "pushed " + commit, nil
	case isRejected(err):
		return "kept: push rejected", nil
	}
	return "", err
}
