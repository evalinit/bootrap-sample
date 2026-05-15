//go:build !windows

package main

import (
	"syscall"
	"unsafe"
)

func isTerminal(fd uintptr) bool {
	var ws [4]uint16
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	return errno == 0
}

// isInteractiveTTY reports whether fd is a real TTY *and* this process is its
// foreground process group. This catches the case where xdg-open (or similar)
// inherits the parent terminal's file descriptors but immediately exits, giving
// the shell the foreground back — leaving us with a TTY fd we cannot own.
func isInteractiveTTY(fd uintptr) bool {
	if !isTerminal(fd) {
		return false
	}
	var tpgrp int32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGPGRP, uintptr(unsafe.Pointer(&tpgrp)))
	if errno != 0 {
		return false
	}
	return int(tpgrp) == syscall.Getpgrp()
}
