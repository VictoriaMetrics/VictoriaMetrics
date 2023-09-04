package graphite

import (
	"strconv"
)

func naturalLess(a, b string) bool {
	for {
		var aPrefix, bPrefix string
		aPrefix, a = getNonNumPrefix(a)
		bPrefix, b = getNonNumPrefix(b)
		if aPrefix != bPrefix {
			return aPrefix < bPrefix
		}
		if len(a) == 0 || len(b) == 0 {
			return a < b
		}
		var aNum, bNum int
		aNum, a = getNumPrefix(a)
		bNum, b = getNumPrefix(b)
		if aNum != bNum {
			return aNum < bNum
		}
	}
}

func getNonNumPrefix(s string) (prefix string, tail string) {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= '0' && ch <= '9' {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func getNumPrefix(s string) (prefix int, tail string) {
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch < '0' || ch > '9' {
			break
		}
		i++
	}
	prefix, _ = strconv.Atoi(s[:i])
	return prefix, s[i:]
}
