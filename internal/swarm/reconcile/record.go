package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	iofs "io/fs"
	"maps"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/env"
	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/fence"
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
	// The row is dated by the run's end, as a backfill dates it; the
	// journal line by its collection.
	when, ok := endTime(c.lines)
	if !ok {
		when = now
	}
	row, turns := c.row(outcome, when)
	if err := s.queue(now.Format("2006-01-02"), journalLine(row, turns, now)); err != nil {
		return err
	}
	s.central(c.head, row)
	if isReviewer(c.r.Role) && outcome == "posted" {
		c.earlierEffects(outcome)
	}
	return c.step("recorded")
}

// row is the run as a metrics row (section 7.6, The run's record) and
// its turns ("" when not reported), which the row's notes carry.
func (c *collection) row(outcome string, now time.Time) (metrics.Row, string) {
	s := c.s
	r := s.runKey(c.r)
	r.When, r.Outcome = now.Format(time.RFC3339), outcome
	// The persona the launch recorded; a run without the line (launched
	// by an older Daisy, or refused before its credential) the one its
	// argv launched, else the one swarm.json declares now.
	p := s.Cfg.Personas[c.r.Role]
	r.Provider, r.Model, r.Effort = p.Provider, p.Model, p.Effort
	persona, since16b := Has(c.lines, "persona")
	if f := strings.Fields(persona); len(f) == 4 {
		r.Provider, r.Model, r.Effort = f[1], f[2], f[3]
	} else if argv, err := s.O.FS().ReadFile(filepath.Join(c.r.Dir, "argv")); err == nil {
		if name, model, effort, ok := provider.Persona(strings.Split(string(argv), "\n")); ok {
			r.Provider, r.Model, r.Effort = name, model, effort
		}
	}
	if l, ok := Has(c.lines, "round"); ok {
		r.ReviewRounds = strings.TrimPrefix(l, "round ")
	}
	if isReviewer(c.r.Role) {
		r.ReviewEffect = c.reviewEffect(outcome)
	}
	r.Cause = c.cause(outcome)
	var turns string
	if l, ok := Has(c.lines, "cost"); ok {
		// A run launched before the persona line wrote a cost its CLI
		// did not report as 0 (codex's always), where it now writes `-`.
		if u, ok := readCost(l); ok {
			if u.usd != "-" && (since16b || u.usd != "0") {
				r.CostReported = u.usd
			}
			usage(&r, u.in, u.out, u.cached)
			turns = u.turns
		}
	}
	if b, err := s.O.FS().ReadFile(filepath.Join(c.r.Dir, "io", "system.md")); err == nil {
		sum := sha256.Sum256(b)
		r.PromptHash = hex.EncodeToString(sum[:])
		r.ConfigHash = configHash(b)
	}
	// A worker's lines are its pushed commits', start to pushed, counted
	// in its repository's mirror.
	var uncounted bool
	if start, ok := Has(c.lines, "start"); ok && c.r.Role == "worker" {
		if pushed, ok := Has(c.lines, "pushed"); ok {
			rng := strings.TrimPrefix(start, "start ") + ".." + strings.TrimPrefix(pushed, "pushed ")
			if lines, err := metrics.Measure(s.O, s.git().Mirror(branchRepo(c.lines)), rng); err != nil {
				fmt.Fprintf(s.Err, "⚠ %slines not counted: %v\n", c.head, err)
				uncounted = true
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
	if uncounted {
		r.Notes += ", lines not counted"
	}
	return r, turns
}

// configHash is the sha256 of the run's configuration as its system.md
// holds it (section 7.6, The run's record): the text from its `# Persona`
// line up to the first block after its prompt parts, the role's prompt
// and its parts under their headings, which Render writes alike for
// every run of the role whatever its issue, stage or time. That block is
// `# Task` in a system.md written before Step E1 of the plan implementing
// the 2026-11-19 swarm roadmap, and `# Praxis`, `# Repository notes:
// <name>` or `# Resources` since, so one persona and its parts hash
// alike in either order. The role's `# Output` block, between the
// persona and the parts since the verdict fix, is Daisy's text, not the
// configuration, and is left out, so one persona and its parts hash
// alike with it and without it. "" when the file has no `# Persona`
// line followed by another block's. A line in a fenced block is
// neither, and Render shifts every heading of the persona and the parts
// below `# `.
func configHash(system []byte) string {
	var fences fence.Scan
	start, at := -1, 0
	output, outputEnd := -1, -1 // the # Output block's bytes, once seen
	for _, line := range strings.SplitAfter(string(system), "\n") {
		from := at
		at += len(line)
		l := strings.TrimSuffix(line, "\n")
		if fences.Line(l) || !strings.HasPrefix(l, "# ") {
			continue
		}
		if output >= 0 && outputEnd < 0 {
			outputEnd = from
		}
		switch {
		case start < 0 && l == "# Persona":
			start = from
		case start >= 0 && l == "# Output" && output < 0:
			output = from
		case start >= 0 && !strings.HasPrefix(l, "# Prompt part: "):
			text := system[start:from]
			if output >= 0 {
				text = append(slices.Clone(system[start:output]), system[outputEnd:from]...)
			}
			sum := sha256.Sum256(text)
			return hex.EncodeToString(sum[:])
		}
	}
	return ""
}

// costUsage is a result's cost line read: `cost <usd|-> tokens
// <in>/<out> turns <t>[ cached <c>]` (section 2), cached "" when the
// line has no count.
type costUsage struct{ usd, in, out, turns, cached string }

func readCost(line string) (costUsage, bool) {
	f := strings.Fields(line)
	if !((len(f) == 6 || len(f) == 8 && f[6] == "cached") && f[0] == "cost" && f[2] == "tokens" && f[4] == "turns") {
		return costUsage{}, false
	}
	u := costUsage{usd: f[1], turns: f[5]}
	u.in, u.out, _ = strings.Cut(f[3], "/")
	if len(f) == 8 {
		u.cached = f[7]
	}
	return u, true
}

// usage sets the row's usage cells from in input tokens, out output
// tokens and cached of the input read from the cache ("" when not
// reported): tokens_in, tokens_out and tokens_cached as given,
// tokens_total in and out added, and cost_estimated those tokens at the
// model's list price (section 9), none when the cached part is not
// known: on a cost line written before Step 28 of the plan remediating
// the 2026-11-15 design review, claude's and grok's input count left the
// cache out, and codex's before Step 11b of the plan for the inbox
// requests F27-F35 held a cached part it did not say.
func usage(r *metrics.Row, in, out, cached string) {
	r.TokensIn, r.TokensOut, r.TokensCached, r.TokensTotal, r.CostEstimated = in, out, cached, "", ""
	i, err1 := strconv.Atoi(in)
	o, err2 := strconv.Atoi(out)
	if err1 != nil || err2 != nil {
		return
	}
	r.TokensTotal = strconv.Itoa(i + o)
	if c, err := strconv.Atoi(cached); err == nil {
		if est, ok := provider.Estimate(r.Model, i, c, o); ok {
			r.CostEstimated = strconv.FormatFloat(math.Round(est*1e6)/1e6, 'f', -1, 64)
		}
	}
}

// runKey is the run's row with its key's cells alone (metrics.Row.Key):
// its kind, the swarm, the run and its role.
func (s *Swarm) runKey(r Run) metrics.Row {
	return metrics.Row{Kind: metrics.SwarmRun, Source: s.Cfg.Forge.Repo, Unit: filepath.Base(r.Dir), Role: r.Role}
}

// journalLine is the run's line in its day's journal file, as `daisy
// log` writes a day's log: `- HHMM <run>: <outcome>; <persona>; <usage>;
// <cost>`, then a worker's lines and the issue.
func journalLine(r metrics.Row, turns string, now time.Time) string {
	persona := strings.Join(slices.DeleteFunc([]string{r.Provider, r.Model, r.Effort}, func(s string) bool { return s == "" }), " ")
	if persona == "" {
		persona = "unknown persona"
	}
	parts := []string{r.Outcome, persona}
	if r.ReviewRounds != "" {
		parts = append(parts, "round "+r.ReviewRounds)
	}
	switch {
	case r.TokensIn == "":
		parts = append(parts, "no usage reported")
	case r.TokensCached != "":
		parts = append(parts, fmt.Sprintf("tokens %s/%s (%s cached), %s turns", r.TokensIn, r.TokensOut, r.TokensCached, turns))
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
func (s *Swarm) journalQueue() string { return journalQueue(s.state()) }

func journalQueue(state string) string { return filepath.Join(state, "journal") }

// JournalLine is one line of the swarm's journal: its day, the line as
// written (`- HHMM <run>: …`) and the run it names (`<n>-<role>-<k>`, or
// `<n>-command-<id>`), which no other line names.
type JournalLine struct{ Day, Line, Run string }

// ReadJournal is the swarm's journal as this machine knows it (section
// 7.6, The run's record): the lines of journal/<day>.md in <state>/notes,
// the host's clone the journal is pushed from, then the lines
// <state>/journal still queues, each in its day after the day's pushed
// lines, the days in order. A queued line whose run a day's file names
// was pushed already (the queue is removed only after the push lands),
// so each run is one line.
func ReadJournal(fs ops.FS, state string) ([]JournalLine, error) {
	byDay := map[string][]JournalLine{}
	seen := map[string]bool{}
	add := func(day, line string) {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "-" || !strings.HasSuffix(f[2], ":") {
			return // the day's heading, a blank line
		}
		run := strings.TrimSuffix(f[2], ":")
		if seen[run] {
			return
		}
		seen[run] = true
		byDay[day] = append(byDay[day], JournalLine{day, line, run})
	}
	dir := filepath.Join(state, "notes", declare.JournalDir)
	entries, err := fs.ReadDir(dir)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		day, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		b, err := fs.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, l := range strings.Split(string(b), "\n") {
			add(day, l)
		}
	}
	b, err := fs.ReadFile(journalQueue(state))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return nil, err
	}
	for _, q := range strings.Split(string(b), "\n") {
		if day, line, ok := strings.Cut(q, " "); ok {
			add(day, line)
		}
	}
	var lines []JournalLine
	for _, day := range slices.Sorted(maps.Keys(byDay)) {
		lines = append(lines, byDay[day]...)
	}
	return lines, nil
}

// journalLocked runs change under <state>/journal.lock, the queue's lock,
// which every rewrite of <state>/journal takes: apply queueing a line or
// dropping the lines it pushed, and a maintainer's verb queueing its
// intervention's (section 11, The interventions' record), each its own
// process.
func (s *Swarm) journalLocked(change func() error) error {
	lock := filepath.Join(s.state(), "journal.lock")
	release, err := metrics.Lock(s.O, lock, "swarm "+s.Cfg.Forge.Repo+" since "+s.O.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("cannot take %s: %v", lock, err)
	}
	defer release()
	return change()
}

// queue adds the line to <state>/journal under day, unless the run's line
// is there already (a collection retried before its `recorded` line).
func (s *Swarm) queue(day, line string) error {
	return s.journalLocked(func() error {
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
	})
}

// homeName is the Daisy home <state>/home names, "" for none.
func (s *Swarm) homeName() string {
	b, err := s.O.FS().ReadFile(filepath.Join(s.state(), "home"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// metricsHome is the home <state>/home names, "" when it names none; an
// error when the home cannot be used, naming the file and the home.
func (s *Swarm) metricsHome() (string, error) {
	name := s.homeName()
	if name == "" {
		return "", nil
	}
	home, err := env.HomeDir(s.O, env.Env{Root: s.Root, Config: s.Config, HomeName: name})
	if err != nil {
		first, _, _ := strings.Cut(strings.TrimPrefix(err.Error(), "Error: "), "\n")
		return "", fmt.Errorf("%s names %s: %s", filepath.Join(s.state(), "home"), name, first)
	}
	return home, nil
}

// central appends the row to the metrics CSV of the home <state>/home
// names, under the machine's metrics lock, with the root's and the
// home's commits; no home named, no row. A home that cannot be used, or
// a write refused, is a warning.
func (s *Swarm) central(head string, row metrics.Row) {
	warn := func(format string, a ...any) {
		fmt.Fprintf(s.Err, "⚠ %smetrics row not written: "+format+"\n", append([]any{head}, a...)...)
	}
	home, err := s.metricsHome()
	if home == "" {
		if err != nil {
			warn("%v", err)
		}
		return
	}
	row.RootSHA, row.HomeSHA = s.headOf(head, s.Root, "root_sha"), s.headOf(head, home, "home_sha")
	if _, err := s.locked(metrics.Append, env.MetricsFile(home), "swarm apply", row); err != nil {
		warn("%v", err)
	}
}

// locked writes rows to file with write, metrics.Append (each row whose
// run the file holds no row of: its kind, swarm, run and role) or
// metrics.Supersede, under the machine's metrics lock, held by who for
// one read and one rewrite.
func (s *Swarm) locked(write func(ops.FS, string, ...metrics.Row) (int, error), file, who string, rows ...metrics.Row) (int, error) {
	lock := env.MetricsLock(s.Local)
	release, err := metrics.Lock(s.O, lock, who+" "+s.Cfg.Forge.Repo+" since "+s.O.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("cannot take %s: %v", lock, err)
	}
	defer release()
	return write(s.O.FS(), file, rows...)
}

// Backfill writes the row of each collected run that the metrics CSV of
// the home <state>/home names holds none for (section 11, swarm
// metrics): the row a collection makes (row), at the time of the run's
// end line, its root's and home's commits unknown, its notes ending
// `backfilled`; then, for each issue and reviewer role of a backfilled
// review, the row of each review of that role on the issue whose known
// effect the runs now give otherwise is superseded, as a collection
// supersedes it (earlierEffects). Every row is made, a worker's lines
// measured, before the metrics lock is taken; under it, the CSV is read
// and rewritten once for the new rows and once for the superseding
// ones, so a live apply waits on it no longer than on two rows. No
// journal line is written.
func (s *Swarm) Backfill() (Backfilled, error) {
	var b Backfilled
	var before []byte
	var err error
	if b.File, before, err = s.metricsCSV(); err != nil {
		return b, err
	}
	// The runs the CSV holds a row for now are skipped unread; Append
	// skips any a writer adds before the lock.
	held, err := metrics.Keys(before)
	if err != nil {
		return b, fmt.Errorf("cannot record in %s: %v", b.File, err)
	}
	runs, err := Runs(s.O.FS(), s.state())
	if err != nil {
		return b, err
	}
	var rows []metrics.Row
	reviewed := map[string]Run{} // a backfilled review's issue and role, by one of its runs
	undated := 0
	for _, r := range runs {
		lines, err := r.Result(s.O.FS())
		if err != nil {
			return b, err
		}
		if _, ok := Has(lines, "collected"); !ok {
			continue
		}
		b.Collected++
		if held[s.runKey(r).Key()] {
			continue
		}
		c := &collection{s: s, r: r, lines: lines, head: runHead(r)}
		when, ok := endTime(lines)
		if !ok {
			fmt.Fprintf(s.Err, "⚠ %sno row: its result has no end line or started line to date it\n", c.head)
			undated++
			continue
		}
		outcome, _ := Has(lines, "outcome")
		row, _ := c.row(strings.TrimPrefix(outcome, "outcome "), when)
		row.Notes += ", backfilled"
		rows = append(rows, row)
		if isReviewer(r.Role) {
			reviewed[fmt.Sprintf("%d-%s", r.N, r.Role)] = r
		}
	}
	slices.SortStableFunc(rows, func(a, b metrics.Row) int { return strings.Compare(a.When, b.When) })
	// A backfilled review may decide the effect of an earlier review of
	// its role on the issue whose row the CSV holds: superseded as a
	// collection supersedes it (earlierEffects).
	var effects [][]runEffect
	for _, k := range slices.Sorted(maps.Keys(reviewed)) {
		e, err := s.reviewEffects(reviewed[k], "")
		if err != nil {
			return b, err
		}
		effects = append(effects, e)
	}
	write := func(fsys ops.FS, file string, rows ...metrics.Row) (int, error) {
		n, err := metrics.Append(fsys, file, rows...)
		if err != nil {
			return n, err
		}
		for i, k := range slices.Sorted(maps.Keys(reviewed)) {
			m, err := s.supersedeEffects(fsys, file, reviewed[k].Role, effects[i])
			if err != nil {
				return n, err
			}
			b.Superseded += m
		}
		return n, nil
	}
	b.Written, err = s.locked(write, b.File, "swarm metrics --backfill", rows...)
	b.Had = b.Collected - undated - b.Written
	return b, err
}

// metricsCSV is the metrics CSV of the home <state>/home names, and its
// bytes, none when it does not exist; an error when no usable home is
// named or the file cannot be read.
func (s *Swarm) metricsCSV() (string, []byte, error) {
	home, err := s.metricsHome()
	if err != nil {
		return "", nil, err
	}
	if home == "" {
		return "", nil, fmt.Errorf("%s names no Daisy home, whose metrics CSV would take the rows; write a registered home's name in it", filepath.Join(s.state(), "home"))
	}
	file := env.MetricsFile(home)
	b, err := s.O.FS().ReadFile(file)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return file, nil, fmt.Errorf("cannot read %s: %v", file, err)
	}
	return file, b, nil
}

// Reprice supersedes the row of each collected run that the metrics CSV
// of the home <state>/home names holds (section 11, swarm metrics), the
// last of its key, when the usage its <run>/stdout reports, read by the
// adapter of the row's provider, gives other usage cells than the row's
// (usage: tokens_in with the cache's input, tokens_cached, tokens_total,
// cost_estimated at today's list price), or, for a reviewer's run, the
// runs give another known review_effect (reviewEffects; an unknown one
// keeps the held row's, which may be set by hand), or its notes still
// hold `<c> cached input` beside a tokens_cached, or its kept
// <run>/io/system.md gives another config_hash (configHash; a run
// without one keeps the held row's): the new row is the
// held row with those cells, that part of its notes dropped, every
// other cell carried over, its notes ending `repriced`. A run without stdout, one whose stdout reports no
// usage, or a row naming a provider the adapter table lacks is unread,
// its usage cells left as they are. Every row is made before the
// metrics lock is taken; under
// it, the CSV is read once and a row whose key's last row is already
// it dropped (metrics.Supersede), so a second reprice writes nothing.
// With dryRun the rows are made and nothing is written.
func (s *Swarm) Reprice(dryRun bool) (Repriced, error) {
	var p Repriced
	var before []byte
	var err error
	if p.File, before, err = s.metricsCSV(); err != nil {
		return p, err
	}
	latest, err := metrics.Latest(before)
	if err != nil {
		return p, fmt.Errorf("cannot record in %s: %v", p.File, err)
	}
	runs, err := Runs(s.O.FS(), s.state())
	if err != nil {
		return p, err
	}
	effects := map[string][]runEffect{} // by issue and reviewer
	for _, r := range runs {
		lines, err := r.Result(s.O.FS())
		if err != nil {
			return p, err
		}
		if _, ok := Has(lines, "collected"); !ok {
			continue
		}
		p.Collected++
		held, ok := latest[s.runKey(r).Key()]
		if !ok {
			continue
		}
		p.Held++
		prov, known := provider.Providers[held.Provider]
		stdout, err := s.O.FS().ReadFile(filepath.Join(r.Dir, "stdout"))
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return p, err
		}
		var rep provider.Report
		if known && err == nil {
			rep = prov.Adapter.Report(stdout)
		}
		row := held
		if rep.Known {
			cached := ""
			if rep.CachedKnown {
				cached = strconv.Itoa(rep.CachedTokens)
			}
			usage(&row, strconv.Itoa(rep.InTokens), strconv.Itoa(rep.OutTokens), cached)
		} else {
			p.Unread++
		}
		// The configuration's hash from the run's kept system.md; a run
		// without one keeps the held row's.
		if b, err := s.O.FS().ReadFile(filepath.Join(r.Dir, "io", "system.md")); err == nil {
			if h := configHash(b); h != "" {
				row.ConfigHash = h
			}
		} else if !errors.Is(err, iofs.ErrNotExist) {
			return p, err
		}
		if isReviewer(r.Role) {
			k := fmt.Sprintf("%d-%s", r.N, r.Role)
			if effects[k] == nil {
				if effects[k], err = s.reviewEffects(r, ""); err != nil {
					return p, err
				}
			}
			for _, e := range effects[k] {
				if e.k == r.K && e.effect != "" { // unknown keeps the held effect, which may be set by hand
					row.ReviewEffect = e.effect
				}
			}
		}
		row.Notes = withoutCachedNote(row)
		if row.TokensIn == held.TokensIn && row.TokensOut == held.TokensOut && row.TokensCached == held.TokensCached &&
			row.TokensTotal == held.TokensTotal && row.CostEstimated == held.CostEstimated && row.ReviewEffect == held.ReviewEffect && row.Notes == held.Notes &&
			row.ConfigHash == held.ConfigHash {
			if rep.Known {
				p.Current++
			}
			continue
		}
		if !strings.HasSuffix(row.Notes, ", repriced") {
			row.Notes += ", repriced"
		}
		p.Changes = append(p.Changes, Change{held, row})
	}
	if dryRun || len(p.Changes) == 0 {
		return p, nil
	}
	rows := make([]metrics.Row, len(p.Changes))
	for i, c := range p.Changes {
		rows[i] = c.To
	}
	p.Written, err = s.locked(metrics.Supersede, p.File, "swarm metrics --reprice", rows...)
	return p, err
}

// cachedNote is the notes' part a row recorded before Step 28 of the
// plan remediating the 2026-11-15 design review held its cached input
// in, `<c> cached input`, which tokens_cached now holds.
var cachedNote = regexp.MustCompile(`^[0-9]+ cached input$`)

// withoutCachedNote is the row's notes without that part once its
// tokens_cached is set.
func withoutCachedNote(r metrics.Row) string {
	if r.TokensCached == "" {
		return r.Notes
	}
	parts := strings.Split(r.Notes, ", ")
	return strings.Join(slices.DeleteFunc(parts, cachedNote.MatchString), ", ")
}

// Repriced is what a reprice did: the CSV, the rows it made (each from
// the row it supersedes) and those it wrote, the runs collected, those
// of them the CSV holds a row of, and of those the rows already current,
// their usage read, and the runs whose usage could not be read.
type Repriced struct {
	File                                      string
	Changes                                   []Change
	Written, Collected, Held, Current, Unread int
}

// Change is a held row and the row that supersedes it.
type Change struct{ From, To metrics.Row }

// Backfilled is what a backfill did: the CSV it wrote to, the rows it
// wrote, the runs collected and those of them the CSV held a row for,
// and the earlier reviews' rows it superseded with their effect.
type Backfilled struct {
	File                                string
	Written, Collected, Had, Superseded int
}

// endTime is when the run ended, its end line's time (section 2), else
// its started line's.
func endTime(lines []string) (time.Time, bool) {
	for _, prefix := range []string{Ended(lines), "started"} {
		if prefix == "" {
			continue
		}
		if t, ok := lineTime(lines, prefix); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// lineTime is the time the result's line beginning prefix ends with.
func lineTime(lines []string, prefix string) (time.Time, bool) {
	if l, ok := Has(lines, prefix); ok {
		f := strings.Fields(l)
		if t, err := time.Parse(time.RFC3339Nano, f[len(f)-1]); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
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
		return s.unqueue(nil)
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
	return s.unqueue(queued)
}

// unqueue drops the pushed lines from <state>/journal under its lock,
// keeping a line queued since they were read, and removes the file when
// none is left.
func (s *Swarm) unqueue(pushed []string) error {
	return s.journalLocked(func() error {
		fs := s.O.FS()
		b, err := fs.ReadFile(s.journalQueue())
		if errors.Is(err, iofs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		var kept []byte
		for _, l := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(l) != "" && !slices.Contains(pushed, l) {
				kept = append(kept, l+"\n"...)
			}
		}
		if len(kept) == 0 {
			return fs.Remove(s.journalQueue())
		}
		return fs.WriteFile(s.journalQueue(), kept, 0o644)
	})
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
