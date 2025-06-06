// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Exported LRE interface.

package match

import (
	"fmt"
	"sync"
)

// An LRE is a compiled license regular expression.
//
// TODO: Move this comment somewhere non-internal later.
//
// A license regular expression (LRE) is a pattern syntax intended for
// describing large English texts such as software licenses, with minor
// allowed variations. The pattern syntax and the matching are word-based
// and case-insensitive; punctuation is ignored in the pattern and in
// the matched text.
//
// The valid LRE patterns are:
//
//	word            - a single case-insensitive word
//	__N__           - any sequence of up to N words
//	expr1 expr2     - concatenation
//	expr1 || expr2  - alternation
//	(( expr ))      - grouping
//	expr??          - zero or one instances of expr
//	//** text **//  - a comment
//
// To make patterns harder to misread in large texts:
//
//	- || must only appear inside (( ))
//	- ?? must only follow (( ))
//	- (( must be at the start of a line, preceded only by spaces
//	- )) must be at the end of a line, followed only by spaces and ??.
//
// For example:
//
//	//** https://en.wikipedia.org/wiki/Filler_text **//
//	Now is
//	((not))??
//	the time for all good
//	((men || women || people))
//	to come to the aid of their __1__.
//
type LRE struct {
	dict   *Dict
	file   string
	syntax *reSyntax
	prog   reProg

	onceDFA sync.Once
	dfa     reDFA
}

// ParseLRE parses the string s as a license regexp.
// The file name is used in error messages if non-empty.
func ParseLRE(d *Dict, file, s string) (*LRE, error) {
	syntax, err := reParse(d, s, true)
	if err != nil {
		return nil, err
	}
	prog, err := syntax.compile(nil, 0)
	if err != nil {
		return nil, err
	}
	return &LRE{dict: d, file: file, syntax: syntax, prog: prog}, nil
}

// Dict returns the Dict used by the LRE.
func (re *LRE) Dict() *Dict {
	return re.dict
}

// File returns the file name passed to ParseLRE.
func (re *LRE) File() string {
	return re.file
}

// Match reports whether text matches the license regexp.
func (re *LRE) match(text string) bool {
	re.onceDFA.Do(re.compile)
	match, _ := re.dfa.match(re.dict, text, re.dict.Split(text))
	return match >= 0
}

// compile initializes lre.dfa.
// It is invoked lazily (in Match) because most LREs end up only
// being inputs to a MultiLRE; we never need their DFAs directly.
func (re *LRE) compile() {
	re.dfa = reCompileDFA(re.prog)
}

// A MultiLRE matches multiple LREs simultaneously against a text.
// It is more efficient than matching each LRE in sequence against the text.
type MultiLRE struct {
	dict *Dict // dict shared by all LREs
	dfa  reDFA // compiled DFA for all LREs

	// start contains the two-word phrases
	// where a match can validly start,
	// to allow for faster scans over non-license text.
	start map[phrase]struct{}
}

// A phrase is a phrase of up to two words.
// The zero-word phrase is phrase{NoWord, NoWord}.
// A single-word phrase w is phrase{w, NoWord}.
type phrase [2]WordID

// NewMultiLRE returns a MultiLRE looking for the given LREs.
// All the LREs must have been parsed using the same Dict;
// if not, NewMultiLRE panics.
func NewMultiLRE(list []*LRE) (_ *MultiLRE, err error) {
	if len(list) == 0 {
		return &MultiLRE{}, nil
	}

	dict := list[0].dict
	for _, sub := range list[1:] {
		if sub.dict != dict {
			panic("MultiRE: LREs parsed with different Dicts")
		}
	}

	var progs []reProg
	for _, sub := range list {
		progs = append(progs, sub.prog)
	}

	start := make(map[phrase]struct{})
	for _, sub := range list {
		phrases := sub.syntax.leadingPhrases()
		if len(phrases) == 0 {
			return nil, fmt.Errorf("%s: no leading phrases", sub.File())
		}
		for _, p := range phrases {
			if p[0] == BadWord {
				return nil, fmt.Errorf("%s: invalid pattern: matches empty text", sub.File())
			}
			if p[0] == AnyWord {
				if p[1] == BadWord {
					return nil, fmt.Errorf("%s: invalid pattern: matches a single wildcard", sub.File())
				}
				if p[1] == AnyWord {
					return nil, fmt.Errorf("%s: invalid pattern: begins with two wildcards", sub.File())
				}
				return nil, fmt.Errorf("%s: invalid pattern: begins with wildcard phrase: __ %s", sub.File(), dict.Words()[p[1]])
			}
			if p[1] == BadWord {
				return nil, fmt.Errorf("%s: invalid pattern: matches single word %s", sub.File(), dict.Words()[p[0]])
			}
			if p[1] == AnyWord {
				return nil, fmt.Errorf("%s: invalid pattern: begins with wildcard phrase: %s __", sub.File(), dict.Words()[p[0]])
			}
			start[p] = struct{}{}
		}
	}

	prog := reCompileMulti(progs)
	dfa := reCompileDFA(prog)

	return &MultiLRE{dict, dfa, start}, nil
}

// Dict returns the Dict used by the MultiLRE.
func (re *MultiLRE) Dict() *Dict {
	return re.dict
}

// A Matches is a collection of all leftmost-longest, non-overlapping matches in text.
type Matches struct {
	Text  string  // the entire text
	Words []Word  // the text, split into Words
	List  []Match // the matches
}

// A Match records the position of a single match in a text.
type Match struct {
	ID    int // index of LRE in list passed to NewMultiLRE
	Start int // word index of start of match
	End   int // word index of end of match
}

// Match reports all leftmost-longest, non-overlapping matches in text.
// It always returns a non-nil *Matches, in order to return the split text.
// Check len(matches.List) to see whether any matches were found.
func (re *MultiLRE) Match(text string) *Matches {
	m := &Matches{
		Text:  text,
		Words: re.dict.Split(text),
	}
	p := phrase{BadWord, BadWord}
	for i := 0; i < len(m.Words); i++ {
		p[0], p[1] = p[1], m.Words[i].ID
		if _, ok := re.start[p]; ok {
			match, end := re.dfa.match(re.dict, text, m.Words[i-1:])
			if match >= 0 && end > 0 {
				end += i - 1 // translate from index in m.Words[i-1:] to index in m.Words
				m.List = append(m.List, Match{ID: int(match), Start: i - 1, End: end})

				// Continue search at end of match.
				i = end - 1 // loop will i++
				p[0] = BadWord
				continue
			}
		}
	}
	return m
}
