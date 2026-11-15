package ops_test

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

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
