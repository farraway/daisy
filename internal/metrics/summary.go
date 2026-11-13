package metrics

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"slices"
	"strconv"
	"strings"
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

// Summarize reads the CSV AppendCSV writes.
func Summarize(file []byte) (Summary, error) {
	var s Summary
	if len(bytes.TrimSpace(file)) == 0 {
		return s, nil
	}
	if err := checkHeader(file); err != nil {
		return s, err
	}
	r := csv.NewReader(bytes.NewReader(file))
	r.FieldsPerRecord = len(Header)
	records, err := r.ReadAll()
	if err != nil {
		return s, err
	}
	records = records[1:]
	s.Rows = len(records)
	col := func(name string) int { return slices.Index(Header, name) }
	latest := map[string]int{} // a row's key -> its last row
	key := func(rec []string) string {
		return strings.Join([]string{rec[col("kind")], rec[col("plan_or_swarm")], rec[col("step_or_run")], rec[col("role")]}, "\x00")
	}
	for i, rec := range records {
		latest[key(rec)] = i
	}
	groups := map[[2]string]*Group{}
	for i, rec := range records {
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
