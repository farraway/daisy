package reconcile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"daisy/internal/ops"
)

// The runs' cost sums are rounded to six decimals before they go into
// status's document: runs of 0.1 and 0.2 give 0.3 today and in all, not
// 0.30000000000000004.
func TestThroughputRoundsCosts(t *testing.T) {
	state := t.TempDir()
	for i, cost := range []string{"0.1", "0.2"} {
		dir := filepath.Join(state, "runs", []string{"1-worker-1", "2-worker-1"}[i])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "result"), []byte("started 2026-11-21T09:00:00Z\ncost "+cost+"\noutcome posted\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := &ops.Fake{NowFn: func() time.Time { return time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC) }}
	tp, err := throughput(o, state)
	if err != nil {
		t.Fatal(err)
	}
	if tp.Cost.All != 0.3 || tp.Cost.Today != 0.3 {
		t.Errorf("cost all %v, today %v; want 0.3 and 0.3", tp.Cost.All, tp.Cost.Today)
	}
}
