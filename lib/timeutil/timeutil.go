package timeutil

import (
	"time"

	"github.com/valyala/fastrand"
)

var monotonicStart = time.Now()

// MonotonicMilliseconds returns the number of milliseconds elapsed since package initialization.
// Use it for relative deadlines that must be unaffected by wall clock adjustments.
func MonotonicMilliseconds() int64 {
	return time.Since(monotonicStart).Milliseconds()
}

// AddJitterToDuration adds up to 10% random jitter to d and returns the resulting duration.
//
// The maximum jitter is limited by 10 seconds.
func AddJitterToDuration(d time.Duration) time.Duration {
	dv := min(d/10, 10*time.Second)
	p := float64(fastrand.Uint32()) / (1 << 32)
	return d + time.Duration(p*float64(dv))
}
