// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package http

import (
	"fmt"
	"net/http"
	"strconv"
)

// Error represent error response from InfluxDBServer or http error
type Error struct {
	StatusCode int
	Code       string
	Message    string
	Err        error
	RetryAfter uint
	Header     http.Header
}

// Error fulfils error interface
func (e *Error) Error() string {
	switch {
	case e.Err != nil:
		return e.Err.Error()
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	default:
		return "Unexpected status code " + strconv.Itoa(e.StatusCode)
	}
}

func (e *Error) Unwrap() error {
	if e.Err != nil {
		return e.Err
	}
	return nil
}

// HeaderToString generates a string value from the Header property.  Useful in logging.
func (e *Error) HeaderToString(selected []string) string {
	headerString := ""
	if len(selected) == 0 {
		for key := range e.Header {
			k := http.CanonicalHeaderKey(key)
			headerString += fmt.Sprintf("%s: %s\r\n", k, e.Header.Get(k))
		}
	} else {
		for _, candidate := range selected {
			c := http.CanonicalHeaderKey(candidate)
			if e.Header.Get(c) != "" {
				headerString += fmt.Sprintf("%s: %s\n", c, e.Header.Get(c))
			}
		}
	}
	return headerString
}

// NewError returns newly created Error initialised with nested error and default values
func NewError(err error) *Error {
	return &Error{
		StatusCode: 0,
		Code:       "",
		Message:    "",
		Err:        err,
		RetryAfter: 0,
		Header:     http.Header{},
	}
}
