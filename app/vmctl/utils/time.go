package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = 0 // use 0 instead of `int64(-1<<63) / 1e6` because the storage engine doesn't actually support negative time
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

func parseTime(s string) (float64, error) {
	if len(s) > 0 && (s[len(s)-1] != 'Z' && s[len(s)-1] > '9' || s[0] == '-') {
		// Parse duration relative to the current time
		d, err := promutils.ParseDuration(s)
		if err != nil {
			return 0, err
		}
		if d > 0 {
			d = -d
		}
		t := time.Now().Add(d)
		return float64(t.UnixNano()) / 1e9, nil
	}
	if len(s) == 4 {
		// Parse YYYY
		t, err := time.Parse("2006", s)
		if err != nil {
			return 0, err
		}
		return float64(t.UnixNano()) / 1e9, nil
	}
	if !strings.Contains(s, "-") {
		// Parse the timestamp in milliseconds
		return strconv.ParseFloat(s, 64)
	}
	if len(s) == 7 {
		// Parse YYYY-MM
		t, err := time.Parse("2006-01", s)
		if err != nil {
			return 0, err
		}
		return float64(t.UnixNano()) / 1e9, nil
	}
	if len(s) == 10 {
		// Parse YYYY-MM-DD
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return 0, err
		}
		return float64(t.UnixNano()) / 1e9, nil
	}
	if len(s) == 13 {
		// Parse YYYY-MM-DDTHH
		t, err := time.Parse("2006-01-02T15", s)
		if err != nil {
			return 0, err
		}
		return float64(t.UnixNano()) / 1e9, nil
	}
	if len(s) == 16 {
		// Parse YYYY-MM-DDTHH:MM
		t, err := time.Parse("2006-01-02T15:04", s)
		if err != nil {
			return 0, err
		}
		return float64(t.UnixNano()) / 1e9, nil
	}
	if len(s) == 19 {
		// Parse YYYY-MM-DDTHH:MM:SS
		t, err := time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			return 0, err
		}
		return float64(t.UnixNano()) / 1e9, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, err
	}
	return float64(t.UnixNano()) / 1e9, nil
}

// GetTime  returns time from the given string.
func GetTime(s string) (time.Time, error) {
	secs, err := parseTime(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse %s: %w", s, err)
	}
	msecs := int64(secs * 1e3)
	if msecs < minTimeMsecs {
		msecs = 0
	}
	if msecs > maxTimeMsecs {
		msecs = maxTimeMsecs
	}

	return time.Unix(0, msecs*int64(time.Millisecond)), nil
}
