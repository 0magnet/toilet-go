//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"syscall"
	"unsafe"
)

// winsize matches struct winsize, of which only ws_col is read.
type winsize struct {
	rows, cols, xpixel, ypixel uint16
}

// terminalWidth returns the width of the controlling terminal, asking stdout,
// stderr and then stdin, which is the order -t uses.
func terminalWidth() (int, bool) {
	for _, fd := range []uintptr{1, 2, 0} {
		var ws winsize
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd,
			uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
		if errno == 0 && ws.cols != 0 {
			return int(ws.cols), true
		}
	}
	return 0, false
}
