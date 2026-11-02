package ops

import (
	"syscall"
	"unsafe"
)

// isatty asks the terminal driver for fd's settings, as isatty(3) does.
func isatty(fd uintptr) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return errno == 0
}
