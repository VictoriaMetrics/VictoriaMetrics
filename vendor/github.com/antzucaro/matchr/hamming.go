package matchr

import "errors"

// Hamming computes the Hamming distance between two equal-length strings.
// This is the number of times the two strings differ between characters at
// the same index. This implementation is based off of the algorithm
// description found at http://en.wikipedia.org/wiki/Hamming_distance.
func Hamming(s1 string, s2 string) (distance int, err error) {
	// index by code point, not byte
	r1 := []rune(s1)
	r2 := []rune(s2)

	if len(r1) != len(r2) {
		err = errors.New("Hamming distance of different sized strings.")
		return
	}

	for i, v := range r1 {
		if r2[i] != v {
			distance += 1
		}
	}
	return
}
