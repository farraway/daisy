package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"daisy/internal/env"
	"daisy/internal/metrics"
	"daisy/internal/ops"
)

// Metrics prints the summary of the home's metrics CSV: its rows grouped
// by kind and model, a row a later one for the same step or run
// supersedes left out; with --swarm, that swarm's runs alone, and with
// --since, the rows written at or after that time.
var Metrics = readOnly(metricsSummary)

// metricsReport is the summary: the CSV, whether it exists, the flags
// that took its rows, the rows taken and how many of them a later record
// supersedes, and a group per kind and model.
type metricsReport struct {
	File      string `json:"file"`
	Exists    bool   `json:"exists"`
	RowsTaken struct {
		Swarm *string `json:"swarm"`
		Since *string `json:"since"`
	} `json:"rows_taken"`
	Rows       int            `json:"rows"`
	Superseded int            `json:"superseded"`
	Groups     []metricsGroup `json:"groups"`
	warned
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

func metricsSummary(c *Context) error {
	return printReport(c, func() (*metricsReport, error) { return readMetrics(c) }, func(r *metricsReport) {
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
		fmt.Fprintln(w, "  kind\tmodel\trows\ttokens\tcost\testimated\tmeasured\tcode\ttests\tprose")
		for _, g := range r.Groups {
			model := "(none)"
			if g.Model != nil {
				model = *g.Model
			}
			code, tests, prose := "—", "—", "—"
			if g.Code != nil {
				code, tests, prose = g.Code.String(), g.Tests.String(), g.Prose.String()
			}
			fmt.Fprintf(w, "  %s\t%s\t%d\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n", g.Kind, model, g.Rows,
				some(g.Tokens, "%d", g.RowsWithTokens, g.Rows), some(g.Cost, "$%.2f", g.RowsWithCost, g.Rows),
				some(g.Estimated, "$%.2f", g.RowsWithEstimated, g.Rows), g.Measured, code, tests, prose)
		}
		w.Flush()
		fmt.Fprintln(c.Stdout, "\n  A sum counts only the rows that have the value: (k/n) when k of the n do.")
	})
}

// readMetrics reads the summary of the home's metrics CSV.
func readMetrics(c *Context) (*metricsReport, error) {
	var f metrics.Filter
	r := &metricsReport{File: env.MetricsFile(c.Env.Home), Groups: []metricsGroup{}}
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
	return r, nil
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
	{"--tokens", "tokens_total"}, {"--cost", "cost_reported"}, {"--cost-estimated", "cost_estimated"},
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
		TokensTotal: val("--tokens"), CostReported: val("--cost"), CostEstimated: val("--cost-estimated"),
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
		real, err := fsys.EvalSymlinks(c.planLink())
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
	for _, dir := range []string{c.plansDir(), filepath.Join(c.plansDir(), "archive")} {
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
		return "", fmt.Errorf("Error: --plan %s names no plan file, and no plan in %s or its archive/", name, c.plansDir())
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
