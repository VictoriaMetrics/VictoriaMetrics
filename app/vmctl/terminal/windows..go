//go:build windows

package terminal

func isTerminal(fd int) bool {
	return true
}
