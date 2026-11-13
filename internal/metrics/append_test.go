package metrics

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"daisy/internal/ops"
)

// Two writers, as `metrics add` and `swarm apply` are, each appending
// rows to one CSV under the machine's metrics lock: every row of both
// lands, none lost to the other's read-then-rewrite.
func TestAppendUnderTheLockLosesNoRow(t *testing.T) {
	dir := t.TempDir()
	file, lock := filepath.Join(dir, "home", "metrics", "metrics.csv"), filepath.Join(dir, "local", "metrics.lock")
	o := ops.Real()
	const each = 40
	var wg sync.WaitGroup
	errs := make(chan error, 2*each)
	for w := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				row := Row{When: "2026-11-12T10:00:00Z", Kind: SwarmRun, Source: "acme/swarm-app", Unit: fmt.Sprintf("%d-worker-%d", w, i)}
				release, err := Lock(o, lock, "test writer")
				if err != nil {
					errs <- err
					return
				}
				err = Append(o.FS(), file, row)
				release()
				if err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1+2*each {
		t.Fatalf("%d records, want the header and %d rows", len(records), 2*each)
	}
}

// A lock held past the wait is an error naming its holder, not a write.
func TestLockGivesUpNamingTheHolder(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "metrics.lock")
	release, err := ops.Real().FS().Lock(lock, "swarm apply")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	o := &ops.Fake{
		NowFn: func() time.Time { return now },
		AfterFn: func(d time.Duration) <-chan time.Time {
			now = now.Add(d)
			c := make(chan time.Time, 1)
			c <- now
			return c
		},
	}
	_, err = Lock(o, lock, "metrics add")
	var held *ops.LockedError
	if !errors.As(err, &held) || held.Holder == "" {
		t.Fatalf("Lock = %v, want the holder named", err)
	}
}
