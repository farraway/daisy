//go:build unix

package ops

import (
	"os"
	"syscall"
)

func (real) Reexec(path string, args ...string) error {
	return syscall.Exec(path, append([]string{path}, args...), os.Environ())
}
