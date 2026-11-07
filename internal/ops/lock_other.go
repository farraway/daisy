//go:build !unix

package ops

import "errors"

func (realFS) Lock(name, _ string) (func(), error) {
	return nil, errors.New("locking " + name + ": flock is not available on this platform")
}
