package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"daisy/internal/env"
	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/verb"
)

// The read helpers `/daisy swarm connect` (prompts/swarm-connect.md)
// briefs from: the swarms this machine runs, and the journal since the
// last connect (docs/swarm.md section 11).

// SwarmList is `daisy swarm list` (docs/swarm.md section 11): each swarm
// with a state directory under $DAISY_LOCAL/swarm, its clone as the last
// apply recorded it, its home and its pause, run from anywhere, under
// ReadOnly.
func SwarmList(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	c.Env = m
	return swarmList(c)
}

// swarmListReport is swarm list's report: $DAISY_LOCAL/swarm and each
// swarm whose state it holds, in path order.
type swarmListReport struct {
	Dir    string        `json:"dir"`
	Swarms []swarmListed `json:"swarms"`
	verb.Warned
}

// swarmListed is a swarm the machine runs: its clone as <state>/clone
// records it (nil when it records none), its home (nil for none), its
// pause, and the clone the working folder is in when it is one of it.
type swarmListed struct {
	Repository string                 `json:"repository"`
	Host       string                 `json:"host"`
	Clone      *swarmClone            `json:"clone"`
	Home       *string                `json:"home"`
	Paused     *reconcile.StatusSince `json:"paused"`
	Here       *string                `json:"here"`
}

// swarmClone is the recorded clone, and whether it is a clone of the
// swarm now.
type swarmClone struct {
	Path    string `json:"path"`
	Current bool   `json:"current"`
}

func swarmList(c *Context) error {
	return verb.PrintReport(c, func() (*swarmListReport, error) { return readSwarmList(c) }, func(r *swarmListReport) {
		if len(r.Swarms) == 0 {
			fmt.Fprintf(c.Stdout, "No swarm on this machine (%s holds no swarm's state)\n", r.Dir)
			return
		}
		fmt.Fprintf(c.Stdout, "Swarms on this machine (%s):\n", r.Dir)
		for _, s := range r.Swarms {
			var fields []string
			switch {
			case s.Clone == nil:
				fields = append(fields, "clone not recorded")
			case !s.Clone.Current:
				fields = append(fields, "clone "+s.Clone.Path+" (not a clone of it now)")
			default:
				fields = append(fields, "clone "+s.Clone.Path)
			}
			if s.Home != nil {
				fields = append(fields, "home "+*s.Home)
			} else {
				fields = append(fields, "no home")
			}
			if p := s.Paused; p != nil && p.Reason != "" {
				fields = append(fields, "paused since "+p.Since+": "+p.Reason)
			} else if p != nil {
				fields = append(fields, "paused since "+p.Since)
			} else {
				fields = append(fields, "not paused")
			}
			if s.Here != nil {
				fields = append(fields, "here "+*s.Here)
			}
			fmt.Fprintf(c.Stdout, "  %s on %s: %s\n", s.Repository, s.Host, strings.Join(fields, "; "))
		}
	})
}

// readSwarmList reads swarm list's report.
func readSwarmList(c *Context) (*swarmListReport, error) {
	fs := c.Ops.FS()
	r := &swarmListReport{Dir: filepath.Join(c.Env.Local, "swarm"), Swarms: []swarmListed{}}
	states, err := stateDirs(fs, r.Dir)
	if err != nil {
		return nil, fmt.Errorf("Error: swarm list: %v", err)
	}
	if len(states) == 0 {
		return r, nil
	}
	here, hereState := "", ""
	if cwd, err := c.Ops.Getwd(); err == nil {
		if p, err := declare.Find(fs, cwd, false); err == nil {
			here, hereState = p.Root, cloneState(fs, p.Root, c.Env.Local)
		}
	}
	for _, st := range states {
		rel, _ := filepath.Rel(r.Dir, st)
		host, repo, _ := strings.Cut(filepath.ToSlash(rel), "/")
		s := swarmListed{Repository: repo, Host: host, Clone: recordedClone(fs, st, c.Env.Local)}
		if b, err := fs.ReadFile(filepath.Join(st, "home")); err == nil && reconcile.FirstLine(b) != "" {
			home := reconcile.FirstLine(b)
			s.Home = &home
		}
		if b, err := fs.ReadFile(filepath.Join(st, "paused")); err == nil {
			s.Paused = reconcile.PausedSince(b)
		}
		if hereState == st {
			s.Here = &here
		}
		r.Swarms = append(r.Swarms, s)
	}
	return r, nil
}

// stateDirs is every <state> under dir, $DAISY_LOCAL/swarm:
// <forge-host>/<owner>/<repo>, the slots' credentials/ left out, in path
// order.
func stateDirs(fs ops.FS, dir string) ([]string, error) {
	var states []string
	var walk func(d string, depth int) error
	walk = func(d string, depth int) error {
		entries, err := fs.ReadDir(d)
		if errors.Is(err, iofs.ErrNotExist) && depth == 0 {
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() || depth == 0 && e.Name() == "credentials" {
				continue
			}
			p := filepath.Join(d, e.Name())
			if depth == 2 {
				states = append(states, p)
				continue
			}
			if err := walk(p, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return states, walk(dir, 0)
}

// cloneState is the <state> of the swarm whose clone root is, "" when
// its swarm.json does not name one.
func cloneState(fs ops.FS, root, local string) string {
	cfg, _, err := declare.Load(fs, root)
	if err != nil || declare.ValidateForge(cfg.Forge) != nil {
		return ""
	}
	return cfg.Forge.State(local)
}

// recordedClone is the clone <state>/clone records, checked: whether
// the path is a clone of the swarm now; nil when none is recorded.
func recordedClone(fs ops.FS, state, local string) *swarmClone {
	b, err := fs.ReadFile(filepath.Join(state, "clone"))
	path := reconcile.FirstLine(b)
	if err != nil || path == "" {
		return nil
	}
	return &swarmClone{path, isCloneAt(fs, path) && cloneState(fs, path, local) == state}
}

// isCloneAt is whether dir is the top of a swarm repository's clone.
func isCloneAt(fs ops.FS, dir string) bool {
	p, err := declare.Find(fs, dir, false)
	return err == nil && p.Root == dir
}

// recordClone writes <state>/clone, the clone apply runs from, when it
// names another (docs/swarm.md section 2), so `swarm list` names the
// clone from anywhere; a refused write is a warning and apply goes on.
func recordClone(c *Context, state, root string) {
	fs, path := c.Ops.FS(), filepath.Join(state, "clone")
	if b, err := fs.ReadFile(path); err == nil && string(b) == root+"\n" {
		return
	}
	if err := fs.WriteFile(path, []byte(root+"\n"), 0o644); err != nil {
		fmt.Fprintf(c.Stderr, "⚠ %s not written: %v; swarm list will not name this clone\n", path, err)
	}
}

// SwarmJournal is `daisy swarm journal [--json] [--issue <n>] | --mark
// <run>` (docs/swarm.md section 11): the swarm's journal since the line
// <state>/connect-seen marks, under ReadOnly, with --issue only the lines
// of issue <n>; with --mark, that mark moved to the run's line.
func SwarmJournal(c *Context) error {
	_, mark := c.Value("--mark")
	word, filtered := c.Value("--issue")
	switch {
	case mark && c.Flag(verb.JSONFlag):
		return UsageError("--json prints the journal and moves no mark; give --mark alone")
	case mark && filtered:
		return UsageError("--issue filters the journal it prints and moves no mark; give --mark alone")
	case !mark:
		c.Ops = ops.ReadOnly(c.Ops)
	}
	issue := 0 // every line's
	if filtered {
		n, err := parseIssue(word)
		if err != nil {
			return err
		}
		issue = n
	}
	return inSwarm(c, func() error { return swarmJournal(c, issue) })
}

// journalReport is swarm journal's report: the mark <state>/connect-seen
// names when the journal holds its line (nil otherwise), the day shown
// when no line is marked (nil when one is, or the journal is empty), and
// the lines shown.
type journalReport struct {
	Repository string         `json:"repository"`
	Marked     *journalMark   `json:"marked"`
	Day        *string        `json:"day"`
	Lines      []journalEntry `json:"lines"`
	verb.Warned
}

type journalMark struct {
	Run string `json:"run"`
	At  string `json:"at"`
}

// journalEntry is a journal line: its day, its run and the line as the
// journal holds it.
type journalEntry struct {
	Day  string `json:"day"`
	Run  string `json:"run"`
	Line string `json:"line"`
}

// swarmJournal prints the journal's report, issue's lines alone when it
// is not 0. Then no Mark line: the mark is the whole journal's, and one
// issue's last line would mark the other issues' lines before it seen.
func swarmJournal(c *Context, issue int) error {
	if run, ok := c.Value("--mark"); ok {
		return markJournal(c, run)
	}
	return verb.PrintReport(c, func() (*journalReport, error) { return readJournal(c, issue) }, func(r *journalReport) {
		on := ""
		if issue != 0 {
			on = fmt.Sprintf(" on #%d", issue)
		}
		fmt.Fprintln(c.Stdout, journalHead(r, on))
		if r.Marked == nil && r.Day == nil || len(r.Lines) == 0 {
			return
		}
		for _, l := range r.Lines {
			fmt.Fprintf(c.Stdout, "%s %s\n", l.Day, l.Line)
		}
		if issue == 0 {
			fmt.Fprintln(c.Stdout, markLine(r.Lines))
		}
	})
}

// journalHead is the journal report's first line, its count followed by
// on (` on #<n>`, or "").
func journalHead(r *journalReport, on string) string {
	count := plural(len(r.Lines), "line", "lines") + on
	if len(r.Lines) == 0 {
		count = "nothing" + on
	}
	switch {
	case r.Marked == nil && r.Day == nil:
		return fmt.Sprintf("Journal of %s: no line yet", r.Repository)
	case r.Marked != nil && len(r.Lines) == 0:
		return fmt.Sprintf("Journal of %s: %s since %s (marked seen %s)", r.Repository, count, r.Marked.Run, r.Marked.At)
	case r.Marked != nil:
		return fmt.Sprintf("Journal of %s since %s (marked seen %s): %s", r.Repository, r.Marked.Run, r.Marked.At, count)
	}
	return fmt.Sprintf("Journal of %s, %s (no line marked seen): %s", r.Repository, *r.Day, count)
}

// markLine is the line that marks the lines shown seen, naming the last.
func markLine(lines []journalEntry) string {
	return "Mark them seen: daisy swarm journal --mark " + lines[len(lines)-1].Run
}

// swarmJournalLines is the swarm's journal as this machine holds it,
// its <state> and its repository.
func swarmJournalLines(c *Context) (lines []reconcile.JournalLine, state, repo string, err error) {
	cfg, err := forgeOnly(c)
	if err != nil {
		return nil, "", "", err
	}
	state = cfg.Forge.State(c.Env.Local)
	lines, err = reconcile.ReadJournal(c.Ops.FS(), state)
	if err != nil {
		return nil, "", "", fmt.Errorf("Error: swarm journal: %v", err)
	}
	return lines, state, cfg.Forge.Repo, nil
}

// markJournal is `swarm journal --mark <run>`: <state>/connect-seen
// moved to the run's line.
func markJournal(c *Context, run string) error {
	lines, state, _, err := swarmJournalLines(c)
	if err != nil {
		return err
	}
	if journalIndex(lines, run) < 0 {
		return fmt.Errorf("Error: swarm journal --mark: the journal holds no line of %s; mark a run its lines name", run)
	}
	seen := filepath.Join(state, "connect-seen")
	if err := c.Ops.FS().WriteFile(seen, []byte(run+" "+c.Ops.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return fmt.Errorf("Error: swarm journal --mark: %v", err)
	}
	fmt.Fprintf(c.Stdout, "✓ Marked the journal seen through %s (%s)\n", run, seen)
	return nil
}

// readJournal reads swarm journal's report: the lines after the marked
// one; with no mark, or one the journal does not hold (a warning), the
// newest day's; of those, issue's alone when it is not 0.
func readJournal(c *Context, issue int) (*journalReport, error) {
	lines, state, repo, err := swarmJournalLines(c)
	if err != nil {
		return nil, err
	}
	r := &journalReport{Repository: repo, Lines: []journalEntry{}}
	if len(lines) == 0 {
		return r, nil
	}
	seen := filepath.Join(state, "connect-seen")
	b, err := c.Ops.FS().ReadFile(seen)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return nil, fmt.Errorf("Error: swarm journal: %v", err)
	}
	mark, at, _ := strings.Cut(reconcile.FirstLine(b), " ")
	var shown []reconcile.JournalLine
	if i := journalIndex(lines, mark); i >= 0 {
		r.Marked = &journalMark{mark, at}
		shown = lines[i+1:]
	} else {
		if mark != "" {
			fmt.Fprintf(c.Stderr, "⚠ %s names %s, which the journal does not hold: showing its newest day\n", seen, mark)
		}
		day := lines[len(lines)-1].Day
		r.Day = &day
		for i := len(lines) - 1; i >= 0 && lines[i].Day == day; i-- {
			shown = lines[i:]
		}
	}
	for _, l := range shown {
		if issue == 0 || runIssue(l.Run) == issue {
			r.Lines = append(r.Lines, journalEntry{l.Day, l.Run, l.Line})
		}
	}
	return r, nil
}

// runIssue is the issue a journal line's run names, its leading number
// (`<n>-<role>-<k>`, `<n>-command-<id>`): 0 for a line of no issue (the
// owner's batch, a restart, a released hold).
func runIssue(run string) int {
	n, _, _ := strings.Cut(run, "-")
	i, err := strconv.Atoi(n)
	if err != nil {
		return 0
	}
	return i
}

// journalIndex is the place of run's line in lines, -1 when none names it.
func journalIndex(lines []reconcile.JournalLine, run string) int {
	for i, l := range lines {
		if run != "" && l.Run == run {
			return i
		}
	}
	return -1
}

// SwarmConnect is `daisy swarm connect [--json]` (docs/swarm.md section
// 11): the briefing `/daisy swarm connect` opens with
// (prompts/swarm-connect.md), as one report, under ReadOnly as status
// runs: the swarm's state, the journal since the mark by issue, what
// waits for the maintainer with each issue's gate note, what is held,
// the runs, the open questions the swarm answers, AUTHORITIES.md quoted,
// and the metrics of the swarm's runs since the last model or prompt
// change. It reads what the briefing reads and summarises nothing: the
// agent does.
func SwarmConnect(c *Context) error {
	if err := locate(c, false); err != nil {
		return err
	}
	return swarmConnect(c, openForge)
}

// AuthoritiesFile is the maintainer's standing authorities at the clone's
// top (docs/swarm.md section 2), which connect quotes and Daisy acts on in
// no way.
const AuthoritiesFile = "AUTHORITIES.md"

// connectReport is connect's report: status's facts the briefing names,
// the journal by issue, the gate notes, the questions, the authorities
// and the metrics.
type connectReport struct {
	Repository  string                  `json:"repository"`
	Host        string                  `json:"host"`
	Clone       string                  `json:"clone"`
	State       reconcile.StatusState   `json:"state"`
	Running     reconcile.StatusRunning `json:"running"`
	Journal     connectJournal          `json:"journal"`
	Waiting     connectWaiting          `json:"waiting"`
	Held        reconcile.StatusHeld    `json:"held"`
	Runs        reconcile.StatusRuns    `json:"runs"`
	Questions   []reconcile.Question    `json:"questions"`
	Authorities *connectAuthorities     `json:"authorities"`
	Metrics     connectMetrics          `json:"metrics"`
	verb.Warned
}

// connectJournal is the journal's report with its lines by issue, in
// issue order and the lines of no issue last, and the run --mark takes,
// the last line's (nil with none); Head is its first line as `swarm
// journal` prints it.
type connectJournal struct {
	Head   string         `json:"-"`
	Marked *journalMark   `json:"marked"`
	Day    *string        `json:"day"`
	Issues []journalIssue `json:"issues"`
	Mark   *string        `json:"mark"`
}

// journalIssue is one issue's lines, Issue nil for the lines of no issue.
type journalIssue struct {
	Issue *int           `json:"issue"`
	Lines []journalEntry `json:"lines"`
}

// connectWaiting is status's waiting, each issue with the text of its
// gate note (reconcile.CurrentGateNote), nil when none is on it.
type connectWaiting struct {
	Issues    []connectWait `json:"issues"`
	Notes     []string      `json:"notes"`
	NoCheck   []string      `json:"no_check"`
	CIDiffers []string      `json:"ci_differs"`
}

type connectWait struct {
	reconcile.StatusWait
	GateNote *string `json:"gate_note"`
}

// connectAuthorities is AUTHORITIES.md, its path and its bytes as they
// stand.
type connectAuthorities struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// connectMetrics is the swarm's runs in the metrics CSV of the home
// <state>/home names, by model: since the later of the clone's newest
// commit to swarm.json or prompts/ (Change, nil when none is) and the
// first run on the Daisy root the newest run names, when an earlier run
// names another (RootChange, else nil), and in all; Problem says why
// there are none (no home, no CSV, a read that failed).
type connectMetrics struct {
	Home        *string        `json:"home"`
	File        *string        `json:"file"`
	Change      *connectChange `json:"change"`
	RootChange  *connectRoot   `json:"root_change"`
	SinceChange []connectModel `json:"since_change"`
	All         []connectModel `json:"all"`
	Problem     *string        `json:"problem"`
}

type connectChange struct {
	At      string `json:"at"`
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
}

// connectRoot is the first run on the Daisy root the newest run names:
// its row's time and root_sha.
type connectRoot struct {
	At      string `json:"at"`
	RootSHA string `json:"root_sha"`
}

// connectModel is the swarm's runs of one model (nil when the rows leave
// it empty): each sum over the rows that have the value, nil when none
// does, and how many do.
type connectModel struct {
	Model             *string  `json:"model"`
	Rows              int      `json:"rows"`
	Tokens            *int     `json:"tokens"`
	RowsWithTokens    int      `json:"rows_with_tokens"`
	Cost              *float64 `json:"cost"`
	RowsWithCost      int      `json:"rows_with_cost"`
	Estimated         *float64 `json:"estimated"`
	RowsWithEstimated int      `json:"rows_with_estimated"`
}

func swarmConnect(c *Context, open opener) error {
	return verb.PrintReport(c, func() (*connectReport, error) { return readConnect(c, open) }, func(r *connectReport) { writeConnect(c.Stdout, r) })
}

// readConnect makes every read connect makes: status's, the journal's,
// AUTHORITIES.md and the metrics; a failure of the first three is the
// command's, the metrics' is their Problem.
func readConnect(c *Context, open opener) (*connectReport, error) {
	cfg, f, err := openStatus(c, open)
	if err != nil {
		return nil, err
	}
	b, err := reconcile.ReadBriefing(c.Ops, c.Swarm, cfg, c.Env.Local, f, c.Stderr)
	if err != nil {
		return nil, fmt.Errorf("Error: swarm: %v", err)
	}
	s := b.Status
	j, err := readJournal(c, 0)
	if err != nil {
		return nil, err
	}
	r := &connectReport{Repository: s.Repository, Host: s.Forge.Host, Clone: c.Swarm.Root, State: s.State, Running: s.Running,
		Journal: byIssue(j), Held: s.Held, Runs: s.Throughput.Runs, Questions: b.Questions}
	r.Waiting = connectWaiting{Issues: []connectWait{}, Notes: s.Waiting.Notes, NoCheck: s.Waiting.NoCheck, CIDiffers: s.Waiting.CIDiffers}
	for _, w := range s.Waiting.Issues {
		cw := connectWait{StatusWait: w}
		if text, ok := b.GateNotes[w.Issue]; ok {
			cw.GateNote = &text
		}
		r.Waiting.Issues = append(r.Waiting.Issues, cw)
	}
	path := filepath.Join(c.Swarm.Root, AuthoritiesFile)
	text, err := c.Ops.FS().ReadFile(path)
	switch {
	case err == nil:
		r.Authorities = &connectAuthorities{path, string(text)}
	case !errors.Is(err, iofs.ErrNotExist):
		return nil, fmt.Errorf("Error: swarm connect: %v", err)
	}
	r.Metrics = readConnectMetrics(c, cfg.Forge.State(c.Env.Local), cfg.Forge.Repo)
	return r, nil
}

// byIssue is the journal's report with its lines grouped by the issue
// each names (runIssue).
func byIssue(j *journalReport) connectJournal {
	cj := connectJournal{Head: journalHead(j, ""), Marked: j.Marked, Day: j.Day, Issues: []journalIssue{}}
	groups := map[int][]journalEntry{}
	for _, l := range j.Lines {
		groups[runIssue(l.Run)] = append(groups[runIssue(l.Run)], l)
	}
	for _, n := range slices.Sorted(maps.Keys(groups)) {
		if n != 0 {
			cj.Issues = append(cj.Issues, journalIssue{&n, groups[n]})
		}
	}
	if lines, ok := groups[0]; ok {
		cj.Issues = append(cj.Issues, journalIssue{nil, lines})
	}
	if len(j.Lines) > 0 {
		cj.Mark = &j.Lines[len(j.Lines)-1].Run
	}
	return cj
}

// readConnectMetrics reads the metrics of the swarm's runs, repo's, from
// the CSV of the home <state>/home names, and the change they are counted
// since: the clone's newest commit to swarm.json or prompts/, or the
// first run on a newer Daisy root (rootChange), whichever is later.
func readConnectMetrics(c *Context, state, repo string) connectMetrics {
	m := connectMetrics{SinceChange: []connectModel{}, All: []connectModel{}}
	none := func(format string, a ...any) connectMetrics {
		p := fmt.Sprintf(format, a...)
		m.Problem = &p
		return m
	}
	fs, homeFile := c.Ops.FS(), filepath.Join(state, "home")
	b, err := fs.ReadFile(homeFile)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return none("%v", err)
	}
	name := reconcile.FirstLine(b)
	if name == "" {
		return none("%s names no Daisy home", homeFile)
	}
	m.Home = &name
	home, err := env.HomeDir(c.Ops, env.Env{Root: c.Env.Root, Config: c.Env.Config, HomeName: name})
	if err != nil {
		first, _, _ := strings.Cut(strings.TrimPrefix(err.Error(), "Error: "), "\n")
		return none("%s names %s: %s", homeFile, name, first)
	}
	file := env.MetricsFile(home)
	m.File = &file
	csv, err := fs.ReadFile(file)
	if errors.Is(err, iofs.ErrNotExist) {
		return none("no metrics recorded yet: %s does not exist", file)
	}
	if err != nil {
		return none("%v", err)
	}
	out, errb, err := c.Ops.Exec(c.Swarm.Root, "git", "log", "-1", "--format=%cI %h %s", "--", "swarm.json", "prompts/")
	if err != nil {
		return none("git log in %s: %s", c.Swarm.Root, cmp.Or(strings.TrimSpace(string(errb)), err.Error()))
	}
	var since time.Time
	if line := strings.TrimSpace(string(out)); line != "" {
		at, rest, _ := strings.Cut(line, " ")
		commit, subject, _ := strings.Cut(rest, " ")
		if since, err = time.Parse(time.RFC3339, at); err != nil {
			return none("git log in %s: a commit time that is not RFC 3339: %s", c.Swarm.Root, at)
		}
		m.Change = &connectChange{since.UTC().Format(time.RFC3339), commit, subject}
	}
	all, err := metrics.Summarize(csv, metrics.Filter{Swarm: repo})
	if err != nil {
		return none("cannot read %s: %v", file, err)
	}
	m.All = connectModels(all)
	latest, err := metrics.Latest(csv)
	if err != nil {
		return none("cannot read %s: %v", file, err)
	}
	if at, sha, ok := rootChange(latest, repo); ok && at.After(since) {
		since, m.RootChange = at, &connectRoot{at.UTC().Format(time.RFC3339), sha}
	}
	if !since.IsZero() {
		s, err := metrics.Summarize(csv, metrics.Filter{Swarm: repo, Since: since})
		if err != nil {
			return none("cannot read %s: %v", file, err)
		}
		m.SinceChange = connectModels(s)
	}
	return m
}

// rootChange is the time and root_sha of the first of repo's runs, by
// their rows' times (each key's latest row), on the root the newest names,
// when an earlier run names another; ok is false when none does. A row
// whose time does not parse is left out, as Summarize refuses it.
func rootChange(latest map[string]metrics.Row, repo string) (at time.Time, sha string, ok bool) {
	type run struct {
		at  time.Time
		sha string
	}
	var runs []run
	for _, r := range latest {
		if t, err := time.Parse(time.RFC3339, r.When); err == nil && r.Kind == metrics.SwarmRun && r.Source == repo {
			runs = append(runs, run{t, r.RootSHA})
		}
	}
	slices.SortStableFunc(runs, func(a, b run) int { return a.at.Compare(b.at) })
	for i := len(runs) - 1; i > 0; i-- {
		if runs[i-1].sha != runs[i].sha {
			return runs[i].at, runs[i].sha, true
		}
	}
	return time.Time{}, "", false
}

// connectModels is a summary's groups, every one of kind swarm-run under
// the swarm's filter, by model.
func connectModels(s metrics.Summary) []connectModel {
	out := []connectModel{}
	for _, g := range s.Groups {
		cm := connectModel{Rows: g.Rows, RowsWithTokens: g.WithTokens, RowsWithCost: g.WithCost, RowsWithEstimated: g.WithEstimate}
		if g.Model != "" {
			cm.Model = &g.Model
		}
		if g.WithTokens > 0 {
			cm.Tokens = &g.Tokens
		}
		if g.WithCost > 0 {
			cm.Cost = &g.Cost
		}
		if g.WithEstimate > 0 {
			cm.Estimated = &g.Estimate
		}
		out = append(out, cm)
	}
	return out
}

// writeConnect writes connect's report as text, in rule 2's order.
func writeConnect(out io.Writer, r *connectReport) {
	fmt.Fprintf(out, "Swarm: %s on %s, clone %s\n", r.Repository, r.Host, r.Clone)
	fmt.Fprintf(out, "State: %s%s\n", r.State.Path, stateNotes(r.State))
	fmt.Fprintf(out, "Running: %s\n", runningLine(r.Running))

	j := r.Journal
	fmt.Fprintf(out, "\nSince the last connect: %s\n", j.Head)
	for _, g := range j.Issues {
		if g.Issue == nil {
			fmt.Fprintln(out, "  no issue")
		} else {
			fmt.Fprintf(out, "  #%d\n", *g.Issue)
		}
		for _, l := range g.Lines {
			fmt.Fprintf(out, "    %s %s\n", l.Day, l.Line)
		}
	}
	if j.Mark != nil {
		fmt.Fprintln(out, "Mark them seen: daisy swarm journal --mark "+*j.Mark)
	}

	waiting := reconcile.StatusWaiting{Issues: []reconcile.StatusWait{}, Notes: r.Waiting.Notes, NoCheck: r.Waiting.NoCheck, CIDiffers: r.Waiting.CIDiffers}
	for _, w := range r.Waiting.Issues {
		waiting.Issues = append(waiting.Issues, w.StatusWait)
	}
	fmt.Fprintln(out, "\n"+waitingLine(waiting))
	for _, w := range r.Waiting.Issues {
		if w.GateNote == nil {
			fmt.Fprintf(out, "  #%d: no gate note of this gating on the issue\n", w.Issue)
			continue
		}
		fmt.Fprintf(out, "  #%d, its gate note:\n%s\n", w.Issue, indent(*w.GateNote, "    "))
	}
	fmt.Fprintln(out, heldLine(r.Held))
	fmt.Fprintln(out, "Runs: "+runsLine(r.Runs))

	if len(r.Questions) == 0 {
		fmt.Fprintln(out, "\nQuestions the swarm answers: none")
	} else {
		var heads []string
		for _, q := range r.Questions {
			heads = append(heads, fmt.Sprintf("#%d for the %s", q.Issue, q.Answerer))
		}
		fmt.Fprintln(out, "\nQuestions the swarm answers: "+strings.Join(heads, "; "))
		for _, q := range r.Questions {
			if q.Asked == nil {
				fmt.Fprintf(out, "  #%d: no question of the swarm's on the issue\n", q.Issue)
				continue
			}
			fmt.Fprintf(out, "  #%d, %s run %d at %s:\n%s\n", q.Issue, q.Asked.Role, q.Asked.Run, q.Asked.At, indent(strings.TrimRight(q.Asked.Text, "\n"), "    "))
		}
	}

	if a := r.Authorities; a == nil {
		fmt.Fprintf(out, "\nAuthorities: none granted (no %s in %s)\n", AuthoritiesFile, r.Clone)
	} else {
		fmt.Fprintf(out, "\nAuthorities: %s, quoted verbatim; Daisy acts on none of it\n%s", a.Path, a.Text)
		if !strings.HasSuffix(a.Text, "\n") {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, "End of "+AuthoritiesFile)
	}

	m := r.Metrics
	if m.Problem != nil {
		fmt.Fprintf(out, "\nMetrics of the swarm's runs: none (%s)\n", *m.Problem)
		return
	}
	fmt.Fprintf(out, "\nMetrics of the swarm's runs: %s (home %s)\n", *m.File, *m.Home)
	switch c, root := m.Change, m.RootChange; {
	case root != nil && c != nil:
		fmt.Fprintf(out, "  since the last model or prompt change, %s root %s (the first run on that Daisy root, after the clone's newest commit to swarm.json or prompts/):\n", root.At, short(root.RootSHA))
		writeModels(out, m.SinceChange)
	case root != nil:
		fmt.Fprintf(out, "  since the last model or prompt change, %s root %s (the first run on that Daisy root; no commit to swarm.json or prompts/ in the clone):\n", root.At, short(root.RootSHA))
		writeModels(out, m.SinceChange)
	case c != nil:
		fmt.Fprintf(out, "  since the last model or prompt change, %s %s %s (the clone's newest commit to swarm.json or prompts/):\n", c.At, c.Commit, c.Subject)
		writeModels(out, m.SinceChange)
	default:
		fmt.Fprintln(out, "  no commit to swarm.json or prompts/ in the clone")
	}
	fmt.Fprintln(out, "  in all:")
	writeModels(out, m.All)
}

// writeModels writes a line per model, or `none`.
func writeModels(out io.Writer, ms []connectModel) {
	if len(ms) == 0 {
		fmt.Fprintln(out, "    none")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, g := range ms {
		model := "(none)"
		if g.Model != nil {
			model = *g.Model
		}
		fmt.Fprintf(w, "    %s\t%s, tokens %s, cost %s, estimated %s\n", model, plural(g.Rows, "row", "rows"),
			partial(g.Tokens, "%d", g.RowsWithTokens, g.Rows), partial(g.Cost, "$%.2f", g.RowsWithCost, g.Rows), partial(g.Estimated, "$%.2f", g.RowsWithEstimated, g.Rows))
	}
	w.Flush()
}

// partial is a sum over the k of n rows that have the value, in format:
// `—` for none, the sum and (k/n) for some, as `daisy metrics` prints it.
func partial[T any](sum *T, format string, k, n int) string {
	switch {
	case sum == nil:
		return "—"
	case k < n:
		return fmt.Sprintf(format+" (%d/%d)", *sum, k, n)
	}
	return fmt.Sprintf(format, *sum)
}

// indent is text with each of its lines but the empty ones after by.
func indent(text, by string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = by + l
		}
	}
	return strings.Join(lines, "\n")
}
