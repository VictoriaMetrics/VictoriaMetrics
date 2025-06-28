//go:build !goexperiment.synctest

package fasttime

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
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

var currentTimestamp = func() *atomicutil.Uint64 {
	var x atomicutil.Uint64
	x.Store(uint64(time.Now().Unix()))
	return &x
}()

// UnixTimestamp returns the current unix timestamp in seconds.
//
// It is faster than time.Now().Unix()
func UnixTimestamp() uint64 {
	return currentTimestamp.Load()
}
