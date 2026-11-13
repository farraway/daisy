package reconcile

import (
	"os"
	"strings"
	"testing"

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
