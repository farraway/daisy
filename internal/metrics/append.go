package metrics

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"time"

	"daisy/internal/ops"
)

// lockWait is how long Lock waits for another writer, polling every
// lockPoll: a writer holds the lock for one read and one rewrite of the
// CSV, so a lock held longer belongs to a process that is stuck.
const (
	lockWait = 30 * time.Second
	lockPoll = 50 * time.Millisecond
)

// Lock takes the machine's metrics lock, name (env.MetricsLock), which
// every writer of a metrics CSV holds from its read to its rewrite, so a
// row another writer appends meanwhile is never written over. It waits
// while another holds it; past lockWait the error is the
// *ops.LockedError naming the holder.
func Lock(o ops.Ops, name, holder string) (release func(), err error) {
	if err := o.FS().MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return nil, err
	}
	deadline := o.Now().Add(lockWait)
	for {
		release, err := o.FS().Lock(name, holder)
		var held *ops.LockedError
		if !errors.As(err, &held) || !o.Now().Before(deadline) {
			return release, err
		}
		<-o.After(lockPoll)
	}
}

// Append appends r to the CSV at file (AppendCSV), creating the file and
// its directory when absent; the caller holds Lock.
func Append(fsys ops.FS, file string, r Row) error {
	before, err := fsys.ReadFile(file)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return fmt.Errorf("cannot read %s: %v", file, err)
	}
	out, err := AppendCSV(before, r)
	if err != nil {
		return fmt.Errorf("cannot record in %s: %v", file, err)
	}
	if err := fsys.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %v", filepath.Dir(file), err)
	}
	if err := ops.Rewrite(fsys, file, out, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %v", file, err)
	}
	return nil
}
