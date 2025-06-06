// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package match defines matching algorithms and support code for the license checker.
package match

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// A Dict maps words to integer indexes in a word list, of type WordID.
// The zero Dict is an empty dictionary ready for use.
//
// Lookup and Words are read-only operations,
// safe for any number of concurrent calls from multiple goroutines.
// Insert is a write operation; it must not run concurrently with
// any other call, whether to Insert, Lookup, or Words.
type Dict struct {
	dict map[string]WordID // dict maps word to index in list
	list []string          // list of known words
}

// A WordID is the index of a word in a dictionary.
type WordID int32

// BadWord represents a word not present in the dictionary.
const BadWord WordID = -1

// AnyWord represents a wildcard matching any word.
const AnyWord WordID = -2

// Insert adds the word w to the word list, returning its index.
// If w is already in the word list, it is not added again; Insert returns the existing index.
func (d *Dict) Insert(w string) WordID {
	id, ok := d.dict[w]
	if ok {
		return id
	}
	if d.dict == nil {
		d.dict = make(map[string]WordID)
	}
	id = WordID(len(d.list))
	if int(id) != len(d.list) {
		panic("dictionary too large")
	}
	d.list = append(d.list, w)
	d.dict[w] = id
	return id
}

// Lookup looks for the word w in the word list and returns its index.
// If w is not in the word list, Lookup returns BadWord.
func (d *Dict) Lookup(w string) WordID {
	id, ok := d.dict[w]
	if !ok {
		return BadWord
	}
	return id
}

// Words returns the current word list.
// The list is not a copy; the caller can read but must not modify the list.
func (d *Dict) Words() []string {
	return d.list
}

// A Word represents a single word found in a text.
type Word struct {
	ID WordID
	Lo int32 // Word appears at text[Lo:Hi].
	Hi int32
}

// InsertSplit splits text into a sequence of lowercase words,
// inserting any new words in the dictionary.
func (d *Dict) InsertSplit(text string) []Word {
	return d.split(text, true)
}

// Split splits text into a sequence of lowercase words.
// It does not add any new words to the dictionary.
// Unrecognized words are reported as having ID = BadWord.
func (d *Dict) Split(text string) []Word {
	return d.split(text, false)
}

// © is rewritten to this text.
var copyright = []byte("copyright")

func (d *Dict) split(text string, insert bool) []Word {
	var wbuf []byte
	var words []Word
	t := text
	for t != "" {
		var w []byte
		var lo, hi int32
		{
			switch t[0] {
			case '<':
				if size := htmlTagSize(t); size > 0 {
					t = t[size:]
					continue
				}
			case '{':
				if size := markdownAnchorSize(t); size > 0 {
					t = t[size:]
					continue
				}
			case '&':
				// Assume HTML entity is punctuation.
				if size := htmlEntitySize(t); size > 0 {
					if t[:size] == "&copy;" {
						lo = int32(len(text) - len(t))
						hi = lo + int32(size)
						w = copyright
						t = t[size:]
						goto Emit
					}
					t = t[size:]
					continue
				}
			}
			if len(t) >= 2 && t[0] == ']' && t[1] == '(' {
				if size := markdownLinkSize(t); size > 0 {
					t = t[size:]
					continue
				}
			}

			r, size := utf8.DecodeRuneInString(t)
			if !isWordStart(r) {
				t = t[size:]
				continue
			}
			wbuf = appendFoldRune(wbuf[:0], r)

			// Scan whole word
			// (except © which is already a word by itself,
			// even when it appears next to other text,
			// like ©1996).
			lo = int32(len(text) - len(t))
			if r != '©' {
				for size < len(t) {
					r, s := utf8.DecodeRuneInString(t[size:])
					if !isWordContinue(r) {
						break
					}
					size += s
					wbuf = appendFoldRune(wbuf, r)
				}
				if size+3 <= len(t) && t[size:size+3] == "(s)" {
					// Read "notice(s)" as "notices" and let spell-check accept "notice" too.
					wbuf = append(wbuf, 's')
					size += 3
				}
			}
			hi = lo + int32(size)
			t = t[size:]

			w = wbuf

			// Special case rewrites suggested by SPDX.
			switch {
			case string(w) == "https":
				// "https" -> "http".
				w = w[:4]

			case string(w) == "c" && lo > 0 && text[lo-1] == '(' && int(hi) < len(text) && text[hi] == ')':
				w = copyright
				lo--
				hi++

			case string(w) == "©":
				w = copyright
			}

			// More of our own.
			for _, m := range canonicalRewrites {
				if string(w) == m.y {
					w = append(w[:0], m.x...)
				}
			}
		}

	Emit:
		id, ok := d.dict[string(w)]
		if ok {
			if len(words) > 0 && words[len(words)-1].ID == id && string(w) == "copyright" {
				// Treat "Copyright ©" as a single "copyright" instead of two.
				continue
			}
			words = append(words, Word{id, lo, hi})
			continue
		}

		if insert {
			words = append(words, Word{d.Insert(string(w)), lo, hi})
			continue
		}

		// Unknown word
		words = append(words, Word{BadWord, lo, hi})
	}

	return words
}

// foldRune returns the folded rune r.
// It returns -1 if the rune r should be omitted entirely.
//
// Folding can be any canonicalizing transformation we want.
// For now folding means:
//	- fold to consistent case (unicode.SimpleFold, but moving to lower-case afterward)
//	- return -1 for (drop) combining grave and acute U+0300, U+0301
//	- strip pre-combined graves and acutes on vowels:
//		é to e, etc. (for Canadian or European licenses
//		mentioning Québec or Commissariat à l'Energie Atomique)
//
// If necessary we could do a full Unicode-based conversion,
// but that will require more thought about exactly what to do
// and doing it efficiently. For now, the accents are enough.
func foldRune(r rune) rune {
	// Iterate SimpleFold until we hit the min equivalent rune,
	// which - for the ones we care about - is the upper case ASCII rune.
	for {
		r1 := unicode.SimpleFold(r)
		if r1 >= r {
			break
		}
		r = r1
	}

	switch r {
	case 'Á', 'À':
		return 'a'
	case 'É', 'È':
		return 'e'
	case 'Í', 'Ì':
		return 'i'
	case 'Ó', 'Ò':
		return 'o'
	case 'Ú', 'Ù':
		return 'u'
	}

	if 'A' <= r && r <= 'Z' {
		r += 'a' - 'A'
	}
	if r == '(' || r == ')' {
		// delete ( ) in (c) or notice(s)
		return -1
	}

	return r
}

// toFold converts s to folded form.
func toFold(s string) string {
	var buf []byte
	for _, r := range s {
		buf = appendFoldRune(buf, r)
	}
	return string(buf)
}

// appendFoldRune appends foldRune(r) to buf and returns the updated buffer.
func appendFoldRune(buf []byte, r rune) []byte {
	r = foldRune(r)
	if r < 0 {
		return buf
	}
	if r < utf8.RuneSelf {
		return append(buf, byte(r))
	}

	n := len(buf)
	s := utf8.RuneLen(r)
	for cap(buf) < n+s {
		buf = append(buf[:cap(buf)], 0)
	}
	buf = buf[:n+s]
	utf8.EncodeRune(buf[n:], r)
	return buf
}

// isWordStart reports whether r can appear at the start of a word.
func isWordStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '©'
}

// isWordContinue reports whether r can appear in a word, after the start.
func isWordContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// htmlTagSize returns the length of the HTML tag at the start of t, or else 0.
func htmlTagSize(t string) int {
	if len(t) < 3 || t[0] != '<' {
		return 0
	}
	i := 1
	if t[i] == '/' {
		i++
	}
	if !('A' <= t[i] && t[i] <= 'Z' || 'a' <= t[i] && t[i] <= 'z') {
		return 0
	}
	space := false
	nl := 0
	for ; i < len(t); i++ {
		switch t[i] {
		case '@':
			// Keep <me@example.com>
			if !space {
				return 0
			}
		case ':':
			// Keep <http://example.com>
			if !space && i+1 < len(t) && t[i+1] == '/' {
				return 0
			}
		case '\r', '\n':
			if nl++; nl > 2 {
				return 0
			}
		case '<':
			return 0
		case '>':
			return i + 1
		case ' ':
			space = true
		}
	}
	return 0
}

// htmlEntitySize returns the length of the HTML entity expression at the start of t, or else 0.
func htmlEntitySize(t string) int {
	if len(t) < 3 || t[0] != '&' {
		return 0
	}
	if t[1] == '#' {
		if t[2] == 'x' {
			// &#xHEX;
			i := 3
			for i < len(t) && ('0' <= t[i] && t[i] <= '9' || 'A' <= t[i] && t[i] <= 'F' || 'a' <= t[i] && t[i] <= 'f') {
				i++
			}
			if i > 3 && i < len(t) && t[i] == ';' {
				return i + 1
			}
			return 0
		}
		// &#DECIMAL;
		i := 2
		for i < len(t) && '0' <= t[i] && t[i] <= '9' {
			i++
		}
		if i > 2 && i < len(t) && t[i] == ';' {
			return i + 1
		}
		return 0
	}

	// &name;
	i := 1
	for i < len(t) && ('A' <= t[i] && t[i] <= 'Z' || 'a' <= t[i] && t[i] <= 'z') {
		i++
	}
	if i > 1 && i < len(t) && t[i] == ';' {
		return i + 1
	}
	return 0
}

// markdownAnchorSize returns the length of the Markdown anchor at the start of t, or else 0.
// (like {#head})
func markdownAnchorSize(t string) int {
	if len(t) < 4 || t[0] != '{' || t[1] != '#' {
		return 0
	}
	i := 2
	for ; i < len(t); i++ {
		switch t[i] {
		case '}':
			return i + 1
		case ' ', '\r', '\n':
			return 0
		}
	}
	return 0
}

var markdownLinkPrefixes = []string{
	"http://",
	"https://",
	"mailto:",
	"file:",
	"#",
}

// markdownLinkSize returns the length of the Markdown link target at the start of t, or else 0.
// Instead of fully parsing Markdown, this looks for ](http:// or ](https://.
func markdownLinkSize(t string) int {
	if len(t) < 2 || t[0] != ']' || t[1] != '(' {
		return 0
	}
	ok := false
	for _, prefix := range markdownLinkPrefixes {
		if strings.HasPrefix(t[2:], prefix) {
			ok = true
			break
		}
	}
	if !ok {
		return 0
	}

	for i := 2; i < len(t); i++ {
		c := t[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			return 0
		}
		if c == ')' {
			return i + 1
		}
	}
	return 0
}

// canonicalRewrites is a list of pairs that are canonicalized during word splittting.
// The words on the right are parsed as if they were the words on the left.
// This happens during dictionary splitting, so canMisspell will never see any
// of the words on the right.
var canonicalRewrites = []struct {
	x, y string
}{
	{"is", "are"},
	{"it", "them"},
	{"it", "they"},
	{"the", "these"},
	{"the", "this"},
	{"the", "those"},
	{"copy", "copies"}, // most plurals are handled as 1-letter typos
}
