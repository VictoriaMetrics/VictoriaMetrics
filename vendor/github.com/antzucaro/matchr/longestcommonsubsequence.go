package matchr

// LongestCommonSubsequence computes the longest substring
// between two strings. The returned value is the length
// of the substring, which contains letters from both
// strings, while maintaining the order of the letters.
func LongestCommonSubsequence(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	table := make([][]int, len(s1)+1)

	// Construct 2D table
	for i := range table {
		table[i] = make([]int, len(s2)+1)
	}

	var i int
	var j int

	for i = len(r1) - 1; i >= 0; i-- {
		for j = len(r2) - 1; j >= 0; j-- {
			if r1[i] == r2[j] {
				table[i][j] = 1 + table[i+1][j+1]
			} else {
				table[i][j] = maxI(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table[0][0]
}
