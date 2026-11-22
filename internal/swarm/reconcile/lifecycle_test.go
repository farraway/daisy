package reconcile

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
)

// transition is a row of section 5's table as the explorer checks
// Daisy's writes against it: the stages it leaves ("*" any), what it
// writes (a stage, a terminal, `closed`, or a flag set, `+<flag>`, or
// cleared, `-<flag>`), the gate it holds under ("" every setting) and
// whether Daisy makes it (the maintainer's rows launch nothing and are
// written here so that the table is the spec's whole).
type transition struct {
	id    string
	from  []string
	to    string
	gate  string
	daisy bool
}

// sectionFive is section 5's table, row by row: a row the spec draws as
// several moves (T11's two targets and its second time, T15's close and
// terminal, T22's homes) is one transition per move. TestSectionFiveTable
// holds it to docs/swarm.md.
var sectionFive = func() []transition {
	const (
		pp, pa, p0, p1, p2 = forge.ProposalPosted, forge.ProposalAccepted, forge.PlanPosted, forge.PlanAccepted, forge.PlanInProgress
		r0, r1, r2, r3     = forge.ReviewReady, forge.ReviewInProgress, forge.ReviewApproved, forge.ChangesRequested
		blocked, question  = "+" + forge.Blocked, "+" + forge.Question
	)
	any := []string{"*"}
	return []transition{
		{"T1", []string{""}, pp, "G1 on", true},
		{"T2", []string{""}, pa, "G1 off", true},
		{"T3", []string{pp}, pa, "G1 on", false},
		{"T4", []string{pp}, pa, "G1 off", true},
		{"T5", []string{pa}, p0, "G2 on", true},
		{"T6", []string{pa}, p1, "G2 off", true},
		{"T7", []string{p0}, p1, "", false},
		{"T8", []string{p2, p1}, r0, "", true},
		{"T9", []string{r1, r0}, r2, "", true},
		{"T10", []string{r1, r0}, r3, "", true},
		{"T11", []string{r1, r0}, p0, "G2 on", true},
		{"T11", []string{r1, r0}, pa, "G2 off", true},
		{"T11", []string{r1, r0}, blocked, "", true},
		{"T12", []string{p2, r3}, r0, "", true},
		{"T13", []string{r2}, p1, "G3 on", false},
		{"T14", []string{r2}, p1, "G3 off", true},
		{"T15", []string{r2}, "closed", "G3 off", true},
		{"T15", []string{r2}, forge.Done, "G3 off", true},
		{"T16", []string{r2}, blocked, "G3 off", true},
		{"T17", any, blocked, "", true},
		{"T17", any, "-" + forge.Question, "", true},
		{"T18", any, "-" + forge.Blocked, "", true},
		{"T19", any, "closed", "", false},
		{"T19", any, forge.Done, "", false},
		{"T19", any, forge.Rejected, "", false},
		{"T20", []string{p1, r3}, p2, "", true},
		{"T21", []string{r0}, r1, "", true},
		{"T22", []string{p2}, p1, "", true},
		{"T22", []string{p2}, r3, "", true},
		{"T22", []string{r1}, r0, "", true},
		{"T23", []string{pp}, pa, "G1 on", true},
		{"T24", []string{p0}, p1, "G2 on", true},
		{"T25", []string{r2}, r3, "", true},
		{"T25", []string{r2}, blocked, "", true},
		{"T26", []string{r2}, r3, "", true},
		{"T27", []string{p1}, p0, "G2 on", true},
		{"T28", any, question, "", true},
		{"T29", any, "-" + forge.Question, "", true},
		{"T30", any, "-" + forge.Question, "", true},
		{"T30", any, blocked, "", true},
		{"T31", []string{p1}, pa, "", false},
		{"T31", []string{r0, r1, r2, r3}, p1, "", false},
	}
}()

// holds: the gate condition holds under g.
func holds(gate string, g declare.Gates) bool {
	if gate == "" {
		return true
	}
	name, state, _ := strings.Cut(gate, " ")
	return map[string]bool{"G1": g.Triage, "G2": g.Plan, "G3": g.Merge}[name] == (state == "on")
}

// allowed matches the write that took an issue from before to after
// against section 5's table: each move it makes (the stage left for the
// stage or terminal written, a close, each flag set or cleared) is
// matched alone with the rows of Daisy's that make it under the gates,
// and taken is each move with those rows; why is "" when every move has
// one. swarm:gated is step 8's, by the predicate (section 5, Gated), and
// is no row; the flags removed from a closed issue are the close's (T15,
// T19).
func (w *world) allowed(before, after forge.Issue) (taken map[string]string, why string) {
	g := w.s.Cfg.Gates
	from, _ := StageOf(before)
	to, _ := StageOf(after)
	if from == "" {
		from = "no stage"
	}
	if to == "" {
		to = "no stage"
	}
	var moves []string
	if before.Open && !after.Open {
		moves = append(moves, "closed")
	}
	for _, l := range after.Labels {
		if !slices.Contains(before.Labels, l) && (l == forge.Done || l == forge.Rejected) {
			to = l
		}
	}
	if from != to {
		moves = append(moves, to)
	}
	for _, l := range after.Labels {
		if !slices.Contains(before.Labels, l) && slices.Contains(forge.Flags, l) && l != forge.Gated {
			moves = append(moves, "+"+l)
		}
	}
	for _, l := range before.Labels {
		if !slices.Contains(after.Labels, l) && slices.Contains(forge.Flags, l) && l != forge.Gated && after.Open {
			moves = append(moves, "-"+l)
		}
	}
	taken = map[string]string{}
	for _, m := range moves {
		var rows []string
		for _, t := range sectionFive {
			if t.daisy && t.to == m && holds(t.gate, g) && (slices.Contains(t.from, "*") || slices.Contains(t.from, from)) && !slices.Contains(rows, t.id) {
				rows = append(rows, t.id)
			}
		}
		if len(rows) == 0 {
			return taken, "no row of section 5's table moves " + from + " to " + m + " by Daisy"
		}
		taken[from+" → "+m] = strings.Join(rows, "/")
	}
	return taken, ""
}

// described is how a row of section 5's table names a stage it does not
// write out, by the row and the stage: T10 and T11 leave review-0-ready
// "as T9" does, when T21's write was refused.
var described = map[string]string{
	"T10 " + forge.ReviewReady:      "as T9",
	"T11 " + forge.ReviewReady:      "as T9",
	"T22 " + forge.PlanInProgress:   "the in-progress stage a run's launch wrote",
	"T22 " + forge.PlanAccepted:     "the stage the run was launched on",
	"T22 " + forge.ChangesRequested: "the stage the run was launched on",
	"T31 " + forge.ReviewReady:      "a review stage",
	"T31 " + forge.ReviewInProgress: "a review stage",
	"T31 " + forge.ReviewApproved:   "a review stage",
	"T31 " + forge.ChangesRequested: "a review stage",
}

// named: the row holds the description, one being given.
func named(row, description string) bool {
	return description != "" && strings.Contains(row, description)
}

// sectionFiveRow is a row of section 5's table in docs/swarm.md.
var sectionFiveRow = regexp.MustCompile("^\\| (T[0-9]+) \\|")

// The explorer's table is section 5's (docs/swarm.md): every row of the
// spec's table is one of its ids, and each of its transitions names, in
// that row, the labels it moves between, its gate and its setter, so a
// row renumbered, retargeted or given to another setter fails here
// before the explorer checks against a table the spec no longer has.
func TestSectionFiveTable(t *testing.T) {
	b, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	_, five, ok := strings.Cut(string(b), "\n## 5. ")
	five, _, _ = strings.Cut(five, "\n## 6. ")
	if !ok {
		t.Fatal("docs/swarm.md has no section 5 heading, ## 5.")
	}
	rows := map[string]string{}
	var ids []string
	for _, l := range strings.Split(five, "\n") {
		if m := sectionFiveRow.FindStringSubmatch(l); m != nil {
			rows[m[1]] = l
			ids = append(ids, m[1])
		}
	}
	var mine []string
	for _, tr := range sectionFive {
		if !slices.Contains(mine, tr.id) {
			mine = append(mine, tr.id)
		}
		row, ok := rows[tr.id]
		if !ok {
			t.Errorf("%s: no such row in docs/swarm.md section 5", tr.id)
			continue
		}
		labels := append(slices.Clone(tr.from), strings.TrimLeft(tr.to, "+-"))
		for _, l := range labels {
			switch {
			case l == "" || l == "*":
			case l == "closed":
				if !strings.Contains(row, "closed") {
					t.Errorf("%s: the row closes nothing:\n%s", tr.id, row)
				}
			case !strings.Contains(row, l) && !strings.Contains(row, "`"+strings.TrimPrefix(l, "swarm:")+"`") && !named(row, described[tr.id+" "+l]):
				t.Errorf("%s: the row names no %s:\n%s", tr.id, l, row)
			}
		}
		if tr.gate != "" && !strings.Contains(row, tr.gate) {
			t.Errorf("%s: the row holds under no %s:\n%s", tr.id, tr.gate, row)
		}
		if setter := map[bool]string{true: "Daisy", false: "the maintainer"}[tr.daisy]; !strings.Contains(row, setter) {
			t.Errorf("%s: the row's setter is not %s:\n%s", tr.id, setter, row)
		}
	}
	if !slices.Equal(ids, mine) {
		t.Errorf("section 5's rows are %v; the explorer's table has %v", ids, mine)
	}
}
