//go:build unix

package ops

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// held is the lock files this process holds through Lock, by device and
// inode, so that Lock can tell a lock it released from one it holds.
var held = struct {
	sync.Mutex
	files map[[2]uint64]bool
}{files: map[[2]uint64]bool{}}

// Lock takes name's flock. A child this process forks while it holds a
// lock shares the lock's descriptor until the child's exec closes it
// (close-on-exec), so a lock just released can stay held for a moment
// by that child alone: when the file names this process as its holder
// and this process does not hold it, Lock tries again until the child
// has let it go, and refuses only a lock held by another process or by
// this one.
func (realFS) Lock(name, holder string) (func(), error) {
	own := fmt.Sprintf(" pid %d", os.Getpid())
	for {
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			return nil, err
		}
		key, err := fileKey(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("locking %s: %w", name, err)
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return locked(f, key, name, holder)
		}
		f.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("locking %s: %w", name, err)
		}
		b, _ := os.ReadFile(name)
		line := strings.TrimSpace(string(b))
		held.Lock()
		mine := held.files[key]
		held.Unlock()
		if mine || !strings.HasSuffix(line, own) {
			return nil, &LockedError{name, line}
		}
		time.Sleep(time.Millisecond) // a child of this process holds what this process released
	}
}

// fileKey is f's device and inode.
func fileKey(f *os.File) ([2]uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		return [2]uint64{}, err
	}
	return [2]uint64{uint64(st.Dev), uint64(st.Ino)}, nil
}

// locked records the lock whose flock f holds as this process's, before
// anything else, and writes the holder's line into its file; release
// drops it, once.
func locked(f *os.File, key [2]uint64, name, holder string) (func(), error) {
	held.Lock()
	held.files[key] = true
	held.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			held.Lock()
			delete(held.files, key)
			held.Unlock()
			f.Close()
		})
	}
	err := f.Truncate(0)
	if err == nil {
		_, err = fmt.Fprintf(f, "%s pid %d\n", holder, os.Getpid())
	}
	if err != nil {
		release()
		return nil, fmt.Errorf("writing %s: %w", name, err)
	}
	return release, nil
}
