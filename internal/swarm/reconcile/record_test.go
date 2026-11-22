package reconcile

import (
	"os"
	"strings"
	"testing"
	"time"

	"daisy/internal/metrics"
	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// A collection retried before its `recorded` line queues the run's
// journal line once; another run's line is added after it.
func TestQueueHoldsARunOnce(t *testing.T) {
	s := &Swarm{O: ops.Real(), Local: t.TempDir(), Cfg: declare.Config{Forge: declare.Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/swarm-app"}}}
	os.MkdirAll(s.state(), 0o755)
	for _, q := range []struct{ day, line string }{
		{"2026-11-12", "- 1500 1-architect-1: posted; #1"},
		{"2026-11-12", "- 1501 1-architect-1: posted; #1"},
		{"2026-11-12", "- 1501 1-architect-2: posted; #1"},
	} {
		if err := s.queue(q.day, q.line); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(s.journalQueue())
	if want := "2026-11-12 - 1500 1-architect-1: posted; #1\n2026-11-12 - 1501 1-architect-2: posted; #1\n"; string(b) != want {
		t.Errorf("queue:\n%s", strings.ReplaceAll(string(b), "\n", "⏎\n"))
	}
}

// A run whose persona is not known (no persona line, no argv, a role
// swarm.json does not declare) has its journal line say so, never an
// empty part (Step 7 of the plan implementing the 2026-11-19 swarm
// roadmap, L329); a known one is written as before.
func TestJournalLineUnknownPersona(t *testing.T) {
	at := time.Date(2026, 11, 12, 15, 4, 0, 0, time.UTC)
	for _, c := range []struct {
		row  metrics.Row
		want string
	}{
		{metrics.Row{Unit: "0-qa-1", Outcome: "failed", Notes: "batch"}, "- 1504 0-qa-1: failed; unknown persona; no usage reported; batch"},
		{metrics.Row{Unit: "1-worker-1", Outcome: "posted", Provider: "xai", Model: "grok-4.7", Notes: "#1"}, "- 1504 1-worker-1: posted; xai grok-4.7; no usage reported; #1"},
	} {
		if got := journalLine(c.row, "", at); got != c.want {
			t.Errorf("journalLine = %q, want %q", got, c.want)
		}
	}
}

// A cost line gives an estimate only in its two forms, six words or
// eight whose seventh is `cached` and whose count is a number, and only
// with the cached count within the input (section 7.6, The run's
// record): eight words with another seventh, a count that is no number
// and a cached count over the input give the row no estimate, the line
// of the form an estimate.
func TestRunRowCostLineForms(t *testing.T) {
	for _, c := range []struct {
		line     string
		estimate bool
	}{
		{"cost 0.50 tokens 1000/200 turns 3 cached 400", true},
		{"cost 0.50 tokens 1000/200 turns 3 kept 400", false},
		{"cost 0.50 tokens 1000/200 turns 3 cached many", false},
		{"cost 0.50 tokens 1000/200 turns 3 cached 1001", false},
	} {
		r := metrics.Row{Model: "claude-sonnet-5-5"}
		if u, ok := readCost(c.line); ok {
			usage(&r, u.in, u.out, u.cached)
		}
		if got := r.CostEstimated != ""; got != c.estimate {
			t.Errorf("%q: estimate %q, want one: %v", c.line, r.CostEstimated, c.estimate)
		}
	}
}
