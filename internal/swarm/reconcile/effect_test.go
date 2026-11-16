package reconcile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
)

// Step 30's swarm-side definition (the plan remediating the 2026-11-15
// design review, P29; decision 142): over scratch runs, a review not
// posted or approving has effect 0; one requesting changes is credited
// its [blocking] findings when its role's next posted review on the
// issue is of the next round, on another head, and approves; 0 when
// that review is on the same head or requests changes again; unknown
// while there is none, or when the next is of another round; the other
// reviewer's runs and another issue's are not read as next; a run being
// collected, its outcome not yet in its result, counts as posted when
// the collection says so.
func TestReviewEffects(t *testing.T) {
	s := &Swarm{O: ops.Real(), Local: t.TempDir(), Cfg: declare.Config{Forge: declare.Forge{Kind: "gitea", API: "https://forge.example.net/api/v1", Repo: "acme/swarm-app"}}}
	rc := func(n int) string {
		return "VERDICT: request-changes\n\n" + strings.Repeat("[blocking] main.go:1 wrong.\n", n) + "[non-blocking] main.go:2 a nit.\n"
	}
	const approve = "VERDICT: approve\n\n[non-blocking] main.go:2 a nit.\n"
	write := func(run, result, out string) {
		dir := filepath.Join(s.state(), "runs", run)
		os.MkdirAll(filepath.Join(dir, "io"), 0o755)
		os.WriteFile(filepath.Join(dir, "result"), []byte(result), 0o644)
		os.WriteFile(filepath.Join(dir, "io", "out.md"), []byte(out), 0o644)
	}
	posted := func(round, head string) string {
		return "start " + head + "\nlabel swarm:review-0-ready\nround " + round + "\nexited 0 2026-11-16T10:00:00Z\nposted review\nrecorded\noutcome posted\ncollected\n"
	}
	h1, h2, h3 := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	write("1-adversary-1", posted("1", h1), rc(2))
	write("1-adversary-2", "start "+h2+"\nround 2\nexited 0 2026-11-16T10:00:00Z\nfailed: bad verdict\nrecorded\noutcome failed\ncollected\n", "VERDICT: request-changes\n")
	write("1-adversary-3", posted("2", h2), approve)
	write("2-adversary-1", posted("1", h1), rc(1))
	write("2-adversary-2", posted("2", h1), approve) // the worker answered, no commit
	write("3-adversary-1", posted("1", h1), rc(1))
	write("3-adversary-2", posted("2", h2), rc(1))
	write("3-adversary-3", posted("1", h3), approve) // a re-plan: the rounds counted again
	write("4-sympathetic-1", posted("1", h1), approve)
	write("4-adversary-1", posted("1", h1), rc(3))
	write("4-sympathetic-2", posted("2", h2), approve)
	write("4-adversary-2", "start "+h2+"\nround 2\nexited 0 2026-11-16T10:00:00Z\nposted review\n", approve) // being collected

	got := map[string]string{}
	for _, r := range []Run{{N: 1, K: 1, Role: "adversary"}, {N: 2, K: 1, Role: "adversary"}, {N: 3, K: 1, Role: "adversary"}, {N: 4, K: 1, Role: "adversary"}, {N: 4, K: 1, Role: "sympathetic"}} {
		effects, err := s.reviewEffects(r, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range effects {
			got[e.run] = e.effect
		}
	}
	want := map[string]string{
		"1-adversary-1": "2", "1-adversary-2": "0", "1-adversary-3": "0",
		"2-adversary-1": "0", "2-adversary-2": "0",
		"3-adversary-1": "0", "3-adversary-2": "", "3-adversary-3": "0",
		"4-adversary-1": "", "4-adversary-2": "0", "4-sympathetic-1": "0", "4-sympathetic-2": "0",
	}
	for run, w := range want {
		if e, ok := got[run]; !ok || e != w {
			t.Errorf("%s: effect %q (%v), want %q", run, e, ok, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("effects %v", got)
	}

	// 4-adversary-2 being collected, its outcome posted: 4-adversary-1's
	// three blocking findings were fixed.
	effects, err := s.reviewEffects(Run{N: 4, K: 2, Role: "adversary"}, "posted")
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 2 || effects[0] != (runEffect{"4-adversary-1", 1, "3"}) || effects[1] != (runEffect{"4-adversary-2", 2, "0"}) {
		t.Errorf("being collected: %+v", effects)
	}
	if effects, err := s.reviewEffects(Run{N: 4, K: 2, Role: "adversary"}, "stale"); err != nil || effects[0].effect != "" || effects[1].effect != "0" {
		t.Errorf("being collected, stale: %+v, %v", effects, err)
	}
}
