// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package traceutil

import (
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultSpanName is the default name we assign a span if it's missing and we have no reasonable fallback
	DefaultSpanName = "unnamed_operation"
	// DefaultServiceName is the default name we assign a service if it's missing and we have no reasonable fallback
	DefaultServiceName = "unnamed-service"
)

const (
	// MaxNameLen the maximum length a name can have
	MaxNameLen = 100
	// MaxServiceLen the maximum length a service can have
	MaxServiceLen = 100
)

var (
	// ErrEmpty specifies that the passed input was empty.
	ErrEmpty = errors.New("empty")
	// ErrTooLong signifies that the input was too long.
	ErrTooLong = errors.New("too long")
	// ErrInvalid signifies that the input was invalid.
	ErrInvalid = errors.New("invalid")
)

// NormalizeName normalizes a span name and returns an error describing the reason
// (if any) why the name was modified.
func NormalizeName(name string) (string, error) {
	if name == "" {
		return DefaultSpanName, ErrEmpty
	}
	var err error
	if len(name) > MaxNameLen {
		name = TruncateUTF8(name, MaxNameLen)
		err = ErrTooLong
	}
	name, ok := normMetricNameParse(name)
	if !ok {
		return DefaultSpanName, ErrInvalid
	}
	return name, err
}

// NormalizeService normalizes a span service and returns an error describing the reason
// (if any) why the name was modified.
func NormalizeService(svc string, lang string) (string, error) {
	if svc == "" {
		return fallbackService(lang), ErrEmpty
	}
	var err error
	if len(svc) > MaxServiceLen {
		svc = TruncateUTF8(svc, MaxServiceLen)
		err = ErrTooLong
	}
	s := NormalizeTag(svc)
	if s == "" {
		return fallbackService(lang), ErrInvalid
	}
	return s, err
}

// fallbackServiceNames is a cache of default service names to use
// when the span's service is unset or invalid.
var fallbackServiceNames sync.Map

// fallbackService returns the fallback service name for a service
// belonging to language lang.
func fallbackService(lang string) string {
	if lang == "" {
		return DefaultServiceName
	}
	if v, ok := fallbackServiceNames.Load(lang); ok {
		return v.(string)
	}
	var str strings.Builder
	str.WriteString("unnamed-")
	str.WriteString(lang)
	str.WriteString("-service")
	fallbackServiceNames.Store(lang, str.String())
	return str.String()
}

const maxTagLength = 200

// NormalizeTag applies some normalization to ensure the tags match the backend requirements.
func NormalizeTag(v string) string {
	// the algorithm works by creating a set of cuts marking start and end offsets in v
	// that have to be replaced with underscore (_)
	if len(v) == 0 {
		return ""
	}
	var (
		trim  int      // start character (if trimming)
		cuts  [][2]int // sections to discard: (start, end) pairs
		chars int      // number of characters processed
	)
	var (
		i    int  // current byte
		r    rune // current rune
		jump int  // tracks how many bytes the for range advances on its next iteration
	)
	tag := []byte(v)
	for i, r = range v {
		jump = utf8.RuneLen(r) // next i will be i+jump
		if r == utf8.RuneError {
			// On invalid UTF-8, the for range advances only 1 byte (see: https://golang.org/ref/spec#For_range (point 2)).
			// However, utf8.RuneError is equivalent to unicode.ReplacementChar so we should rely on utf8.DecodeRune to tell
			// us whether this is an actual error or just a unicode.ReplacementChar that was present in the string.
			_, width := utf8.DecodeRune(tag[i:])
			jump = width
		}
		// fast path; all letters (and colons) are ok
		switch {
		case r >= 'a' && r <= 'z' || r == ':':
			chars++
			goto end
		case r >= 'A' && r <= 'Z':
			// lower-case
			tag[i] += 'a' - 'A'
			chars++
			goto end
		}
		if unicode.IsUpper(r) {
			// lowercase this character
			if low := unicode.ToLower(r); utf8.RuneLen(r) == utf8.RuneLen(low) {
				// but only if the width of the lowercased character is the same;
				// there are some rare edge-cases where this is not the case, such
				// as \u017F (ſ)
				utf8.EncodeRune(tag[i:], low)
				r = low
			}
		}
		switch {
		case unicode.IsLetter(r):
			chars++
		case chars == 0:
			// this character can not start the string, trim
			trim = i + jump
			goto end
		case unicode.IsDigit(r) || r == '.' || r == '/' || r == '-':
			chars++
		default:
			// illegal character
			if n := len(cuts); n > 0 && cuts[n-1][1] >= i {
				// merge intersecting cuts
				cuts[n-1][1] += jump
			} else {
				// start a new cut
				cuts = append(cuts, [2]int{i, i + jump})
			}
		}
	end:
		if i+jump >= 2*maxTagLength {
			// bail early if the tag contains a lot of non-letter/digit characters.
			// If a tag is test🍣🍣[...]🍣, then it's unlikely to be a properly formatted tag
			break
		}
		if chars >= maxTagLength {
			// we've reached the maximum
			break
		}
	}

	tag = tag[trim : i+jump] // trim start and end
	if len(cuts) == 0 {
		// tag was ok, return it as it is
		return string(tag)
	}
	delta := trim // cut offsets delta
	for _, cut := range cuts {
		// start and end of cut, including delta from previous cuts:
		start, end := cut[0]-delta, cut[1]-delta

		if end >= len(tag) {
			// this cut includes the end of the string; discard it
			// completely and finish the loop.
			tag = tag[:start]
			break
		}
		// replace the beginning of the cut with '_'
		tag[start] = '_'
		if end-start == 1 {
			// nothing to discard
			continue
		}
		// discard remaining characters in the cut
		copy(tag[start+1:], tag[end:])

		// shorten the slice
		tag = tag[:len(tag)-(end-start)+1]

		// count the new delta for future cuts
		delta += cut[1] - cut[0] - 1
	}
	return string(tag)
}

// This code is borrowed from dd-go metric normalization

// fast isAlpha for ascii
func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// fast isAlphaNumeric for ascii
func isAlphaNum(b byte) bool {
	return isAlpha(b) || (b >= '0' && b <= '9')
}

// normMetricNameParse normalizes metric names with a parser instead of using
// garbage-creating string replacement routines.
func normMetricNameParse(name string) (string, bool) {
	if name == "" || len(name) > MaxNameLen {
		return name, false
	}

	var i, ptr int
	res := make([]byte, 0, len(name))

	// skip non-alphabetic characters
	for ; i < len(name) && !isAlpha(name[i]); i++ {
	}

	// if there were no alphabetic characters it wasn't valid
	if i == len(name) {
		return "", false
	}

	for ; i < len(name); i++ {
		switch {
		case isAlphaNum(name[i]):
			res = append(res, name[i])
			ptr++
		case name[i] == '.':
			// we skipped all non-alpha chars up front so we have seen at least one
			switch res[ptr-1] {
			// overwrite underscores that happen before periods
			case '_':
				res[ptr-1] = '.'
			default:
				res = append(res, '.')
				ptr++
			}
		default:
			// we skipped all non-alpha chars up front so we have seen at least one
			switch res[ptr-1] {
			// no double underscores, no underscores after periods
			case '.', '_':
			default:
				res = append(res, '_')
				ptr++
			}
		}
	}

	if res[ptr-1] == '_' {
		res = res[:ptr-1]
	}

	return string(res), true
}
