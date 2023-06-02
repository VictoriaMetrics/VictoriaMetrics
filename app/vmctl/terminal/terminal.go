package terminal

// IsTerminal returns true if the file descriptor is terminal
func IsTerminal(fd int) bool {
	return isTerminal(fd)
}
