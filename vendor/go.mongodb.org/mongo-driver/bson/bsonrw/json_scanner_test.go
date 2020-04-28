// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"strings"
	"testing"
	"testing/iotest"

	"github.com/google/go-cmp/cmp"
)

func jttDiff(t *testing.T, expected, actual jsonTokenType, desc string) {
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Helper()
		t.Errorf("%s: Incorrect JSON Token Type (-want, +got): %s\n", desc, diff)
		t.FailNow()
	}
}

func jtvDiff(t *testing.T, expected, actual interface{}, desc string) {
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Helper()
		t.Errorf("%s: Incorrect JSON Token Value (-want, +got): %s\n", desc, diff)
		t.FailNow()
	}
}

func expectNilToken(t *testing.T, v *jsonToken, desc string) {
	if v != nil {
		t.Helper()
		t.Errorf("%s: Expected nil JSON token", desc)
		t.FailNow()
	}
}

func expectError(t *testing.T, err error, desc string) {
	if err == nil {
		t.Helper()
		t.Errorf("%s: Expected error", desc)
		t.FailNow()
	}
}

func expectNoError(t *testing.T, err error, desc string) {
	if err != nil {
		t.Helper()
		t.Errorf("%s: Unepexted error: %v", desc, err)
		t.FailNow()
	}
}

type jsonScannerTestCase struct {
	desc   string
	input  string
	tokens []jsonToken
}

// length = 512
const longKey = "abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz" +
	"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr"

func TestJsonScannerValidInputs(t *testing.T) {
	cases := []jsonScannerTestCase{
		{
			desc: "empty input", input: "",
			tokens: []jsonToken{},
		},
		{
			desc: "empty object", input: "{}",
			tokens: []jsonToken{{t: jttBeginObject, v: byte('{')}, {t: jttEndObject, v: byte('}')}},
		},
		{
			desc: "empty array", input: "[]",
			tokens: []jsonToken{{t: jttBeginArray, v: byte('[')}, {t: jttEndArray, v: byte(']')}},
		},
		{
			desc: "valid empty string", input: `""`,
			tokens: []jsonToken{{t: jttString, v: ""}},
		},
		{
			desc:   "valid string--no escaped characters",
			input:  `"string"`,
			tokens: []jsonToken{{t: jttString, v: "string"}},
		},
		{
			desc:   "valid string--escaped characters",
			input:  `"\"\\\/\b\f\n\r\t"`,
			tokens: []jsonToken{{t: jttString, v: "\"\\/\b\f\n\r\t"}},
		},
		{
			desc: "valid literal--true", input: "true",
			tokens: []jsonToken{{t: jttBool, v: true}},
		},
		{
			desc: "valid literal--false", input: "false",
			tokens: []jsonToken{{t: jttBool, v: false}},
		},
		{
			desc: "valid literal--null", input: "null",
			tokens: []jsonToken{{t: jttNull}},
		},
		{
			desc: "valid int32: 0", input: "0",
			tokens: []jsonToken{{t: jttInt32, v: int32(0)}},
		},
		{
			desc: "valid int32: -0", input: "-0",
			tokens: []jsonToken{{t: jttInt32, v: int32(0)}},
		},
		{
			desc: "valid int32: 1", input: "1",
			tokens: []jsonToken{{t: jttInt32, v: int32(1)}},
		},
		{
			desc: "valid int32: -1", input: "-1",
			tokens: []jsonToken{{t: jttInt32, v: int32(-1)}},
		},
		{
			desc: "valid int32: 10", input: "10",
			tokens: []jsonToken{{t: jttInt32, v: int32(10)}},
		},
		{
			desc: "valid int32: 1234", input: "1234",
			tokens: []jsonToken{{t: jttInt32, v: int32(1234)}},
		},
		{
			desc: "valid int32: -10", input: "-10",
			tokens: []jsonToken{{t: jttInt32, v: int32(-10)}},
		},
		{
			desc: "valid int32: -1234", input: "-1234",
			tokens: []jsonToken{{t: jttInt32, v: int32(-1234)}},
		},
		{
			desc: "valid int64: 2147483648", input: "2147483648",
			tokens: []jsonToken{{t: jttInt64, v: int64(2147483648)}},
		},
		{
			desc: "valid int64: -2147483649", input: "-2147483649",
			tokens: []jsonToken{{t: jttInt64, v: int64(-2147483649)}},
		},
		{
			desc: "valid double: 0.0", input: "0.0",
			tokens: []jsonToken{{t: jttDouble, v: 0.0}},
		},
		{
			desc: "valid double: -0.0", input: "-0.0",
			tokens: []jsonToken{{t: jttDouble, v: 0.0}},
		},
		{
			desc: "valid double: 0.1", input: "0.1",
			tokens: []jsonToken{{t: jttDouble, v: 0.1}},
		},
		{
			desc: "valid double: 0.1234", input: "0.1234",
			tokens: []jsonToken{{t: jttDouble, v: 0.1234}},
		},
		{
			desc: "valid double: 1.0", input: "1.0",
			tokens: []jsonToken{{t: jttDouble, v: 1.0}},
		},
		{
			desc: "valid double: -1.0", input: "-1.0",
			tokens: []jsonToken{{t: jttDouble, v: -1.0}},
		},
		{
			desc: "valid double: 1.234", input: "1.234",
			tokens: []jsonToken{{t: jttDouble, v: 1.234}},
		},
		{
			desc: "valid double: -1.234", input: "-1.234",
			tokens: []jsonToken{{t: jttDouble, v: -1.234}},
		},
		{
			desc: "valid double: 1e10", input: "1e10",
			tokens: []jsonToken{{t: jttDouble, v: 1e+10}},
		},
		{
			desc: "valid double: 1E10", input: "1E10",
			tokens: []jsonToken{{t: jttDouble, v: 1e+10}},
		},
		{
			desc: "valid double: 1.2e10", input: "1.2e10",
			tokens: []jsonToken{{t: jttDouble, v: 1.2e+10}},
		},
		{
			desc: "valid double: 1.2E10", input: "1.2E10",
			tokens: []jsonToken{{t: jttDouble, v: 1.2e+10}},
		},
		{
			desc: "valid double: -1.2e10", input: "-1.2e10",
			tokens: []jsonToken{{t: jttDouble, v: -1.2e+10}},
		},
		{
			desc: "valid double: -1.2E10", input: "-1.2E10",
			tokens: []jsonToken{{t: jttDouble, v: -1.2e+10}},
		},
		{
			desc: "valid double: -1.2e+10", input: "-1.2e+10",
			tokens: []jsonToken{{t: jttDouble, v: -1.2e+10}},
		},
		{
			desc: "valid double: -1.2E+10", input: "-1.2E+10",
			tokens: []jsonToken{{t: jttDouble, v: -1.2e+10}},
		},
		{
			desc: "valid double: 1.2e-10", input: "1.2e-10",
			tokens: []jsonToken{{t: jttDouble, v: 1.2e-10}},
		},
		{
			desc: "valid double: 1.2E-10", input: "1.2e-10",
			tokens: []jsonToken{{t: jttDouble, v: 1.2e-10}},
		},
		{
			desc: "valid double: -1.2e-10", input: "-1.2e-10",
			tokens: []jsonToken{{t: jttDouble, v: -1.2e-10}},
		},
		{
			desc: "valid double: -1.2E-10", input: "-1.2E-10",
			tokens: []jsonToken{{t: jttDouble, v: -1.2e-10}},
		},
		{
			desc: "valid double: 8005332285744496613785600", input: "8005332285744496613785600",
			tokens: []jsonToken{{t: jttDouble, v: float64(8005332285744496613785600)}},
		},
		{
			desc:  "valid object, only spaces",
			input: `{"key": "string", "key2": 2, "key3": {}, "key4": [], "key5": false }`,
			tokens: []jsonToken{
				{t: jttBeginObject, v: byte('{')}, {t: jttString, v: "key"}, {t: jttColon, v: byte(':')}, {t: jttString, v: "string"},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key2"}, {t: jttColon, v: byte(':')}, {t: jttInt32, v: int32(2)},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key3"}, {t: jttColon, v: byte(':')}, {t: jttBeginObject, v: byte('{')}, {t: jttEndObject, v: byte('}')},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key4"}, {t: jttColon, v: byte(':')}, {t: jttBeginArray, v: byte('[')}, {t: jttEndArray, v: byte(']')},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key5"}, {t: jttColon, v: byte(':')}, {t: jttBool, v: false}, {t: jttEndObject, v: byte('}')},
			},
		},
		{
			desc: "valid object, mixed whitespace",
			input: `
					{ "key" : "string"
					, "key2": 2
					, "key3": {}
					, "key4": []
					, "key5": false
					}`,
			tokens: []jsonToken{
				{t: jttBeginObject, v: byte('{')}, {t: jttString, v: "key"}, {t: jttColon, v: byte(':')}, {t: jttString, v: "string"},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key2"}, {t: jttColon, v: byte(':')}, {t: jttInt32, v: int32(2)},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key3"}, {t: jttColon, v: byte(':')}, {t: jttBeginObject, v: byte('{')}, {t: jttEndObject, v: byte('}')},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key4"}, {t: jttColon, v: byte(':')}, {t: jttBeginArray, v: byte('[')}, {t: jttEndArray, v: byte(']')},
				{t: jttComma, v: byte(',')}, {t: jttString, v: "key5"}, {t: jttColon, v: byte(':')}, {t: jttBool, v: false}, {t: jttEndObject, v: byte('}')},
			},
		},
		{
			desc:  "input greater than buffer size",
			input: `{"` + longKey + `": 1}`,
			tokens: []jsonToken{
				{t: jttBeginObject, v: byte('{')}, {t: jttString, v: longKey}, {t: jttColon, v: byte(':')},
				{t: jttInt32, v: int32(1)}, {t: jttEndObject, v: byte('}')},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			js := &jsonScanner{r: strings.NewReader(tc.input)}

			for _, token := range tc.tokens {
				c, err := js.nextToken()
				jttDiff(t, token.t, c.t, tc.desc)
				jtvDiff(t, token.v, c.v, tc.desc)
				expectNoError(t, err, tc.desc)
			}

			c, err := js.nextToken()
			jttDiff(t, jttEOF, c.t, tc.desc)
			noerr(t, err)

			// testing early EOF reading
			js = &jsonScanner{r: iotest.DataErrReader(strings.NewReader(tc.input))}

			for _, token := range tc.tokens {
				c, err := js.nextToken()
				jttDiff(t, token.t, c.t, tc.desc)
				jtvDiff(t, token.v, c.v, tc.desc)
				expectNoError(t, err, tc.desc)
			}

			c, err = js.nextToken()
			jttDiff(t, jttEOF, c.t, tc.desc)
			noerr(t, err)
		})
	}
}

func TestJsonScannerInvalidInputs(t *testing.T) {
	cases := []jsonScannerTestCase{
		{desc: "missing quotation", input: `"missing`},
		{desc: "invalid escape character--first character", input: `"\invalid"`},
		{desc: "invalid escape character--middle", input: `"i\nv\alid"`},
		{desc: "invalid escape character--single quote", input: `"f\'oo"`},
		{desc: "invalid literal--trueee", input: "trueee"},
		{desc: "invalid literal--tire", input: "tire"},
		{desc: "invalid literal--nulll", input: "nulll"},
		{desc: "invalid literal--fals", input: "fals"},
		{desc: "invalid literal--falsee", input: "falsee"},
		{desc: "invalid literal--fake", input: "fake"},
		{desc: "invalid literal--bad", input: "bad"},
		{desc: "invalid number: -", input: "-"},
		{desc: "invalid number: --0", input: "--0"},
		{desc: "invalid number: -a", input: "-a"},
		{desc: "invalid number: 00", input: "00"},
		{desc: "invalid number: 01", input: "01"},
		{desc: "invalid number: 0-", input: "0-"},
		{desc: "invalid number: 1-", input: "1-"},
		{desc: "invalid number: 0..", input: "0.."},
		{desc: "invalid number: 0.-", input: "0.-"},
		{desc: "invalid number: 0..0", input: "0..0"},
		{desc: "invalid number: 0.1.0", input: "0.1.0"},
		{desc: "invalid number: 0e", input: "0e"},
		{desc: "invalid number: 0e.", input: "0e."},
		{desc: "invalid number: 0e1.", input: "0e1."},
		{desc: "invalid number: 0e1e", input: "0e1e"},
		{desc: "invalid number: 0e+.1", input: "0e+.1"},
		{desc: "invalid number: 0e+1.", input: "0e+1."},
		{desc: "invalid number: 0e+1e", input: "0e+1e"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			js := &jsonScanner{r: strings.NewReader(tc.input)}

			c, err := js.nextToken()
			expectNilToken(t, c, tc.desc)
			expectError(t, err, tc.desc)
		})
	}
}
