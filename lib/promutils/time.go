package promutils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// ParseTimeMsec parses time s in different formats.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#timestamp-formats
//
// It returns unix timestamp in milliseconds.
func ParseTimeMsec(s string) (int64, error) {
	currentTimestamp := time.Now().UnixNano()
	nsecs, err := ParseTimeAt(s, currentTimestamp)
	if err != nil {
		return 0, err
	}
	msecs := int64(math.Round(float64(nsecs) / 1e6))
	return msecs, nil
}

const (
	// time.UnixNano can only store maxInt64, which is 2262
	maxValidYear = 2262
	minValidYear = 1970
)

// ParseTimeAt parses time s in different formats, assuming the given currentTimestamp.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#timestamp-formats
//
// If s doesn't contain timezone information, then the local timezone is used.
//
// It returns unix timestamp in nanoseconds.
func ParseTimeAt(s string, currentTimestamp int64) (int64, error) {
	if s == "now" {
		return currentTimestamp, nil
	}
	sOrig := s
	tzOffset := int64(0)
	if len(sOrig) > 6 {
		// Try parsing timezone offset
		tz := sOrig[len(sOrig)-6:]
		if (tz[0] == '-' || tz[0] == '+') && tz[3] == ':' {
			isPlus := tz[0] == '+'
			hour, err := strconv.ParseUint(tz[1:3], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("cannot parse hour from timezone offset %q: %w", tz, err)
			}
			minute, err := strconv.ParseUint(tz[4:], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("cannot parse minute from timezone offset %q: %w", tz, err)
			}
			tzOffset = int64(hour*3600+minute*60) * 1e9
			if isPlus {
				tzOffset = -tzOffset
			}
			s = sOrig[:len(sOrig)-6]
		} else {
			if !strings.HasSuffix(s, "Z") {
				tzOffset = -timeutil.GetLocalTimezoneOffsetNsecs()
			} else {
				s = s[:len(s)-1]
			}
		}
	}
	s = strings.TrimSuffix(s, "Z")
	if len(s) > 0 && (s[len(s)-1] > '9' || s[0] == '-') || strings.HasPrefix(s, "now") {
		// Parse duration relative to the current time
		s = strings.TrimPrefix(s, "now")
		d, err := ParseDuration(s)
		if err != nil {
			return 0, err
		}
		if d > 0 {
			d = -d
		}
		return currentTimestamp + int64(d), nil
	}
	if len(s) == 4 {
		// Parse YYYY
		t, err := time.Parse("2006", s)
		if err != nil {
			return 0, err
		}
		y := t.Year()
		if y > maxValidYear || y < minValidYear {
			return 0, fmt.Errorf("cannot parse year from %q: year must in range [%d, %d]", s, minValidYear, maxValidYear)
		}
		return tzOffset + t.UnixNano(), nil
	}
	if !strings.Contains(sOrig, "-") {
		// Parse the timestamp in seconds or in milliseconds
		ts, err := strconv.ParseFloat(sOrig, 64)
		if err != nil {
			return 0, err
		}
		if ts >= (1 << 32) {
			// The timestamp is in milliseconds. Convert it to seconds.
			ts /= 1000
		}
		return int64(math.Round(ts*1e3)) * 1e6, nil
	}
	if len(s) == 7 {
		// Parse YYYY-MM
		t, err := time.Parse("2006-01", s)
		if err != nil {
			return 0, err
		}
		return tzOffset + t.UnixNano(), nil
	}
	if len(s) == 10 {
		// Parse YYYY-MM-DD
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return 0, err
		}
		return tzOffset + t.UnixNano(), nil
	}
	if len(s) == 13 {
		// Parse YYYY-MM-DDTHH
		t, err := time.Parse("2006-01-02T15", s)
		if err != nil {
			return 0, err
		}
		return tzOffset + t.UnixNano(), nil
	}
	if len(s) == 16 {
		// Parse YYYY-MM-DDTHH:MM
		t, err := time.Parse("2006-01-02T15:04", s)
		if err != nil {
			return 0, err
		}
		return tzOffset + t.UnixNano(), nil
	}
	if len(s) == 19 {
		// Parse YYYY-MM-DDTHH:MM:SS
		t, err := time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			return 0, err
		}
		return tzOffset + t.UnixNano(), nil
	}
	// Parse RFC3339
	t, err := time.Parse(time.RFC3339, sOrig)
	if err != nil {
		return 0, err
	}
	return t.UnixNano(), nil
}
