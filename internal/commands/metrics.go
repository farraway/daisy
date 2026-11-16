package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"daisy/internal/env"
	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/reconcile"
	"daisy/internal/verb"
)

// metricsReport is the summary: the CSV, whether it exists, the flags
// that took its rows, the rows taken and how many of them a later record
// supersedes, a group per kind and model, a plan's figures per plan, the
// repositories a plan step's range is looked up in for its reverts, and
// a swarm's merged and reverted pull requests per swarm.
type metricsReport struct {
	File      string `json:"file"`
	Exists    bool   `json:"exists"`
	RowsTaken struct {
		Swarm *string `json:"swarm"`
		Since *string `json:"since"`
	} `json:"rows_taken"`
	Rows         int            `json:"rows"`
	Superseded   int            `json:"superseded"`
	Groups       []metricsGroup `json:"groups"`
	Plans        []planFigures  `json:"plans"`
	Repositories []string       `json:"repositories"`
	Swarms       []swarmFigures `json:"swarms"`
	verb.Warned
}

// planFigures is a plan's plan-step rows, every role's: their tokens
// (nil when none reports them), the code lines their measured rows added
// (nil when none is measured), the tokens per code line (nil without
// tokens or code), and its steps' reverts: the distinct ranges its rows
// record, those a repository holds, those reverted, and the rate (nil
// with none found).
type planFigures struct {
	Plan              string   `json:"plan"`
	Rows              int      `json:"rows"`
	Tokens            *int     `json:"tokens"`
	RowsWithTokens    int      `json:"rows_with_tokens"`
	Measured          int      `json:"measured"`
	CodeAdded         *int     `json:"code_added"`
	TokensPerCodeLine *int     `json:"tokens_per_code_line"`
	Steps             int      `json:"steps"`
	StepsFound        int      `json:"steps_found"`
	StepsReverted     int      `json:"steps_reverted"`
	RevertRate        *float64 `json:"revert_rate"`
}

// swarmFigures is a swarm's pull requests merged and reverted, read from
// one <state>'s mirrors (reconcile.ReadPullReverts), on the base its
// mirrored swarm.json names; Problem says why there are none (no state
// on this machine, no base), the counts then nil.
type swarmFigures struct {
	Swarm         string   `json:"swarm"`
	State         *string  `json:"state"`
	Base          *string  `json:"base"`
	MergedPulls   *int     `json:"merged_pulls"`
	RevertedPulls *int     `json:"reverted_pulls"`
	RevertRate    *float64 `json:"revert_rate"`
	Fetched       *string  `json:"fetched"`
	Problem       *string  `json:"problem"`
}

// metricsGroup is the rows of one kind and model (nil, the text's
// (none), when left empty): each sum over the rows that have the value,
// nil when none does, and how many do; the lines over the measured rows,
// nil when none is.
type metricsGroup struct {
	Kind              string        `json:"kind"`
	Model             *string       `json:"model"`
	Rows              int           `json:"rows"`
	Tokens            *int          `json:"tokens"`
	RowsWithTokens    int           `json:"rows_with_tokens"`
	Cached            *int          `json:"tokens_cached"`
	RowsWithCached    int           `json:"rows_with_tokens_cached"`
	Cost              *float64      `json:"cost"`
	RowsWithCost      int           `json:"rows_with_cost"`
	Estimated         *float64      `json:"estimated"`
	RowsWithEstimated int           `json:"rows_with_estimated"`
	Measured          int           `json:"measured"`
	Code              *metricsLines `json:"code"`
	Tests             *metricsLines `json:"tests"`
	Prose             *metricsLines `json:"prose"`
}

type metricsLines struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

func (l *metricsLines) String() string { return fmt.Sprintf("+%d/-%d", l.Added, l.Removed) }

// Metrics prints the summary of the home's metrics CSV: its rows grouped
// by kind and model, a row a later one for the same step or run
// supersedes left out, then a plan's tokens per code line it added and
// its reverted steps, per plan, and a swarm's reverted pull requests,
// per swarm; with --swarm, that swarm's runs alone, and with --since,
// the rows written at or after that time.
func Metrics(c *Context) error {
	return verb.PrintReport(c, func() (*metricsReport, error) { return readMetrics(c) }, func(r *metricsReport) {
		if !r.Exists {
			fmt.Fprintf(c.Stdout, "No metrics recorded yet (%s does not exist)\n  Record a plan step: daisy metrics add --step <n> --range <a>..<b>\n", r.File)
			return
		}
		fmt.Fprintf(c.Stdout, "Metrics: %s\n", r.File)
		var given []string
		if v := r.RowsTaken.Swarm; v != nil {
			given = append(given, "--swarm "+*v)
		}
		if v := r.RowsTaken.Since; v != nil {
			given = append(given, "--since "+*v)
		}
		if len(given) > 0 {
			fmt.Fprintf(c.Stdout, "  Rows taken: %s\n", strings.Join(given, " "))
		}
		fmt.Fprintf(c.Stdout, "  %d rows, %d superseded by a later record of the same step or run\n", r.Rows, r.Superseded)
		if len(r.Groups) == 0 {
			return
		}
		fmt.Fprintln(c.Stdout)
		w := tabwriter.NewWriter(c.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  kind\tmodel\trows\ttokens\tcached\tcost\testimated\tmeasured\tcode\ttests\tprose")
		for _, g := range r.Groups {
			model := "(none)"
			if g.Model != nil {
				model = *g.Model
			}
			code, tests, prose := "—", "—", "—"
			if g.Code != nil {
				code, tests, prose = g.Code.String(), g.Tests.String(), g.Prose.String()
			}
			fmt.Fprintf(w, "  %s\t%s\t%d\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n", g.Kind, model, g.Rows,
				some(g.Tokens, "%d", g.RowsWithTokens, g.Rows), some(g.Cached, "%d", g.RowsWithCached, g.Rows),
				some(g.Cost, "$%.2f", g.RowsWithCost, g.Rows), some(g.Estimated, "$%.2f", g.RowsWithEstimated, g.Rows), g.Measured, code, tests, prose)
		}
		w.Flush()
		if len(r.Plans) > 0 {
			fmt.Fprintln(c.Stdout)
			w = tabwriter.NewWriter(c.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  plan\trows\ttokens\tcode added\ttokens per code line\treverted")
			for _, p := range r.Plans {
				fmt.Fprintf(w, "  %s\t%d\t%s\t%s\t%s\t%s\n", p.Plan, p.Rows, some(p.Tokens, "%d", p.RowsWithTokens, p.Rows),
					some(p.CodeAdded, "+%d", p.Measured, p.Rows), some(p.TokensPerCodeLine, "%d", 1, 1), stepsReverted(p))
			}
			w.Flush()
			where := "no repository (neither the working directory nor $DAISY_ROOT is in one)"
			if len(r.Repositories) > 0 {
				where = strings.Join(r.Repositories, ", then ")
			}
			fmt.Fprintf(c.Stdout, "\n  A step is reverted when a commit on HEAD says \"This reverts commit <sha>\" of a commit in its range,\n  each range looked up in %s.\n", where)
		}
		if len(r.Swarms) > 0 {
			fmt.Fprintln(c.Stdout)
			w = tabwriter.NewWriter(c.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  swarm\tmerged pulls\treverted\tas of")
			for _, sw := range r.Swarms {
				if sw.Problem != nil {
					fmt.Fprintf(w, "  %s\t—\t—\t%s\n", sw.Swarm, *sw.Problem)
					continue
				}
				asOf := "no mirror fetched"
				if sw.Fetched != nil {
					asOf = "the mirrors' last fetch, " + *sw.Fetched
				}
				fmt.Fprintf(w, "  %s\t%d\t%s\t%s\n", sw.Swarm, *sw.MergedPulls, ofRate(*sw.RevertedPulls, *sw.MergedPulls), asOf)
			}
			w.Flush()
			fmt.Fprintln(c.Stdout, "\n  A swarm's pull request is merged by a merge commit on its base whose second parent a worker run pushed,\n  and reverted when a commit on base says \"This reverts commit <sha>\" of it; no fetch is made.")
		}
		fmt.Fprintln(c.Stdout, "\n  A sum counts only the rows that have the value: (k/n) when k of the n do.")
	})
}

// readMetrics reads the summary of the home's metrics CSV.
func readMetrics(c *Context) (*metricsReport, error) {
	var f metrics.Filter
	r := &metricsReport{File: env.MetricsFile(c.Env.Home), Groups: []metricsGroup{}, Plans: []planFigures{}, Repositories: []string{}, Swarms: []swarmFigures{}}
	if v, ok := c.Value("--swarm"); ok {
		f.Swarm, r.RowsTaken.Swarm = v, &v
	}
	if v, ok := c.Value("--since"); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, fmt.Errorf("Error: --since must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: %s)", v)
		}
		f.Since, r.RowsTaken.Since = t, &v
	}
	b, err := c.Ops.FS().ReadFile(r.File)
	if errors.Is(err, fs.ErrNotExist) && !lexists(c.Ops.FS(), r.File) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Error: cannot read %s: %v", r.File, err)
	}
	s, err := metrics.Summarize(b, f)
	if err != nil {
		return nil, fmt.Errorf("Error: cannot read %s: %v", r.File, err)
	}
	r.Exists, r.Rows, r.Superseded = true, s.Rows, s.Superseded
	for _, g := range s.Groups {
		mg := metricsGroup{Kind: string(g.Kind), Rows: g.Rows, RowsWithTokens: g.WithTokens, RowsWithCost: g.WithCost,
			RowsWithEstimated: g.WithEstimate, Measured: g.Measured}
		if g.Model != "" {
			mg.Model = &g.Model
		}
		if g.WithTokens > 0 {
			mg.Tokens = &g.Tokens
		}
		if g.WithCached > 0 {
			mg.Cached, mg.RowsWithCached = &g.Cached, g.WithCached
		}
		if g.WithCost > 0 {
			mg.Cost = &g.Cost
		}
		if g.WithEstimate > 0 {
			mg.Estimated = &g.Estimate
		}
		if g.Measured > 0 {
			l := g.Lines
			mg.Code = &metricsLines{l.CodeAdded, l.CodeRemoved}
			mg.Tests = &metricsLines{l.TestsAdded, l.TestsRemoved}
			mg.Prose = &metricsLines{l.ProseAdded, l.ProseRemoved}
		}
		r.Groups = append(r.Groups, mg)
	}
	for _, p := range s.Plans {
		mp := planFigures{Plan: p.Name, Rows: p.Rows, RowsWithTokens: p.WithTokens, Measured: p.Measured}
		if p.WithTokens > 0 {
			mp.Tokens = &p.Tokens
		}
		if p.Measured > 0 {
			mp.CodeAdded = &p.CodeAdded
		}
		if n, ok := p.TokensPerCodeLine(); ok {
			mp.TokensPerCodeLine = &n
		}
		r.Plans = append(r.Plans, mp)
	}
	if len(s.Plans) > 0 {
		if err := planReverts(c, r, s.Plans); err != nil {
			return nil, err
		}
	}
	for _, name := range s.Swarms {
		figures, err := swarmReverts(c, name, f.Since)
		if err != nil {
			return nil, err
		}
		r.Swarms = append(r.Swarms, figures...)
	}
	return r, nil
}

// planReverts fills each plan's reverted steps: each of its ranges looked
// up in the working directory's repository, then $DAISY_ROOT's
// (metrics.StepReverts), the repositories named in r.
func planReverts(c *Context, r *metricsReport, plans []metrics.Plan) error {
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	for _, dir := range []string{cwd, c.Env.Root} {
		out, _, err := c.Ops.Exec(dir, "git", "rev-parse", "--show-toplevel")
		if top := strings.TrimSpace(string(out)); err == nil && top != "" && !slices.Contains(r.Repositories, top) {
			r.Repositories = append(r.Repositories, top)
		}
	}
	check := metrics.StepReverts{O: c.Ops, Dirs: r.Repositories}
	for i, p := range plans {
		mp := &r.Plans[i]
		mp.Steps = len(p.Ranges)
		for _, rng := range p.Ranges {
			dir, reverted, err := check.Check(rng)
			if err != nil {
				return fmt.Errorf("Error: cannot read the reverts in %s: %v", dir, err)
			}
			if dir != "" {
				mp.StepsFound++
			}
			if reverted {
				mp.StepsReverted++
			}
		}
		if mp.StepsFound > 0 {
			rate := float64(mp.StepsReverted) / float64(mp.StepsFound)
			mp.RevertRate = &rate
		}
	}
	return nil
}

// swarmReverts is the swarm's figures, one per <state> of it on this
// machine, merges made since since (all of them when zero); one with its
// Problem when there is none, or its mirrored swarm.json names no base.
func swarmReverts(c *Context, name string, since time.Time) ([]swarmFigures, error) {
	states, err := reconcile.States(c.Ops.FS(), c.Env.Local, name)
	if err != nil {
		return nil, fmt.Errorf("Error: cannot read %s: %v", filepath.Join(c.Env.Local, "swarm"), err)
	}
	if len(states) == 0 {
		problem := "no state on this machine"
		return []swarmFigures{{Swarm: name, Problem: &problem}}, nil
	}
	var out []swarmFigures
	for _, state := range states {
		sf := swarmFigures{Swarm: name, State: &state}
		base, err := reconcile.MirroredBase(c.Ops, state, name)
		if err != nil {
			problem := err.Error()
			sf.Problem = &problem
			out = append(out, sf)
			continue
		}
		sf.Base = &base
		pr, err := reconcile.ReadPullReverts(c.Ops, state, base, since)
		if err != nil {
			return nil, fmt.Errorf("Error: cannot read the reverts in %s's mirrors: %v", state, err)
		}
		sf.MergedPulls, sf.RevertedPulls = &pr.Merged, &pr.Reverted
		if pr.Merged > 0 {
			rate := float64(pr.Reverted) / float64(pr.Merged)
			sf.RevertRate = &rate
		}
		if !pr.Fetched.IsZero() {
			at := pr.Fetched.Format(time.RFC3339)
			sf.Fetched = &at
		}
		out = append(out, sf)
	}
	return out, nil
}

// stepsReverted is a plan's reverted cell: `<k> of <n> (<p>%)` over the
// steps found, then how many were not; `—` for a plan with no range.
func stepsReverted(p planFigures) string {
	missing := p.Steps - p.StepsFound
	switch {
	case p.Steps == 0:
		return "—"
	case p.StepsFound == 0:
		return fmt.Sprintf("%d not found", missing)
	case missing > 0:
		return ofRate(p.StepsReverted, p.StepsFound) + fmt.Sprintf(", %d not found", missing)
	}
	return ofRate(p.StepsReverted, p.StepsFound)
}

// ofRate is `<k> of <n>`, with the rate as a whole percentage when n is
// not 0.
func ofRate(k, n int) string {
	if n == 0 {
		return fmt.Sprintf("%d of %d", k, n)
	}
	return fmt.Sprintf("%d of %d (%.0f%%)", k, n, 100*float64(k)/float64(n))
}

// some is a sum over the k of n rows that have the value, in format: "—"
// for none, the sum and (k/n) for some.
func some[T any](sum *T, format string, k, n int) string {
	switch {
	case sum == nil:
		return "—"
	case k < n:
		return fmt.Sprintf(format+" (%d/%d)", *sum, k, n)
	}
	return fmt.Sprintf(format, *sum)
}

// metricsCells are the flags whose values are cells CheckCell rules on.
var metricsCells = []struct{ flag, column string }{
	{"--review-rounds", "review_rounds"}, {"--tokens-in", "tokens_in"}, {"--tokens-out", "tokens_out"},
	{"--tokens", "tokens_total"}, {"--tokens-cached", "tokens_cached"}, {"--cost", "cost_reported"}, {"--cost-estimated", "cost_estimated"},
	{"--root-sha", "root_sha"}, {"--home-sha", "home_sha"},
}

// MetricsAdd records a plan step: one row appended to the home's
// metrics/metrics.csv and the step's row in the plan's ## Metrics table,
// in one call. The lines per class are measured from the step's commit
// range by git, never taken from the caller. Every refusal is decided
// before the first write; a failed write takes back the one before it.
// Nothing is committed: the next home commit sweeps both files up.
func MetricsAdd(c *Context) error {
	fsys := c.Ops.FS()
	val := func(flag string) string { v, _ := c.Value(flag); return v }
	step, rng, repo := val("--step"), val("--range"), val("--repo")
	switch {
	case step == "":
		return UsageError("--step is required")
	case repo != "" && rng == "":
		return UsageError("--repo without --range: the repository is where the range is measured")
	}
	for _, f := range metricsCells {
		if why := metrics.CheckCell(f.column, val(f.flag)); why != "" {
			return fmt.Errorf("Error: %s %s (got: %s)", f.flag, why, val(f.flag))
		}
	}
	when := c.Ops.Now()
	if v, ok := c.Value("--when"); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return fmt.Errorf("Error: --when must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: %s)", v)
		}
		when = t
	}
	planFile, err := metricsPlan(c, val("--plan"))
	if err != nil {
		return err
	}
	row := metrics.Row{
		When: when.Format(time.RFC3339), Kind: metrics.PlanStep, Source: planName(planFile), Unit: step,
		Role: "step", Provider: val("--provider"), Model: val("--model"), Effort: val("--effort"),
		RootSHA: val("--root-sha"), HomeSHA: val("--home-sha"), PromptHash: val("--prompt-hash"), Outcome: val("--outcome"),
		ReviewRounds: val("--review-rounds"), TokensIn: val("--tokens-in"), TokensOut: val("--tokens-out"),
		TokensTotal: val("--tokens"), TokensCached: val("--tokens-cached"), CostReported: val("--cost"), CostEstimated: val("--cost-estimated"),
		Range: rng, Notes: val("--notes"),
	}
	if v, ok := c.Value("--role"); ok {
		row.Role = v
	}
	if rng != "" {
		if err := metrics.CheckRange(rng); err != nil {
			return fmt.Errorf("Error: --range: %v", err)
		}
		dir, err := absFromCwd(c, repo)
		if err != nil {
			return err
		}
		lines, err := metrics.Measure(c.Ops, dir, rng)
		if err != nil {
			return fmt.Errorf("Error: %v\nNothing was recorded.", err)
		}
		row.Lines = &lines
	}
	if row.RootSHA == "" {
		row.RootSHA = head(c, c.Env.Root, "root_sha", "--root-sha")
	}
	if row.HomeSHA == "" {
		row.HomeSHA = head(c, c.Env.Home, "home_sha", "--home-sha")
	}

	csvFile := env.MetricsFile(c.Env.Home)
	// The lock from the read to the last write, so that a row another
	// writer (a swarm's apply) appends meanwhile is not written over.
	release, err := metrics.Lock(c.Ops, env.MetricsLock(c.Env.Local), "metrics add since "+c.Ops.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("Error: cannot take %s: %v\nNothing was recorded.", env.MetricsLock(c.Env.Local), err)
	}
	defer release()
	before, err := fsys.ReadFile(csvFile)
	if err != nil && (!errors.Is(err, fs.ErrNotExist) || lexists(fsys, csvFile)) {
		return fmt.Errorf("Error: cannot read %s: %v", csvFile, err)
	}
	csvOut, err := metrics.AppendCSV(before, row)
	if err != nil {
		return fmt.Errorf("Error: cannot record in %s: %v\nNothing was recorded.", csvFile, err)
	}
	doc, err := fsys.ReadFile(planFile)
	if err != nil {
		return fmt.Errorf("Error: cannot read %s: %v", planFile, err)
	}
	planOut, err := metrics.PlanRow(doc, row)
	if err != nil {
		return fmt.Errorf("Error: cannot record in %s: %v\nNothing was recorded.", planFile, err)
	}

	if err := fsys.MkdirAll(filepath.Dir(csvFile), 0o755); err != nil {
		return fmt.Errorf("Error: cannot create %s: %v", filepath.Dir(csvFile), err)
	}
	if err := ops.Rewrite(fsys, csvFile, csvOut, 0o644); err != nil {
		return fmt.Errorf("Error: cannot write %s: %v", csvFile, err)
	}
	undo := undoStack{func() error {
		if before == nil {
			return fsys.Remove(csvFile)
		}
		return ops.Rewrite(fsys, csvFile, before, 0o644)
	}}
	if err := fsys.WriteFile(planFile, planOut, 0o644); err != nil {
		return undo.fail(fmt.Errorf("Error: cannot write %s: %v", planFile, err), "recorded")
	}

	measured := " (no --range: no lines measured)"
	if l := row.Lines; l != nil {
		measured = fmt.Sprintf(": code %s, tests %s, prose %s", l.Code(), l.Tests(), l.Prose())
		if l.Binary > 0 {
			measured += fmt.Sprintf(" (%d binary file(s), counted as no lines)", l.Binary)
		}
	}
	fmt.Fprintf(c.Stdout, "  ✓ Recorded step %s of %s%s\n  CSV:  %s\n  Plan: %s\n", step, row.Source, measured, csvFile, planFile)
	return nil
}

// metricsPlan is the real path of the plan --plan names: a file at that
// path (the working directory's when relative); else the one plan in the
// home's plans/ or plans/archive/ named in full, without `.plan.md`, by
// its timestamp or by its slug. Without --plan, the plan PLAN.md links to.
func metricsPlan(c *Context, name string) (string, error) {
	fsys := c.Ops.FS()
	if name == "" {
		real, err := fsys.EvalSymlinks(planLink(c))
		if err != nil {
			return "", fmt.Errorf("Error: no PLAN.md in this workspace (or its link is broken)\n  Name the plan: daisy metrics add --plan <path or name> --step <n>")
		}
		return real, nil
	}
	if p, err := absFromCwd(c, name); err == nil {
		if fi, err := fsys.Stat(p); err == nil && fi.Mode().IsRegular() {
			return fsys.EvalSymlinks(p)
		}
	}
	var found []string
	for _, dir := range []string{plansDir(c), filepath.Join(plansDir(c), "archive")} {
		entries, _ := fsys.ReadDir(dir) // no directory, no plans
		for _, en := range entries {
			base := en.Name()
			if !strings.HasSuffix(base, ".plan.md") || !en.Type().IsRegular() {
				continue
			}
			if base == name || base == name+".plan.md" || strings.HasPrefix(base, name+".") || strings.HasSuffix(base, "."+name+".plan.md") {
				found = append(found, filepath.Join(dir, base))
			}
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("Error: --plan %s names no plan file, and no plan in %s or its archive/", name, plansDir(c))
	case 1:
		return fsys.EvalSymlinks(found[0])
	}
	return "", fmt.Errorf("Error: --plan %s names more than one plan:\n  %s", name, strings.Join(found, "\n  "))
}

// planName is a plan file's name without `.plan.md` (or `.md`).
func planName(path string) string {
	base := filepath.Base(path)
	if s, ok := strings.CutSuffix(base, ".plan.md"); ok {
		return s
	}
	return strings.TrimSuffix(base, ".md")
}

// absFromCwd is p, the working directory's when relative; "" is the
// working directory.
func absFromCwd(c *Context, p string) (string, error) {
	if filepath.IsAbs(p) {
		return p, nil
	}
	cwd, err := c.Ops.Getwd()
	if err != nil {
		return "", fmt.Errorf("Error: cannot read the working directory: %v", err)
	}
	return filepath.Join(cwd, p), nil
}

// head is the commit dir is at: $DAISY_ROOT's, the root the step ran
// under, and the home's, so the row names every prompt and skill in use;
// "" with a warning when git cannot say (a root that is no clone, a home
// with no commit).
func head(c *Context, dir, column, flag string) string {
	out, stderr, err := c.Ops.Exec(dir, "git", "rev-parse", "--verify", "HEAD")
	if err != nil {
		fmt.Fprintf(c.Stderr, "⚠️  %s left empty: git rev-parse HEAD in %s failed: %s\n  Pass %s <sha> to record it.\n", column, dir, strings.TrimSpace(string(stderr)), flag)
		return ""
	}
	return strings.TrimSpace(string(out))
}
