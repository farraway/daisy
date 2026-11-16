package metrics

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
				_, err = Append(o.FS(), file, row)
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

// Append writes no row whose key (kind, plan or swarm, step or run,
// role) the CSV holds, nor one twice, in one rewrite; with none to write
// the file is not touched.
func TestAppendSkipsHeldKeys(t *testing.T) {
	file := filepath.Join(t.TempDir(), "metrics", "metrics.csv")
	fs := ops.Real().FS()
	run := func(unit, role, notes string) Row {
		return Row{When: "2026-11-12T10:00:00Z", Kind: SwarmRun, Source: "acme/swarm-app", Unit: unit, Role: role, Notes: notes}
	}
	if n, err := Append(fs, file, run("1-worker-1", "worker", "live")); n != 1 || err != nil {
		t.Fatalf("Append = %d, %v", n, err)
	}
	n, err := Append(fs, file, run("1-worker-1", "worker", "again"), run("2-owner-1", "owner", "new"), run("2-owner-1", "owner", "twice"),
		Row{When: "2026-11-12T10:00:00Z", Kind: PlanStep, Source: "acme/swarm-app", Unit: "1-worker-1", Role: "worker", Notes: "a step"})
	if n != 2 || err != nil {
		t.Fatalf("Append = %d, %v", n, err)
	}
	b, _ := os.ReadFile(file)
	records, _ := csv.NewReader(bytes.NewReader(b)).ReadAll()
	var notes []string
	for _, r := range records[1:] {
		notes = append(notes, r[slices.Index(Header, "notes")])
	}
	if fmt.Sprint(notes) != "[live new a step]" {
		t.Fatalf("rows' notes %q", notes)
	}
	os.Chmod(filepath.Dir(file), 0o555) // a rewrite would fail
	defer os.Chmod(filepath.Dir(file), 0o755)
	if n, err := Append(fs, file, run("2-owner-1", "owner", "held")); n != 0 || err != nil {
		t.Errorf("Append of a held row = %d, %v", n, err)
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

// Supersede appends each row whose cells differ from its key's last row
// in the CSV (a key the CSV lacks included), none whose cells are that
// row's, nor one twice, in one rewrite; with none to write the file is
// not touched. Latest reads each key's last row back as it was written.
func TestSupersede(t *testing.T) {
	file := filepath.Join(t.TempDir(), "metrics", "metrics.csv")
	fs := ops.Real().FS()
	run := func(unit, in, notes string) Row {
		return Row{When: "2026-11-12T10:00:00Z", Kind: SwarmRun, Source: "acme/swarm-app", Unit: unit, Role: "worker", TokensIn: in, TokensCached: "1", Notes: notes,
			Lines: &Lines{CodeAdded: 3, TestsRemoved: 1}, Range: "a..b"}
	}
	if n, err := Append(fs, file, run("1-worker-1", "10", "#1"), run("2-worker-1", "20", "#2")); n != 2 || err != nil {
		t.Fatalf("Append = %d, %v", n, err)
	}
	b, _ := os.ReadFile(file)
	latest, err := Latest(b)
	if err != nil || len(latest) != 2 {
		t.Fatalf("Latest = %v, %v", latest, err)
	}
	held := latest[run("1-worker-1", "", "").Key()]
	if !slices.Equal(held.cells(), run("1-worker-1", "10", "#1").cells()) {
		t.Fatalf("Latest row %q", held.cells())
	}
	repriced := held
	repriced.TokensIn, repriced.TokensCached, repriced.Notes = "15", "5", "#1, repriced"
	n, err := Supersede(fs, file, repriced, run("2-worker-1", "20", "#2"), repriced, run("3-worker-1", "30", "#3"))
	if n != 2 || err != nil {
		t.Fatalf("Supersede = %d, %v", n, err)
	}
	after, _ := os.ReadFile(file)
	records, _ := csv.NewReader(bytes.NewReader(after)).ReadAll()
	var notes []string
	for _, r := range records[1:] {
		notes = append(notes, r[slices.Index(Header, "notes")])
	}
	if fmt.Sprint(notes) != "[#1 #2 #1, repriced #3]" {
		t.Fatalf("rows' notes %q", notes)
	}
	if s, err := Summarize(after, Filter{}); err != nil || s.Rows != 4 || s.Superseded != 1 {
		t.Fatalf("Summarize = %+v, %v", s, err)
	}
	os.Chmod(filepath.Dir(file), 0o555) // a rewrite would fail
	defer os.Chmod(filepath.Dir(file), 0o755)
	if n, err := Supersede(fs, file, repriced, run("3-worker-1", "30", "#3")); n != 0 || err != nil {
		t.Errorf("Supersede of current rows = %d, %v", n, err)
	}
}
