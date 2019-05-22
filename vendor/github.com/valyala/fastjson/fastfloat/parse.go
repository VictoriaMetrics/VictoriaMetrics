package fastfloat

import (
	"math"
	"strconv"
)

// ParseUint64BestEffort parses uint64 number s.
//
// It is equivalent to strconv.ParseUint(s, 10, 64), but is faster.
//
// 0 is returned if the number cannot be parsed.
func ParseUint64BestEffort(s string) uint64 {
	if len(s) == 0 {
		return 0
	}
	i := uint(0)
	d := uint64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + uint64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for uint64.
				// Fall back to slow parsing.
				dd, err := strconv.ParseUint(s, 10, 64)
				if err != nil {
					return 0
				}
				return dd
			}
			continue
		}
		break
	}
	if i <= j {
		return 0
	}
	if i < uint(len(s)) {
		// Unparsed tail left.
		return 0
	}
	return d
}

// ParseInt64BestEffort parses int64 number s.
//
// It is equivalent to strconv.ParseInt(s, 10, 64), but is faster.
//
// 0 is returned if the number cannot be parsed.
func ParseInt64BestEffort(s string) int64 {
	if len(s) == 0 {
		return 0
	}
	i := uint(0)
	minus := s[0] == '-'
	if minus {
		i++
		if i >= uint(len(s)) {
			return 0
		}
	}

	d := int64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + int64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for int64.
				// Fall back to slow parsing.
				dd, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return 0
				}
				return dd
			}
			continue
		}
		break
	}
	if i <= j {
		return 0
	}
	if i < uint(len(s)) {
		// Unparsed tail left.
		return 0
	}
	if minus {
		d = -d
	}
	return d
}

// ParseBestEffort parses floating-point number s.
//
// It is equivalent to strconv.ParseFloat(s, 64), but is faster.
//
// 0 is returned if the number cannot be parsed.
func ParseBestEffort(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	i := uint(0)
	minus := s[0] == '-'
	if minus {
		i++
		if i >= uint(len(s)) {
			return 0
		}
	}

	d := uint64(0)
	j := i
	for i < uint(len(s)) {
		if s[i] >= '0' && s[i] <= '9' {
			d = d*10 + uint64(s[i]-'0')
			i++
			if i > 18 {
				// The integer part may be out of range for uint64.
				// Fall back to slow parsing.
				f, err := strconv.ParseFloat(s, 64)
				if err != nil && !math.IsInf(f, 0) {
					return 0
				}
				return f
			}
			continue
		}
		break
	}
	if i <= j {
		return 0
	}
	f := float64(d)
	if i >= uint(len(s)) {
		// Fast path - just integer.
		if minus {
			f = -f
		}
		return f
	}

	if s[i] == '.' {
		// Parse fractional part.
		i++
		if i >= uint(len(s)) {
			return 0
		}
		fr := uint64(0)
		j := i
		for i < uint(len(s)) {
			if s[i] >= '0' && s[i] <= '9' {
				fr = fr*10 + uint64(s[i]-'0')
				i++
				if i-j > 18 {
					// The fractional part may be out of range for uint64.
					// Fall back to standard parsing.
					f, err := strconv.ParseFloat(s, 64)
					if err != nil && !math.IsInf(f, 0) {
						return 0
					}
					return f
				}
				continue
			}
			break
		}
		if i <= j {
			return 0
		}
		f += float64(fr) / math.Pow10(int(i-j))
		if i >= uint(len(s)) {
			// Fast path - parsed fractional number.
			if minus {
				f = -f
			}
			return f
		}
	}
	if s[i] == 'e' || s[i] == 'E' {
		// Parse exponent part.
		i++
		if i >= uint(len(s)) {
			return 0
		}
		expMinus := false
		if s[i] == '+' || s[i] == '-' {
			expMinus = s[i] == '-'
			i++
			if i >= uint(len(s)) {
				return 0
			}
		}
		exp := int16(0)
		j := i
		for i < uint(len(s)) {
			if s[i] >= '0' && s[i] <= '9' {
				exp = exp*10 + int16(s[i]-'0')
				i++
				if exp > 300 {
					// The exponent may be too big for float64.
					// Fall back to standard parsing.
					f, err := strconv.ParseFloat(s, 64)
					if err != nil && !math.IsInf(f, 0) {
						return 0
					}
					return f
				}
				continue
			}
			break
		}
		if i <= j {
			return 0
		}
		if expMinus {
			exp = -exp
		}
		f *= math.Pow10(int(exp))
		if i >= uint(len(s)) {
			if minus {
				f = -f
			}
			return f
		}
	}
	return 0
}
