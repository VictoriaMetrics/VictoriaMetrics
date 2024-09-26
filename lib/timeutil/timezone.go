package timeutil

import (
	"sync/atomic"
	"time"
)

// GetLocalTimezoneOffsetNsecs returns local timezone offset in nanoseconds.
func GetLocalTimezoneOffsetNsecs() int64 {
	return localTimezoneOffsetNsecs.Load()
}

var localTimezoneOffsetNsecs atomic.Int64

func updateLocalTimezoneOffsetNsecs() {
	_, offset := time.Now().Zone()
	nsecs := int64(offset) * 1e9
	localTimezoneOffsetNsecs.Store(nsecs)
}

func init() {
	updateLocalTimezoneOffsetNsecs()
	// Update local timezone offset in a loop, since it may change over the year due to DST.
	go func() {
		t := time.NewTicker(5 * time.Second)
		for range t.C {
			updateLocalTimezoneOffsetNsecs()
		}
	}()
}
