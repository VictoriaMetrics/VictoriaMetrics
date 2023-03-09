package tty

import (
	"os"
)

// IsTerminal return true if the file descriptor is terminal.
func IsTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

