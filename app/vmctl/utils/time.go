package utils

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = 0 // use 0 instead of `int64(-1<<63) / 1e6` because the storage engine doesn't actually support negative time
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

// ParseTime parses time in s string and returns time.Time object
// if parse correctly or error if not
func ParseTime(s string) (time.Time, error) {
	msecs, err := promutils.ParseTimeMsec(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse %s: %w", s, err)
	}
	if msecs < minTimeMsecs {
		msecs = 0
	}
	if msecs > maxTimeMsecs {
		msecs = maxTimeMsecs
	}

	return time.Unix(0, msecs*int64(time.Millisecond)).UTC(), nil
}
