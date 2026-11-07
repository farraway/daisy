package ops_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"daisy/internal/ops"
)

// A second Lock of a held file is refused with the holder's line; after
// the release it is taken again and the line is the new holder's.
func TestLockIsExclusiveAndNamesTheHolder(t *testing.T) {
	fs := ops.Real().FS()
	name := filepath.Join(t.TempDir(), "lock")
	release, err := fs.Lock(name, "apply --once since 2026-11-06T15:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pid := " pid " + strconv.Itoa(os.Getpid())
	_, err = fs.Lock(name, "other")
	var held *ops.LockedError
	if !errors.As(err, &held) || held.Holder != "apply --once since 2026-11-06T15:00:00Z"+pid {
		t.Fatalf("second Lock: %v", err)
	}
	release()
	release, err = fs.Lock(name, "second")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	defer release()
	if b, _ := os.ReadFile(name); string(b) != "second"+pid+"\n" {
		t.Errorf("holder line %q", b)
	}
	if _, err := ops.ReadOnly(ops.Real()).FS().Lock(name, "x"); err == nil || !strings.Contains(err.Error(), "Lock") {
		t.Errorf("Lock under ReadOnly: %v", err)
	}
}

func TestFakeAfter(t *testing.T) {
	var got time.Duration
	f := &ops.Fake{AfterFn: func(d time.Duration) <-chan time.Time {
		got = d
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}}
	<-ops.ReadOnly(f).After(3 * time.Second)
	if got != 3*time.Second {
		t.Errorf("After(%v)", got)
	}
}
