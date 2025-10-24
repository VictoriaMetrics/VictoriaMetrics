package logstorage

import (
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type patternMatcher struct {
	isFull       bool
	separators   []string
	placeholders []patternMatcherPlaceholder
}

func (pm *patternMatcher) String() string {
	var a []string
	for i, sep := range pm.separators {
		a = append(a, sep)
		if i < len(pm.placeholders) {
			a = append(a, pm.placeholders[i].String())
		}
	}
	return strings.Join(a, "")
}

type patternMatcherPlaceholder int

// See appendPrettifyCollapsedNums()
const (
	patternMatcherPlaceholderUnknown  = 0
	patternMatcherPlaceholderNum      = 1
	patternMatcherPlaceholderUUID     = 2
	patternMatcherPlaceholderIP4      = 3
	patternMatcherPlaceholderTime     = 4
	patternMatcherPlaceholderDate     = 5
	patternMatcherPlaceholderDateTime = 6
	patternMatcherPlaceholderWord     = 7
)

func getPatternMatcherPlaceholder(s string) patternMatcherPlaceholder {
	switch s {
	case "<N>":
		return patternMatcherPlaceholderNum
	case "<UUID>":
		return patternMatcherPlaceholderUUID
	case "<IP4>":
		return patternMatcherPlaceholderIP4
	case "<TIME>":
		return patternMatcherPlaceholderTime
	case "<DATE>":
		return patternMatcherPlaceholderDate
	case "<DATETIME>":
		return patternMatcherPlaceholderDateTime
	case "<W>":
		return patternMatcherPlaceholderWord
	default:
		return patternMatcherPlaceholderUnknown
	}
}

func (ph patternMatcherPlaceholder) String() string {
	switch ph {
	case patternMatcherPlaceholderUnknown:
		return "<UNKNOWN>"
	case patternMatcherPlaceholderNum:
		return "<N>"
	case patternMatcherPlaceholderUUID:
		return "<UUID>"
	case patternMatcherPlaceholderIP4:
		return "<IP4>"
	case patternMatcherPlaceholderTime:
		return "<TIME>"
	case patternMatcherPlaceholderDate:
		return "<DATE>"
	case patternMatcherPlaceholderDateTime:
		return "<DATETIME>"
	case patternMatcherPlaceholderWord:
		return "<W>"
	default:
		logger.Panicf("BUG: unexpected placeholder=%d", ph)
		return ""
	}
}

func newPatternMatcher(s string, isFull bool) *patternMatcher {
	var separators []string
	var placeholders []patternMatcherPlaceholder

	offset := 0
	separator := ""
	for offset < len(s) {
		n := strings.IndexByte(s[offset:], '<')
		if n < 0 {
			separator += s[offset:]
			break
		}
		separator += s[offset : offset+n]
		offset += n

		n = strings.IndexByte(s[offset:], '>')
		if n < 0 {
			separator += s[offset:]
			break
		}
		placeholder := s[offset : offset+n+1]
		offset += n + 1

		ph := getPatternMatcherPlaceholder(placeholder)
		if ph == patternMatcherPlaceholderUnknown {
			separator += placeholder
			continue
		}

		separators = append(separators, separator)
		placeholders = append(placeholders, ph)
		separator = ""
	}
	separators = append(separators, separator)

	return &patternMatcher{
		isFull:       isFull,
		separators:   separators,
		placeholders: placeholders,
	}
}

// Match returns true if s matches the given pm.
//
// if pm.isFull is set, then the s must match pm in full, from the beginning to the end.
// Otherwise the pm may be matched by any substring of s.
func (pm *patternMatcher) Match(s string) bool {
	if pm.isFull {
		end := pm.indexEnd(s, 0)
		return end == len(s)
	}
	return pm.matchSubstring(s)
}

func (pm *patternMatcher) matchSubstring(s string) bool {
	offset := 0
	for {
		start := pm.indexPatternStart(s, offset)
		if start < 0 {
			return false
		}
		end := pm.indexEnd(s, start)
		if end >= 0 {
			return true
		}
		offset = start + 1
	}
}

func (pm *patternMatcher) indexPatternStart(s string, offset int) int {
	if firstSep := pm.separators[0]; firstSep != "" {
		return strings.Index(s[offset:], firstSep)
	}

	placeholders := pm.placeholders
	if len(placeholders) == 0 {
		return 0
	}

	if placeholders[0] == patternMatcherPlaceholderWord {
		return indexWordStart(s, offset)
	}
	return indexNumStart(s, offset)
}

func (pm *patternMatcher) indexEnd(s string, start int) int {
	placeholders := pm.placeholders

	offset := start
	for i, sep := range pm.separators {
		if sep != "" {
			if !strings.HasPrefix(s[offset:], sep) {
				return -1
			}
			offset += len(sep)
		}

		if i >= len(placeholders) {
			return offset
		}

		offset = placeholders[i].indexEnd(s, offset)
		if offset < 0 {
			return -1
		}
	}
	return offset
}

func (ph patternMatcherPlaceholder) indexEnd(s string, start int) int {
	switch ph {
	case patternMatcherPlaceholderNum:
		return indexPlaceholderNumEnd(s, start)
	case patternMatcherPlaceholderUUID:
		return indexPlaceholderUUIDEnd(s, start)
	case patternMatcherPlaceholderIP4:
		return indexPlaceholderIP4End(s, start)
	case patternMatcherPlaceholderTime:
		return indexPlaceholderTimeEnd(s, start)
	case patternMatcherPlaceholderDate:
		return indexPlaceholderDateEnd(s, start)
	case patternMatcherPlaceholderDateTime:
		return indexPlaceholderDateTimeEnd(s, start)
	case patternMatcherPlaceholderWord:
		return indexPlaceholderWordEnd(s, start)
	default:
		logger.Panicf("BUG: unexpected patternMatcherPlaceholder=%d", ph)
		return -1
	}
}

func indexPlaceholderNumEnd(s string, start int) int {
	end := indexNumEnd(s, start)
	if !isValidNum(s, start, end) {
		return -1
	}
	return end
}

func indexPlaceholderUUIDEnd(s string, start int) int {
	// <UUID> is <N>-<N>-<N>-<N>-<N>
	return indexGenericPlaceholderEnd(s, start, 5, '-')
}

func indexPlaceholderIP4End(s string, start int) int {
	// <IP4> is <N>.<N>.<N>.<N>
	return indexGenericPlaceholderEnd(s, start, 4, '.')
}

func indexPlaceholderTimeEnd(s string, start int) int {
	// <TIME> is <N>:<N>:<N> with optional subseconds .<N> or ,<N>
	end := indexGenericPlaceholderEnd(s, start, 3, ':')
	if end < 0 {
		return -1
	}

	// Check optional subseconds
	if end < len(s) && (s[end] == '.' || s[end] == ',') {
		n := indexPlaceholderNumEnd(s, end+1)
		if n >= 0 {
			return n
		}
	}

	return end
}

func indexPlaceholderDateEnd(s string, start int) int {
	// <DATE> is <N>-<N>-<N> or <N>/<N>/<N>
	end := indexGenericPlaceholderEnd(s, start, 3, '-')
	if end >= 0 {
		return end
	}
	return indexGenericPlaceholderEnd(s, start, 3, '/')
}

func indexPlaceholderDateTimeEnd(s string, start int) int {
	// <DATETIME> is '<DATE>T<TIME>' or '<DATE> <TIME>' with optional timezone
	end := indexPlaceholderDateEnd(s, start)
	if end < 0 {
		return -1
	}
	if end >= len(s) || (s[end] != 'T' && s[end] != ' ') {
		return -1
	}

	end = indexPlaceholderTimeEnd(s, end+1)
	if end < 0 {
		return -1
	}

	if end >= len(s) {
		return end
	}

	// Check optional timezone
	if s[end] == 'Z' {
		return end + 1
	}
	if s[end] == '-' || s[end] == '+' {
		n := indexTimezoneEnd(s, end+1)
		if n >= 0 {
			return n
		}
	}

	return end
}

func indexPlaceholderWordEnd(s string, start int) int {
	// <W> is a word or a quoted string
	if start >= len(s) {
		return -1
	}
	if s[start] == '"' || s[start] == '\'' || s[start] == '`' {
		return indexQuotedStringEnd(s, start)
	}
	return indexWordEnd(s, start)
}

func indexWordEnd(s string, start int) int {
	for off, r := range s[start:] {
		if !isTokenRune(r) {
			return start + off
		}
	}
	return len(s)
}

func indexWordStart(s string, offset int) int {
	for off, r := range s[offset:] {
		if isTokenRune(r) {
			return offset + off
		}
	}
	return -1
}

func indexQuotedStringEnd(s string, start int) int {
	switch s[start] {
	case '"', '`':
		qp, err := strconv.QuotedPrefix(s[start:])
		if err != nil {
			return -1
		}
		return start + len(qp)
	case '\'':
		end := start + 1
		for !strings.HasPrefix(s[end:], "'") {
			_, _, tail, err := strconv.UnquoteChar(s[end:], '\'')
			if err != nil {
				return -1
			}
			end = len(s) - len(tail)
		}
		return end + 1
	default:
		logger.Panicf("BUG: unexpected starting char for quoted string: %c", s[start])
		return -1
	}
}

func indexTimezoneEnd(s string, start int) int {
	// Timezone is <N>:<N>
	return indexGenericPlaceholderEnd(s, start, 2, ':')
}

func indexGenericPlaceholderEnd(s string, start int, nums int, separator byte) int {
	end := indexPlaceholderNumEnd(s, start)
	if end < 0 {
		return -1
	}
	for i := 0; i < nums-1; i++ {
		if end >= len(s) || s[end] != separator {
			return -1
		}
		end = indexPlaceholderNumEnd(s, end+1)
		if end < 0 {
			return -1
		}
	}
	return end
}
