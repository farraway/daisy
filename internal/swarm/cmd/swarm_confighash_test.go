package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/metrics"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// configText is the configuration's text of a run's system.md as section
// 7.6 names it: from its `# Persona` line up to its `# Task` line.
func configText(t *testing.T, sys string) string {
	t.Helper()
	at, task := strings.Index(sys, "\n# Persona\n"), strings.Index(sys, "\n# Task\n")
	if at < 0 || task < at {
		t.Fatalf("no # Persona then # Task:\n%s", sys)
	}
	return sys[at+1 : task+1]
}

func sha(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// writeRows writes the home's metrics CSV as rows, under this Daisy's
// header.
func writeRows(t *testing.T, home string, rs []map[string]string) {
	t.Helper()
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Write(metrics.Header)
	for _, r := range rs {
		cells := make([]string, len(metrics.Header))
		for i, c := range metrics.Header {
			cells[i] = r[c]
		}
		w.Write(cells)
	}
	w.Flush()
	if err := os.WriteFile(filepath.Join(home, "metrics", "metrics.csv"), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Step R3-12b's claim (the plan implementing the 2026-11-19 swarm
// roadmap; sections 7.6, The run's record, and 11, The scorecard): two
// architect runs with the same persona and prompt part on issues 1 and 2
// record one config_hash, the sha256 of their system.md's text from
// `# Persona` to `# Task` (their prompt_hash, over the whole file,
// differs), and read as one line of the scorecard by configuration; a
// changed part gives issue 3's run another config_hash and a second
// line. An earlier row without the cell reads as `config unknown`; a
// reprice fills it from the run's kept system.md, and a backfill writes
// it, each matching the hash a new run of the same text records.
func TestSwarmConfigHash(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	home := a.metricsHome()
	a.write("prompts/parts/style.md", "# Style\n\nShort plans.\n")
	a.edit(func(c *declare.Config) {
		c.PromptParts = []declare.PromptPart{{Path: "prompts/parts/style.md", Roles: []string{"architect"}}}
	})
	a.forge.CreateIssue("Drift check", "Check the drift.", []string{"swarm:proposal-accepted"})
	a.forge.CreateIssue("Lint check", "Check the lint.", []string{"swarm:proposal-accepted"})
	a.runner.Script(1, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.runner.Script(2, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	got := rows(t, home)
	if len(got) != 2 {
		t.Fatalf("%d rows: %v", len(got), got)
	}
	one := sha(configText(t, a.file("1-architect-1", "io/system.md")))
	if !strings.Contains(configText(t, a.file("1-architect-1", "io/system.md")), "# Prompt part: prompts/parts/style.md\n\n## Style\n\nShort plans.\n") {
		t.Fatalf("the part is not in the configuration's text:\n%s", a.file("1-architect-1", "io/system.md"))
	}
	for i, r := range got {
		if r["config_hash"] != one {
			t.Errorf("row %d (%s): config_hash %q, want %q", i+1, r["step_or_run"], r["config_hash"], one)
		}
	}
	if got[0]["prompt_hash"] == got[1]["prompt_hash"] {
		t.Errorf("prompt_hash equal over two runs' system.md: %s", got[0]["prompt_hash"])
	}
	line := func(hash string, runs string) string {
		return "  architect anthropic claude-fable-5-1 high, config " + hash + ": runs " + runs + ";"
	}
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if out := a.stdout.String(); !strings.Contains(out, line(one[:12], "2")) || strings.Count(out, "  architect ") != 1 {
		t.Errorf("scorecard:\n%s\nwant one architect line: %s", out, line(one[:12], "2"))
	}

	// A changed part: issue 3's run is another configuration.
	a.write("prompts/parts/style.md", "# Style\n\nShorter plans.\n")
	a.forge.CreateIssue("Vet check", "Check the vet.", []string{"swarm:proposal-accepted"})
	a.runner.Script(3, "architect", runner.Canned{Out: archPlan, Stdout: claudeResult})
	a.setNow("2026-11-06T16:00:00Z")
	if err := a.apply(a.forge, "--once"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	got = rows(t, home)
	if len(got) != 3 || got[2]["step_or_run"] != "3-architect-1" {
		t.Fatalf("rows: %v", got)
	}
	three := sha(configText(t, a.file("3-architect-1", "io/system.md")))
	if got[2]["config_hash"] != three || three == one {
		t.Errorf("3-architect-1: config_hash %q, want %q (not %q)", got[2]["config_hash"], three, one)
	}
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if out := a.stdout.String(); !strings.Contains(out, line(one[:12], "2")) ||
		!strings.Contains(out, line(three[:12], "1")) || strings.Count(out, "  architect ") != 2 {
		t.Errorf("scorecard:\n%s\nwant two architect lines: %s, %s", out, line(one[:12], "2"), line(three[:12], "1"))
	}

	// An earlier row, recorded before the column: unknown, until a
	// reprice reads its run's kept system.md.
	got[0]["config_hash"] = ""
	writeRows(t, home, got)
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if out := a.stdout.String(); !strings.Contains(out, line("unknown", "1")) || !strings.Contains(out, line(one[:12], "1")) {
		t.Errorf("scorecard:\n%s\nwant %s and %s", out, line("unknown", "1"), line(one[:12], "1"))
	}
	if err := a.metricsVerb("--reprice", "--dry-run"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if want := "  1-architect-1 anthropic claude-fable-5-1: tokens_in 10 → 10, tokens_cached 0 → 0, tokens_total 30 → 30, cost_estimated 0.0011 → 0.0011, config_hash — → " + one + "\n" +
		"Would reprice 1 rows in "; !strings.HasPrefix(a.stdout.String(), want) {
		t.Errorf("dry run:\n%s\nwant to start:\n%s", a.stdout.String(), want)
	}
	if err := a.metricsVerb("--reprice"); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	got = rows(t, home)
	if len(got) != 4 || got[3]["step_or_run"] != "1-architect-1" || got[3]["config_hash"] != one || !strings.HasSuffix(got[3]["notes"], ", repriced") {
		t.Fatalf("reprice: %v\n%s", got, a.stdout.String())
	}
	if err := a.scorecard(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	if out := a.stdout.String(); !strings.Contains(out, line(one[:12], "2")) || strings.Contains(out, "config unknown") {
		t.Errorf("scorecard after the reprice:\n%s", out)
	}

	// A backfill of a run with no row: its config_hash is the new run's
	// of the same text.
	writeRows(t, home, got[1:3])
	if err := a.backfill(); err != nil {
		t.Fatalf("%v\n%s", err, a.stderr.String())
	}
	got = rows(t, home)
	if len(got) != 3 || got[2]["step_or_run"] != "1-architect-1" || got[2]["config_hash"] != got[0]["config_hash"] || got[2]["config_hash"] != one {
		t.Errorf("backfill: %v", got)
	}
}
