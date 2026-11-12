package vmctlutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = 0 // use 0 instead of `int64(-1<<63) / 1e6` because the storage engine doesn't actually support negative time
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

// ParseTime parses time in s string and returns time.Time object
// if parsed correctly or error if not.
//
// The `<` and `>` prefixes make the time exclusive, e.g. `<2026-02-01` is the last millisecond before 2026-02-01.
func ParseTime(s string) (time.Time, error) {
	// The ±1ms step applies after rounding to msecs: for inputs with sub-ms precision, the rounding error might interfere
	halfOpenDelta := int64(0)
	if strings.HasPrefix(s, "<") {
		halfOpenDelta--
		s = s[1:]
	} else if strings.HasPrefix(s, ">") {
		halfOpenDelta++
		s = s[1:]
	}
	msecs, err := timeutil.ParseTimeMsec(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse %s: %w", s, err)
	}
	msecs += halfOpenDelta
	if msecs < minTimeMsecs {
		msecs = 0
	}
	if msecs > maxTimeMsecs {
		msecs = maxTimeMsecs
	}

	return time.Unix(0, msecs*int64(time.Millisecond)).UTC(), nil
}
