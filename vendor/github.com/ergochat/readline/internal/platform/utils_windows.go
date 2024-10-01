//go:build windows

package platform

import (
	"syscall"

	"github.com/ergochat/readline/internal/term"
)

const (
	IsWindows = true
)

func SuspendProcess() {
}

// GetScreenSize returns the width, height of the terminal or -1,-1
func GetScreenSize() (width int, height int) {
	width, height, err := term.GetSize(int(syscall.Stdout))
	if err == nil {
		return width, height
	} else {
		return 0, 0
	}
}

func DefaultIsTerminal() bool {
	return term.IsTerminal(int(syscall.Stdin)) && term.IsTerminal(int(syscall.Stdout))
}

func DefaultOnWidthChanged(f func()) {
	DefaultOnSizeChanged(f)
}

func DefaultOnSizeChanged(f func()) {
	// TODO: does Windows have a SIGWINCH analogue?
}
