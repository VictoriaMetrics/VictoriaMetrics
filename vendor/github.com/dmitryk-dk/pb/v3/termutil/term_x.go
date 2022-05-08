//go:build (linux || darwin || freebsd || netbsd || openbsd || solaris || dragonfly) && !appengine

package termutil

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/term"
)

var (
	tty *os.File

	unlockSignals = []os.Signal{
		os.Interrupt, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGKILL,
	}
	termState *term.State
)

type window struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func init() {
	var err error
	tty, err = os.Open("/dev/tty")
	if err != nil {
		tty = os.Stdin
	}
}

// TerminalWidth returns width of the terminal.
func TerminalWidth() (int, error) {
	_, c, err := TerminalSize()
	return c, err
}

// TerminalSize returns size of the terminal.
func TerminalSize() (rows, cols int, err error) {
	w := new(window)
	res, _, err := syscall.Syscall(sysIoctl,
		tty.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(w)),
	)
	if int(res) == -1 {
		return 0, 0, err
	}
	return int(w.Row), int(w.Col), nil
}

func lockEcho() error {
	fd := tty.Fd()

	var err error
	if termState, err = term.MakeRaw(int(fd)); err != nil {
		return fmt.Errorf("error when puts the terminal connected to the given file descriptor: %v", err)
	}
	return nil
}

func unlockEcho() error {
	fd := tty.Fd()
	if err := term.Restore(int(fd), termState); err != nil {
		return fmt.Errorf("error restores the terminal connected to the given file descriptor: %w", err)
	}
	return nil
}
