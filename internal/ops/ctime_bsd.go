//go:build darwin || freebsd || netbsd

package ops

import (
	"os"
	"syscall"
)

// ChangeTime is fi's change time (ctime) in nanoseconds since the epoch:
// what a write changes though it keeps the file's size and its times
// (`cp -p`); 0 where the system does not say.
func ChangeTime(fi os.FileInfo) int64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return st.Ctimespec.Nano()
	}
	return 0
}
