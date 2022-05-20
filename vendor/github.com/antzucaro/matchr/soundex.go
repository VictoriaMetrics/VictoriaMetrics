package matchr

import "strings"

// Soundex computes the Soundex phonetic representation of the input string. It
// attempts to encode homophones with the same characters. More information can
// be found at http://en.wikipedia.org/wiki/Soundex.
func Soundex(s1 string) string {
	if len(s1) == 0 {
		return ""
	}

	// we should work with all uppercase
	s1 = strings.ToUpper(s1)

	input := NewString(s1)

	// the encoded value
	enc := input.Slice(0, 1)

	c := ""
	prev := ""
	hw := false

	for i := 0; i < input.RuneCount(); i++ {
		switch rune(input.At(i)) {
		case 'B', 'F', 'P', 'V':
			c = "1"
		case 'C', 'G', 'J', 'K', 'Q', 'S', 'X', 'Z':
			c = "2"
		case 'D', 'T':
			c = "3"
		case 'L':
			c = "4"
		case 'M', 'N':
			c = "5"
		case 'R':
			c = "6"
		case 'H', 'W':
			hw = true
		default:
			c = ""
		}

		// don't encode the first position, but we need its code value
		// to prevent repeats
		if c != "" && c != prev && i > 0 {
			// if the next encoded digit is different, we can add it right away
			// if it is the same, though, it must not have been preceded
			// by an 'H' or a 'W'
			if enc[len(enc)-1:len(enc)] != c || !hw {
				enc = enc + c
			}

			// we're done when we reach four encoded characters
			if len(enc) == 4 {
				break
			}
		}

		prev = c
		hw = false
	}

	// if we've fallen short of 4 "real" encoded characters,
	// it gets padded with zeros
	for len(enc) < 4 {
		enc = enc + "0"
	}

	return enc
}
