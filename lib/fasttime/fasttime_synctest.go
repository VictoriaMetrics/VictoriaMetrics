//go:build goexperiment.synctest

package fasttime

import (
	"time"
)

// UnixTimestamp returns the current unix timestamp in seconds.
func UnixTimestamp() uint64 {
	// Fall back to time.Now().Unix(), since this is needed for synctest.
	return uint64(time.Now().Unix())
}
