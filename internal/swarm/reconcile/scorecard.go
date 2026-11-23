package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
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
// model, effort and config hash as their rows record them (R3-12, the
// config hash since R3-12b; "" for rows without one): the
// runs, their usage, the stops (as a slice's are counted), and for a
// reviewer, the review_effect summed over the runs that record one,
// Known of them.
type ConfigScore struct {
	Role, Provider, Model, Effort, ConfigHash string
	Runs                                      int
	Usage
	Stops         int
	Effect, Known int
	Balance
}

// Balance is a reviewer configuration's side of the balance (section
// 7.11, rule 7): the merged heads its runs approved, their approval
// standing, and those slices' escapes; the follow-up items its runs
// raised, and those proved real.
type Balance struct {
	Approved, Escaped  int
	Raised, RaisedReal int
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
	Escapes       Escapes
	FollowUp      int            // its follow-up's issue, 0 for none
	Items         []FollowUpItem // the follow-up's items, in its order
	Approvers     []string       // the reviewer runs whose approval of its head stood
}

// FollowUpItem is an item of a merged slice's follow-up (section 7.11,
// Follow-ups): the run that raised it, `<n>-<role>-<k>`, and whether it
// proved real, a `Follow-up: #<f> <i>` line naming it in the body of an
// issue with a merged slice (The balance).
type FollowUpItem struct {
	Run  string
	Real bool
}

// ProvedReal is how many of the slice's follow-up items proved real.
func (sc SliceScore) ProvedReal() int {
	n := 0
	for _, it := range sc.Items {
		if it.Real {
			n++
		}
	}
	return n
}

// Escapes is a merged slice's escapes (section 7.11, The balance), each
// made up to --until: its merge reverted; the issues of forge.repo, open
// or closed, whose body holds a line `Defect in: <name>#<m>` for its pull
// request (Filed); the commits on base's first-parent line in a mirror
// that are no swarm merge and whose message holds that line (ByHand).
type Escapes struct {
	Reverted      bool
	Filed, ByHand int
}

// Count is how many escapes e holds.
func (e Escapes) Count() int {
	n := e.Filed + e.ByHand
	if e.Reverted {
		n++
	}
	return n
}

// ScoreSummary is the window's figures: the slices merged in it, and of
// the swarm's records dated in it (every issue's, the batch's included,
// merged or not), the runs, their input (tokens_in, the cache's
// included), the worker runs and those that stopped (outcome blocked or
// question), and the interventions; and the escapes and follow-up items
// of the slices merged in it, summed (section 7.11, The balance).
type ScoreSummary struct {
	Merges, Runs, Input     int
	WorkerRuns, WorkerStops int
	Interventions           int
	Escapes                 EscapeTotals
	Items, ProvedReal       int
	Blocking                map[string]int // the window's posted reviews' blocking findings, by kind
}

// EscapeTotals is the escapes of the window's merged slices (section
// 7.11, The balance): the slices reverted, the issues filed and the hand
// fixes, each counted per slice it names.
type EscapeTotals struct {
	Reverted, Filed, ByHand int
}

// Count is how many escapes t holds.
func (t EscapeTotals) Count() int { return t.Reverted + t.Filed + t.ByHand }

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
	c := Scorecard{Since: since, Until: until, Start: since, End: s.O.Now().UTC(), Summary: ScoreSummary{Blocking: map[string]int{}}}
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
	merges, fetched, _, err := ReadMerges(s.O, s.state(), s.Cfg.Base, until)
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
		rows[i] = SliceScore{Merge: m, Runs: map[string]int{}, Stops: map[string]int{}, Interventions: map[string]int{}, Escapes: Escapes{Reverted: m.Reverted}}
	}
	var issues []forge.Issue // forge.repo's, open and closed, opened up to until
	for _, state := range []forge.State{forge.Open, forge.Closed} {
		is, err := s.Forge.Issues(state, "")
		if err != nil {
			return c, fmt.Errorf("cannot read the issues: %v", err)
		}
		issues = append(issues, slices.DeleteFunc(is, func(i forge.Issue) bool { return after(i.Created) })...)
	}
	slices.SortFunc(issues, func(a, b forge.Issue) int { return a.Number - b.Number })
	if err := s.escapes(rows, issues, after); err != nil {
		return c, err
	}
	owner, _, _ := strings.Cut(s.Cfg.Forge.Repo, "/")
	followUps(rows, issues, owner)
	configs := map[string]int{}         // a configuration's index in c.Configs
	ofRun := map[string]int{}           // a window's run's configuration, by its index
	reviews := map[slotOf]scoreRecord{} // a slice's newest posted review of a role
	for _, r := range recs {
		if i, ok := sliceOfRecord(r, merges, byIssue[r.n]); ok {
			rows[i].add(r)
			if r.Kind == metrics.SwarmRun && isReviewer(r.Role) && r.Outcome == "posted" {
				k := slotOf{i, r.Role}
				if last, ok := reviews[k]; !ok || runNumber(last.Unit) < runNumber(r.Unit) {
					reviews[k] = r
				}
			}
		}
		if r.when.Before(since) {
			continue
		}
		switch r.Kind {
		case metrics.Intervention:
			c.Summary.Interventions++
		case metrics.SwarmRun:
			key := strings.Join([]string{r.Role, r.Provider, r.Model, r.Effort, r.ConfigHash}, "\x00")
			i, ok := configs[key]
			if !ok {
				i = len(c.Configs)
				configs[key] = i
				c.Configs = append(c.Configs, ConfigScore{Role: r.Role, Provider: r.Provider, Model: r.Model, Effort: r.Effort, ConfigHash: r.ConfigHash})
			}
			c.Configs[i].add(r)
			ofRun[r.Unit] = i
			if err := s.countBlocking(r, c.Summary.Blocking); err != nil {
				return c, err
			}
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
	if err := s.approvers(rows, reviews); err != nil {
		return c, err
	}
	for _, row := range rows {
		for _, run := range row.Approvers {
			if i, ok := ofRun[run]; ok {
				c.Configs[i].Approved++
				c.Configs[i].Escaped += row.Escapes.Count()
			}
		}
		for _, it := range row.Items {
			if i, ok := ofRun[it.Run]; ok {
				c.Configs[i].Raised++
				if it.Real {
					c.Configs[i].RaisedReal++
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
		if row.Escapes.Reverted {
			c.Summary.Escapes.Reverted++
		}
		c.Summary.Escapes.Filed += row.Escapes.Filed
		c.Summary.Escapes.ByHand += row.Escapes.ByHand
		c.Summary.Items += len(row.Items)
		c.Summary.ProvedReal += row.ProvedReal()
	}
	return c, nil
}

// defectIn is a line naming a merged swarm pull request a defect is in
// (section 7.11, The balance).
var defectIn = regexp.MustCompile(`^Defect in: ([^\s#]+)#([0-9]+)$`)

// defects is the pull requests text's `Defect in: <name>#<m>` lines
// name, each once, as `<name>#<m>`.
func defects(text string) map[string]bool {
	named := map[string]bool{}
	for _, l := range strings.Split(text, "\n") {
		if m := defectIn.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			named[fmt.Sprintf("%s#%d", m[1], atoi(m[2]))] = true
		}
	}
	return named
}

// escapes counts each merged slice's escapes filed and fixed by hand
// (Escapes): of issues, those whose body names its pull request on a
// `Defect in:` line; of the commits on base's first-parent line in each
// declared repository's mirror, not after until (after), those no swarm
// merge whose message names it.
func (s *Swarm) escapes(rows []SliceScore, issues []forge.Issue, after func(time.Time) bool) error {
	named := map[string]*SliceScore{}
	merged := map[string]bool{}
	for i, r := range rows {
		merged[r.SHA] = true
		if r.Pull > 0 {
			named[fmt.Sprintf("%s#%d", r.Repo, r.Pull)] = &rows[i]
		}
	}
	for _, is := range issues {
		for k := range defects(is.Body) {
			if r := named[k]; r != nil {
				r.Escapes.Filed++
			}
		}
	}
	g := Git{O: s.O, State: s.state()}
	ref := "refs/heads/" + s.Cfg.Base
	for _, repo := range s.Cfg.Repos {
		m := g.Mirror(repo.Name)
		if g.Head(m, ref) == "" {
			continue
		}
		out, err := g.run(m, "log", "--first-parent", "--format=%x00%H %ct%n%B", ref, "--")
		if err != nil {
			return err
		}
		for _, entry := range strings.Split(out, "\x00")[1:] {
			head, body, _ := strings.Cut(entry, "\n")
			sha, at, _ := strings.Cut(head, " ")
			if merged[sha] || after(time.Unix(int64(atoi(at)), 0)) {
				continue
			}
			for k := range defects(body) {
				if r := named[k]; r != nil {
					r.Escapes.ByHand++
				}
			}
		}
	}
	return nil
}

// followUpNames is a proposal's line naming the items of a follow-up its
// job fixes (section 7.5): the follow-up's number and the items'.
var followUpNames = regexp.MustCompile(`^Follow-up: #([0-9]+) (.+)$`)

// followUpItem is an item of a follow-up as the reconciler numbers it
// (section 7.11, Follow-ups): its number, the role and the run's number
// that raised it.
var followUpItem = regexp.MustCompile(`^([0-9]+)\. ([a-z]+) run ([0-9]+), round [0-9]+: `)

// followUps fills each merged slice's follow-up from issues, in the order
// of their numbers: the first carrying swarm:follow-up that the
// reconciler filed for its pull request (followUpOf), its items each
// proved real when a `Follow-up:` line of an issue with a merged slice
// names it.
func followUps(rows []SliceScore, issues []forge.Issue, owner string) {
	merged := map[int]bool{}
	for _, r := range rows {
		merged[r.N] = true
	}
	fixed := map[[2]int]bool{} // a follow-up's number and an item's
	for _, is := range issues {
		if !merged[is.Number] {
			continue
		}
		for _, l := range strings.Split(is.Body, "\n") {
			if m := followUpNames.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
				for _, i := range strings.Split(m[2], ",") {
					fixed[[2]int{atoi(m[1]), atoi(strings.TrimSpace(i))}] = true
				}
			}
		}
	}
	for i := range rows {
		r := &rows[i]
		if r.Pull == 0 {
			continue
		}
		for _, is := range issues {
			if !slices.Contains(is.Labels, forge.FollowUp) || !followUpOf(is.Body, owner, forge.Pull{Repo: r.Repo, Number: r.Pull}) {
				continue
			}
			r.FollowUp = is.Number
			for _, l := range strings.Split(followUpItems(is.Body), "\n") {
				if m := followUpItem.FindStringSubmatch(l); m != nil {
					r.Items = append(r.Items, FollowUpItem{Run: fmt.Sprintf("%d-%s-%s", r.N, m[2], m[3]), Real: fixed[[2]int{is.Number, atoi(m[1])}]})
				}
			}
			break
		}
	}
}

// slotOf is a merged slice, by its index, and a reviewer role.
type slotOf struct {
	i    int
	role string
}

// approvers fills each merged slice's Approvers from reviews, its newest
// posted review of each reviewer role: the run, when its out.md approves
// and its `start` line is the head merged (a run whose files are gone
// approved nothing).
func (s *Swarm) approvers(rows []SliceScore, reviews map[slotOf]scoreRecord) error {
	fs := s.O.FS()
	for k, r := range reviews {
		i := k.i
		run := Run{Dir: filepath.Join(s.state(), "runs", r.Unit), N: r.n, K: runNumber(r.Unit), Role: r.Role}
		lines, err := run.Result(fs)
		if err != nil {
			return err
		}
		out, err := run.Output(fs)
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return err
		}
		if head, _ := Has(lines, "start"); verdictOf(string(out)) == "approve" && strings.TrimPrefix(head, "start ") == rows[i].Head {
			rows[i].Approvers = append(rows[i].Approvers, r.Unit)
		}
	}
	for i := range rows {
		slices.Sort(rows[i].Approvers)
	}
	return nil
}

// BlockingKinds is the kinds of blocking finding, in the blocking
// sentence's order (section 7.11, The list, in the Task), then `unnamed`
// for a finding that names none.
var BlockingKinds = []string{"claim", "test", "defect", "introduced", "unclosed", "unnamed"}

// countBlocking counts, in kinds, the blocking findings of r when it is
// a posted review: each `[blocking]` line of its run's out.md by its kind
// (blockingKind; section 7.11; a run whose out.md is gone counts none).
func (s *Swarm) countBlocking(r scoreRecord, kinds map[string]int) error {
	if !isReviewer(r.Role) || r.Outcome != "posted" {
		return nil
	}
	out, err := Run{Dir: filepath.Join(s.state(), "runs", r.Unit)}.Output(s.O.FS())
	if errors.Is(err, iofs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, l := range blockingLines(string(out)) {
		kinds[blockingKind(l)]++
	}
	return nil
}

// blockingKind is the kind a `[blocking]` line names (section 7.11): the
// first word after the marker, case and a trailing colon aside, when it
// is one of BlockingKinds but `unnamed`; `unnamed` otherwise.
func blockingKind(line string) string {
	if f := strings.Fields(strings.TrimPrefix(line, "[blocking]")); len(f) > 0 {
		if k := strings.ToLower(strings.TrimSuffix(f[0], ":")); slices.Contains(BlockingKinds[:5], k) {
			return k
		}
	}
	return "unnamed"
}

// runNumber is a run's number, the `<k>` of its `<n>-<role>-<k>`.
func runNumber(unit string) int { return atoi(unit[strings.LastIndex(unit, "-")+1:]) }

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
