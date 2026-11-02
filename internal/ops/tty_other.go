//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd

package ops

// isatty: without a terminal ioctl Daisy knows here, nothing is a
// terminal: stdin is read as piped input (a bare `daisy feedback` waits for
// it) and no prompt is offered (`eval --record` asks for no notes).
func isatty(uintptr) bool { return false }
