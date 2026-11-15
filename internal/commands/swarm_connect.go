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
var SwarmList = readOnly(func(c *Context) error {
	m, err := env.Machine(c.Ops)
	if err != nil {
		return err
	}
	c.Env = m
	return swarmList(c)
})

func swarmList(c *Context) error {
	fs := c.Ops.FS()
	dir := filepath.Join(c.Env.Local, "swarm")
	states, err := stateDirs(fs, dir)
	if err != nil {
		return fmt.Errorf("Error: swarm list: %v", err)
	}
	if len(states) == 0 {
		fmt.Fprintf(c.Stdout, "No swarm on this machine (%s holds no swarm's state)\n", dir)
		return nil
	}
	here, hereState := "", ""
	if cwd, err := c.Ops.Getwd(); err == nil {
		if p, err := declare.Find(fs, cwd, false); err == nil {
			here, hereState = p.Root, cloneState(fs, p.Root, c.Env.Local)
		}
	}
	fmt.Fprintf(c.Stdout, "Swarms on this machine (%s):\n", dir)
	for _, st := range states {
		rel, _ := filepath.Rel(dir, st)
		host, repo, _ := strings.Cut(filepath.ToSlash(rel), "/")
		fields := []string{cloneField(fs, st, c.Env.Local)}
		if b, err := fs.ReadFile(filepath.Join(st, "home")); err == nil && firstLine(b) != "" {
			fields = append(fields, "home "+firstLine(b))
		} else {
			fields = append(fields, "no home")
		}
		if b, err := fs.ReadFile(filepath.Join(st, "paused")); err == nil {
			fields = append(fields, "paused since "+strings.TrimPrefix(firstLine(b), "paused "))
		} else {
			fields = append(fields, "not paused")
		}
		if hereState == st {
			fields = append(fields, "here "+here)
		}
		fmt.Fprintf(c.Stdout, "  %s on %s: %s\n", repo, host, strings.Join(fields, "; "))
	}
	return nil
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

// cloneField is `clone <path>` from <state>/clone, checked: a path that
// is no longer a clone of the swarm says so, and none recorded says that.
func cloneField(fs ops.FS, state, local string) string {
	b, err := fs.ReadFile(filepath.Join(state, "clone"))
	path := firstLine(b)
	switch {
	case err != nil || path == "":
		return "clone not recorded"
	case !isCloneAt(fs, path) || cloneState(fs, path, local) != state:
		return "clone " + path + " (not a clone of it now)"
	}
	return "clone " + path
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

// SwarmJournal is `daisy swarm journal [--mark <run>]` (docs/swarm.md
// section 11): the swarm's journal since the line <state>/connect-seen
// marks, under ReadOnly; with --mark, that mark moved to the run's line.
func SwarmJournal(c *Context) error {
	if _, mark := c.Value("--mark"); !mark {
		c.Ops = ops.ReadOnly(c.Ops)
	}
	return inSwarm(c, func() error { return swarmJournal(c) })
}

func swarmJournal(c *Context) error {
	cfg, err := forgeOnly(c)
	if err != nil {
		return err
	}
	fs, state := c.Ops.FS(), cfg.Forge.State(c.Env.Local)
	lines, err := reconcile.ReadJournal(fs, state)
	if err != nil {
		return fmt.Errorf("Error: swarm journal: %v", err)
	}
	seen := filepath.Join(state, "connect-seen")
	if run, ok := c.Value("--mark"); ok {
		if journalIndex(lines, run) < 0 {
			return fmt.Errorf("Error: swarm journal --mark: the journal holds no line of %s; mark a run its lines name", run)
		}
		if err := fs.WriteFile(seen, []byte(run+" "+c.Ops.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
			return fmt.Errorf("Error: swarm journal --mark: %v", err)
		}
		fmt.Fprintf(c.Stdout, "✓ Marked the journal seen through %s (%s)\n", run, seen)
		return nil
	}
	repo := cfg.Forge.Repo
	if len(lines) == 0 {
		fmt.Fprintf(c.Stdout, "Journal of %s: no line yet\n", repo)
		return nil
	}
	b, err := fs.ReadFile(seen)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return fmt.Errorf("Error: swarm journal: %v", err)
	}
	mark, at, _ := strings.Cut(firstLine(b), " ")
	var shown []reconcile.JournalLine
	if i := journalIndex(lines, mark); i >= 0 {
		shown = lines[i+1:]
		if len(shown) == 0 {
			fmt.Fprintf(c.Stdout, "Journal of %s: nothing since %s (marked seen %s)\n", repo, mark, at)
			return nil
		}
		fmt.Fprintf(c.Stdout, "Journal of %s since %s (marked seen %s): %s\n", repo, mark, at, plural(len(shown), "line", "lines"))
	} else {
		if mark != "" {
			fmt.Fprintf(c.Stderr, "⚠ %s names %s, which the journal does not hold: showing its newest day\n", seen, mark)
		}
		day := lines[len(lines)-1].Day
		for i := len(lines) - 1; i >= 0 && lines[i].Day == day; i-- {
			shown = lines[i:]
		}
		fmt.Fprintf(c.Stdout, "Journal of %s, %s (no line marked seen): %s\n", repo, day, plural(len(shown), "line", "lines"))
	}
	for _, l := range shown {
		fmt.Fprintf(c.Stdout, "%s %s\n", l.Day, l.Line)
	}
	fmt.Fprintf(c.Stdout, "Mark them seen: daisy swarm journal --mark %s\n", shown[len(shown)-1].Run)
	return nil
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
