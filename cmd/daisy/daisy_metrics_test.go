package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
)

// metricsOps pins the clock to a fixed zone, so the `when` cell is a
// literal.
func (m *machine) metricsOps(cwd string) *ops.Fake {
	return &ops.Fake{
		GetenvFn: func(k string) string { return m.vars[k] },
		GetwdFn:  func() (string, error) { return cwd, nil },
		NowFn:    func() time.Time { return time.Date(2026, 11, 12, 14, 5, 0, 0, time.FixedZone("", 3600)) },
	}
}

// commitAll stages everything in dir and commits it; the new HEAD.
func commitAll(t *testing.T, dir, message string) string {
	t.Helper()
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "-c", "user.name=S", "-c", "user.email=s@x", "commit", "-q", "-m", message)
	return strings.TrimSpace(gitIn(t, dir, "rev-parse", "HEAD"))
}

// stepRepo is a repository whose second commit, the step, changes a
// .go file (3 added, 1 removed), adds a _test.go file (5), a file under
// tests/ (1) and a .md file (3), and moves a file under docs/ (4
// changed: 1 added, 1 removed); base and head are the step's range.
func stepRepo(t *testing.T) (dir, base, head string) {
	t.Helper()
	dir = t.TempDir()
	gitIn(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "docs", "guide.txt"), "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\n")
	base = commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {\n\tprintln(1)\n}\n")
	writeFile(t, filepath.Join(dir, "main_test.go"), "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "tests", "case.sh"), "true\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# Step\n\nWhat it does.\n")
	os.Remove(filepath.Join(dir, "docs", "guide.txt"))
	writeFile(t, filepath.Join(dir, "docs", "manual.txt"), "one\ntwo\nthree\nfour\nfive\nsix\nseven\nEIGHT\n")
	head = commitAll(t, dir, "the step")
	return dir, base, head
}

// The step's claim: one `metrics add` writes the home's CSV row and the
// plan's table row, each with the per-class counts of the range's diff
// (code main.go +3/-1; tests main_test.go and tests/case.sh +6/-0; prose
// README.md +3/-0 and the renamed docs/ file +1/-1).
func TestMetricsAddRecordsAStep(t *testing.T) {
	m := newMachine(t)
	gitIn(t, m.root, "init", "-q")
	writeFile(t, filepath.Join(m.root, "VERSION"), "1\n")
	rootSHA := commitAll(t, m.root, "root")
	m.initHomeRepo(t)
	homeSHA := strings.TrimSpace(m.git(t, "rev-parse", "HEAD"))
	o := m.metricsOps(m.ws)
	if code, _, errs := runWith(o, "plan", "new", "Measure things"); code != 0 {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	planName := "261112140500.Measure-things"
	planFile := filepath.Join(m.home, "plans", planName+".plan.md")
	before := m.read(t, filepath.Join("plans", planName+".plan.md"))
	repo, base, head := stepRepo(t)
	rng := base[:7] + ".." + head[:7]

	code, out, errs := runWith(o, "metrics", "add", "--step", "3", "--range", rng, "--repo", repo,
		"--provider", "claude", "--model", "claude-opus-5-5", "--effort", "high", "--outcome", "committed",
		"--review-rounds", "2", "--tokens-in", "1200", "--tokens-out", "340", "--cost", "1.25",
		"--notes", "worker model changed, \"again\"")
	if code != 0 || errs != "" {
		t.Fatalf("metrics add = %d: %s", code, errs)
	}

	csvFile := filepath.Join(m.home, "metrics", "metrics.csv")
	wantCSV := "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds,tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed,binary_files,range,notes\n" +
		"2026-11-12T14:05:00+01:00,plan-step," + planName + ",3,step,claude,claude-opus-5-5,high," + rootSHA + "," + homeSHA + ",,committed,2,1200,340,,1.25,,3,1,6,0,4,1,0," + rng + ",\"worker model changed, \"\"again\"\"\"\n"
	if b, err := os.ReadFile(csvFile); err != nil || string(b) != wantCSV {
		t.Fatalf("metrics.csv (%v):\n%s\nwant\n%s", err, b, wantCSV)
	}

	wantTable := "## Metrics\n\n" +
		"| Step | Role | Model | Effort | Root | Home | Outcome | Reviews | Tokens | Cost | Code | Tests | Prose | Range | Notes |\n" +
		"|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n" +
		"| 3 | step | claude-opus-5-5 | high | " + rootSHA[:7] + " | " + homeSHA[:7] + " | committed | 2 | 1200 in, 340 out | $1.25 | +3/-1 | +6/-0 | +4/-1 | " + rng + " | worker model changed, \"again\" |\n\n"
	got := m.read(t, filepath.Join("plans", planName+".plan.md"))
	if !strings.Contains(before, "## Decisions\n\n## Metrics\n\n## Deferred\n") {
		t.Fatalf("the template has no empty ## Metrics after ## Decisions:\n%s", before)
	}
	want := strings.Replace(before, "## Metrics\n\n", wantTable, 1)
	if got != want {
		t.Fatalf("plan:\n%s\nwant\n%s", got, want)
	}
	if fi, err := os.Lstat(filepath.Join(m.ws, "PLAN.md")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("PLAN.md is no longer the link: %v %v", fi, err)
	}
	wantOut := "  ✓ Recorded step 3 of " + planName + ": code +3/-1, tests +6/-0, prose +4/-1\n" +
		"  CSV:  " + csvFile + "\n  Plan: " + planFile + "\n"
	if out != wantOut {
		t.Fatalf("stdout:\n%q\nwant\n%q", out, wantOut)
	}

	// A second record of the step appends to the CSV, under the one
	// header, and replaces the step's row in the plan's table.
	if code, _, errs := runWith(o, "metrics", "add", "--plan", planName, "--step", "3", "--root-sha", "abcdef1", "--tokens", "9"); code != 0 {
		t.Fatalf("second metrics add = %d: %s", code, errs)
	}
	b, _ := os.ReadFile(csvFile)
	if lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"); len(lines) != 3 || !strings.HasPrefix(string(b), wantCSV) ||
		lines[2] != "2026-11-12T14:05:00+01:00,plan-step,"+planName+",3,step,,,,abcdef1,"+homeSHA+",,,,,,9,,,,,,,,,,," {
		t.Fatalf("metrics.csv after the second record:\n%s", b)
	}
	got = m.read(t, filepath.Join("plans", planName+".plan.md"))
	if strings.Count(got, "| 3 |") != 1 || !strings.Contains(got, "| 3 | step |  |  | abcdef1 | "+homeSHA[:7]+" |  |  | 9 |  |  |  |  |  |  |\n") {
		t.Fatalf("plan after the second record:\n%s", got)
	}

	// The summary reads the central file: the later row supersedes the
	// earlier one for the same step.
	code, out, errs = runWith(o, "metrics")
	if code != 0 || errs != "" || !strings.Contains(out, "Metrics: "+csvFile+"\n") || !strings.Contains(out, "1 superseded") {
		t.Fatalf("metrics = %d:\n%s\n%s", code, out, errs)
	}
}

// Every refusal is decided before the first write.
func TestMetricsAddRefusals(t *testing.T) {
	m := newMachine(t)
	o := m.metricsOps(m.ws)
	repo, base, head := stepRepo(t)
	if code, _, errs := runWith(o, "metrics", "add", "--step", "1", "--root-sha", "abcdef1"); code != 1 || !strings.Contains(errs, "no PLAN.md") {
		t.Fatalf("no plan = %d: %s", code, errs)
	}
	if code, _, errs := runWith(o, "plan", "new", "Measure things"); code != 0 {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	before := snapshot(t, m.home, m.ws)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--root-sha", "abcdef1"}, "--step is required"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--tokens", "12k"}, "--tokens must be a whole number"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--cost", "-1"}, "--cost must be a number"},
		{[]string{"--step", "1", "--root-sha", "xyz"}, "--root-sha must be"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--home-sha", "HEAD"}, "--home-sha must be"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--when", "yesterday"}, "--when must be"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--range", head, "--repo", repo}, "a range is <a>..<b> or <commit>^!"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--range", "--output=x..y", "--repo", repo}, "a range is <a>..<b> or <commit>^!"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--range", base + "..nosuch", "--repo", repo}, "git diff"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--plan", "nosuch"}, "names no plan"},
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--repo", repo}, "--repo without --range"},
	} {
		code, _, errs := runWith(o, append([]string{"metrics", "add"}, c.args...)...)
		if code == 0 || !strings.Contains(errs, c.want) {
			t.Errorf("metrics add %v = %d: %s", c.args, code, errs)
		}
	}
	sameFiles(t, "refused metrics add", before, snapshot(t, m.home, m.ws))
}
