package stringsutil

// LimitStringLen limits the length of s with maxLen.
//
// If len(s) > maxLen, then s is replaced with "s_prefix..s_suffix",
// so the total length of the returned string doesn't exceed maxLen.
func LimitStringLen(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	n := (maxLen / 2) - 1
	return s[:n] + ".." + s[len(s)-n:]
}
