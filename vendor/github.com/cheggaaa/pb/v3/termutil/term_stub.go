// Fallback for platforms without terminal control (appengine, js,
// wasip1, and any future GOOS not listed here).
//go:build appengine || (!aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !plan9 && !solaris && !windows)
// +build appengine !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!plan9,!solaris,!windows

package termutil

import (
	"errors"
	"os"
)

var errNotSupported = errors.New("not supported")

// Only os.Interrupt is portable here; it is never delivered because
// lockEcho always fails.
var unlockSignals = []os.Signal{os.Interrupt}

// TerminalWidth returns error, no terminal size API on these platforms.
func TerminalWidth() (int, error) {
	return 0, errNotSupported
}

// TerminalSize returns error, no terminal size API on these platforms.
func TerminalSize() (rows, cols int, err error) {
	return 0, 0, errNotSupported
}

func lockEcho() error {
	return errNotSupported
}

func unlockEcho() error {
	return nil
}
