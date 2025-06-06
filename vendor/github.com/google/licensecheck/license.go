// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package licensecheck classifies license files and heuristically determines
// how well they correspond to known open source licenses.
//
// Scanning
//
// A text (a slice of bytes) can be scanned for known licenses by calling Scan.
// The resulting Coverage structure describes all the matches found as well
// as what percentage of the file was covered by known matches.
//
//	cov := licensecheck.Scan(text)
//	fmt.Printf("%.1f%% of text covered by licenses:\n", cov.Percent)
//	for _, m := range cov.Match {
//		fmt.Printf("%s at [%d:%d] IsURL=%v\n", m.Name, m.Start, m.End, m.IsURL)
//	}
//
// The Scan function uses a built-in license set, which is the known SPDX licenses
// augmented with some other commonly seen licenses.
// (See licenses/README.md for details about the license set.)
//
// A custom scanner can be created using NewScanner, passing in a set of
// license patterns to scan for. The license patterns are written as license regular expressions (LREs).
// BuiltinLicenses returns the set of license patterns used by Scan.
//
// License Regular Expressions
// Each license to be recognized is specified by writing a license regular expression (LRE) for it.
// The pattern syntax and the matching are word-based and case-insensitive;
// punctuation is ignored in the pattern and in the matched text.
//
// The valid LRE patterns are:
//
//  - word, a single case-insensitive word
//  - __N__, any sequence of up to N words
//  - expr1 expr2, concatenation of two expressions
//  - expr1 || expr2, alternation of two expressions
//  - (( expr )), grouping
//  - (( expr ))??, zero or one instances of the grouped expression
//  - //** text **//, a comment ignored by the parser
//
// To make patterns harder to misread in large texts:
// (( must only appear at the start of a line (possibly indented);
// )) and ))?? must only appear at the end of a line (with possible trailing spaces);
// and || must only appear inside a (( )) or (( ))?? group.
//
// For example:
//
// 	//** https://en.wikipedia.org/wiki/Filler_text **//
// 	Now is
// 	((not))??
// 	the time for all good
// 	((men || women || people))
// 	to come to the aid of their __1__.
//
// The Old Cover and Checker API
//
// An older, less precise matcher using the names Cover, New, and Checker
// was removed from this package.
// Use v0.1.0 for the final version of that API.
//
package licensecheck

import (
	"fmt"
	"strconv"
	"strings"
)

// The order matters here.
// gen_data.go imports licensecheck for Type, so we copy over
// a trivial data.gen.go in order to build gen_data.go during "go run".
//go:generate cp data.gen.go.triv data.gen.go
//go:generate go run gen_data.go

// A License describes a single license that can be recognized.
// At least one of LRE or URL should be set.
type License struct {
	ID   string // reported license ID
	Type Type   // reported license type
	LRE  string // license regular expression (see licenses/README.md)
	URL  string // identifying URL
}

// Coverage describes how the text matches various licenses.
type Coverage struct {
	// Percent is the fraction of the total text, in normalized words, that
	// matches any valid license, expressed as a percentage across all of the
	// licenses matched.
	Percent float64

	// Match describes, in sequential order, the matches of the input text
	// across the various licenses. Typically it will be only one match long,
	// but if the input text is a concatenation of licenses it will contain
	// a match value for each element of the concatenation.
	Match []Match
}

// When we build the Match, Start and End are word offsets,
// but they are converted to byte offsets in the original
// before being passed back to the caller.

// Match describes how a section of the input matches a license.
type Match struct {
	ID    string // License identifier. (See licenses/README.md.)
	Type  Type   // The type of the license: BSD, MIT, etc.
	Start int    // Start offset of match in text; match is at text[Start:End].
	End   int    // End offset of match in text.
	IsURL bool   // Whether match is a URL.
}

// Type is a bit set describing the requirements imposed by a license or group of licenses.
type Type uint

const (
	// Unknown indicates that the license requirements are unknown.
	Unknown Type = 0

	// Unrestricted indicates that the license carries no requirements.
	// Examples: 0BSD, CC0, Unlicense.
	Unrestricted Type = 1 << iota

	// Notice indicates that the license carries notice or advertising requirements.
	// Examples: MIT, BSD, Apache 1.1, Apache 2.0
	Notice

	// ShareChanges indicates that source and binary distributions must include
	// source code for any modifications made to the original code itself,
	// made available under the same license as the original.
	// Example: LGPL.
	ShareChanges

	// ShareProgram indicates that source and binary distributions must include
	// source code for the entire program using the original code library,
	// made available under the same license as the original.
	// Example: GPL.
	ShareProgram

	// ShareServer indicates that any server program making use of the source code
	// must provide a download link for the entire source code of the server.
	// Examples: AGPL, CPAL, EUPL, SSPL.
	ShareServer

	// NonCommercial indicates that commercial use of the software is disallowed.
	// Examples: CommonsClause, CC-BY-NC-4.0, Prosperity 3.0.0.
	NonCommercial

	// Discouraged indicates that there is some legal problem with the license,
	// making it difficult to comply with or vague about what it permits.
	// Examples: Beerware, SISSL, WTFPL.
	Discouraged
)

// Merge returns the result of merging the requirements of license types t and u.
//
// If either is Unknown, the result is Unknown.
// Among the bits Unrestricted, Notice, ShareChanges, ShareProgram, ShareServer,
// the result will use the one that appears latest in the list and is present in either t or u.
// The NonCommercial and Discouraged bits are set in the result if they are set in either t or u.
func (t Type) Merge(u Type) Type {
	if t == Unknown || u == Unknown {
		return Unknown
	}

	var m Type
	for bit := ShareServer; bit > 0; bit >>= 1 {
		if (t|u)&bit != 0 {
			m |= bit
			break
		}
	}
	m |= (t | u) & (NonCommercial | Discouraged)

	// Special case: NonCommercial is a restriction,
	// so drop the unrestricted bit if still set.
	if m&Unrestricted != 0 && m&NonCommercial != 0 {
		m &^= Unrestricted
	}

	return m
}

var typeBits = []struct {
	t Type
	s string
}{
	{Unknown, "Unknown"},
	{Unrestricted, "Unrestricted"},
	{Notice, "Notice"},
	{ShareChanges, "ShareChanges"},
	{ShareProgram, "ShareProgram"},
	{ShareServer, "ShareServer"},
	{NonCommercial, "NonCommercial"},
	{Discouraged, "Discouraged"},
}

// String returns the type t in string form.
// The form is a |-separated list of bit values (Notice, NonCommercial, and so on)
// with a final Type(0x###) if needed.
//
// For example, (ShareServer|NonCommercial).String() = "ShareServer|NonCommercial".
func (t Type) String() string {
	if t == 0 {
		return "Unknown"
	}
	s := ""
	for _, b := range typeBits {
		if b.t != 0 && t&b.t == b.t {
			t &^= b.t
			s += "|" + b.s
		}
	}
	if t != 0 {
		s += fmt.Sprintf("|Type(%#x)", uint(t))
	}
	return s[1:]
}

// ParseType parses s into a Type.
// The string s should be of the same form returned by Type's String method.
func ParseType(s string) (Type, error) {
	var t Type
Fields:
	for _, f := range strings.Split(s, "|") {
		for _, b := range typeBits {
			if b.s == f {
				t |= b.t
				continue Fields
			}
		}
		if strings.HasPrefix(f, "Type(") && strings.HasSuffix(f, ")") {
			if n, err := strconv.ParseUint(f[5:len(f)-1], 0, 0); err == nil {
				t |= Type(n)
				continue Fields
			}
		}
		if f == "" {
			return 0, fmt.Errorf("parsing %q: malformed Type", s)
		}
		return 0, fmt.Errorf("parsing %q: unknown Type %q", s, f)
	}
	return t, nil
}
