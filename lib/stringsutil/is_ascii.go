package stringsutil

import (
	"unicode/utf8"
	"unsafe"
)

func IsASCII(s string) bool {
	l := len(s)
	end := l - l&7
	ptr := unsafe.StringData(s)
	for start := 0; start < end; start += 8 {
		// compare 8 bytes at once
		var chars uint64 = *((*uint64)(unsafe.Add(unsafe.Pointer(ptr), start)))
		if chars&0x8080808080808080 != 0 {
			return false
		}
	}
	s = s[end:]
	for i := range s {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
