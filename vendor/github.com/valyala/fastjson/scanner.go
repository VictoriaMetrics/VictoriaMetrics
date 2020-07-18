package fastjson

import (
	"errors"
)

// Scanner scans a series of JSON values. Values may be delimited by whitespace.
//
// Scanner may parse JSON lines ( http://jsonlines.org/ ).
//
// Scanner may be re-used for subsequent parsing.
//
// Scanner cannot be used from concurrent goroutines.
//
// Use Parser for parsing only a single JSON value.
type Scanner struct {
	// b contains a working copy of json value passed to Init.
	b []byte

	// s points to the next JSON value to parse.
	s string

	// err contains the last error.
	err error

	// v contains the last parsed JSON value.
	v *Value

	// c is used for caching JSON values.
	c cache
}

// Init initializes sc with the given s.
//
// s may contain multiple JSON values, which may be delimited by whitespace.
func (sc *Scanner) Init(s string) {
	sc.b = append(sc.b[:0], s...)
	sc.s = b2s(sc.b)
	sc.err = nil
	sc.v = nil
}

// InitBytes initializes sc with the given b.
//
// b may contain multiple JSON values, which may be delimited by whitespace.
func (sc *Scanner) InitBytes(b []byte) {
	sc.Init(b2s(b))
}

// Next parses the next JSON value from s passed to Init.
//
// Returns true on success. The parsed value is available via Value call.
//
// Returns false either on error or on the end of s.
// Call Error in order to determine the cause of the returned false.
func (sc *Scanner) Next() bool {
	if sc.err != nil {
		return false
	}

	sc.s = skipWS(sc.s)
	if len(sc.s) == 0 {
		sc.err = errEOF
		return false
	}

	sc.c.reset()
	v, tail, err := parseValue(sc.s, &sc.c, 0)
	if err != nil {
		sc.err = err
		return false
	}

	sc.s = tail
	sc.v = v
	return true
}

// Error returns the last error.
func (sc *Scanner) Error() error {
	if sc.err == errEOF {
		return nil
	}
	return sc.err
}

// Value returns the last parsed value.
//
// The value is valid until the Next call.
func (sc *Scanner) Value() *Value {
	return sc.v
}

var errEOF = errors.New("end of s")
