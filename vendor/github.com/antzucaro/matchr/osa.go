package matchr

// OSA computes the Optimal String Alignment distance between two
// strings. The returned value - distance - is the number of insertions,
// deletions, substitutions, and transpositions it takes to transform one
// string (s1) into another (s2). Each step in the transformation "costs"
// one distance point. It is similar to Damerau-Levenshtein, but is simpler
// because it does not allow multiple edits on any substring.
func OSA(s1 string, s2 string) (distance int) {
	// index by code point, not byte
	r1 := []rune(s1)
	r2 := []rune(s2)

	rows := len(r1) + 1
	cols := len(r2) + 1

	var i, j, d1, d2, d3, d_now, cost int

	dist := make([]int, rows*cols)

	for i = 0; i < rows; i++ {
		dist[i*cols] = i
	}

	for j = 0; j < cols; j++ {
		dist[j] = j
	}

	for i = 1; i < rows; i++ {
		for j = 1; j < cols; j++ {
			if r1[i-1] == r2[j-1] {
				cost = 0
			} else {
				cost = 1
			}

			d1 = dist[((i-1)*cols)+j] + 1
			d2 = dist[(i*cols)+(j-1)] + 1
			d3 = dist[((i-1)*cols)+(j-1)] + cost

			d_now = min(d1, min(d2, d3))

			if i > 2 && j > 2 && r1[i-1] == r2[j-2] &&
				r1[i-2] == r2[j-1] {
				d1 = dist[((i-2)*cols)+(j-2)] + cost
				d_now = min(d_now, d1)
			}

			dist[(i*cols)+j] = d_now
		}
	}

	distance = dist[(cols*rows)-1]

	return
}
