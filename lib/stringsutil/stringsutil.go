package stringsutil

import (
	"slices"
	"sync"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

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

// AppendLowercase appends lowercase s to dst and returns the result.
// It is recommended to use ToLowercaseFunc if possible to avoid copying of s.
func AppendLowercase(dst []byte, s string) []byte {
	// Try to find the first uppercase character.
	n := uppercaseIndex(s)
	if n < 0 {
		// Fast path: no uppercase characters found.
		dst = append(dst, s...)
		return dst
	}

	// Slow path: convert s to lowercase.
	dst = slices.Grow(dst, len(s))
	dst = append(dst, s[:n]...)
	s = s[n:]
	return appendLowercaseInternal(dst, s)
}

func appendLowercaseInternal(dst []byte, s string) []byte {
	dstLen := len(dst)

	// Try fast path at first by assuming that s contains only ASCII chars.
	hasUnicodeChars := false
	for i := range len(s) {
		c := s[i]
		if c >= utf8.RuneSelf {
			hasUnicodeChars = true
			break
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		dst = append(dst, c)
	}
	if hasUnicodeChars {
		// Slow path - s contains non-ASCII chars. Use Unicode encoding.
		dst = dst[:dstLen]
		for _, r := range s {
			r = unicode.ToLower(r)
			dst = utf8.AppendRune(dst, r)
		}
	}
	return dst
}

// ToLowercaseFunc calls f with a lowercase version of s.
// The resulting value is only valid during the f call.
func ToLowercaseFunc(s string, f func(s string)) {
	// Try to find the first uppercase character.
	n := uppercaseIndex(s)
	if n < 0 {
		// Fast path: no uppercase characters found.
		f(s)
		return
	}

	sb := getStringBuilder()
	defer putStringBuilder(sb)

	sb.buf = slices.Grow(sb.buf, len(s))
	sb.appendString(s[:n])
	sb.buf = appendLowercaseInternal(sb.buf, s[n:])
	f(sb.string())
}

// IsLowercase returns true if the given string does not contain uppercase characters.
func IsLowercase(s string) bool {
	return uppercaseIndex(s) < 0
}

// uppercaseIndex returns the index of the first uppercase character in s,
// or -1 if s does not contain uppercase characters.
func uppercaseIndex(s string) int {
	idx := 0

	// Fast path for ASCII-only strings - process 8 bytes at a time.
	for idx <= len(s)-8 {
		v := uint64FromString(s[idx:])
		// ASCII characters have the 8th bit clear.
		// The operation bellow is the same as s[idx] < utf8.RuneSelf, but for multiple bytes.
		if isASCII := v&0x8080808080808080 == 0; !isASCII {
			break
		}

		// Check if any byte lacks the 6th bit, which indicates uppercase symbol or '@', '[', '\', ']', '^', '_'.
		mightHaveUpper := ^v&0x2020202020202020 != 0
		if mightHaveUpper {
			for j := 0; j < 8; j++ {
				c := s[idx+j]
				if c >= 'A' && c <= 'Z' {
					return idx + j
				}
			}
		}
		idx += 8
	}

	// Handle the rest of the s.
	for idx < len(s) {
		if c := s[idx]; c < utf8.RuneSelf {
			if c >= 'A' && c <= 'Z' {
				return idx
			}
			idx++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[idx:])
		if r != unicode.ToLower(r) {
			return idx
		}
		idx += size
	}
	return -1
}

// uint64FromString interprets the first 8 bytes of string b as a little-endian uint64.
// The same as binary.LittleEndian.Uint64, but operates on strings.
//
// This function is a bit slower than (*uint64)(unsafe.Pointer(ptr)) alternative,
// but does not have the issue with data alignment. See: https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927
func uint64FromString(b string) uint64 {
	_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

type stringBuilder struct {
	buf []byte
}

func (sb *stringBuilder) appendString(s string) {
	sb.buf = append(sb.buf, s...)
}

func (sb *stringBuilder) reset() {
	sb.buf = sb.buf[:0]
}

func (sb *stringBuilder) string() string {
	return unsafe.String(unsafe.SliceData(sb.buf), len(sb.buf))
}

var stringBuilderPool = sync.Pool{
	New: func() any {
		return &stringBuilder{}
	},
}

func getStringBuilder() *stringBuilder {
	return stringBuilderPool.Get().(*stringBuilder)
}

func putStringBuilder(sb *stringBuilder) {
	sb.reset()
	stringBuilderPool.Put(sb)
}
