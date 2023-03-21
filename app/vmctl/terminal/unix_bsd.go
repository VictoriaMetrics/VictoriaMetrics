//go:build darwin
// +build darwin

package terminal

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TIOCGETA
