package netutil

import (
	"strings"
)

// IsTrivialNetworkError returns true if the err can be ignored during logging.
func IsTrivialNetworkError(err error) bool {
	// Suppress trivial network errors, which could occur at remote side.
	s := err.Error()
	if strings.Contains(s, "broken pipe") || strings.Contains(s, "reset by peer") {
		return true
	}
	return false
}
