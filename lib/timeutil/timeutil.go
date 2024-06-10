package timeutil

import (
	"time"

	"github.com/valyala/fastrand"
)

// AddJitterToDuration adds up to 10% random jitter to d and returns the resulting duration.
//
// The maximum jitter is limited by 10 seconds.
func AddJitterToDuration(d time.Duration) time.Duration {
	dv := d / 10
	if dv > 10*time.Second {
		dv = 10 * time.Second
	}
	p := float64(fastrand.Uint32()) / (1 << 32)
	return d + time.Duration(p*float64(dv))
}
