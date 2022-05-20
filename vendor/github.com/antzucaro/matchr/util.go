package matchr

import (
	"math"
	"strings"
)

// min of two integers
func min(a int, b int) (res int) {
	if a < b {
		res = a
	} else {
		res = b
	}

	return
}

// max of two integers
func maxI(a int, b int) (res int) {
	if a < b {
		res = b
	} else {
		res = a
	}

	return
}

// max of two float64s
func max(a float64, b float64) (res float64) {
	if a < b {
		res = b
	} else {
		res = a
	}

	return
}

// is this string index outside of the ASCII numeric code points?
func nan(c rune) bool {
	return ((c > 57) || (c < 48))
}

// Round a float64 to the given precision
//
// http://play.golang.org/p/S654PxAe_N
//
// (via Rory McGuire at
// https://groups.google.com/forum/#!topic/golang-nuts/ITZV08gAugI)
func round(x float64, prec int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}

	sign := 1.0
	if x < 0 {
		sign = -1
		x *= -1
	}

	var rounder float64
	pow := math.Pow(10, float64(prec))
	intermed := x * pow
	_, frac := math.Modf(intermed)

	if frac >= 0.5 {
		rounder = math.Ceil(intermed)
	} else {
		rounder = math.Floor(intermed)
	}

	return rounder / pow * sign
}

// A helper to determine if any substrings exist within the given string
func contains(value *String, start int, length int, criteria ...string) bool {
	substring := substring(value, start, length)
	for _, c := range criteria {
		if substring == c {
			return true
		}
	}
	return false
}

// A fault-tolerant version of Slice. It will return nothing ("") if the index
// is out of bounds. This allows substring-ing without having to bound check
// every time.
func substring(value *String, start int, length int) string {
	if start >= 0 && start+length <= value.RuneCount() {
		return value.Slice(start, start+length)
	} else {
		return ""
	}
}

func isVowel(c rune) bool {
	switch c {
	case 'A', 'E', 'I', 'O', 'U', 'Y':
		return true
	default:
		return false
	}
}

func isVowelNoY(c rune) bool {
	switch c {
	case 'A', 'E', 'I', 'O', 'U':
		return true
	default:
		return false
	}
}

func cleanInput(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}
