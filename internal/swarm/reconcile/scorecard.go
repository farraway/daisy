package reconcile

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/swarm/declare"
)

// The scorecard (section 11, swarm metrics --scorecard; decision 0156):
// one row per merged slice, read from the metrics CSV of the home
// <state>/home names, the merges in the mirrors (ReadMerges) and each
// merged issue's opening on the forge, and a summary over a window. It
// writes nothing.

// Scorecard is the swarm's scorecard: the CSV it read, the mirrors' last
// fetch, the window (Since, zero for every record; Until, zero for none;
// Start, Since or else the swarm's first record or merge not after
// Until; End, Until or now, whichever is earlier), a row per slice
// merged in the window, in the order of the merges, and the window's
// summary, and the window's runs by configuration (R3-12).
type Scorecard struct {
	File                     string
	Fetched                  time.Time
	Since, Until, Start, End time.Time
	Slices                   []SliceScore
	Summary                  ScoreSummary
	Configs                  []ConfigScore
}

// Usage is a set of records' usage: the input not read from a cache
// (tokens_in less tokens_cached, all of it for a row with no
// tokens_cached), the cached and the output tokens, over the rows that
// report usage, NoUsage those that do not; the estimate at list price
// (cost_estimated), Unpriced the rows with none, counted as $0.
type Usage struct {
	Uncached, Cached, Output int
	NoUsage                  int
	Cost                     float64
	Unpriced                 int
}

// ConfigScore is one role's runs under one configuration, its provider,
// model, effort and prompt hash as their rows record them (R3-12): the
// runs, their usage, the stops (as a slice's are counted), and for a
// reviewer, the review_effect summed over the runs that record one,
// Known of them.
type ConfigScore struct {
	Role, Provider, Model, Effort, PromptHash string
	Runs                                      int
	Usage
	Stops         int
	Effect, Known int
}

// SliceScore is one merged slice's row: its merge; From, when its time
// starts (the issue's opening on the forge for the issue's first merged
// slice, FromOpening, else the issue's previous merge); its records'
// figures (a record is the slice's when it is of the issue and dated
// after the previous merge and not after this one; the record of the
// `merge` verb's intervention, made once the merge is, is the slice's
// whose merge is the last not after it): the runs by role; their usage;
// the highest review round; the stops by cause (runs whose outcome is
// blocked or question, or whose cause is set; "none" for a stop with no
// cause); the interventions by type.
type SliceScore struct {
	Merge
	From        time.Time
	FromOpening bool
	Runs        map[string]int
	Usage
	Rounds        int
	Stops         map[string]int
	Interventions map[string]int
}

// ScoreSummary is the window's figures: the slices merged in it, and of
// the swarm's records dated in it (every issue's, the batch's included,
// merged or not), the runs, their input (tokens_in, the cache's
// included), the worker runs and those that stopped (outcome blocked or
// question), and the interventions.
type ScoreSummary struct {
	Merges, Runs, Input     int
	WorkerRuns, WorkerStops int
	Interventions           int
}

// Days is the window's length in days.
func (c Scorecard) Days() float64 { return c.End.Sub(c.Start).Hours() / 24 }

// scoreRecord is a row of the swarm's in the CSV as the scorecard reads
// it: the row, its time and its issue.
type scoreRecord struct {
	metrics.Row
	when time.Time
	n    int
}

// Scorecard reads the scorecard of the records and merges at or after
// since (every one when it is zero), its slices' openings through
// s.Forge. The records and merges after until (none when it is zero)
// are left out altogether, of the slices' rows as of the summary.
func (s *Swarm) Scorecard(since, until time.Time) (Scorecard, error) {
	c := Scorecard{Since: since, Until: until, Start: since, End: s.O.Now().UTC()}
	after := func(t time.Time) bool { return !until.IsZero() && t.After(until) }
	if !until.IsZero() && until.Before(c.End) {
		c.End = until
	}
	file, b, err := s.metricsCSV()
	c.File = file
	if err != nil {
		return c, err
	}
	latest, err := metrics.Latest(b)
	if err != nil {
		return c, fmt.Errorf("cannot read %s: %v", file, err)
	}
	var recs []scoreRecord
	for _, r := range latest {
		if r.Source != s.Cfg.Forge.Repo || r.Kind != metrics.SwarmRun && r.Kind != metrics.Intervention {
			continue
		}
		when, err := time.Parse(time.RFC3339, r.When)
		if err != nil {
			return c, fmt.Errorf("%s: %s's when is not an RFC 3339 time (got: %s)", file, r.Unit, r.When)
		}
		if after(when) {
			continue
		}
		n, err := strconv.Atoi(strings.SplitN(r.Unit, "-", 2)[0])
		if err != nil {
			return c, fmt.Errorf("%s: %s names no issue", file, r.Unit)
		}
		recs = append(recs, scoreRecord{r, when.UTC(), n})
	}
	slices.SortFunc(recs, func(a, b scoreRecord) int { return a.when.Compare(b.when) })
	merges, fetched, err := ReadMerges(s.O, s.state(), s.Cfg.Base)
	if err != nil {
		return c, err
	}
	c.Fetched = fetched
	merges = slices.DeleteFunc(merges, func(m Merge) bool { return after(m.At) })
	if c.Start.IsZero() {
		if len(recs) > 0 {
			c.Start = recs[0].when
		}
		if len(merges) > 0 && (c.Start.IsZero() || merges[0].At.Before(c.Start)) {
			c.Start = merges[0].At
		}
	}

	byIssue := map[int][]int{} // an issue's merges, by their index, in time order
	for i, m := range merges {
		byIssue[m.N] = append(byIssue[m.N], i)
	}
	rows := make([]SliceScore, len(merges))
	for i, m := range merges {
		rows[i] = SliceScore{Merge: m, Runs: map[string]int{}, Stops: map[string]int{}, Interventions: map[string]int{}}
	}
	configs := map[string]int{} // a configuration's index in c.Configs
	for _, r := range recs {
		if i, ok := sliceOfRecord(r, merges, byIssue[r.n]); ok {
			rows[i].add(r)
		}
		if r.when.Before(since) {
			continue
		}
		switch r.Kind {
		case metrics.Intervention:
			c.Summary.Interventions++
		case metrics.SwarmRun:
			key := strings.Join([]string{r.Role, r.Provider, r.Model, r.Effort, r.PromptHash}, "\x00")
			i, ok := configs[key]
			if !ok {
				i = len(c.Configs)
				configs[key] = i
				c.Configs = append(c.Configs, ConfigScore{Role: r.Role, Provider: r.Provider, Model: r.Model, Effort: r.Effort, PromptHash: r.PromptHash})
			}
			c.Configs[i].add(r)
			c.Summary.Runs++
			c.Summary.Input += atoi(r.TokensIn)
			if r.Role == "worker" {
				c.Summary.WorkerRuns++
				if r.Outcome == "blocked" || r.Outcome == "question" {
					c.Summary.WorkerStops++
				}
			}
		}
	}
	// The roles in their order, the QA's last and any other after it;
	// each role's configurations in the order of their first run.
	order := func(role string) int {
		if i := slices.Index(declare.AllRoles, role); i >= 0 {
			return i
		}
		return len(declare.AllRoles)
	}
	slices.SortStableFunc(c.Configs, func(a, b ConfigScore) int { return order(a.Role) - order(b.Role) })
	opened := map[int]time.Time{}
	for i, m := range merges {
		if m.At.Before(since) {
			continue
		}
		row := rows[i]
		if j := slices.Index(byIssue[m.N], i); j > 0 {
			row.From = merges[byIssue[m.N][j-1]].At
		} else {
			if _, ok := opened[m.N]; !ok {
				is, err := s.Forge.Issue(m.N)
				if err != nil {
					return c, fmt.Errorf("cannot read #%d's opening: %v", m.N, err)
				}
				opened[m.N] = is.Created.UTC()
			}
			row.From, row.FromOpening = opened[m.N], true
		}
		c.Slices = append(c.Slices, row)
		c.Summary.Merges++
	}
	return c, nil
}

// sliceOfRecord is the index in merges of the slice r is a record of,
// among its issue's merges (their indexes, in time order): the first
// merged not before r, but for the `merge` verb's intervention, the last
// merged not after it; ok is false for a record of no merged slice.
func sliceOfRecord(r scoreRecord, merges []Merge, issue []int) (int, bool) {
	if r.Kind == metrics.Intervention && r.Outcome == "merge" {
		for j := len(issue) - 1; j >= 0; j-- {
			if !merges[issue[j]].At.After(r.when) {
				return issue[j], true
			}
		}
		return 0, false
	}
	for _, i := range issue {
		if !merges[i].At.Before(r.when) {
			return i, true
		}
	}
	return 0, false
}

// add counts a record of the slice's.
func (sc *SliceScore) add(r scoreRecord) {
	if r.Kind == metrics.Intervention {
		sc.Interventions[r.Cause]++
		return
	}
	sc.Runs[r.Role]++
	sc.Usage.add(r.Row)
	sc.Rounds = max(sc.Rounds, atoi(r.ReviewRounds))
	if stopped(r.Row) {
		cause := r.Cause
		if cause == "" {
			cause = "none"
		}
		sc.Stops[cause]++
	}
}

// add counts a run of the configuration's.
func (cs *ConfigScore) add(r scoreRecord) {
	cs.Runs++
	cs.Usage.add(r.Row)
	if stopped(r.Row) {
		cs.Stops++
	}
	if e, err := strconv.Atoi(r.ReviewEffect); err == nil {
		cs.Effect += e
		cs.Known++
	}
}

// add counts a run's usage.
func (u *Usage) add(r metrics.Row) {
	if r.TokensIn == "" {
		u.NoUsage++
	} else {
		in, cached := atoi(r.TokensIn), atoi(r.TokensCached)
		u.Uncached += in - cached
		u.Cached += cached
		u.Output += atoi(r.TokensOut)
	}
	if v, err := strconv.ParseFloat(r.CostEstimated, 64); err == nil {
		u.Cost += v
	} else {
		u.Unpriced++
	}
}

// stopped is whether a run's row is a stop: its outcome blocked or
// question (a member's), or its cause set (Daisy's own).
func stopped(r metrics.Row) bool {
	return r.Outcome == "blocked" || r.Outcome == "question" || r.Cause != ""
}

// atoi reads a cell metrics wrote; an empty or unreadable one is 0.
func atoi(s string) int { n, _ := strconv.Atoi(s); return n }
