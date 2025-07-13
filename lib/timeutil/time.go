package timeutil

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ParseTimeMsec parses time s in different formats.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#timestamp-formats
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
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#timestamp-formats
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
				tzOffset = -GetLocalTimezoneOffsetNsecs()
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
		nsec, ok := TryParseUnixTimestamp(sOrig)
		if !ok {
			return 0, fmt.Errorf("cannot parse numeric timestamp %q", sOrig)
		}
		return nsec, nil
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

// TryParseUnixTimestamp parses s as unix timestamp in seconds, milliseconds, microseconds or nanoseconds and returns the parsed timestamp in nanoseconds.
//
// The supported formats for s:
//
// - Integer. For example, 1234567890
// - Fractional. For example, 1234567890.123
// - Scientific. For example, 1.23456789e9
func TryParseUnixTimestamp(s string) (int64, bool) {
	if expIdx := getExpIndex(s); expIdx >= 0 {
		// The timestamp is a scientific number such as 1.234e5
		decimalExp, ok := tryParseInt64(s[expIdx+1:])
		if !ok {
			return 0, false
		}
		if decimalExp > math.MaxInt64 || decimalExp < math.MinInt64 {
			return 0, false
		}
		n, ok := tryParseScientificNumberForUnixTimestamp(s[:expIdx], int(decimalExp))
		if !ok {
			return 0, false
		}
		return getUnixTimestampNanoseconds(n), true
	}

	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		// The timestamp is integer.
		n, ok := tryParseInt64(s)
		if !ok {
			return 0, false
		}
		return getUnixTimestampNanoseconds(n), true
	}

	// The timestamp is fractional.
	intStr := s[:dotIdx]
	fracStr := s[dotIdx+1:]
	n, ok := tryParseFractionalNumberForUnixTimestamp(intStr, fracStr)
	if !ok {
		return 0, false
	}

	// Adjust the n to multiples of thousands, since this is expected by getUnixTimestampNanoseconds.
	decimalExp := len(fracStr)
	for decimalExp%3 != 0 {
		if n >= 0 && n > math.MaxInt64/10 || n < 0 && n < math.MinInt64/10 {
			return 0, false
		}
		n *= 10
		decimalExp++
	}

	return getUnixTimestampNanoseconds(n), true
}

func getExpIndex(s string) int {
	if n := strings.IndexByte(s, 'e'); n >= 0 {
		return n
	}
	if n := strings.IndexByte(s, 'E'); n >= 0 {
		return n
	}
	return -1
}

func tryParseScientificNumberForUnixTimestamp(s string, decimalExp int) (int64, bool) {
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		n, ok := tryParseInt64(s)
		if !ok {
			return 0, false
		}
		return multiplyByDecimalExp(n, decimalExp)
	}

	intStr := s[:dotIdx]
	fracStr := s[dotIdx+1:]
	if decimalExp < len(fracStr) {
		return 0, false
	}
	n, ok := tryParseFractionalNumberForUnixTimestamp(intStr, fracStr)
	if !ok {
		return 0, false
	}
	decimalExp -= len(fracStr)
	return multiplyByDecimalExp(n, decimalExp)
}

func tryParseFractionalNumberForUnixTimestamp(intStr, fracStr string) (int64, bool) {
	n, ok := tryParseInt64(intStr)
	if !ok {
		return 0, false
	}

	decimalExp := len(fracStr)
	num, ok := multiplyByDecimalExp(n, decimalExp)
	if !ok {
		return 0, false
	}

	frac, ok := tryParseInt64(fracStr)
	if !ok {
		return 0, false
	}

	if num >= 0 {
		if num > math.MaxInt64-frac {
			return 0, false
		}
		num += frac
	} else {
		if num < math.MinInt64+frac {
			return 0, false
		}
		num -= frac
	}

	return num, true
}

func multiplyByDecimalExp(n int64, decimalExp int) (int64, bool) {
	if decimalExp < 0 {
		return 0, false
	}
	if decimalExp >= len(decimalMultipliers) {
		return 0, false
	}
	if decimalExp == 0 {
		return n, true
	}

	m := decimalMultipliers[decimalExp]

	if n >= 0 && n > math.MaxInt64/m || n < 0 && n < math.MinInt64/m {
		return 0, false
	}

	return n * m, true
}

var decimalMultipliers = [...]int64{0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18}

func getUnixTimestampNanoseconds(n int64) int64 {
	if n < (1<<31) && n >= (-1<<31) {
		// The timestamp is in seconds.
		return n * 1e9
	}
	if n < 1e3*(1<<31) && n >= 1e3*(-1<<31) {
		// The timestamp is in milliseconds.
		return n * 1e6
	}
	if n < 1e6*(1<<31) && n >= 1e6*(-1<<31) {
		// The timestamp is in microseconds.
		return n * 1e3
	}
	// The timestamp is in nanoseconds
	return n
}

func tryParseInt64(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
