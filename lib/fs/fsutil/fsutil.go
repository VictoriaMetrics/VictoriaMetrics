package fsutil

import (
	"os"
	"strconv"
	"testing"
)

// IsFsyncDisabled returns true if fsync must be disabled
//
// The fsync is disabled in tests, since it significantly slows down tests which work with files.
// The fsync can be enabled in tests by setting DISABLE_FSYNC_FOR_TESTING environment variable to false.
//
// The fsync is enabled for ordinary programs. It can be disabled by setting DISABLE_FSYNC_FOR_TESTING
// environment variable to true.
func IsFsyncDisabled() bool {
	return isFsyncDisabled
}

var isFsyncDisabled = isFsyncDisabledInternal()

func isFsyncDisabledInternal() bool {
	s := os.Getenv("DISABLE_FSYNC_FOR_TESTING")
	if s == "" {
		return testing.Testing()
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}
