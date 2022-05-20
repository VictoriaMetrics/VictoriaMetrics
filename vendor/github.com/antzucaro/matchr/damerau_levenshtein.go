package matchr

// DamerauLevenshtein computes the Damerau-Levenshtein distance between two
// strings. The returned value - distance - is the number of insertions,
// deletions, substitutions, and transpositions it takes to transform one
// string (s1) into another (s2). Each step in the transformation "costs"
// one distance point. It is similar to the Optimal String Alignment,
// algorithm, but is more complex because it allows multiple edits on
// substrings.
//
// This implementation is based off of the one found on Wikipedia at
// http://en.wikipedia.org/wiki/Damerau%E2%80%93Levenshtein_distance#Distance_with_adjacent_transpositions
// as well as KevinStern's Java implementation found at
// https://github.com/KevinStern/software-and-algorithms.
func DamerauLevenshtein(s1 string, s2 string) (distance int) {
	// index by code point, not byte
	r1 := []rune(s1)
	r2 := []rune(s2)

	// the maximum possible distance
	inf := len(r1) + len(r2)

	// if one string is blank, we needs insertions
	// for all characters in the other one
	if len(r1) == 0 {
		return len(r2)
	}

	if len(r2) == 0 {
		return len(r1)
	}

	// construct the edit-tracking matrix
	matrix := make([][]int, len(r1))
	for i := range matrix {
		matrix[i] = make([]int, len(r2))
	}

	// seen characters
	seenRunes := make(map[rune]int)

	if r1[0] != r2[0] {
		matrix[0][0] = 1
	}

	seenRunes[r1[0]] = 0
	for i := 1; i < len(r1); i++ {
		deleteDist := matrix[i-1][0] + 1
		insertDist := (i+1)*1 + 1
		var matchDist int
		if r1[i] == r2[0] {
			matchDist = i
		} else {
			matchDist = i + 1
		}
		matrix[i][0] = min(min(deleteDist, insertDist), matchDist)
	}

	for j := 1; j < len(r2); j++ {
		deleteDist := (j + 1) * 2
		insertDist := matrix[0][j-1] + 1
		var matchDist int
		if r1[0] == r2[j] {
			matchDist = j
		} else {
			matchDist = j + 1
		}

		matrix[0][j] = min(min(deleteDist, insertDist), matchDist)
	}

	for i := 1; i < len(r1); i++ {
		var maxSrcMatchIndex int
		if r1[i] == r2[0] {
			maxSrcMatchIndex = 0
		} else {
			maxSrcMatchIndex = -1
		}

		for j := 1; j < len(r2); j++ {
			swapIndex, ok := seenRunes[r2[j]]
			jSwap := maxSrcMatchIndex
			deleteDist := matrix[i-1][j] + 1
			insertDist := matrix[i][j-1] + 1
			matchDist := matrix[i-1][j-1]
			if r1[i] != r2[j] {
				matchDist += 1
			} else {
				maxSrcMatchIndex = j
			}

			// for transpositions
			var swapDist int
			if ok && jSwap != -1 {
				iSwap := swapIndex
				var preSwapCost int
				if iSwap == 0 && jSwap == 0 {
					preSwapCost = 0
				} else {
					preSwapCost = matrix[maxI(0, iSwap-1)][maxI(0, jSwap-1)]
				}
				swapDist = i + j + preSwapCost - iSwap - jSwap - 1
			} else {
				swapDist = inf
			}
			matrix[i][j] = min(min(min(deleteDist, insertDist), matchDist), swapDist)
		}
		seenRunes[r1[i]] = i
	}

	return matrix[len(r1)-1][len(r2)-1]
}
