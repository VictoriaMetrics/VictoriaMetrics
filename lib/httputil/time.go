package httputil

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// GetTime returns time in milliseconds from the given argKey query arg.
//
// If argKey is missing in r, then defaultMs rounded to seconds is returned.
// The rounding is needed in order to align query results in Grafana
// executed at different times. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/720
func GetTime(r *http.Request, argKey string, defaultMs int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return roundToSeconds(defaultMs), nil
	}
	// Handle Prometheus'-provided minTime and maxTime.
	// See https://github.com/prometheus/client_golang/issues/614
	switch argValue {
	case prometheusMinTimeFormatted:
		return minTimeMsecs, nil
	case prometheusMaxTimeFormatted:
		return maxTimeMsecs, nil
	}
	// Parse argValue
	msecs, err := timeutil.ParseTimeMsec(argValue)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %s=%s: %w", argKey, argValue, err)
	}
	if msecs < minTimeMsecs {
		msecs = 0
	}
	if msecs > maxTimeMsecs {
		msecs = maxTimeMsecs
	}
	return msecs, nil
}

var (
	// These constants were obtained from https://github.com/prometheus/prometheus/blob/91d7175eaac18b00e370965f3a8186cc40bf9f55/web/api/v1/api.go#L442
	// See https://github.com/prometheus/client_golang/issues/614 for details.
	prometheusMinTimeFormatted = time.Unix(math.MinInt64/1000+62135596801, 0).UTC().Format(time.RFC3339Nano)
	prometheusMaxTimeFormatted = time.Unix(math.MaxInt64/1000-62135596801, 999999999).UTC().Format(time.RFC3339Nano)
)

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = 0 // use 0 instead of `int64(-1<<63) / 1e6` because the storage engine doesn't actually support negative time
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

func roundToSeconds(ms int64) int64 {
	return ms - ms%1000
}
