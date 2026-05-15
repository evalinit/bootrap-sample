//go:build !windows

package main

import (
	"syscall"
	"unsafe"
)

// isTerminal reports whether fd is a real TTY.
// TIOCGWINSZ succeeds only on actual terminal file descriptors, not /dev/null.
func isTerminal(fd uintptr) bool {
	var ws [4]uint16
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	return errno == 0
}
