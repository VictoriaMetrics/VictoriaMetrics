//go:build aix || darwin || dragonfly || freebsd || (linux && !appengine) || netbsd || openbsd || os400 || solaris || zos

package platform

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ergochat/readline/internal/term"
)

const (
	IsWindows = false
)

// SuspendProcess suspends the process with SIGTSTP,
// then blocks until it is resumed.
func SuspendProcess() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGCONT)
	defer stop()

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		panic(err)
	}
	p.Signal(syscall.SIGTSTP)
	// wait for SIGCONT
	<-ctx.Done()
}

// getWidthHeight of the terminal using given file descriptor
func getWidthHeight(stdoutFd int) (width int, height int) {
	width, height, err := term.GetSize(stdoutFd)
	if err != nil {
		return -1, -1
	}
	return
}

// GetScreenSize returns the width/height of the terminal or -1,-1 or error
func GetScreenSize() (width int, height int) {
	width, height = getWidthHeight(syscall.Stdout)
	if width < 0 {
		width, height = getWidthHeight(syscall.Stderr)
	}
	return
}

func DefaultIsTerminal() bool {
	return term.IsTerminal(syscall.Stdin) && (term.IsTerminal(syscall.Stdout) || term.IsTerminal(syscall.Stderr))
}

// -----------------------------------------------------------------------------

var (
	sizeChange         sync.Once
	sizeChangeCallback func()
)

func DefaultOnWidthChanged(f func()) {
	DefaultOnSizeChanged(f)
}

func DefaultOnSizeChanged(f func()) {
	sizeChangeCallback = f
	sizeChange.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)

		go func() {
			for {
				_, ok := <-ch
				if !ok {
					break
				}
				sizeChangeCallback()
			}
		}()
	})
}
