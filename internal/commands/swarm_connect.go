package commands

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"strings"
	"time"

	"daisy/internal/env"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/reconcile"
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
	warned
}

// swarmListed is a swarm the machine runs: its clone as <state>/clone
// records it (nil when it records none), its home (nil for none), its
// pause, and the clone the working folder is in when it is one of it.
type swarmListed struct {
	Repository string       `json:"repository"`
	Host       string       `json:"host"`
	Clone      *swarmClone  `json:"clone"`
	Home       *string      `json:"home"`
	Paused     *statusSince `json:"paused"`
	Here       *string      `json:"here"`
}

// swarmClone is the recorded clone, and whether it is a clone of the
// swarm now.
type swarmClone struct {
	Path    string `json:"path"`
	Current bool   `json:"current"`
}

func swarmList(c *Context) error {
	return printReport(c, func() (*swarmListReport, error) { return readSwarmList(c) }, func(r *swarmListReport) {
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
		if b, err := fs.ReadFile(filepath.Join(st, "home")); err == nil && firstLine(b) != "" {
			home := firstLine(b)
			s.Home = &home
		}
		if b, err := fs.ReadFile(filepath.Join(st, "paused")); err == nil {
			s.Paused = pausedSince(b)
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
	path := firstLine(b)
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

// SwarmJournal is `daisy swarm journal [--json|--mark <run>]`
// (docs/swarm.md section 11): the swarm's journal since the line
// <state>/connect-seen marks, under ReadOnly; with --mark, that mark
// moved to the run's line.
func SwarmJournal(c *Context) error {
	_, mark := c.Value("--mark")
	switch {
	case mark && c.Flag(JSONFlag):
		return UsageError("--json prints the journal and moves no mark; give --mark alone")
	case !mark:
		c.Ops = ops.ReadOnly(c.Ops)
	}
	return inSwarm(c, func() error { return swarmJournal(c) })
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
	warned
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

func swarmJournal(c *Context) error {
	if run, ok := c.Value("--mark"); ok {
		return markJournal(c, run)
	}
	return printReport(c, func() (*journalReport, error) { return readJournal(c) }, func(r *journalReport) {
		switch {
		case r.Marked == nil && r.Day == nil:
			fmt.Fprintf(c.Stdout, "Journal of %s: no line yet\n", r.Repository)
			return
		case r.Marked != nil && len(r.Lines) == 0:
			fmt.Fprintf(c.Stdout, "Journal of %s: nothing since %s (marked seen %s)\n", r.Repository, r.Marked.Run, r.Marked.At)
			return
		case r.Marked != nil:
			fmt.Fprintf(c.Stdout, "Journal of %s since %s (marked seen %s): %s\n", r.Repository, r.Marked.Run, r.Marked.At, plural(len(r.Lines), "line", "lines"))
		default:
			fmt.Fprintf(c.Stdout, "Journal of %s, %s (no line marked seen): %s\n", r.Repository, *r.Day, plural(len(r.Lines), "line", "lines"))
		}
		for _, l := range r.Lines {
			fmt.Fprintf(c.Stdout, "%s %s\n", l.Day, l.Line)
		}
		fmt.Fprintf(c.Stdout, "Mark them seen: daisy swarm journal --mark %s\n", r.Lines[len(r.Lines)-1].Run)
	})
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
// newest day's.
func readJournal(c *Context) (*journalReport, error) {
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
	mark, at, _ := strings.Cut(firstLine(b), " ")
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
		r.Lines = append(r.Lines, journalEntry{l.Day, l.Run, l.Line})
	}
	return r, nil
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
