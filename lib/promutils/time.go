package promutils

import (
	"strconv"
	"strings"
	"time"
)

// ParseTime parses time s in different formats.
//
// See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#timestamp-formats
//
// It returns unix timestamp in seconds.
func ParseTime(s string) (float64, error) {
	if len(s) > 0 && (s[len(s)-1] != 'Z' && s[len(s)-1] > '9' || s[0] == '-') {
		// Parse duration relative to the current time
		d, err := ParseDuration(s)
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
