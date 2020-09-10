package searchutils

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	maxExportDuration   = flag.Duration("search.maxExportDuration", time.Hour*24*30, "The maximum duration for /api/v1/export call")
	maxQueryDuration    = flag.Duration("search.maxQueryDuration", time.Second*30, "The maximum duration for search query execution")
	denyPartialResponse = flag.Bool("search.denyPartialResponse", false, "Whether to deny partial responses when some of vmstorage nodes are unavailable. This trades consistency over availability")
)

// GetTime returns time from the given argKey query arg.
func GetTime(r *http.Request, argKey string, defaultValue int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue, nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		t, err := time.Parse(time.RFC3339, argValue)
		if err != nil {
			// Handle Prometheus'-provided minTime and maxTime.
			// See https://github.com/prometheus/client_golang/issues/614
			switch argValue {
			case prometheusMinTimeFormatted:
				return minTimeMsecs, nil
			case prometheusMaxTimeFormatted:
				return maxTimeMsecs, nil
			}
			// Try parsing duration relative to the current time
			d, err1 := metricsql.DurationValue(argValue, 0)
			if err1 != nil {
				return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
			}
			if d > 0 {
				d = -d
			}
			t = time.Now().Add(time.Duration(d) * time.Millisecond)
		}
		secs = float64(t.UnixNano()) / 1e9
	}
	msecs := int64(secs * 1e3)
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

// GetDuration returns duration from the given argKey query arg.
func GetDuration(r *http.Request, argKey string, defaultValue int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue, nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		d, err := metricsql.DurationValue(argValue, 0)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
		}
		secs = float64(d) / 1000
	}
	msecs := int64(secs * 1e3)
	if msecs <= 0 || msecs > maxDurationMsecs {
		return 0, fmt.Errorf("%q=%dms is out of allowed range [%d ... %d]", argKey, msecs, 0, int64(maxDurationMsecs))
	}
	return msecs, nil
}

const maxDurationMsecs = 100 * 365 * 24 * 3600 * 1000

// GetDeadlineForQuery returns deadline for the given query r.
func GetDeadlineForQuery(r *http.Request, startTime time.Time) netstorage.Deadline {
	dMax := maxQueryDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxQueryDuration")
}

// GetDeadlineForExport returns deadline for the given request to /api/v1/export.
func GetDeadlineForExport(r *http.Request, startTime time.Time) netstorage.Deadline {
	dMax := maxExportDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxExportDuration")
}

func getDeadlineWithMaxDuration(r *http.Request, startTime time.Time, dMax int64, flagHint string) netstorage.Deadline {
	d, err := GetDuration(r, "timeout", 0)
	if err != nil {
		d = 0
	}
	if d <= 0 || d > dMax {
		d = dMax
	}
	timeout := time.Duration(d) * time.Millisecond
	return netstorage.NewDeadline(startTime, timeout, flagHint)
}

// GetBool returns boolean value from the given argKey query arg.
func GetBool(r *http.Request, argKey string) bool {
	argValue := r.FormValue(argKey)
	switch strings.ToLower(argValue) {
	case "", "0", "f", "false", "no":
		return false
	default:
		return true
	}
}

// GetDenyPartialResponse returns whether partial responses are denied.
func GetDenyPartialResponse(r *http.Request) bool {
	if *denyPartialResponse {
		return true
	}
	return GetBool(r, "deny_partial_response")
}
