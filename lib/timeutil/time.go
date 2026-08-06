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
	if expIdx := getExpIndex(s); expIdx >= 0 {
		// The timestamp is a scientific number such as 1.234e5
		decimalExp, ok := tryParseInt64(s[expIdx+1:])
		if !ok {
			return 0, false
		}
		n, ok := tryParseScientificUnixTimestamp(s[:expIdx], decimalExp)
		if !ok {
			return 0, false
		}
		return n, true
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
	n, ok := tryParseFractionalUnixTimestamp(intStr, fracStr)
	if !ok {
		return 0, false
	}
	return n, true
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

func tryParseScientificUnixTimestamp(s string, decimalExp int64) (int64, bool) {
	if decimalExp < 0 {
		// Negative exponents on a fractional mantissa are intentionally not
		// supported. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11268
		return 0, false
	}
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		n, ok := tryParseInt64(s)
		if !ok {
			return 0, false
		}
		n, ok = multiplyByDecimalExp(n, decimalExp)
		if !ok {
			return 0, false
		}
		return getUnixTimestampNanoseconds(n), true
	}

	intStr := s[:dotIdx]
	fracStr := s[dotIdx+1:]
	if decimalExp >= int64(len(fracStr)) {
		// The exponent shifts the decimal point past every fractional digit.
		n, ok := tryParseDecimalMantissaAsInt(intStr, fracStr)
		if !ok {
			return 0, false
		}
		decimalExp -= int64(len(fracStr))
		n, ok = multiplyByDecimalExp(n, decimalExp)
		if !ok {
			return 0, false
		}
		return getUnixTimestampNanoseconds(n), true
	}

	// The exponent leaves fractional digits, e.g. 1.784144612388E9 == 1784144612.388
	if decimalExp >= int64(len(decimalMultipliers)) {
		return 0, false
	}
	decimalExpInt := int(decimalExp)
	intStr = s[:dotIdx] + fracStr[:decimalExpInt]
	fracStr = fracStr[decimalExpInt:]
	return tryParseFractionalUnixTimestamp(intStr, fracStr)
}

func tryParseDecimalMantissaAsInt(intStr, fracStr string) (int64, bool) {
	n, ok := tryParseInt64(intStr)
	if !ok {
		return 0, false
	}

	decimalExp := int64(len(fracStr))
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

func tryParseFractionalUnixTimestamp(intStr, fracStr string) (int64, bool) {
	n, ok := tryParseInt64(intStr)
	if !ok {
		return 0, false
	}
	isNegative := n < 0 || n == 0 && strings.HasPrefix(intStr, "-")

	multiplier, maxFracDigits := getUnixTimestampMultiplier(n)
	// Truncate the fractional digits to valid length according to the unit precision.
	if len(fracStr) > maxFracDigits {
		// 1.123456789XXX is invalid.
		tail := fracStr[maxFracDigits:]
		for i := 0; i < len(tail); i++ {
			if tail[i] < '0' || tail[i] > '9' {
				return 0, false
			}
		}
		fracStr = fracStr[:maxFracDigits]
	}
	if len(fracStr) == 0 {
		return n * multiplier, true
	}

	frac, ok := tryParseInt64(fracStr)
	if !ok {
		return 0, false
	}
	decimalExp := len(fracStr)
	if decimalExp >= len(decimalMultipliers) {
		return 0, false
	}
	n *= multiplier
	scale := decimalMultipliers[decimalExp]
	frac *= multiplier / scale

	if isNegative {
		if n < math.MinInt64+frac {
			return 0, false
		}
		return n - frac, true
	}
	if n > math.MaxInt64-frac {
		return 0, false
	}
	return n + frac, true
}

func multiplyByDecimalExp(n int64, decimalExp int64) (int64, bool) {
	if decimalExp < 0 {
		return 0, false
	}
	if decimalExp >= int64(len(decimalMultipliers)) {
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

const (
	maxValidSecond = math.MaxInt64 / 1_000_000_000
	maxValidMilli  = math.MaxInt64 / 1_000_000
	maxValidMicro  = math.MaxInt64 / 1_000
	minValidSecond = math.MinInt64 / 1_000_000_000
	minValidMilli  = math.MinInt64 / 1_000_000
	minValidMicro  = math.MinInt64 / 1_000
)

func getUnixTimestampNanoseconds(n int64) int64 {
	multiplier, _ := getUnixTimestampMultiplier(n)
	return n * multiplier
}

func getUnixTimestampMultiplier(n int64) (int64, int) {
	if n <= maxValidSecond && n >= minValidSecond {
		// The timestamp is in seconds.
		return 1e9, 9
	}
	if n <= maxValidMilli && n >= minValidMilli {
		// The timestamp is in milliseconds.
		return 1e6, 6
	}
	if n <= maxValidMicro && n >= minValidMicro {
		// The timestamp is in microseconds.
		return 1e3, 3
	}
	// The timestamp is in nanoseconds
	return 1, 0
}

func tryParseInt64(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
