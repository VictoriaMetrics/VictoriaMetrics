package matchr

func jaroWinklerBase(s1 string, s2 string,
	longTolerance bool, winklerize bool) (distance float64) {

	// index by code point, not byte
	r1 := []rune(s1)
	r2 := []rune(s2)

	r1Length := len(r1)
	r2Length := len(r2)

	if r1Length == 0 || r2Length == 0 {
		return
	}

	minLength := 0
	if r1Length > r2Length {
		minLength = r1Length
	} else {
		minLength = r2Length
	}

	searchRange := minLength
	searchRange = (searchRange / 2) - 1
	if searchRange < 0 {
		searchRange = 0
	}
	var lowLim, hiLim, transCount, commonChars int
	var i, j, k int

	r1Flag := make([]bool, r1Length+1)
	r2Flag := make([]bool, r2Length+1)

	// find the common chars within the acceptable range
	commonChars = 0
	for i, _ = range r1 {
		if i >= searchRange {
			lowLim = i - searchRange
		} else {
			lowLim = 0
		}

		if (i + searchRange) <= (r2Length - 1) {
			hiLim = i + searchRange
		} else {
			hiLim = r2Length - 1
		}

		for j := lowLim; j <= hiLim; j++ {
			if !r2Flag[j] && r2[j] == r1[i] {
				r2Flag[j] = true
				r1Flag[i] = true
				commonChars++

				break
			}
		}
	}

	// if we have nothing in common at this point, nothing else can be done
	if commonChars == 0 {
		return
	}

	// otherwise we count the transpositions
	k = 0
	transCount = 0
	for i, _ := range r1 {
		if r1Flag[i] {
			for j = k; j < r2Length; j++ {
				if r2Flag[j] {
					k = j + 1
					break
				}
			}
			if r1[i] != r2[j] {
				transCount++
			}
		}
	}
	transCount /= 2

	// adjust for similarities in nonmatched characters
	distance = float64(commonChars)/float64(r1Length) +
		float64(commonChars)/float64(r2Length) +
		(float64(commonChars-transCount))/float64(commonChars)
	distance /= 3.0

	// give more weight to already-similar strings
	if winklerize && distance > 0.7 {

		// the first 4 characters in common
		if minLength >= 4 {
			j = 4
		} else {
			j = minLength
		}

		for i = 0; i < j && len(r1) > i && len(r2) > i && r1[i] == r2[i] && nan(r1[i]); i++ {
		}

		if i > 0 {
			distance += float64(i) * 0.1 * (1.0 - distance)
		}

		if longTolerance && (minLength > 4) && (commonChars > i+1) &&
			(2*commonChars >= minLength+i) {
			if nan(r1[0]) {
				distance += (1.0 - distance) * (float64(commonChars-i-1) /
					(float64(r1Length) + float64(r2Length) - float64(i*2) + 2))
			}
		}
	}

	return
}

// Jaro computes the Jaro edit distance between two strings. It represents
// this with a float64 between 0 and 1 inclusive, with 0 indicating the two
// strings are not at all similar and 1 indicating the two strings are exact
// matches.
//
// See http://en.wikipedia.org/wiki/Jaro%E2%80%93Winkler_distance for a
// full description.
func Jaro(r1 string, r2 string) (distance float64) {
	return jaroWinklerBase(r1, r2, false, false)
}

// JaroWinkler computes the Jaro-Winkler edit distance between two strings.
// This is a modification of the Jaro algorithm that gives additional weight
// to prefix matches.
func JaroWinkler(r1 string, r2 string, longTolerance bool) (distance float64) {
	return jaroWinklerBase(r1, r2, longTolerance, true)
}
