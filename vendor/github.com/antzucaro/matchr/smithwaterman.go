package matchr

const GAP_COST = float64(0.5)

func getCost(r1 []rune, r1Index int, r2 []rune, r2Index int) float64 {
	if r1[r1Index] == r2[r2Index] {
		return 1.0
	} else {
		return -2.0
	}
}

// SmithWaterman computes the Smith-Waterman local sequence alignment for the
// two input strings. This was originally designed to find similar regions in
// strings representing DNA or protein sequences.
func SmithWaterman(s1 string, s2 string) float64 {
	var cost float64

	// index by code point, not byte
	r1 := []rune(s1)
	r2 := []rune(s2)

	r1Len := len(r1)
	r2Len := len(r2)

	if r1Len == 0 {
		return float64(r2Len)
	}

	if r2Len == 0 {
		return float64(r1Len)
	}

	d := make([][]float64, r1Len)
	for i := range d {
		d[i] = make([]float64, r2Len)
	}

	var maxSoFar float64
	for i := 0; i < r1Len; i++ {
		// substitution cost
		cost = getCost(r1, i, r2, 0)
		if i == 0 {
			d[0][0] = max(0.0, max(-GAP_COST, cost))
		} else {
			d[i][0] = max(0.0, max(d[i-1][0]-GAP_COST, cost))
		}

		// save if it is the biggest thus far
		if d[i][0] > maxSoFar {
			maxSoFar = d[i][0]
		}
	}

	for j := 0; j < r2Len; j++ {
		// substitution cost
		cost = getCost(r1, 0, r2, j)
		if j == 0 {
			d[0][0] = max(0, max(-GAP_COST, cost))
		} else {
			d[0][j] = max(0, max(d[0][j-1]-GAP_COST, cost))
		}

		// save if it is the biggest thus far
		if d[0][j] > maxSoFar {
			maxSoFar = d[0][j]
		}
	}

	for i := 1; i < r1Len; i++ {
		for j := 1; j < r2Len; j++ {
			cost = getCost(r1, i, r2, j)

			// find the lowest cost
			d[i][j] = max(
				max(0, d[i-1][j]-GAP_COST),
				max(d[i][j-1]-GAP_COST, d[i-1][j-1]+cost))

			// save if it is the biggest thus far
			if d[i][j] > maxSoFar {
				maxSoFar = d[i][j]
			}
		}
	}

	return maxSoFar
}
