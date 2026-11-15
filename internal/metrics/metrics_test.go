package metrics

import (
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
	first := "2026-11-12T14:05:00+01:00,plan-step,p,1,step,,m,,,,,,,,,10,,,,,,,,,,,\n"
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
	if want := header + first + "2026-11-12T14:05:00+01:00,plan-step,p,2,step,,m,,,,,,,,,10,,,1,0,0,0,0,0,0,,\"a, \"\"b\"\"\nc\"\n"; string(b) != want {
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
