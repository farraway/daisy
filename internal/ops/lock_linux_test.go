package ops_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"daisy/internal/ops"
)

// The lock's descriptor is close-on-exec, so Reexec releases the lock as
// the exec succeeds and a failed exec keeps it (docs/swarm.md section 11,
// The restart): the descriptor /proc/self/fd names the lock file by
// carries FD_CLOEXEC.
func TestLockIsReleasedByAnExec(t *testing.T) {
	name := filepath.Join(t.TempDir(), "lock")
	release, err := ops.Real().FS().Lock(name, "apply since 2026-11-15T15:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		fd, _ := strconv.Atoi(e.Name())
		if target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name())); err != nil || target != name {
			continue
		}
		found = true
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
		if errno != 0 || flags&syscall.FD_CLOEXEC == 0 {
			t.Errorf("fd %d on the lock: flags %#x, errno %v: an exec would keep the lock", fd, flags, errno)
		}
	}
	if !found {
		t.Error("no descriptor names the lock file")
	}
}

// A lock this process released stays held by a child it forked during the
// hold for as long as the child shares the lock's descriptor (until its
// exec closes it; here, until the child, blocked on its stdin, exits):
// Lock waits for the child rather than refusing this process its own
// released lock, and refuses at once a lock whose line names another
// process.
func TestLockWaitsForAChildHoldingWhatItReleased(t *testing.T) {
	for _, other := range []bool{false, true} {
		t.Run(map[bool]string{false: "released by this process", true: "named another process"}[other], func(t *testing.T) {
			fs := ops.Real().FS()
			name := filepath.Join(t.TempDir(), "lock")
			release, err := fs.Lock(name, "apply")
			if err != nil {
				t.Fatal(err)
			}
			dup, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(lockFD(t, name)), syscall.F_DUPFD_CLOEXEC, 0)
			if errno != 0 {
				t.Fatal(errno)
			}
			shared := os.NewFile(dup, "lock")
			child := exec.Command("cat")
			child.ExtraFiles = []*os.File{shared}
			stdin, err := child.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			defer child.Wait()
			defer stdin.Close()
			shared.Close()
			release()
			// The child alone holds it now, until its stdin is closed.
			f, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
				t.Fatalf("flock while the child holds it: %v", err)
			}
			f.Close()
			if other {
				os.WriteFile(name, []byte("apply since 2026-11-20T10:00:00Z pid 1\n"), 0o644)
				_, err := fs.Lock(name, "second")
				var held *ops.LockedError
				if !errors.As(err, &held) || held.Holder != "apply since 2026-11-20T10:00:00Z pid 1" {
					t.Errorf("a lock naming another process: %v", err)
				}
				return
			}
			done := make(chan error, 1)
			go func() {
				again, err := fs.Lock(name, "second")
				if err == nil {
					again()
				}
				done <- err
			}()
			select {
			case err := <-done:
				t.Fatalf("Lock returned while the child held the lock: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			stdin.Close() // the child exits, and its descriptor with it
			if err := <-done; err != nil {
				t.Errorf("this process's released lock: %v", err)
			}
		})
	}
}

// lockFD is the descriptor of this process that names the file name.
func lockFD(t *testing.T, name string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if target, _ := os.Readlink(filepath.Join("/proc/self/fd", e.Name())); target == name {
			fd, _ := strconv.Atoi(e.Name())
			return fd
		}
	}
	t.Fatal("no descriptor names " + name)
	return -1
}
