//go:build !linux && !darwin && !freebsd && !netbsd

package ops

import "os"

// ChangeTime is fi's change time where Daisy knows how to read it; here
// it does not, and it is 0, the build then told by its size and its
// modification time alone.
func ChangeTime(os.FileInfo) int64 { return 0 }
