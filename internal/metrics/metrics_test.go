package metrics

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	for path, want := range map[string]Class{
		"main.go":                         Code,
		"internal/x/x.go":                 Code,
		"internal/x/x_test.go":            Tests,
		"tests/run.sh":                    Tests,
		"tests/cases/01-x.sh":             Tests,
		"pkg/test/fixture.json":           Tests,
		"tests/README.md":                 Tests,
		"README.md":                       Prose,
		"prompts/plan.md":                 Prose,
		"docs/diagram.svg":                Prose,
		"site/docs/index.html":            Prose,
		"testdata/x.json":                 Code,
		"latest/x.go":                     Code,
		"test":                            Code, // a file named test, no directory
		"docs":                            Code,
		"internal/contest/a.go":           Code,
		"internal/x/x_test.go.orig":       Code,
		"skills/testing/SKILL.md":         Prose,
		"templates/plan.md":               Prose,
		"images/member/entrypoint.sh":     Code,
		"cmd/daisy/daisy_metrics_test.go": Tests,
	} {
		if got := Classify(path); got != want {
			t.Errorf("Classify(%q) = %v, want %v", path, got, want)
		}
	}
}

// numstat -z: a plain entry is `a\td\tpath\0`; a rename `a\td\t\0src\0dst\0`,
// classed by where it went; a binary file `-\t-\tpath\0`, zero lines.
func TestCount(t *testing.T) {
	out := "3\t1\tmain.go\x00" +
		"5\t0\tmain_test.go\x00" +
		"1\t1\t\x00docs/guide.txt\x00docs/manual.txt\x00" +
		"2\t2\t\x00notes.md\x00notes.go\x00" +
		"-\t-\tlogo.png\x00" +
		"0\t4\ttests/old.sh\x00"
	got, err := Count([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := Lines{CodeAdded: 5, CodeRemoved: 3, TestsAdded: 5, TestsRemoved: 4, ProseAdded: 1, ProseRemoved: 1, Binary: 1}
	if got != want {
		t.Fatalf("Count = %+v, want %+v", got, want)
	}
	if got, err := Count(nil); err != nil || got != (Lines{}) {
		t.Fatalf("Count(nil) = %+v, %v", got, err)
	}
	for _, bad := range []string{"3\tmain.go\x00", "x\t1\tmain.go\x00", "1\t1\t\x00only-src\x00"} {
		if _, err := Count([]byte(bad)); err == nil {
			t.Errorf("Count(%q) took it", bad)
		}
	}
}

func row(step string) Row {
	return Row{When: "2026-11-12T14:05:00+01:00", Kind: PlanStep, Source: "p", Unit: step, Role: "step", Model: "m", TokensTotal: "10"}
}

func TestAppendCSV(t *testing.T) {
	b, err := AppendCSV(nil, row("1"))
	if err != nil {
		t.Fatal(err)
	}
	header := strings.Join(Header, ",") + "\n"
	first := "2026-11-12T14:05:00+01:00,plan-step,p,1,step,,m,,,,,,,,,10,,,,,,,,,,,,,,\n"
	if string(b) != header+first {
		t.Fatalf("first append:\n%s", b)
	}
	r := row("2")
	r.Notes = "a, \"b\"\nc"
	r.Lines = &Lines{CodeAdded: 1}
	b, err = AppendCSV(b, r)
	if err != nil {
		t.Fatal(err)
	}
	if want := header + first + "2026-11-12T14:05:00+01:00,plan-step,p,2,step,,m,,,,,,,,,10,,,1,0,0,0,0,0,0,,\"a, \"\"b\"\"\nc\",,,\n"; string(b) != want {
		t.Fatalf("second append:\n%q\nwant\n%q", b, want)
	}
	// A file without its final newline gets one before the row.
	if b, err := AppendCSV([]byte(header+strings.TrimSuffix(first, "\n")), row("3")); err != nil || !strings.HasPrefix(string(b), header+first) {
		t.Fatalf("no final newline: %q %v", b, err)
	}
	// Another header is refused: the columns are never reordered.
	if _, err := AppendCSV([]byte("when,kind\n"), row("3")); err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("other header: %v", err)
	}
	bad := row("4")
	bad.TokensIn = "1.5"
	if _, err := AppendCSV(nil, bad); err == nil || !strings.Contains(err.Error(), "tokens_in") {
		t.Fatalf("bad tokens: %v", err)
	}
	bad = row("4")
	bad.Kind = "other"
	if _, err := AppendCSV(nil, bad); err == nil {
		t.Fatal("bad kind taken")
	}
}

const plan = "# P\n\n## Steps\n\n- [x] Step 1\n\n```\n## Decisions\n```\n\n## Decisions\n\n- one\n\n## Deferred\n\n- later\n"

func TestPlanRowInsertsAfterDecisions(t *testing.T) {
	r := row("1")
	r.Lines = &Lines{CodeAdded: 3, CodeRemoved: 1, TestsAdded: 6, ProseAdded: 4, ProseRemoved: 1}
	r.Range = "a..b"
	got, err := PlanRow([]byte(plan), r)
	if err != nil {
		t.Fatal(err)
	}
	table := "## Metrics\n\n" + TableHeader +
		"| 1 | step | m |  |  |  |  |  | 10 |  | +3/-1 | +6/-0 | +4/-1 | a..b |  |\n\n"
	want := strings.Replace(plan, "## Deferred", table+"## Deferred", 1)
	if string(got) != want {
		t.Fatalf("PlanRow:\n%s\nwant\n%s", got, want)
	}

	// A second step appends a row; the first step again replaces its own.
	got, _ = PlanRow(got, row("2"))
	r = row("1")
	r.Notes = "a | b\nc"
	got, err = PlanRow(got, r)
	if err != nil {
		t.Fatal(err)
	}
	table = "## Metrics\n\n" + TableHeader +
		"| 1 | step | m |  |  |  |  |  | 10 |  |  |  |  |  | a \\| b c |\n" +
		"| 2 | step | m |  |  |  |  |  | 10 |  |  |  |  |  |  |\n\n"
	if want := strings.Replace(plan, "## Deferred", table+"## Deferred", 1); string(got) != want {
		t.Fatalf("PlanRow twice:\n%s\nwant\n%s", got, want)
	}

	// Another role of the same step is a row of its own.
	r = row("1")
	r.Role = "review"
	got, _ = PlanRow(got, r)
	if !strings.Contains(string(got), "| 1 | step | m |") || !strings.Contains(string(got), "| 2 | step | m |  |  |  |  |  | 10 |  |  |  |  |  |  |\n| 1 | review | m |") {
		t.Fatalf("PlanRow, another role:\n%s", got)
	}
}

func TestPlanRowPlacement(t *testing.T) {
	table := TableHeader + "| 1 | step | m |  |  |  |  |  | 10 |  |  |  |  |  |  |\n"
	for name, c := range map[string]struct{ doc, want string }{
		"no Decisions: at the end": {
			"# P\n\n## Steps\n", "# P\n\n## Steps\n\n## Metrics\n\n" + table,
		},
		"Decisions last, no final newline": {
			"# P\n\n## Decisions\n\n- one", "# P\n\n## Decisions\n\n- one\n\n## Metrics\n\n" + table,
		},
		"an empty section, as the template has": {
			"# P\n\n## Metrics\n\n## Deferred\n", "# P\n\n## Metrics\n\n" + table + "\n## Deferred\n",
		},
		"a section with prose and no table": {
			"# P\n\n## Metrics\n\nSome words.\n\n\n## Deferred\n", "# P\n\n## Metrics\n\nSome words.\n\n" + table + "\n\n## Deferred\n",
		},
	} {
		got, err := PlanRow([]byte(c.doc), row("1"))
		if err != nil || string(got) != c.want {
			t.Errorf("%s: %v\n%q\nwant\n%q", name, err, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	var b []byte
	add := func(r Row) {
		var err error
		if b, err = AppendCSV(b, r); err != nil {
			t.Fatal(err)
		}
	}
	r := row("1")
	r.TokensTotal, r.CostReported, r.Lines = "100", "1.50", &Lines{CodeAdded: 10, CodeRemoved: 2, ProseAdded: 5}
	add(r)
	r = row("1") // supersedes the first
	r.TokensTotal, r.CostReported, r.Lines = "200", "2.00", &Lines{CodeAdded: 20, TestsAdded: 7}
	add(r)
	r = row("2")
	r.TokensTotal, r.TokensIn, r.TokensOut = "", "30", "12"
	add(r)
	r = row("3")
	r.Model, r.TokensTotal = "other", ""
	add(r)
	s, err := Summarize(b, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Rows != 4 || s.Superseded != 1 || len(s.Groups) != 2 {
		t.Fatalf("Summarize = %+v", s)
	}
	g := s.Groups[0]
	want := Group{Kind: PlanStep, Model: "m", Rows: 2, Tokens: 242, WithTokens: 2, Cost: 2, WithCost: 1, Measured: 1,
		Lines: Lines{CodeAdded: 20, TestsAdded: 7}}
	if g != want {
		t.Fatalf("group m = %+v\nwant %+v", g, want)
	}
	if g := s.Groups[1]; g.Model != "other" || g.Rows != 1 || g.WithTokens != 0 {
		t.Fatalf("group other = %+v", g)
	}
	if _, err := Summarize([]byte("when,kind\nx,y\n"), Filter{}); err == nil {
		t.Fatal("another header summarized")
	}
}

// A filter keeps a swarm's rows (kind swarm-run, that swarm) written at
// or after a time; a row is superseded by its key's last row whoever the
// filter keeps, so a correction outside the window still hides the row
// it corrects; with a time, a row whose time is not RFC 3339 is refused,
// named.
func TestSummarizeFilter(t *testing.T) {
	var b []byte
	add := func(when, swarm, run, model string) {
		r := Row{When: when, Kind: SwarmRun, Source: swarm, Unit: run, Role: "worker", Model: model, TokensIn: "10", TokensOut: "5"}
		if swarm == "" {
			r = row(run)
			r.When = when
		}
		var err error
		if b, err = AppendCSV(b, r); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-11-10T09:00:00Z", "acme/swarm-app", "1-worker-1", "old")
	add("2026-11-12T09:00:00Z", "acme/swarm-app", "2-worker-1", "new")
	add("2026-11-12T10:00:00+01:00", "acme/swarm-app", "3-worker-1", "new") // 09:00Z: at the time, kept
	add("2026-11-13T09:00:00Z", "acme/swarm-other", "1-worker-1", "new")
	add("2026-11-13T09:00:00Z", "", "1", "m")                          // a plan's step
	add("2026-11-09T09:00:00Z", "acme/swarm-app", "2-worker-1", "new") // corrects run 2, before the window
	since := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	s, err := Summarize(b, Filter{Swarm: "acme/swarm-app", Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if s.Rows != 2 || s.Superseded != 1 || len(s.Groups) != 1 || s.Groups[0].Model != "new" || s.Groups[0].Rows != 1 || s.Groups[0].Tokens != 15 {
		t.Errorf("swarm since: %+v", s)
	}
	if s, _ := Summarize(b, Filter{Swarm: "acme/swarm-app"}); s.Rows != 4 || s.Superseded != 1 || len(s.Groups) != 2 {
		t.Errorf("swarm: %+v", s)
	}
	if s, _ := Summarize(b, Filter{Since: since}); s.Rows != 4 || s.Superseded != 1 || len(s.Groups) != 2 {
		t.Errorf("since: %+v", s)
	}
	add("yesterday", "acme/swarm-app", "4-worker-1", "new")
	if _, err := Summarize(b, Filter{Since: since}); err == nil || err.Error() != "row 8: when is not an RFC 3339 time (got: yesterday)" {
		t.Errorf("a bad time: %v", err)
	}
	if _, err := Summarize(b, Filter{Swarm: "acme/swarm-app"}); err != nil {
		t.Errorf("no time, not read: %v", err)
	}
}

// firstHeader is the header the CSV had from Step 16a to Step 28 of the
// plan remediating the 2026-11-15 design review: tokens_cached not yet
// added.
const firstHeader = "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds," +
	"tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed," +
	"binary_files,range,notes\n"

// Step 28's schema change: a CSV under the first header is read, each
// row's tokens_cached empty; appending to it rewrites the header line
// alone, to this Daisy's, every row's bytes kept and the new row whole
// under it; a row with fewer cells than the first header, or more than
// its file's header, is refused, named.
func TestFirstHeaderReadAndExtended(t *testing.T) {
	old := firstHeader + "2026-11-12T14:05:00+01:00,plan-step,p,1,step,,m,,,,,,,,,10,,,,,,,,,,,\n"
	s, err := Summarize([]byte(old), Filter{})
	if err != nil || s.Rows != 1 || len(s.Groups) != 1 || s.Groups[0].Tokens != 10 || s.Groups[0].WithCached != 0 {
		t.Fatalf("Summarize(first header) = %+v, %v", s, err)
	}
	r := row("2")
	r.TokensIn, r.TokensOut, r.TokensCached, r.TokensTotal = "40", "2", "30", "42"
	b, err := AppendCSV([]byte(old), r)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(firstHeader, "\n") + ",tokens_cached,review_effect,cause\n" +
		"2026-11-12T14:05:00+01:00,plan-step,p,1,step,,m,,,,,,,,,10,,,,,,,,,,,\n" +
		"2026-11-12T14:05:00+01:00,plan-step,p,2,step,,m,,,,,,,40,2,42,,,,,,,,,,,,30,,\n"
	if string(b) != want {
		t.Fatalf("appended:\n%s\nwant\n%s", b, want)
	}
	s, err = Summarize(b, Filter{})
	if err != nil || s.Rows != 2 || s.Groups[0].Tokens != 52 || s.Groups[0].Cached != 30 || s.Groups[0].WithCached != 1 {
		t.Fatalf("Summarize(extended) = %+v, %v", s, err)
	}
	if keys, err := Keys(b); err != nil || len(keys) != 2 {
		t.Fatalf("Keys = %v, %v", keys, err)
	}
	for _, c := range []struct{ file, want string }{
		{firstHeader + "2026-11-12T14:05:00+01:00,plan-step,p,1,step,,m,,,,,,,,,10,,,,,,,,,,\n", "row 2 has 26 cells, not 27"},
		{firstHeader + "2026-11-12T14:05:00+01:00,plan-step,p,1,step,,m,,,,,,,,,10,,,,,,,,,,,,\n", "row 2 has 28 cells, not 27"},
		{want + "2026-11-12T14:05:00+01:00,plan-step,p,3,step,,m,,,,,,,,,10,,,,,,,,,,\n", "row 4 has 26 cells, not 27 to 30"},
	} {
		if _, err := Summarize([]byte(c.file), Filter{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Summarize = %v, want %q", err, c.want)
		}
		if _, err := AppendCSV([]byte(c.file), row("9")); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("AppendCSV = %v, want %q", err, c.want)
		}
	}
}

// step28Header is the header the CSV had from Step 28 to Step 30 of the
// plan remediating the 2026-11-15 design review: review_effect not yet
// added.
const step28Header = "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds," +
	"tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed," +
	"binary_files,range,notes,tokens_cached\n"

// Step 30's schema change, as Step 28's: a CSV under Step 28's header,
// holding a row of each width it allows (27 cells, written under the
// first header, and 28), is read, review_effect empty; an append
// rewrites the header line alone, every row's bytes kept and the new
// row whole under it, its review_effect read back.
func TestStep28HeaderReadAndExtended(t *testing.T) {
	rows := "2026-11-12T14:05:00+01:00,plan-step,p,1,review,,m,,,,,,,,,10,,,,,,,,,,,\n" +
		"2026-11-13T14:05:00+01:00,plan-step,p,2,review,,m,,,,,,,,,20,,,,,,,,,,,,5\n"
	old := step28Header + rows
	s, err := Summarize([]byte(old), Filter{})
	if err != nil || s.Rows != 2 || len(s.Reviews) != 1 || s.Reviews[0].Rows != 2 || s.Reviews[0].WithEffect != 0 || s.Reviews[0].Tokens != 30 {
		t.Fatalf("Summarize(Step 28's header) = %+v, %v", s, err)
	}
	r := row("3")
	r.Role, r.ReviewEffect = "review", "2"
	b, err := AppendCSV([]byte(old), r)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(step28Header, "\n") + ",review_effect,cause\n" + rows +
		"2026-11-12T14:05:00+01:00,plan-step,p,3,review,,m,,,,,,,,,10,,,,,,,,,,,,,2,\n"
	if string(b) != want {
		t.Fatalf("appended:\n%s\nwant\n%s", b, want)
	}
	s, err = Summarize(b, Filter{})
	if err != nil || s.Rows != 3 || s.Reviews[0].Effect != 2 || s.Reviews[0].WithEffect != 1 || s.Reviews[0].Tokens != 40 {
		t.Fatalf("Summarize(extended) = %+v, %v", s, err)
	}
	if latest, err := Latest(b); err != nil || latest[r.Key()].ReviewEffect != "2" {
		t.Fatalf("Latest = %+v, %v", latest, err)
	}
}

// step30Header is the header the CSV had from Step 30 of the plan
// remediating the 2026-11-15 design review to Step 4 of the plan
// implementing the 2026-11-19 swarm roadmap: cause not yet added.
const step30Header = "when,kind,plan_or_swarm,step_or_run,role,provider,model,effort,root_sha,home_sha,prompt_hash,outcome,review_rounds," +
	"tokens_in,tokens_out,tokens_total,cost_reported,cost_estimated,code_added,code_removed,tests_added,tests_removed,prose_added,prose_removed," +
	"binary_files,range,notes,tokens_cached,review_effect\n"

// Step 4's schema change (the plan implementing the 2026-11-19 swarm
// roadmap), as Step 30's: a CSV under Step 30's header, holding a row of
// each width it allows (27, 28 and 29 cells), is read, cause empty; an
// append rewrites the header line alone, every row's bytes kept and the
// new row whole under it, its cause read back.
func TestStep30HeaderReadAndExtended(t *testing.T) {
	rows := "2026-11-12T14:05:00+01:00,swarm-run,acme/app,1-worker-1,worker,,m,,,,,blocked,,,,10,,,,,,,,,,,\n" +
		"2026-11-13T14:05:00+01:00,swarm-run,acme/app,1-worker-2,worker,,m,,,,,blocked,,,,20,,,,,,,,,,,,5\n" +
		"2026-11-14T14:05:00+01:00,swarm-run,acme/app,1-worker-3,worker,,m,,,,,blocked,,,,30,,,,,,,,,,,,5,\n"
	old := step30Header + rows
	latest, err := Latest([]byte(old))
	if err != nil || len(latest) != 3 {
		t.Fatalf("Latest(Step 30's header) = %+v, %v", latest, err)
	}
	for k, r := range latest {
		if r.Cause != "" {
			t.Errorf("%q: cause %q", k, r.Cause)
		}
	}
	r := Row{When: "2026-11-15T14:05:00+01:00", Kind: SwarmRun, Source: "acme/app", Unit: "1-worker-4", Role: "worker", Model: "m", Outcome: "question", TokensTotal: "40", Cause: "plan"}
	b, err := AppendCSV([]byte(old), r)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(step30Header, "\n") + ",cause\n" + rows +
		"2026-11-15T14:05:00+01:00,swarm-run,acme/app,1-worker-4,worker,,m,,,,,question,,,,40,,,,,,,,,,,,,,plan\n"
	if string(b) != want {
		t.Fatalf("appended:\n%s\nwant\n%s", b, want)
	}
	if latest, err := Latest(b); err != nil || latest[r.Key()].Cause != "plan" {
		t.Fatalf("Latest = %+v, %v", latest, err)
	}
	if s, err := Summarize(b, Filter{}); err != nil || s.Rows != 4 {
		t.Fatalf("Summarize(extended) = %+v, %v", s, err)
	}
}

// Step 28's per-plan figure: a plan's plan-step rows, every role's (the
// step's, its review's, the orchestrator's), a superseded row left out:
// their tokens over the code lines their measured rows added; a plan
// with no code added has no figure; a swarm's rows are no plan's. Step
// 29's: a plan's ranges, each once, a superseded row's left out, and the
// swarms named.
func TestSummarizePlans(t *testing.T) {
	var b []byte
	ranges := map[string]string{"9999": "x..y", "1000": "a..b", "500": "a..b", "": "c^!"}
	add := func(source, step, role, tokens string, lines *Lines) {
		r := Row{When: "2026-11-12T10:00:00Z", Kind: PlanStep, Source: source, Unit: step, Role: role, Model: "m", TokensTotal: tokens, Lines: lines, Range: ranges[tokens]}
		var err error
		if b, err = AppendCSV(b, r); err != nil {
			t.Fatal(err)
		}
	}
	add("261111.a", "1", "step", "9999", &Lines{CodeAdded: 1})
	add("261111.a", "1", "step", "1000", &Lines{CodeAdded: 10, TestsAdded: 50}) // supersedes the first
	add("261111.a", "1", "review", "500", nil)
	add("261111.a", "1", "orchestrator", "", nil)
	add("261111.a", "2", "step", "", &Lines{CodeAdded: 2})
	add("261110.b", "1", "step", "100", &Lines{ProseAdded: 4})
	var err error
	if b, err = AppendCSV(b, Row{When: "2026-11-12T10:00:00Z", Kind: SwarmRun, Source: "acme/swarm-app", Unit: "1-worker-1", Role: "worker", TokensTotal: "7"}); err != nil {
		t.Fatal(err)
	}
	s, err := Summarize(b, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Plan{
		{Name: "261110.b", Rows: 1, WithTokens: 1, Tokens: 100, Measured: 1, CodeAdded: 0},
		{Name: "261111.a", Rows: 4, WithTokens: 2, Tokens: 1500, Measured: 2, CodeAdded: 12, Ranges: []string{"a..b", "c^!"}},
	}
	if fmt.Sprint(s.Plans) != fmt.Sprint(want) {
		t.Fatalf("Plans = %+v\nwant %+v", s.Plans, want)
	}
	if n, ok := s.Plans[1].TokensPerCodeLine(); !ok || n != 125 {
		t.Errorf("a's tokens per code line = %d, %v", n, ok)
	}
	if _, ok := s.Plans[0].TokensPerCodeLine(); ok {
		t.Error("b, no code added, has a figure")
	}
	if s, _ := Summarize(b, Filter{Swarm: "acme/swarm-app"}); len(s.Plans) != 0 {
		t.Errorf("a swarm's rows as plans: %+v", s.Plans)
	}
	if fmt.Sprint(s.Swarms) != "[acme/swarm-app]" {
		t.Errorf("Swarms = %q", s.Swarms)
	}
}

// Step 30's claim (the plan remediating the 2026-11-15 design review,
// P29): the review rows, those of a reviewer's role (a plan's review,
// a swarm's adversary and sympathetic) or recording a review_effect,
// summed by role and model, a superseded row left out: their rows, the
// effect over the rows that record one, the tokens over those that
// report them, and the tokens per effect over the rows that have both;
// a step's row without an effect is no review.
func TestSummarizeReviews(t *testing.T) {
	var b []byte
	add := func(kind Kind, source, unit, role, model, tokens, effect string) {
		r := Row{When: "2026-11-16T10:00:00Z", Kind: kind, Source: source, Unit: unit, Role: role, Model: model, TokensTotal: tokens, ReviewEffect: effect}
		var err error
		if b, err = AppendCSV(b, r); err != nil {
			t.Fatal(err)
		}
	}
	plan := func(step, role, tokens, effect string) { add(PlanStep, "261116.p", step, role, "opus", tokens, effect) }
	swarm := func(run, role, model, tokens, effect string) {
		add(SwarmRun, "acme/swarm-app", run, role, model, tokens, effect)
	}
	plan("1", "review", "1000", "2")
	plan("2", "review", "3000", "0")
	plan("3", "review", "500", "") // recorded before the column
	plan("4", "review", "9999", "1")
	plan("4", "review", "600", "3") // supersedes the one before
	plan("1", "step", "5000", "")   // no review
	plan("5", "step", "200", "1")   // an effect on another role's row
	swarm("1-adversary-1", "adversary", "gpt", "100", "1")
	swarm("1-adversary-2", "adversary", "gpt", "50", "0")
	swarm("1-adversary-3", "adversary", "gpt", "", "2")        // its usage not reported
	swarm("1-sympathetic-1", "sympathetic", "grok", "400", "") // its next review not yet collected
	swarm("1-sympathetic-2", "sympathetic", "grok", "", "0")
	swarm("1-worker-1", "worker", "grok", "700", "")
	s, err := Summarize(b, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Review{
		{Role: "adversary", Model: "gpt", Rows: 3, WithEffect: 3, Effect: 3, WithTokens: 2, Tokens: 150, PairedTokens: 150, PairedEffect: 1},
		{Role: "review", Model: "opus", Rows: 4, WithEffect: 3, Effect: 5, WithTokens: 4, Tokens: 5100, PairedTokens: 4600, PairedEffect: 5},
		{Role: "step", Model: "opus", Rows: 1, WithEffect: 1, Effect: 1, WithTokens: 1, Tokens: 200, PairedTokens: 200, PairedEffect: 1},
		{Role: "sympathetic", Model: "grok", Rows: 2, WithEffect: 1, Effect: 0, WithTokens: 1, Tokens: 400},
	}
	if fmt.Sprint(s.Reviews) != fmt.Sprint(want) {
		t.Fatalf("Reviews = %+v\nwant %+v", s.Reviews, want)
	}
	for i, w := range []struct {
		n  int
		ok bool
	}{{150, true}, {920, true}, {200, true}, {0, false}} {
		if n, ok := s.Reviews[i].TokensPerEffect(); n != w.n || ok != w.ok {
			t.Errorf("%s %s: tokens per effect = %d, %v, want %d, %v", want[i].Role, want[i].Model, n, ok, w.n, w.ok)
		}
	}
	if s, _ := Summarize(b, Filter{Swarm: "acme/swarm-app"}); len(s.Reviews) != 2 || s.Reviews[0].Role != "adversary" || s.Reviews[1].Role != "sympathetic" {
		t.Errorf("a swarm's reviews: %+v", s.Reviews)
	}
	bad := Row{When: "2026-11-16T10:00:00Z", Kind: PlanStep, Source: "p", Unit: "1", Role: "review", ReviewEffect: "two"}
	if _, err := AppendCSV(nil, bad); err == nil || !strings.Contains(err.Error(), "review_effect must be a whole number") {
		t.Errorf("a bad effect: %v", err)
	}
}
