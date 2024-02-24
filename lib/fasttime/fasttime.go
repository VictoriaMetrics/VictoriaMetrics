package fasttime

import (
	"sync/atomic"
	"time"
)

func init() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for tm := range ticker.C {
			t := uint64(tm.Unix())
			currentTimestamp.Store(t)
		}
	}()
}

var currentTimestamp = func() *atomic.Uint64 {
	var x atomic.Uint64
	x.Store(uint64(time.Now().Unix()))
	return &x
}()

// UnixTimestamp returns the current unix timestamp in seconds.
//
// It is faster than time.Now().Unix()
func UnixTimestamp() uint64 {
	return currentTimestamp.Load()
}

// UnixDate returns date from the current unix timestamp.
//
// The date is calculated by dividing unix timestamp by (24*3600)
func UnixDate() uint64 {
	return UnixTimestamp() / (24 * 3600)
}

// UnixHour returns hour from the current unix timestamp.
//
// The hour is calculated by dividing unix timestamp by 3600
func UnixHour() uint64 {
	return UnixTimestamp() / 3600
}
