//go:build darwin || linux || solaris
// +build darwin linux solaris

package terminal

import (
	"golang.org/x/sys/unix"
)

// IsTerminal returns true if the file descriptor is terminal
func IsTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	return err == nil
}
