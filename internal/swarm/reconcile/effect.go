package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/env"
	"daisy/internal/metrics"
	"daisy/internal/ops"
)

// A review's effect (7.6, The run's record; decision
// [142](docs/swarm/decisions/0142-a-reviews-effect-read-from-the-runs.md)):
// the findings a reviewer run raised that were fixed, its row's
// review_effect, read from what the runs left, no model judging a
// finding. A review not posted raised none; nor did one approving. A
// review requesting changes is credited its `[blocking]` findings when
// its role's next posted review on the issue is on the same pull
// request, whatever round (a later round: the rounds counted on; the
// other reviewer may have completed the rounds between alone, decision
// 163), on another head (the worker pushed since) and approves; 0 when
// that review is on the same head (the findings answered, not fixed) or
// requests changes again; unknown while no next review is collected, or
// when the next is of a round not after its own (a re-plan counts the
// rounds again; another pull request starts at 1).

// runEffect is a reviewer run's review_effect, "" for unknown.
type runEffect struct {
	run    string // its directory's name, <n>-<role>-<k>
	k      int
	effect string
}

// reviewRun is a reviewer run as its effect reads it.
type reviewRun struct {
	posted, approves bool
	round            int // 0 for a run without its round line
	head             string
	blocking         int
}

// isReviewer is whether role reviews a pull request.
func isReviewer(role string) bool { return role == "adversary" || role == "sympathetic" }

// reviewEffects is the effect of each run of r's role on r's issue under
// the state, in the order of their numbers; outcome, when not "", is r's
// own, which its collection writes after the run's record.
func (s *Swarm) reviewEffects(r Run, outcome string) ([]runEffect, error) {
	fs := s.O.FS()
	runs, err := Runs(fs, s.state())
	if err != nil {
		return nil, err
	}
	runs = slices.DeleteFunc(runs, func(o Run) bool { return o.N != r.N || o.Role != r.Role })
	slices.SortFunc(runs, func(a, b Run) int { return a.K - b.K })
	rs := make([]reviewRun, len(runs))
	for i, o := range runs {
		lines, err := o.Result(fs)
		if err != nil {
			return nil, err
		}
		got, _ := Has(lines, "outcome")
		if o.K == r.K && outcome != "" {
			got = "outcome " + outcome
		}
		if rs[i].posted = got == "outcome posted"; !rs[i].posted {
			continue
		}
		out, err := o.Output(fs)
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return nil, err
		}
		rs[i].approves = verdictOf(string(out)) == "approve"
		rs[i].blocking = len(blockingLines(string(out)))
		if l, ok := Has(lines, "round"); ok {
			rs[i].round, _ = strconv.Atoi(strings.TrimPrefix(l, "round "))
		}
		if l, ok := Has(lines, "start"); ok {
			rs[i].head = strings.TrimPrefix(l, "start ")
		}
	}
	out := make([]runEffect, len(runs))
	for i, o := range runs {
		out[i] = runEffect{filepath.Base(o.Dir), o.K, effectOf(rs[i], rs[i+1:])}
	}
	return out, nil
}

// effectOf is a review's effect given the reviews of its role on its
// issue after it.
func effectOf(r reviewRun, after []reviewRun) string {
	if !r.posted || r.approves {
		return "0"
	}
	i := slices.IndexFunc(after, func(n reviewRun) bool { return n.posted })
	switch {
	case i < 0 || r.round == 0 || after[i].round <= r.round:
		return ""
	case after[i].head == r.head || !after[i].approves:
		return "0"
	}
	return strconv.Itoa(r.blocking)
}

// reviewEffect is the effect of the run being recorded, "" with a
// warning when its runs cannot be read.
func (c *collection) reviewEffect(outcome string) string {
	effects, err := c.s.reviewEffects(c.r, outcome)
	if err != nil {
		fmt.Fprintf(c.s.Err, "⚠ %sreview_effect not made: %v\n", c.head, err)
		return ""
	}
	for _, e := range effects {
		if e.k == c.r.K {
			return e.effect
		}
	}
	return ""
}

// earlierEffects supersedes, in the metrics CSV of the home
// <state>/home names, the last row of each earlier run of the recorded
// run's role on its issue whose effect the runs now give otherwise: a
// review requesting changes has its effect once the next is collected
// (reviewEffects). Every other cell is carried over. No home named, no
// row; a failure is a warning.
func (c *collection) earlierEffects(outcome string) {
	s := c.s
	warn := func(err error) { fmt.Fprintf(s.Err, "⚠ %searlier review_effect not written: %v\n", c.head, err) }
	home, err := s.metricsHome()
	if home == "" {
		if err != nil {
			warn(err)
		}
		return
	}
	effects, err := s.reviewEffects(c.r, outcome)
	if err != nil {
		warn(err)
		return
	}
	effects = slices.DeleteFunc(effects, func(e runEffect) bool { return e.k >= c.r.K })
	supersede := func(fsys ops.FS, file string, _ ...metrics.Row) (int, error) {
		return s.supersedeEffects(fsys, file, c.r.Role, effects)
	}
	if _, err := s.locked(supersede, env.MetricsFile(home), "swarm apply"); err != nil {
		warn(err)
	}
}

// supersedeEffects appends to the CSV at file, for each of effects (runs
// of role) whose effect is known and differs from its run's last row's,
// that row with the effect, every other cell carried over; a run the CSV
// holds no row of is left to its own record. An unknown effect never
// supersedes a row, so one set by hand stays. The caller holds the
// metrics lock.
func (s *Swarm) supersedeEffects(fsys ops.FS, file, role string, effects []runEffect) (int, error) {
	b, err := fsys.ReadFile(file)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return 0, err
	}
	latest, err := metrics.Latest(b)
	if err != nil {
		return 0, err
	}
	var rows []metrics.Row
	for _, e := range effects {
		held, ok := latest[metrics.Row{Kind: metrics.SwarmRun, Source: s.Cfg.Forge.Repo, Unit: e.run, Role: role}.Key()]
		if ok && e.effect != "" && held.ReviewEffect != e.effect {
			held.ReviewEffect = e.effect
			rows = append(rows, held)
		}
	}
	return metrics.Supersede(fsys, file, rows...)
}
