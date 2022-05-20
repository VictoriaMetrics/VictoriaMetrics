package matchr

func preProcess(input []rune) []rune {
	output := runestring(make([]rune, 0, len(input)))

	// 0. Remove all non-ASCII characters
	for _, v := range input {
		if v >= 65 && v <= 90 {
			output = append(output, v)
		}
	}

	// 1. Remove all trailing 'S' characters at the end of the name
	for i := len(output) - 1; i >= 0 && output[i] == 'S'; i-- {
		output.Del(i)
	}

	// 2. Convert leading letter pairs as follows
	//    KN -> N, PH -> F, WR -> R
	switch output.SafeSubstr(0, 2) {
	case "KN":
		output = output[1:]
	case "PH":
		output[0] = 'F' // H will be ignored anyway
	case "WR":
		output = output[1:]
	}

	// 3a. Convert leading single letters as follows:
	//    H         -> Remove
	if output.SafeAt(0) == 'H' {
		output = output[1:]
	}

	// 3a. Convert leading single letters as follows:
	//    E,I,O,U,Y -> A
	//    P         -> B
	//    V         -> F
	//    K,Q       -> C
	//    J         -> G
	//    Z         -> S
	switch output.SafeAt(0) {
	case 'E', 'I', 'O', 'U', 'Y':
		output[0] = 'A'
	case 'P':
		output[0] = 'B'
	case 'V':
		output[0] = 'F'
	case 'K', 'Q':
		output[0] = 'C'
	case 'J':
		output[0] = 'G'
	case 'Z':
		output[0] = 'S'
	}

	return output
}

// Phonex computes the Phonex phonetic encoding of the input string. Phonex is
// a modification of the venerable Soundex algorithm. It accounts for a few
// more letter combinations to improve accuracy on some data sets.
//
// This implementation is based off of the original C implementation by the
// creator - A. J. Lait - as found in his research paper entitled "An
// Assessment of Name Matching Algorithms."
func Phonex(s1 string) string {

	// preprocess
	s1 = cleanInput(s1)

	input := runestring(preProcess([]rune(s1)))

	result := make([]rune, 0, len(input))

	last := rune(0)
	code := rune(0)
	for i := 0; i < len(input) &&
		input[i] != ' ' &&
		input[i] != ',' &&
		len(result) < 4; i++ {
		switch input[i] {
		case 'B', 'P', 'F', 'V':
			code = '1'
		case 'C', 'S', 'K', 'G', 'J', 'Q', 'X', 'Z':
			code = '2'
		case 'D', 'T':
			if input.SafeAt(i+1) != 'C' {
				code = '3'
			}
		case 'L':
			if isVowel(input.SafeAt(i+1)) || i == len(input)-1 {
				code = '4'
			}
		case 'M', 'N':
			nextChar := input.SafeAt(i + 1)
			if nextChar == 'D' || nextChar == 'G' {
				// ignore next character
				i++
			}
			code = '5'
		case 'R':
			if isVowel(input.SafeAt(i+1)) || i == len(input)-1 {
				code = '6'
			}
		default:
			code = 0
		}

		if last != code && code != 0 && i != 0 {
			result = append(result, code)
		}

		// special case for 1st character: we use the actual character
		if i == 0 {
			result = append(result, input[i])
			last = code
		} else {
			last = result[len(result)-1]
		}
	}

	for len(result) < 4 {
		result = append(result, '0')
	}

	return string(result)
}
