//go:build unix

package ops

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

func (realFS) Lock(name, holder string) (func(), error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			line, _ := os.ReadFile(name)
			return nil, &LockedError{name, strings.TrimSpace(string(line))}
		}
		return nil, fmt.Errorf("locking %s: %w", name, err)
	}
	err = f.Truncate(0)
	if err == nil {
		_, err = fmt.Fprintf(f, "%s pid %d\n", holder, os.Getpid())
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("writing %s: %w", name, err)
	}
	return func() { f.Close() }, nil
}
