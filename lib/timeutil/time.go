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
		if d < 0 {
			d = -d
		}
		return subInt64NoOverflow(currentTimestamp, int64(d)), nil
	}
	if len(s) == 4 {
		// Parse YYYY
		return parseTimeAt("2006", s, tzOffset, sOrig)
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
		return parseTimeAt("2006-01", s, tzOffset, sOrig)
	}
	if len(s) == 10 {
		// Parse YYYY-MM-DD
		return parseTimeAt("2006-01-02", s, tzOffset, sOrig)
	}
	if len(s) == 13 {
		// Parse YYYY-MM-DDTHH
		return parseTimeAt("2006-01-02T15", s, tzOffset, sOrig)
	}
	if len(s) == 16 {
		// Parse YYYY-MM-DDTHH:MM
		return parseTimeAt("2006-01-02T15:04", s, tzOffset, sOrig)
	}
	if len(s) == 19 {
		// Parse YYYY-MM-DDTHH:MM:SS
		return parseTimeAt("2006-01-02T15:04:05", s, tzOffset, sOrig)
	}
	// Parse RFC3339
	return parseTimeAt(time.RFC3339, sOrig, 0, sOrig)
}

func parseTimeAt(layout, value string, tzOffsetNsec int64, sOrig string) (int64, error) {
	t, err := time.Parse(layout, value)
	if err != nil {
		return 0, err
	}
	nsec := t.UnixNano()

	return subInt64NoOverflow(nsec, -tzOffsetNsec), nil
}

func subInt64NoOverflow(a, b int64) int64 {
	if b >= 0 {
		if a < math.MinInt64+b {
			return math.MinInt64
		}
		return a - b
	}

	if a > math.MaxInt64+b {
		return math.MaxInt64
	}
	return a - b
}

// TryParseUnixTimestamp parses s as unix timestamp in seconds, milliseconds, microseconds or nanoseconds and returns the parsed timestamp in nanoseconds.
//
// The supported formats for s:
//
// - Integer. For example, 1234567890
// - Fractional. For example, 1234567890.123
// - Scientific. For example, 1.23456789e9
func TryParseUnixTimestamp(s string) (int64, bool) {
	s, exp, ok := parseExponent(s)
	if !ok {
		return 0, false
	}
	whole, frac, fracExp, ok := parseFraction(s)
	if !ok {
		return 0, false
	}

	// Move decimal point `exp` positions to the right.
	if whole, ok = scale10xNoOverflow(whole, exp); !ok {
		return 0, false
	}
	if exp >= fracExp {
		if frac, ok = scale10xNoOverflow(frac, exp-fracExp); !ok {
			return 0, false
		}
		fracExp = 0
	} else {
		if whole, ok = addNoOverflow(whole, firstDigits(frac, fracExp-exp)); !ok {
			return 0, false
		}
		frac = lastDigits(frac, fracExp-exp)
		fracExp -= exp
	}

	// Move decimal point `tsExp` positions to the right.
	tsExp := getUnixTimestampExponent(whole)
	if whole, ok = scale10xNoOverflow(whole, tsExp); !ok {
		return 0, false
	}
	if tsExp >= fracExp {
		if frac, ok = scale10xNoOverflow(frac, tsExp-fracExp); !ok {
			return 0, false
		}
	} else {
		frac = firstDigits(frac, fracExp-tsExp)
	}

	return addNoOverflow(whole, frac)
}

func parseExponent(s string) (string, int, bool) {
	i := getExponentIndex(s)
	if i == -1 {
		return s, 0, true
	}

	exp, ok := tryParseInt64(s[i+1:])
	if !ok {
		return "", 0, false
	}

	if exp < 0 || maxExponent < exp {
		return "", 0, false
	}

	return s[:i], int(exp), true
}

func getExponentIndex(s string) int {
	if n := strings.IndexByte(s, 'e'); n >= 0 {
		return n
	}
	if n := strings.IndexByte(s, 'E'); n >= 0 {
		return n
	}
	return -1
}

// TODO: check fraction contains only digits (add test)
// TODO: truncate to max 18 digits first, then remove trailing zeroes
func parseFraction(s string) (whole int64, frac int64, fracExp int, ok bool) {
	if len(s) == 0 || s == "." {
		return 0, 0, 0, false
	}
	var negative bool
	if strings.HasPrefix(s, "-") {
		s = s[1:]
		negative = true
	}

	var wholeStr, fracStr string
	i := strings.IndexByte(s, '.')
	if i == -1 {
		wholeStr = s
	} else if i == 0 {
		fracStr = s[i+1:]
	} else if i == len(s)-1 {
		wholeStr = s[:i]
	} else {
		wholeStr = s[:i]
		fracStr = s[i+1:]
	}

	fracStr = strings.TrimRight(fracStr, "0")
	fracExp = maxExponent
	if len(fracStr) < fracExp {
		fracExp = len(fracStr)
	}
	fracStr = fracStr[0:fracExp]

	if len(wholeStr) > 0 {
		whole, ok = tryParseInt64(wholeStr)
		if !ok {
			return 0, 0, 0, false
		}
	}
	if len(fracStr) > 0 {
		frac, ok = tryParseInt64(fracStr)
		if !ok {
			return 0, 0, 0, false
		}
	}
	if negative {
		whole = -whole
		frac = -frac
	}
	return whole, frac, fracExp, true
}

func tryParseInt64(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func addNoOverflow(a, b int64) (int64, bool) {
	if a > 0 && b > 0 && a > math.MaxInt64-b {
		return 0, false
	}
	if a < 0 && b < 0 && a < math.MinInt64-b {
		return 0, false
	}
	return a + b, true
}

func firstDigits(i int64, n int) int64 {
	return i / decimalMultipliers[n]
}

func lastDigits(i int64, n int) int64 {
	return i % decimalMultipliers[n]
}

func scale10xNoOverflow(n int64, exp int) (int64, bool) {
	m := decimalMultipliers[exp]
	if n >= 0 && n > math.MaxInt64/m || n < 0 && n < math.MinInt64/m {
		return 0, false
	}
	return n * m, true
}

const maxExponent = 18

var decimalMultipliers = [...]int64{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18}

func getUnixTimestampExponent(n int64) int {
	if n <= maxValidSecond && n >= minValidSecond {
		// The timestamp is in seconds.
		return 9
	}
	if n <= maxValidMilli && n >= minValidMilli {
		// The timestamp is in milliseconds.
		return 6
	}
	if n <= maxValidMicro && n >= minValidMicro {
		// The timestamp is in microseconds.
		return 3
	}
	// The timestamp is in nanoseconds
	return 0
}

const (
	maxValidSecond = math.MaxInt64 / 1_000_000_000
	maxValidMilli  = math.MaxInt64 / 1_000_000
	maxValidMicro  = math.MaxInt64 / 1_000
	minValidSecond = math.MinInt64 / 1_000_000_000
	minValidMilli  = math.MinInt64 / 1_000_000
	minValidMicro  = math.MinInt64 / 1_000
)
