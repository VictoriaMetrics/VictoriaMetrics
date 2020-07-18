package quicktemplate

import (
	"fmt"
	"strings"
)

func hasSpecialChars(s string) bool {
	if strings.IndexByte(s, '"') >= 0 || strings.IndexByte(s, '\\') >= 0 || strings.IndexByte(s, '<') >= 0 || strings.IndexByte(s, '\'') >= 0 {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 {
			return true
		}
	}
	return false
}

func appendJSONString(dst []byte, s string, addQuotes bool) []byte {
	if !hasSpecialChars(s) {
		// Fast path - nothing to escape.
		if !addQuotes {
			return append(dst, s...)
		}
		dst = append(dst, '"')
		dst = append(dst, s...)
		dst = append(dst, '"')
		return dst
	}

	// Slow path - there are chars to escape.
	if addQuotes {
		dst = append(dst, '"')
	}
	bb := AcquireByteBuffer()
	var tmp []byte
	tmp, bb.B = bb.B, dst
	_, err := jsonReplacer.WriteString(bb, s)
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error returned from jsonReplacer.WriteString: %s", err))
	}
	dst, bb.B = bb.B, tmp
	ReleaseByteBuffer(bb)
	if addQuotes {
		dst = append(dst, '"')
	}
	return dst
}

var jsonReplacer = strings.NewReplacer(func() []string {
	a := []string{
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"\"", `\"`,
		"\\", `\\`,
		"<", `\u003c`,
		"'", `\u0027`,
	}
	for i := 0; i < 0x20; i++ {
		a = append(a, string([]byte{byte(i)}), fmt.Sprintf(`\u%04x`, i))
	}
	return a
}()...)
