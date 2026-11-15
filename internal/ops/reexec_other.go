//go:build !unix

package ops

import "errors"

func (real) Reexec(path string, _ ...string) error {
	return errors.New("exec " + path + ": replacing the process is not available on this platform")
}
