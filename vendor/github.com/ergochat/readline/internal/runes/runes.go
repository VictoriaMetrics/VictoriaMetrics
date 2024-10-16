package runes

import (
	"bytes"
	"golang.org/x/text/width"
	"unicode"
	"unicode/utf8"
)

var TabWidth = 4

func EqualRune(a, b rune, fold bool) bool {
	if a == b {
		return true
	}
	if !fold {
		return false
	}
	if a > b {
		a, b = b, a
	}
	if b < utf8.RuneSelf && 'A' <= a && a <= 'Z' {
		if b == a+'a'-'A' {
			return true
		}
	}
	return false
}

func EqualRuneFold(a, b rune) bool {
	return EqualRune(a, b, true)
}

func EqualFold(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if EqualRuneFold(a[i], b[i]) {
			continue
		}
		return false
	}

	return true
}

func Equal(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func IndexAllBckEx(r, sub []rune, fold bool) int {
	for i := len(r) - len(sub); i >= 0; i-- {
		found := true
		for j := 0; j < len(sub); j++ {
			if !EqualRune(r[i+j], sub[j], fold) {
				found = false
				break
			}
		}
		if found {
			return i
		}
	}
	return -1
}

// Search in runes from end to front
func IndexAllBck(r, sub []rune) int {
	return IndexAllBckEx(r, sub, false)
}

// Search in runes from front to end
func IndexAll(r, sub []rune) int {
	return IndexAllEx(r, sub, false)
}

func IndexAllEx(r, sub []rune, fold bool) int {
	for i := 0; i < len(r); i++ {
		found := true
		if len(r[i:]) < len(sub) {
			return -1
		}
		for j := 0; j < len(sub); j++ {
			if !EqualRune(r[i+j], sub[j], fold) {
				found = false
				break
			}
		}
		if found {
			return i
		}
	}
	return -1
}

func Index(r rune, rs []rune) int {
	for i := 0; i < len(rs); i++ {
		if rs[i] == r {
			return i
		}
	}
	return -1
}

func ColorFilter(r []rune) []rune {
	newr := make([]rune, 0, len(r))
	for pos := 0; pos < len(r); pos++ {
		if r[pos] == '\033' && r[pos+1] == '[' {
			idx := Index('m', r[pos+2:])
			if idx == -1 {
				continue
			}
			pos += idx + 2
			continue
		}
		newr = append(newr, r[pos])
	}
	return newr
}

var zeroWidth = []*unicode.RangeTable{
	unicode.Mn,
	unicode.Me,
	unicode.Cc,
	unicode.Cf,
}

var doubleWidth = []*unicode.RangeTable{
	unicode.Han,
	unicode.Hangul,
	unicode.Hiragana,
	unicode.Katakana,
}

func Width(r rune) int {
	if r == '\t' {
		return TabWidth
	}
	if unicode.IsOneOf(zeroWidth, r) {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

func WidthAll(r []rune) (length int) {
	for i := 0; i < len(r); i++ {
		length += Width(r[i])
	}
	return
}

func Backspace(r []rune) []byte {
	return bytes.Repeat([]byte{'\b'}, WidthAll(r))
}

func Copy(r []rune) []rune {
	n := make([]rune, len(r))
	copy(n, r)
	return n
}

func HasPrefixFold(r, prefix []rune) bool {
	if len(r) < len(prefix) {
		return false
	}
	return EqualFold(r[:len(prefix)], prefix)
}

func HasPrefix(r, prefix []rune) bool {
	if len(r) < len(prefix) {
		return false
	}
	return Equal(r[:len(prefix)], prefix)
}

func Aggregate(candicate [][]rune) (same []rune, size int) {
	for i := 0; i < len(candicate[0]); i++ {
		for j := 0; j < len(candicate)-1; j++ {
			if i >= len(candicate[j]) || i >= len(candicate[j+1]) {
				goto aggregate
			}
			if candicate[j][i] != candicate[j+1][i] {
				goto aggregate
			}
		}
		size = i + 1
	}
aggregate:
	if size > 0 {
		same = Copy(candicate[0][:size])
		for i := 0; i < len(candicate); i++ {
			n := Copy(candicate[i])
			copy(n, n[size:])
			candicate[i] = n[:len(n)-size]
		}
	}
	return
}

func TrimSpaceLeft(in []rune) []rune {
	firstIndex := len(in)
	for i, r := range in {
		if unicode.IsSpace(r) == false {
			firstIndex = i
			break
		}
	}
	return in[firstIndex:]
}

func IsWordBreak(i rune) bool {
	switch {
	case i >= 'a' && i <= 'z':
	case i >= 'A' && i <= 'Z':
	case i >= '0' && i <= '9':
	default:
		return true
	}
	return false
}

// split prompt + runes into lines by screenwidth starting from an offset.
// the prompt should be filtered before passing to only its display runes.
// if you know the width of the next character, pass it in as it is used
// to decide if we generate an extra empty rune array to show next is new
// line.
func SplitByLine(prompt, rs []rune, offset, screenWidth, nextWidth int) [][]rune {
	ret := make([][]rune, 0)
	prs := append(prompt, rs...)
	si := 0
	currentWidth := offset
	for i, r := range prs {
		w := Width(r)
		if r == '\n' {
			ret = append(ret, prs[si:i+1])
			si = i + 1
			currentWidth = 0
		} else if currentWidth+w > screenWidth {
			ret = append(ret, prs[si:i])
			si = i
			currentWidth = 0
		}
		currentWidth += w
	}
	ret = append(ret, prs[si:])
	if currentWidth+nextWidth > screenWidth {
		ret = append(ret, []rune{})
	}
	return ret
}
