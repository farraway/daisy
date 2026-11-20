package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daisy/internal/metrics"
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
	wantCSV := "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds,tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed,binary_files,range,notes,tokens_cached,review_effect,cause\n" +
		"2026-11-12T14:05:00+01:00,plan-step," + planName + ",3,step,claude,claude-opus-5-5,high," + rootSHA + "," + homeSHA + ",,committed,2,1200,340,,1.25,,3,1,6,0,4,1,0," + rng + ",\"worker model changed, \"\"again\"\"\",,,\n"
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
		lines[2] != "2026-11-12T14:05:00+01:00,plan-step,"+planName+",3,step,,,,abcdef1,"+homeSHA+",,,,,,9,,,,,,,,,,,,,," {
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
		{[]string{"--step", "1", "--root-sha", "abcdef1", "--role", "review", "--review-effect", "two"}, "--review-effect must be a whole number"},
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

// Step 28's claim (the plan remediating the 2026-11-15 design review,
// P22): the orchestrator's row of a step, `--role orchestrator` with no
// tokens (it often cannot count its own), sits beside the step's and the
// review's rows, in the CSV and in the plan's table; `daisy metrics`
// prints each plan's tokens per code line it added, over every role's
// rows: 1500 + 300 tokens over main.go's 3 code lines, 600.
func TestMetricsTokensPerCodeLine(t *testing.T) {
	m := newMachine(t)
	o := m.metricsOps(m.ws)
	if code, _, errs := runWith(o, "plan", "new", "Measure things"); code != 0 {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	planName := "261112140500.Measure-things"
	repo, base, head := stepRepo(t)
	for _, args := range [][]string{
		{"--step", "1", "--range", base + ".." + head, "--repo", repo, "--tokens", "1500", "--tokens-cached", "1000"},
		{"--step", "1", "--role", "review", "--tokens-in", "200", "--tokens-out", "100"},
		{"--step", "1", "--role", "orchestrator"},
	} {
		if code, _, errs := runWith(o, append([]string{"metrics", "add", "--root-sha", "abcdef1", "--home-sha", "abcdef2"}, args...)...); code != 0 {
			t.Fatalf("metrics add %v = %d: %s", args, code, errs)
		}
	}
	plan := m.read(t, filepath.Join("plans", planName+".plan.md"))
	for _, row := range []string{"| 1 | step |", "| 1 | review |", "| 1 | orchestrator |  |  | abcdef1 | abcdef2 |  |  |  |  |  |  |  |  |  |\n"} {
		if !strings.Contains(plan, row) {
			t.Errorf("plan has no %q:\n%s", row, plan)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(m.home, "metrics", "metrics.csv")); !strings.Contains(string(b), ",1500,,,3,1,6,0,4,1,0,"+base+".."+head+",,1000,,\n") {
		t.Errorf("the step's row has no tokens_cached:\n%s", b)
	}
	code, out, errs := runWith(o, "metrics")
	want := "\n  plan                         rows  tokens      code added  tokens per code line  reverted\n" +
		"  " + planName + "  3     1800 (2/3)  +3 (1/3)    600                   1 not found\n"
	if code != 0 || errs != "" || !strings.Contains(out, want) || !strings.Contains(out, "  plan-step  (none)  3     1800 (2/3)  1000 (1/3)") {
		t.Fatalf("metrics = %d:\n%s\nwant within:\n%s\n%s", code, out, want, errs)
	}
}

// Step 30's claim, the plan's side (the plan remediating the 2026-11-15
// design review, P29): `metrics add --role review --review-effect <n>`
// records a step's review passes' findings fixed in review_effect, the
// CSV's 29th column; `daisy metrics` sums the review rows by role and
// model, the effect over the rows that record one (a row recorded
// without it counted apart), and the tokens per effect over the rows
// with both: 1200 + 800 tokens over 3 + 1 findings, 500.
func TestMetricsReviewEffect(t *testing.T) {
	m := newMachine(t)
	o := m.metricsOps(m.ws)
	if code, _, errs := runWith(o, "plan", "new", "Measure things"); code != 0 {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	for _, args := range [][]string{
		{"--step", "1", "--role", "review", "--model", "opus", "--tokens", "1200", "--review-effect", "3"},
		{"--step", "2", "--role", "review", "--model", "opus", "--tokens", "800", "--review-effect", "1"},
		{"--step", "3", "--role", "review", "--model", "opus", "--tokens", "500"},
		{"--step", "3", "--model", "opus", "--tokens", "9000"},
	} {
		if code, _, errs := runWith(o, append([]string{"metrics", "add", "--root-sha", "abcdef1", "--home-sha", "abcdef2"}, args...)...); code != 0 {
			t.Fatalf("metrics add %v = %d: %s", args, code, errs)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(m.home, "metrics", "metrics.csv")); !strings.HasSuffix(strings.SplitN(string(b), "\n", 2)[0], ",tokens_cached,review_effect,cause") ||
		!strings.Contains(string(b), ",1,review,,opus,,abcdef1,abcdef2,,,,,,1200,,,,,,,,,,,,,3,\n") {
		t.Errorf("csv:\n%s", b)
	}
	code, out, errs := runWith(o, "metrics")
	want := "\n  review role  model  rows  effect   tokens  tokens per effect\n" +
		"  review       opus   3     4 (2/3)  2500    500\n" +
		"\n  A review's effect is the in-gate findings it raised that were fixed: a plan review's as its row records it,\n" +
		"  a swarm review's its blocking findings when its role's next review of the pull request approves a new head.\n"
	if code != 0 || errs != "" || !strings.Contains(out, want) {
		t.Fatalf("metrics = %d:\n%s\nwant within:\n%s\n%s", code, out, want, errs)
	}
	code, out, _ = runWith(o, "metrics", "--json")
	var doc struct {
		Reviews []map[string]any `json:"reviews"`
	}
	if err := json.Unmarshal([]byte(out), &doc); code != 0 || err != nil || len(doc.Reviews) != 1 {
		t.Fatalf("metrics --json = %d, %v:\n%s", code, err, out)
	}
	if got := fmt.Sprint(doc.Reviews[0]); got != "map[effect:4 model:opus role:review rows:3 rows_with_effect:2 rows_with_tokens:3 tokens:2500 tokens_per_effect:500]" {
		t.Errorf("reviews[0] = %s", got)
	}
}

// Step 29's claim (the plan remediating the 2026-11-15 design review,
// talk A13): a plan whose steps' ranges are in the root, one of the
// three reverted by `git revert`, reads 1 of 3, a third reverted and
// reapplied (its revert reverted, Step 7 of the plan implementing the
// 2026-11-19 swarm roadmap) not counted; a fourth step, measured
// in a repository neither the working directory nor $DAISY_ROOT is in,
// is not found; a range two roles record counts once.
func TestMetricsStepReverts(t *testing.T) {
	m := newMachine(t)
	o := m.metricsOps(m.ws)
	if code, _, errs := runWith(o, "plan", "new", "Measure things"); code != 0 {
		t.Fatalf("plan new = %d: %s", code, errs)
	}
	gitIn(t, m.root, "init", "-q")
	steps := []string{commitAll(t, m.root, "base")}
	for _, f := range []string{"one.go", "two.go", "three.go"} {
		writeFile(t, filepath.Join(m.root, f), "package x\n")
		steps = append(steps, commitAll(t, m.root, "step "+f))
	}
	gitIn(t, m.root, "-c", "user.name=S", "-c", "user.email=s@x", "revert", "--no-edit", steps[2])
	gitIn(t, m.root, "-c", "user.name=S", "-c", "user.email=s@x", "revert", "--no-edit", steps[3])
	gitIn(t, m.root, "-c", "user.name=S", "-c", "user.email=s@x", "revert", "--no-edit", "HEAD")
	other, base, head := stepRepo(t)
	for _, args := range [][]string{
		{"--step", "1", "--range", steps[0][:7] + ".." + steps[1][:7], "--repo", m.root},
		{"--step", "2", "--range", steps[1] + ".." + steps[2], "--repo", m.root},
		{"--step", "2", "--role", "review", "--range", steps[1] + ".." + steps[2], "--repo", m.root},
		{"--step", "3", "--range", steps[3] + "^!", "--repo", m.root},
		{"--step", "4", "--range", base + ".." + head, "--repo", other},
	} {
		if code, _, errs := runWith(o, append([]string{"metrics", "add", "--root-sha", "abcdef1", "--home-sha", "abcdef2"}, args...)...); code != 0 {
			t.Fatalf("metrics add %v = %d: %s", args, code, errs)
		}
	}
	code, out, errs := runWith(o, "metrics")
	if code != 0 || errs != "" || !strings.Contains(out, "  1 of 3 (33%), 1 not found\n") ||
		!strings.Contains(out, "\n  A step is reverted when a commit on HEAD says \"This reverts commit <sha>\" of a commit in its range,\n  each range looked up in "+m.root+".\n") {
		t.Fatalf("metrics = %d:\n%s\n%s", code, out, errs)
	}
	code, out, errs = runWith(o, "metrics", "--json")
	var doc struct {
		Repositories []string
		Plans        []map[string]any
	}
	if err := json.Unmarshal([]byte(out), &doc); code != 0 || err != nil || len(doc.Plans) != 1 {
		t.Fatalf("metrics --json = %d %v:\n%s\n%s", code, err, out, errs)
	}
	p := doc.Plans[0]
	if fmt.Sprint(doc.Repositories) != "["+m.root+"]" || p["steps"] != 4.0 || p["steps_found"] != 3.0 || p["steps_reverted"] != 1.0 || p["revert_rate"] != 1.0/3 {
		t.Errorf("repositories %q, plan %v", doc.Repositories, p)
	}
}

// Step 29's claim for a swarm: its state's mirror of app holds three
// merges of heads its worker runs pushed and a fourth of a branch none
// pushed; the revert of one of the three and of the fourth on base, the
// base the swarm repository's mirrored swarm.json names, reads 1 of 3,
// as of the mirror's last fetch; a swarm the CSV names with no state on
// this machine has no count.
func TestMetricsSwarmReverts(t *testing.T) {
	m := newMachine(t)
	o := m.metricsOps(m.ws)
	state := filepath.Join(m.local, "swarm", "forge.example.net", "acme", "swarm-app")
	work := t.TempDir()
	gitIn(t, work, "init", "-q", "-b", "trunk")
	writeFile(t, filepath.Join(work, "README"), "app\n")
	commitAll(t, work, "first")
	merges := map[string]string{}
	for i, b := range []string{"swarm/1-1", "swarm/2-1", "swarm/3-1", "feature"} {
		gitIn(t, work, "checkout", "-q", "-b", b, "trunk")
		writeFile(t, filepath.Join(work, b), b)
		head := commitAll(t, work, b)
		if b != "feature" {
			writeFile(t, filepath.Join(state, "runs", fmt.Sprintf("%d-worker-1", i+1), "result"), "branch "+b+" in app\npushed "+head+"\noutcome posted\n")
		}
		gitIn(t, work, "checkout", "-q", "trunk")
		gitIn(t, work, "-c", "user.name=S", "-c", "user.email=s@x", "merge", "-q", "--no-ff", "-m", "Merge "+b, b)
		merges[b] = strings.TrimSpace(gitIn(t, work, "rev-parse", "HEAD"))
	}
	for _, b := range []string{"swarm/2-1", "feature"} {
		gitIn(t, work, "-c", "user.name=S", "-c", "user.email=s@x", "revert", "--no-edit", "-m", "1", merges[b])
	}
	swarmRepo := t.TempDir()
	gitIn(t, swarmRepo, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(swarmRepo, "swarm.json"), `{"base": "trunk", "a_field_this_daisy_lacks": 1}`)
	commitAll(t, swarmRepo, "swarm")
	at := time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)
	for name, from := range map[string]string{"app": work, "swarm-app": swarmRepo} {
		mirror := filepath.Join(state, "git", name)
		os.MkdirAll(mirror, 0o755)
		gitIn(t, mirror, "init", "-q", "--bare")
		gitIn(t, mirror, "fetch", "-q", from, "+refs/heads/*:refs/heads/*")
		gitIn(t, mirror, "symbolic-ref", "HEAD", "refs/heads/main")
		os.Chtimes(filepath.Join(mirror, "FETCH_HEAD"), at, at)
	}
	var csv []byte
	for _, r := range []metrics.Row{
		{When: "2026-11-12T10:00:00Z", Kind: metrics.SwarmRun, Source: "acme/swarm-app", Unit: "1-worker-1", Role: "worker"},
		{When: "2026-11-12T10:00:00Z", Kind: metrics.SwarmRun, Source: "acme/elsewhere", Unit: "1-worker-1", Role: "worker"},
	} {
		var err error
		if csv, err = metrics.AppendCSV(csv, r); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(m.home, "metrics", "metrics.csv"), string(csv))
	code, out, errs := runWith(o, "metrics")
	want := "\n  swarm           merged pulls  reverted      as of\n" +
		"  acme/elsewhere  —             —             no state on this machine\n" +
		"  acme/swarm-app  3             1 of 3 (33%)  the mirrors' last fetch, 2026-11-16T08:00:00Z\n"
	if code != 0 || errs != "" || !strings.Contains(out, want) {
		t.Fatalf("metrics = %d:\n%s\nwant within:\n%s\n%s", code, out, want, errs)
	}
	code, out, _ = runWith(o, "metrics", "--swarm", "acme/swarm-app", "--json")
	var doc struct{ Swarms []any }
	json.Unmarshal([]byte(out), &doc)
	got, _ := json.Marshal(doc.Swarms)
	if wantJSON := `[{"base":"trunk","fetched":"2026-11-16T08:00:00Z","merged_pulls":3,"problem":null,"revert_rate":0.3333333333333333,"reverted_pulls":1,"state":"` + state + `","swarm":"acme/swarm-app"}]`; code != 0 || string(got) != wantJSON {
		t.Errorf("metrics --swarm --json = %d, swarms:\n%s\nwant\n%s", code, got, wantJSON)
	}
}
