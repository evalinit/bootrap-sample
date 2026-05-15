//go:build windows

package main

import "syscall"

func isTerminal(fd uintptr) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(fd), &mode) == nil
}

func isInteractiveTTY(fd uintptr) bool { return isTerminal(fd) }
