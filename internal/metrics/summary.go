package metrics

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Summary is the CSV read back: its rows, those a later row for the same
// kind, plan or swarm, step or run and role supersedes (a correction is
// appended, never written over the row it corrects), and the rest
// grouped by kind and model, and, of kind plan-step, by plan; Swarms
// names the swarms of the rest of kind swarm-run, in name order.
type Summary struct {
	Rows, Superseded int
	Groups           []Group
	Plans            []Plan
	Swarms           []string
}

// Plan is a plan's plan-step rows, every role's (the step's, its
// reviews', the orchestrator's): their tokens, over those that report
// them, the code lines their measured rows added, and the distinct
// commit ranges they record, in row order (a step's, recorded by more
// than one role, once).
type Plan struct {
	Name                       string
	Rows, WithTokens, Measured int
	Tokens, CodeAdded          int
	Ranges                     []string
}

// TokensPerCodeLine is the plan's tokens per code line it added,
// rounded; ok is false when it reports no tokens or added no code.
func (p Plan) TokensPerCodeLine() (n int, ok bool) {
	if p.WithTokens == 0 || p.CodeAdded == 0 {
		return 0, false
	}
	return int(math.Round(float64(p.Tokens) / float64(p.CodeAdded))), true
}

// Group is the rows of one kind and model: how many, the tokens and costs
// of those that report them, and the lines of those measured.
type Group struct {
	Kind                       Kind
	Model                      string
	Rows, WithTokens, WithCost int
	WithEstimate, Measured     int
	WithCached                 int
	Tokens, Cached             int
	Cost, Estimate             float64
	Lines                      Lines
}

// Filter is the rows a summary takes: with Swarm, that swarm's runs
// (kind swarm-run, plan_or_swarm Swarm); with Since, the rows whose
// `when` is not before it. The zero Filter takes every row.
type Filter struct {
	Swarm string
	Since time.Time
}

// keeps is whether the filter takes rec, row number n of the file; with
// Since, a `when` that is not RFC 3339 is refused, the row named.
func (f Filter) keeps(rec []string, n int, col func(string) int) (bool, error) {
	if f.Swarm != "" && (rec[col("kind")] != string(SwarmRun) || rec[col("plan_or_swarm")] != f.Swarm) {
		return false, nil
	}
	if f.Since.IsZero() {
		return true, nil
	}
	when, err := time.Parse(time.RFC3339, rec[col("when")])
	if err != nil {
		return false, fmt.Errorf("row %d: when is not an RFC 3339 time (got: %s)", n, rec[col("when")])
	}
	return !when.Before(f.Since), nil
}

// Summarize reads the CSV AppendCSV writes, the rows f takes: Rows counts
// them, Superseded those of them a later row of the same key supersedes
// (taken or not: a correction outside the filter still hides the row it
// corrects), and the rest are grouped.
func Summarize(file []byte, f Filter) (Summary, error) {
	var s Summary
	recs, err := records(file)
	if err != nil {
		return s, err
	}
	col := func(name string) int { return slices.Index(Header, name) }
	latest := map[string]int{} // a row's key -> its last row
	for i, rec := range recs {
		latest[key(rec)] = i
	}
	groups := map[[2]string]*Group{}
	plans := map[string]*Plan{}
	swarms := map[string]bool{}
	for i, rec := range recs {
		keep, err := f.keeps(rec, i+2, col)
		if err != nil {
			return s, err
		}
		if !keep {
			continue
		}
		s.Rows++
		if latest[key(rec)] != i {
			s.Superseded++
			continue
		}
		k := [2]string{rec[col("kind")], rec[col("model")]}
		g := groups[k]
		if g == nil {
			g = &Group{Kind: Kind(k[0]), Model: k[1]}
			groups[k] = g
		}
		if err := g.add(rec, col); err != nil {
			return s, fmt.Errorf("row %d: %v", i+2, err)
		}
		if k[0] == string(PlanStep) {
			name := rec[col("plan_or_swarm")]
			p := plans[name]
			if p == nil {
				p = &Plan{Name: name}
				plans[name] = p
			}
			p.add(rec, col)
		}
		if k[0] == string(SwarmRun) {
			swarms[rec[col("plan_or_swarm")]] = true
		}
	}
	s.Swarms = slices.Sorted(maps.Keys(swarms))
	for _, name := range slices.Sorted(maps.Keys(plans)) {
		s.Plans = append(s.Plans, *plans[name])
	}
	for _, g := range groups {
		s.Groups = append(s.Groups, *g)
	}
	slices.SortFunc(s.Groups, func(a, b Group) int {
		return strings.Compare(string(a.Kind)+"\x00"+a.Model, string(b.Kind)+"\x00"+b.Model)
	})
	return s, nil
}

func (g *Group) add(rec []string, col func(string) int) error {
	cell := func(name string) string { return rec[col(name)] }
	for _, name := range Header {
		if why := CheckCell(name, cell(name)); why != "" {
			return fmt.Errorf("%s %s (got: %s)", name, why, cell(name))
		}
	}
	g.Rows++
	if n, ok := rowTokens(rec, col); ok {
		g.Tokens += n
		g.WithTokens++
	}
	if c := cell("tokens_cached"); c != "" {
		g.Cached += atoi(c)
		g.WithCached++
	}
	if c := cell("cost_reported"); c != "" {
		g.Cost += atof(c)
		g.WithCost++
	}
	if c := cell("cost_estimated"); c != "" {
		g.Estimate += atof(c)
		g.WithEstimate++
	}
	if cell("code_added") != "" {
		l := Lines{atoi(cell("code_added")), atoi(cell("code_removed")), atoi(cell("tests_added")),
			atoi(cell("tests_removed")), atoi(cell("prose_added")), atoi(cell("prose_removed")), atoi(cell("binary_files"))}
		g.Lines.plus(l)
		g.Measured++
	}
	return nil
}

// add counts a row Group.add has checked.
func (p *Plan) add(rec []string, col func(string) int) {
	p.Rows++
	if n, ok := rowTokens(rec, col); ok {
		p.Tokens += n
		p.WithTokens++
	}
	if c := rec[col("code_added")]; c != "" {
		p.CodeAdded += atoi(c)
		p.Measured++
	}
	if r := rec[col("range")]; r != "" && !slices.Contains(p.Ranges, r) {
		p.Ranges = append(p.Ranges, r)
	}
}

// rowTokens is the row's tokens: tokens_total, else tokens_in and
// tokens_out added; ok is false when it has neither.
func rowTokens(rec []string, col func(string) int) (int, bool) {
	if t := rec[col("tokens_total")]; t != "" {
		return atoi(t), true
	}
	if in, out := rec[col("tokens_in")], rec[col("tokens_out")]; in != "" && out != "" {
		return atoi(in) + atoi(out), true
	}
	return 0, false
}

// atoi and atof read cells CheckCell has passed; an empty one is 0.
func atoi(s string) int     { n, _ := strconv.Atoi(s); return n }
func atof(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
