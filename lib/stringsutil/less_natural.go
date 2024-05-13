package stringsutil

import (
	"math"
)

// LessNatural returns true if a is less than b using natural sort comparison.
//
// See https://en.wikipedia.org/wiki/Natural_sort_order
func LessNatural(a, b string) bool {
	isReverse := false
	for {
		if len(a) > len(b) {
			a, b = b, a
			isReverse = !isReverse
		}

		// Skip common prefix except of decimal digits
		i := 0
		for i < len(a) {
			cA := a[i]
			cB := b[i]

			if cA >= '0' && cA <= '9' {
				if cB >= '0' && cB <= '9' {
					break
				}
				return !isReverse
			}
			if cB >= '0' && cB <= '9' {
				return isReverse
			}
			if cA != cB {
				// This should work properly for utf8 bytes in the middle of encoded unicode char, since:
				// - utf8 bytes for multi-byte chars are bigger than decimal digit chars
				// - sorting of utf8-encoded strings works properly thanks to utf8 properties
				if isReverse {
					return cB < cA
				}
				return cA < cB
			}

			i++
		}
		a = a[i:]
		b = b[i:]
		if len(a) == 0 {
			if isReverse {
				return false
			}
			return len(b) > 0
		}

		// Collect digit prefixes for a and b and then compare them.

		iA := 1
		nA := uint64(a[0] - '0')
		for iA < len(a) {
			c := a[iA]
			if c < '0' || c > '9' {
				break
			}
			if nA > (math.MaxUint64-9)/10 {
				// Too big integer. Fall back to string comparison
				if isReverse {
					return b < a
				}
				return a < b
			}
			nA *= 10
			nA += uint64(c - '0')
			iA++
		}

		iB := 1
		nB := uint64(b[0] - '0')
		for iB < len(b) {
			c := b[iB]
			if c < '0' || c > '9' {
				break
			}
			if nB > (math.MaxUint64-9)/10 {
				// Too big integer. Fall back to string comparison
				if isReverse {
					return b < a
				}
				return a < b
			}
			nB *= 10
			nB += uint64(c - '0')
			iB++
		}

		if nA != nB {
			if isReverse {
				return nB < nA
			}
			return nA < nB
		}

		if iA != iB {
			if isReverse {
				return iB < iA
			}
			return iA < iB
		}

		a = a[iA:]
		b = b[iB:]
	}
}
