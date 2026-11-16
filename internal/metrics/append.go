package metrics

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
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

// Append appends to the CSV at file (AppendCSV) each of rows whose key
// it holds no row of, nor an earlier one of rows, in one rewrite,
// creating the file and its directory when absent; it is how many it
// appended, the file untouched when none. The caller holds Lock.
func Append(fsys ops.FS, file string, rows ...Row) (int, error) {
	return appendIf(fsys, file, rows, func(last []string, _ Row) bool { return last == nil })
}

// Supersede appends to the CSV at file each of rows whose cells differ
// from the last row of its key, the CSV's or an earlier one of rows (a
// key the CSV holds no row of included): a correction supersedes the row
// it corrects, and a row the CSV's last word already is adds nothing.
// As Append, in one rewrite; the caller holds Lock.
func Supersede(fsys ops.FS, file string, rows ...Row) (int, error) {
	return appendIf(fsys, file, rows, func(last []string, r Row) bool { return !slices.Equal(last, r.cells()) })
}

// appendIf appends each of rows that add takes, given the cells of the
// last row of its key (nil for none), the CSV's or an earlier appended
// one's.
func appendIf(fsys ops.FS, file string, rows []Row, add func(last []string, r Row) bool) (int, error) {
	before, err := fsys.ReadFile(file)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return 0, fmt.Errorf("cannot read %s: %v", file, err)
	}
	recs, err := records(before)
	if err != nil {
		return 0, fmt.Errorf("cannot record in %s: %v", file, err)
	}
	last := map[string][]string{}
	for _, rec := range recs {
		last[key(rec)] = rec
	}
	var take []Row
	for _, r := range rows {
		if k := r.Key(); add(last[k], r) {
			last[k] = r.cells()
			take = append(take, r)
		}
	}
	if len(take) == 0 {
		return 0, nil
	}
	out, err := AppendCSV(before, take...)
	if err != nil {
		return 0, fmt.Errorf("cannot record in %s: %v", file, err)
	}
	if err := fsys.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return 0, fmt.Errorf("cannot create %s: %v", filepath.Dir(file), err)
	}
	if err := ops.Rewrite(fsys, file, out, 0o644); err != nil {
		return 0, fmt.Errorf("cannot write %s: %v", file, err)
	}
	return len(take), nil
}
