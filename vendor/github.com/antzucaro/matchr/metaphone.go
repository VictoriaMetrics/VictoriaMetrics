package matchr

import (
	"bytes"
	"strings"
)

type metaphoneresult struct {
	// the maximum number of code values to calculate
	maxLength int

	// whether to calculate an alternate
	calcAlternate bool

	// no direct modifications - only through add()
	primary   bytes.Buffer
	alternate bytes.Buffer

	// length of the private buffers
	PrimaryLength   int
	AlternateLength int
}

func newMetaphoneresult(maxLength int, calcAlternate bool) (r *metaphoneresult) {
	r = &metaphoneresult{maxLength: maxLength, calcAlternate: calcAlternate}
	return
}

func (r *metaphoneresult) add(c1 string, c2 string) {
	if c1 != "" {
		r.primary.WriteString(c1)
		r.PrimaryLength += len(c1)
	}

	if c2 != "" && r.calcAlternate {
		r.alternate.WriteString(c2)
		r.AlternateLength += len(c2)
	}
}

func (r *metaphoneresult) isComplete() bool {
	return r.PrimaryLength >= r.maxLength && r.AlternateLength >= r.maxLength
}

func (r *metaphoneresult) result() (primary string, alternate string) {
	primary = r.primary.String()
	if len(primary) > r.maxLength {
		primary = primary[0:r.maxLength]
	}
	alternate = r.alternate.String()
	if len(alternate) > r.maxLength {
		alternate = alternate[0:r.maxLength]
	}
	return
}

// utility functions for checking things within a string
func isSlavoGermanic(value string) bool {
	return strings.Contains(value, "W") || strings.Contains(value, "K") ||
		strings.Contains(value, "CZ") || strings.Contains(value, "WITZ")
}

func isSilentStart(input runestring) bool {
	SILENT_START := [...]string{"GN", "KN", "PN", "WR", "PS"}

	prefix := input.SafeSubstr(0, 2)

	for _, criteria := range SILENT_START {
		if prefix == criteria {
			return true
		}
	}

	return false
}

func handleVowel(result *metaphoneresult, index int) int {
	if index == 0 {
		result.add("A", "A")
	}

	return index + 1
}

/******************************************************************************
 * Entry handlers for letters.
 *****************************************************************************/
func handleC(input runestring, result *metaphoneresult, index int) int {
	if conditionC0(input, index) {
		result.add("K", "K")
		index += 2
	} else if index == 0 && input.Contains(index, 6, "CAESAR") {
		result.add("S", "S")
		index += 2
	} else if input.Contains(index, 2, "CH") {
		index = handleCH(input, result, index)
	} else if input.Contains(index, 2, "CZ") &&
		!input.Contains(index-2, 4, "WICZ") {
		result.add("S", "X")
		index += 2
	} else if input.Contains(index+1, 3, "CIA") {
		result.add("X", "X")
		index += 3
	} else if input.Contains(index, 2, "CC") &&
		!(index == 1 && input.SafeAt(0) == 'M') {
		return handleCC(input, result, index)
	} else if input.Contains(index, 2, "CK") ||
		input.Contains(index, 2, "CG") ||
		input.Contains(index, 2, "CQ") {
		result.add("K", "K")
		index += 2
	} else if input.Contains(index, 2, "CI") ||
		input.Contains(index, 2, "CE") ||
		input.Contains(index, 2, "CY") {
		if input.Contains(index, 3, "CIO") ||
			input.Contains(index, 3, "CIE") ||
			input.Contains(index, 3, "CIA") {
			result.add("S", "X")
		} else {
			result.add("S", "S")
		}
		index += 2
	} else {
		result.add("K", "K")
		if input.Contains(index+1, 2, " C") ||
			input.Contains(index+1, 2, " Q") ||
			input.Contains(index+1, 2, " G") {
			index += 3
		} else if (input.Contains(index+1, 1, "C") ||
			input.Contains(index+1, 1, "K") ||
			input.Contains(index+1, 1, "Q")) &&
			!(input.Contains(index+1, 2, "CE") ||
				input.Contains(index+1, 2, "CI")) {
			index += 2
		} else {
			index++
		}
	}

	return index
}

func handleCC(input runestring, result *metaphoneresult, index int) int {
	if input.Contains(index+2, 1, "I", "E", "H") &&
		!input.Contains(index+2, 2, "HU") {
		if (index == 1 && input.SafeAt(index-1) == 'A') ||
			(input.Contains(index-1, 5, "UCCEE", "UCCES")) {
			result.add("KS", "KS")
		} else {
			result.add("X", "X")
		}
		index += 3
	} else {
		result.add("K", "K")
		index += 2
	}
	return index
}

func handleCH(input runestring, result *metaphoneresult, index int) int {
	if index > 0 && input.Contains(index, 4, "CHAE") {
		result.add("K", "X")
		return index + 2
	} else if conditionCH0(input, index) {
		result.add("K", "K")
		return index + 2
		// TODO: combine this condition with the one above?
	} else if conditionCH1(input, index) {
		result.add("K", "K")
		return index + 2
	} else {
		if index > 0 {
			if input.Contains(0, 2, "MC") {
				result.add("K", "K")
			} else {
				result.add("X", "K")
			}
		} else {
			result.add("X", "X")
		}
		return index + 2
	}
}

func handleD(input runestring, result *metaphoneresult, index int) int {
	if input.Contains(index, 2, "DG") {
		if input.Contains(index+2, 1, "I", "E", "Y") {
			result.add("J", "J")
			index += 3
		} else {
			result.add("TK", "TK")
			index += 2
		}
	} else if input.Contains(index, 2, "DT", "DD") {
		result.add("T", "T")
		index += 2
	} else {
		result.add("T", "T")
		index++
	}
	return index
}

func handleG(input runestring, result *metaphoneresult, index int, slavoGermanic bool) int {
	if input.SafeAt(index+1) == 'H' {
		index = handleGH(input, result, index)
	} else if input.SafeAt(index+1) == 'N' {
		if index == 1 && isVowel(input.SafeAt(0)) && !slavoGermanic {
			result.add("KN", "N")
		} else if !input.Contains(index+2, 2, "EY") && input.SafeAt(index+1) != 'Y' && !slavoGermanic {
			result.add("N", "KN")
		} else {
			result.add("KN", "KN")
		}
		index += 2
	} else if input.Contains(index+1, 2, "LI") && !slavoGermanic {
		result.add("KL", "L")
		index += 2
	} else if index == 0 && (input.SafeAt(index+1) == 'Y' ||
		input.Contains(index+1, 2, "ES", "EP", "EB", "EL", "EY", "IB", "IL", "IN", "IE", "EI", "ER")) {
		result.add("K", "J")
		index += 2
	} else if (input.Contains(index+1, 2, "ER") ||
		input.SafeAt(index+1) == 'Y') &&
		!input.Contains(0, 6, "DANGER", "RANGER", "MANGER") &&
		!input.Contains(index-1, 1, "E", "I") &&
		!input.Contains(index-1, 3, "RGY", "OGY") {
		result.add("K", "J")
		index += 2
	} else if input.Contains(index+1, 1, "E", "I", "Y") ||
		input.Contains(index-1, 4, "AGGI", "OGGI") {
		if input.Contains(0, 4, "VAN ", "VON ") ||
			input.Contains(0, 3, "SCH") ||
			input.Contains(index+1, 2, "ET") {
			result.add("K", "K")
		} else if input.Contains(index+1, 3, "IER") {
			result.add("J", "J")
		} else {
			result.add("J", "K")
		}
		index += 2
	} else if input.SafeAt(index+1) == 'G' {
		result.add("K", "K")
		index += 2
	} else {
		result.add("K", "K")
		index++
	}
	return index
}

func handleGH(input runestring, result *metaphoneresult, index int) int {
	if index > 0 && !isVowel(input.SafeAt(index-1)) {
		result.add("K", "K")
		index += 2
	} else if index == 0 {
		if input.SafeAt(index+2) == 'I' {
			result.add("J", "J")
		} else {
			result.add("K", "K")
		}
		index += 2
	} else if (index > 1 && input.Contains(index-2, 1, "B", "H", "D")) ||
		(index > 2 && input.Contains(index-3, 1, "B", "H", "D")) ||
		(index > 3 && input.Contains(index-4, 1, "B", "H")) {
		index += 2
	} else {
		if index > 2 && input.SafeAt(index-1) == 'U' &&
			input.Contains(index-3, 1, "C", "G", "L", "R", "T") {
			result.add("F", "F")
		} else if index > 0 && input.SafeAt(index-1) != 'I' {
			result.add("K", "K")
		}
		index += 2
	}
	return index
}

func handleH(input runestring, result *metaphoneresult, index int) int {
	if (index == 0 || isVowel(input.SafeAt(index-1))) &&
		isVowel(input.SafeAt(index+1)) {
		result.add("H", "H")
		index += 2
	} else {
		index++
	}
	return index
}

func handleJ(input runestring, result *metaphoneresult, index int, slavoGermanic bool) int {
	if input.Contains(index, 4, "JOSE") || input.Contains(0, 4, "SAN ") {
		if (index == 0 && (input.SafeAt(index+4) == ' ') ||
			len(input) == 4) || input.Contains(0, 4, "SAN ") {
			result.add("H", "H")
		} else {
			result.add("J", "H")
		}
		index++
	} else {
		if index == 0 && !input.Contains(index, 4, "JOSE") {
			result.add("J", "A")
		} else if isVowel(input.SafeAt(index-1)) && !slavoGermanic &&
			(input.SafeAt(index+1) == 'A' || input.SafeAt(index+1) == 'O') {
			result.add("J", "H")
		} else if index == (len(input) - 1) {
			result.add("J", " ")
		} else if !input.Contains(index+1, 1,
			"L", "T", "K", "S", "N", "M", "B", "Z") &&
			!input.Contains(index-1, 1, "S", "K", "L") {
			result.add("J", "J")
		}

		if input.SafeAt(index+1) == 'J' {
			index += 2
		} else {
			index++
		}
	}
	return index
}

func handleL(input runestring, result *metaphoneresult, index int) int {
	if input.SafeAt(index+1) == 'L' {
		if conditionL0(input, index) {
			result.add("L", "")
		} else {
			result.add("L", "L")
		}
		index += 2
	} else {
		result.add("L", "L")
		index++
	}
	return index
}

func handleP(input runestring, result *metaphoneresult, index int) int {
	if input.SafeAt(index+1) == 'H' {
		result.add("F", "F")
		index += 2
	} else {
		result.add("P", "P")
		if input.Contains(index+1, 1, "P", "B") {
			index += 2
		} else {
			index++
		}
	}
	return index
}

func handleR(input runestring, result *metaphoneresult, index int, slavoGermanic bool) int {
	if index == (len(input)-1) && !slavoGermanic &&
		input.Contains(index-2, 2, "IE") &&
		!input.Contains(index-4, 2, "ME", "MA") {
		result.add("", "R")
	} else {
		result.add("R", "R")
	}

	if input.SafeAt(index+1) == 'R' {
		index += 2
	} else {
		index++
	}
	return index
}

func handleS(input runestring, result *metaphoneresult, index int, slavoGermanic bool) int {
	if input.Contains(index-1, 3, "ISL", "YSL") {
		index++
	} else if index == 0 && input.Contains(index, 5, "SUGAR") {
		result.add("X", "S")
		index++
	} else if input.Contains(index, 2, "SH") {
		if input.Contains(index+1, 4, "HEIM", "HOEK", "HOLM", "HOLZ") {
			result.add("S", "S")
		} else {
			result.add("X", "X")
		}
		index += 2
	} else if input.Contains(index, 3, "SIO", "SIA") ||
		input.Contains(index, 4, "SIAN") {
		if slavoGermanic {
			result.add("S", "S")
		} else {
			result.add("S", "X")
		}
		index += 3
	} else if (index == 0 && input.Contains(index+1, 1, "M", "N", "L", "W")) ||
		input.Contains(index+1, 1, "Z") {
		result.add("S", "X")
		if input.Contains(index+1, 1, "Z") {
			index += 2
		} else {
			index++
		}
	} else if input.Contains(index, 2, "SC") {
		index = handleSC(input, result, index)
	} else {
		if index == len(input)-1 &&
			input.Contains(index-2, 2, "AI", "OI") {
			result.add("", "S")
		} else {
			result.add("S", "S")
		}

		if input.Contains(index+1, 1, "S", "Z") {
			index += 2
		} else {
			index++
		}
	}
	return index
}

func handleSC(input runestring, result *metaphoneresult, index int) int {
	if input.SafeAt(index+2) == 'H' {
		if input.Contains(index+3, 2, "OO", "ER", "EN", "UY", "ED", "EM") {
			if input.Contains(index+3, 2, "ER", "EN") {
				result.add("X", "SK")
			} else {
				result.add("SK", "SK")
			}
		} else {
			if index == 0 && !isVowel(input.SafeAt(3)) && input.SafeAt(3) != 'W' {
				result.add("X", "S")
			} else {
				result.add("X", "X")
			}
		}
	} else if input.Contains(index+2, 1, "I", "E", "Y") {
		result.add("S", "S")
	} else {
		result.add("SK", "SK")
	}
	index += 3

	return index
}

func handleT(input runestring, result *metaphoneresult, index int) int {
	if input.Contains(index, 4, "TION") {
		result.add("X", "X")
		index += 3
	} else if input.Contains(index, 3, "TIA", "TCH") {
		result.add("X", "X")
		index += 3
	} else if input.Contains(index, 2, "TH") || input.Contains(index, 3, "TTH") {
		if input.Contains(index+2, 2, "OM", "AM") ||
			input.Contains(0, 4, "VAN ", "VON ") ||
			input.Contains(0, 3, "SCH") {
			result.add("T", "T")
		} else {
			result.add("0", "T")
		}
		index += 2
	} else {
		result.add("T", "T")
		if input.Contains(index+1, 1, "T", "D") {
			index += 2
		} else {
			index++
		}
	}
	return index
}

func handleW(input runestring, result *metaphoneresult, index int) int {
	if input.Contains(index, 2, "WR") {
		result.add("R", "R")
		index += 2
	} else {
		if index == 0 && (isVowel(input.SafeAt(index+1)) ||
			input.Contains(index, 2, "WH")) {
			if isVowel(input.SafeAt(index + 1)) {
				result.add("A", "F")
			} else {
				result.add("A", "A")
			}
			index++
		} else if (index == len(input)-1 && isVowel(input.SafeAt(index-1))) ||
			input.Contains(index-1, 5, "EWSKI", "EWSKY", "OWSKI", "OWSKY") ||
			input.Contains(0, 3, "SCH") {
			result.add("", "F")
			index++
		} else if input.Contains(index, 4, "WICZ", "WITZ") {
			result.add("TS", "FX")
			index += 4
		} else {
			index++
		}
	}
	return index
}

func handleX(input runestring, result *metaphoneresult, index int) int {
	if index == 0 {
		result.add("S", "S")
		index++
	} else {
		if !((index == len(input)-1) &&
			(input.Contains(index-3, 3, "IAU", "EAU") ||
				input.Contains(index-2, 2, "AU", "OU"))) {
			result.add("KS", "KS")
		}

		if input.Contains(index+1, 1, "C", "X") {
			index += 2
		} else {
			index++
		}
	}
	return index
}

func handleZ(input runestring, result *metaphoneresult, index int, slavoGermanic bool) int {
	if input.SafeAt(index+1) == 'H' {
		result.add("J", "J")
	} else {
		if input.Contains(index+1, 2, "ZO", "ZI", "ZA") ||
			(slavoGermanic && (index > 0 && input.SafeAt(index-1) != 'T')) {
			result.add("S", "TS")
		} else {
			result.add("S", "S")
		}
	}

	if input.SafeAt(index+1) == 'Z' {
		index += 2
	} else {
		index++
	}
	return index
}

/******************************************************************************
 * Complex conditional handlers for letters
 *****************************************************************************/
func conditionC0(input runestring, index int) bool {
	if input.Contains(index, 4, "CHIA") {
		return true
	} else if index <= 1 {
		return false
	} else if isVowel(input.SafeAt(index - 2)) {
		return false
	} else if !input.Contains(index-1, 3, "ACH") {
		return false
	} else {
		c := input.SafeAt(index + 2)
		return (c != 'I' && c != 'E') ||
			(input.Contains(index-2, 6, "BACHER") ||
				input.Contains(index-2, 6, "MACHER"))
	}
}

func conditionCH0(input runestring, index int) bool {
	if index != 0 {
		return false
	} else if !input.Contains(index+1, 5, "HARAC", "HARIS") &&
		!input.Contains(index+1, 3, "HOR", "HYM", "HIA", "HEM") {
		return false
	} else if input.Contains(0, 5, "CHORE") {
		return false
	} else {
		return true
	}
}

func conditionCH1(input runestring, index int) bool {
	// good god this is ugly
	return (input.Contains(0, 4, "VAN ", "VON ") || input.Contains(0, 3, "SCH")) ||
		input.Contains(index-2, 6, "ORCHES", "ARCHIT", "ORCHID") ||
		input.Contains(index+2, 1, "T", "S") ||
		((input.Contains(index-1, 1, "A", "O", "U", "E") || index == 0) &&
			(input.Contains(index+2, 1, "L", "R", "N", "M", "B", "H", "F", "V", "W", " ") ||
				index+1 == len(input)-1))
}

func conditionL0(input runestring, index int) bool {
	if index == (len(input)-3) &&
		input.Contains(index-1, 4, "ILLO", "ILLA", "ALLE") {
		return true
	} else if (input.Contains(len(input)-2, 2, "AS", "OS") ||
		input.Contains(len(input)-1, 1, "A", "O")) &&
		(input.Contains(index-1, 4, "ALLE")) {
		return true
	} else {
		return false
	}
}

func conditionM0(input runestring, index int) bool {
	if input.SafeAt(index+1) == 'M' {
		return true
	}

	return input.Contains(index-1, 3, "UMB") &&
		((index+1) == (len(input)-1) ||
			input.Contains(index+2, 2, "ER"))
}

// DoubleMetaphone computes the Double-Metaphone value of the input string.
// This value is a phonetic representation of how the string sounds, with
// affordances for many different language dialects. It was originally
// developed by Lawrence Phillips in the 1990s.
//
// More information about this algorithm can be found on Wikipedia at
// http://en.wikipedia.org/wiki/Metaphone.
func DoubleMetaphone(s1 string) (string, string) {
	// trim, upper space
	s1 = cleanInput(s1)

	// structure to traverse the string by code point, not byte
	input := runestring(s1)

	slavoGermanic := isSlavoGermanic(s1)

	// where we are in the string
	index := 0

	if isSilentStart(input) {
		index += 1
	}

	result := newMetaphoneresult(4, true)

	for !result.isComplete() && index <= len(input)-1 {
		c := rune(input.SafeAt(index))
		switch c {
		case 'A', 'E', 'I', 'O', 'U', 'Y':
			index = handleVowel(result, index)
		case 'B':
			result.add("P", "P")
			if input.SafeAt(index+1) == 'B' {
				index += 2
			} else {
				index++
			}
		case 'Ç':
			result.add("S", "S")
			index++
		case 'C':
			index = handleC(input, result, index)
		case 'D':
			index = handleD(input, result, index)
		case 'F':
			result.add("F", "F")
			if input.SafeAt(index+1) == 'F' {
				index += 2
			} else {
				index++
			}
		case 'G':
			index = handleG(input, result, index, slavoGermanic)
		case 'H':
			index = handleH(input, result, index)
		case 'J':
			index = handleJ(input, result, index, slavoGermanic)
		case 'K':
			result.add("K", "K")
			if input.SafeAt(index+1) == 'K' {
				index += 2
			} else {
				index++
			}
		case 'L':
			index = handleL(input, result, index)
		case 'M':
			result.add("M", "M")
			if conditionM0(input, index) {
				index += 2
			} else {
				index++
			}
		case 'N':
			result.add("N", "N")
			if input.SafeAt(index+1) == 'N' {
				index += 2
			} else {
				index++
			}
		case 'Ñ':
			result.add("N", "N")
			index++
		case 'P':
			index = handleP(input, result, index)
		case 'Q':
			result.add("K", "K")
			if input.SafeAt(index+1) == 'Q' {
				index += 2
			} else {
				index++
			}
		case 'R':
			index = handleR(input, result, index, slavoGermanic)
		case 'S':
			index = handleS(input, result, index, slavoGermanic)
		case 'T':
			index = handleT(input, result, index)
		case 'V':
			result.add("F", "F")
			if input.SafeAt(index+1) == 'V' {
				index += 2
			} else {
				index++
			}
		case 'W':
			index = handleW(input, result, index)
		case 'X':
			index = handleX(input, result, index)
		case 'Z':
			index = handleZ(input, result, index, slavoGermanic)
		default:
			index++
		}

	}

	return result.result()
}
