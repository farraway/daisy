package metrics

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Summary is the CSV read back: its rows, those a later row for the same
// kind, plan or swarm, step or run and role supersedes (a correction is
// appended, never written over the row it corrects), and the rest
// grouped by kind and model.
type Summary struct {
	Rows, Superseded int
	Groups           []Group
}

// Group is the rows of one kind and model: how many, the tokens and costs
// of those that report them, and the lines of those measured.
type Group struct {
	Kind                       Kind
	Model                      string
	Rows, WithTokens, WithCost int
	WithEstimate, Measured     int
	Tokens                     int
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
	if t := cell("tokens_total"); t != "" {
		g.Tokens += atoi(t)
		g.WithTokens++
	} else if in, out := cell("tokens_in"), cell("tokens_out"); in != "" && out != "" {
		g.Tokens += atoi(in) + atoi(out)
		g.WithTokens++
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

// atoi and atof read cells CheckCell has passed; an empty one is 0.
func atoi(s string) int     { n, _ := strconv.Atoi(s); return n }
func atof(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
