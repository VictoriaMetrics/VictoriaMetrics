// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

var (
	keyDiff = specificDiff("key")
	typDiff = specificDiff("type")
	valDiff = specificDiff("value")

	expectErrEOF = expectSpecificError(io.EOF)
	expectErrEOD = expectSpecificError(ErrEOD)
	expectErrEOA = expectSpecificError(ErrEOA)
)

type expectedErrorFunc func(t *testing.T, err error, desc string)

type peekTypeTestCase struct {
	desc  string
	input string
	typs  []bsontype.Type
	errFs []expectedErrorFunc
}

type readKeyValueTestCase struct {
	desc  string
	input string
	keys  []string
	typs  []bsontype.Type
	vals  []*extJSONValue

	keyEFs []expectedErrorFunc
	valEFs []expectedErrorFunc
}

func expectSpecificError(expected error) expectedErrorFunc {
	return func(t *testing.T, err error, desc string) {
		if err != expected {
			t.Helper()
			t.Errorf("%s: Expected %v but got: %v", desc, expected, err)
			t.FailNow()
		}
	}
}

func specificDiff(name string) func(t *testing.T, expected, actual interface{}, desc string) {
	return func(t *testing.T, expected, actual interface{}, desc string) {
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Helper()
			t.Errorf("%s: Incorrect JSON %s (-want, +got): %s\n", desc, name, diff)
			t.FailNow()
		}
	}
}

func expectErrorNOOP(_ *testing.T, _ error, _ string) {
}

func readKeyDiff(t *testing.T, eKey, aKey string, eTyp, aTyp bsontype.Type, err error, errF expectedErrorFunc, desc string) {
	keyDiff(t, eKey, aKey, desc)
	typDiff(t, eTyp, aTyp, desc)
	errF(t, err, desc)
}

func readValueDiff(t *testing.T, eVal, aVal *extJSONValue, err error, errF expectedErrorFunc, desc string) {
	if aVal != nil {
		typDiff(t, eVal.t, aVal.t, desc)
		valDiff(t, eVal.v, aVal.v, desc)
	} else {
		valDiff(t, eVal, aVal, desc)
	}

	errF(t, err, desc)
}

func TestExtJSONParserPeekType(t *testing.T) {
	makeValidPeekTypeTestCase := func(input string, typ bsontype.Type, desc string) peekTypeTestCase {
		return peekTypeTestCase{
			desc: desc, input: input,
			typs:  []bsontype.Type{typ},
			errFs: []expectedErrorFunc{expectNoError},
		}
	}
	makeInvalidTestCase := func(desc, input string, lastEF expectedErrorFunc) peekTypeTestCase {
		return peekTypeTestCase{
			desc: desc, input: input,
			typs:  []bsontype.Type{bsontype.Type(0)},
			errFs: []expectedErrorFunc{lastEF},
		}
	}

	makeInvalidPeekTypeTestCase := func(desc, input string, lastEF expectedErrorFunc) peekTypeTestCase {
		return peekTypeTestCase{
			desc: desc, input: input,
			typs:  []bsontype.Type{bsontype.Array, bsontype.String, bsontype.Type(0)},
			errFs: []expectedErrorFunc{expectNoError, expectNoError, lastEF},
		}
	}

	cases := []peekTypeTestCase{
		makeValidPeekTypeTestCase(`null`, bsontype.Null, "Null"),
		makeValidPeekTypeTestCase(`"string"`, bsontype.String, "String"),
		makeValidPeekTypeTestCase(`true`, bsontype.Boolean, "Boolean--true"),
		makeValidPeekTypeTestCase(`false`, bsontype.Boolean, "Boolean--false"),
		makeValidPeekTypeTestCase(`{"$minKey": 1}`, bsontype.MinKey, "MinKey"),
		makeValidPeekTypeTestCase(`{"$maxKey": 1}`, bsontype.MaxKey, "MaxKey"),
		makeValidPeekTypeTestCase(`{"$numberInt": "42"}`, bsontype.Int32, "Int32"),
		makeValidPeekTypeTestCase(`{"$numberLong": "42"}`, bsontype.Int64, "Int64"),
		makeValidPeekTypeTestCase(`{"$symbol": "symbol"}`, bsontype.Symbol, "Symbol"),
		makeValidPeekTypeTestCase(`{"$numberDouble": "42.42"}`, bsontype.Double, "Double"),
		makeValidPeekTypeTestCase(`{"$undefined": true}`, bsontype.Undefined, "Undefined"),
		makeValidPeekTypeTestCase(`{"$numberDouble": "NaN"}`, bsontype.Double, "Double--NaN"),
		makeValidPeekTypeTestCase(`{"$numberDecimal": "1234"}`, bsontype.Decimal128, "Decimal"),
		makeValidPeekTypeTestCase(`{"foo": "bar"}`, bsontype.EmbeddedDocument, "Toplevel document"),
		makeValidPeekTypeTestCase(`{"$date": {"$numberLong": "0"}}`, bsontype.DateTime, "Datetime"),
		makeValidPeekTypeTestCase(`{"$code": "function() {}"}`, bsontype.JavaScript, "Code no scope"),
		makeValidPeekTypeTestCase(`[{"$numberInt": "1"},{"$numberInt": "2"}]`, bsontype.Array, "Array"),
		makeValidPeekTypeTestCase(`{"$timestamp": {"t": 42, "i": 1}}`, bsontype.Timestamp, "Timestamp"),
		makeValidPeekTypeTestCase(`{"$oid": "57e193d7a9cc81b4027498b5"}`, bsontype.ObjectID, "Object ID"),
		makeValidPeekTypeTestCase(`{"$binary": {"base64": "AQIDBAU=", "subType": "80"}}`, bsontype.Binary, "Binary"),
		makeValidPeekTypeTestCase(`{"$code": "function() {}", "$scope": {}}`, bsontype.CodeWithScope, "Code With Scope"),
		makeValidPeekTypeTestCase(`{"$binary": {"base64": "o0w498Or7cijeBSpkquNtg==", "subType": "03"}}`, bsontype.Binary, "Binary"),
		makeValidPeekTypeTestCase(`{"$binary": "o0w498Or7cijeBSpkquNtg==", "$type": "03"}`, bsontype.Binary, "Binary"),
		makeValidPeekTypeTestCase(`{"$regularExpression": {"pattern": "foo*", "options": "ix"}}`, bsontype.Regex, "Regular expression"),
		makeValidPeekTypeTestCase(`{"$dbPointer": {"$ref": "db.collection", "$id": {"$oid": "57e193d7a9cc81b4027498b1"}}}`, bsontype.DBPointer, "DBPointer"),
		makeValidPeekTypeTestCase(`{"$ref": "collection", "$id": {"$oid": "57fd71e96e32ab4225b723fb"}, "$db": "database"}`, bsontype.EmbeddedDocument, "DBRef"),
		makeInvalidPeekTypeTestCase("invalid array--missing ]", `["a"`, expectError),
		makeInvalidPeekTypeTestCase("invalid array--colon in array", `["a":`, expectError),
		makeInvalidPeekTypeTestCase("invalid array--extra comma", `["a",,`, expectError),
		makeInvalidPeekTypeTestCase("invalid array--trailing comma", `["a",]`, expectError),
		makeInvalidPeekTypeTestCase("peekType after end of array", `["a"]`, expectErrEOA),
		{
			desc:  "invalid array--leading comma",
			input: `[,`,
			typs:  []bsontype.Type{bsontype.Array, bsontype.Type(0)},
			errFs: []expectedErrorFunc{expectNoError, expectError},
		},
		makeInvalidTestCase("lone $scope", `{"$scope": {}}`, expectError),
		makeInvalidTestCase("empty code with unknown extra key", `{"$code":"", "0":""}`, expectError),
		makeInvalidTestCase("non-empty code with unknown extra key", `{"$code":"foobar", "0":""}`, expectError),
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ejp := newExtJSONParser(strings.NewReader(tc.input), true)
			// Manually set the parser's starting state to jpsSawColon so peekType will read ahead to find the extjson
			// type of the value. If not set, the parser will be in jpsStartState and advance to jpsSawKey, which will
			// cause it to return without peeking the extjson type.
			ejp.s = jpsSawColon

			for i, eTyp := range tc.typs {
				errF := tc.errFs[i]

				typ, err := ejp.peekType()
				errF(t, err, tc.desc)
				if err != nil {
					// Don't inspect the type if there was an error
					return
				}

				typDiff(t, eTyp, typ, tc.desc)
			}
		})
	}
}

func TestExtJSONParserReadKeyReadValue(t *testing.T) {
	// several test cases will use the same keys, types, and values, and only differ on input structure

	keys := []string{"_id", "Symbol", "String", "Int32", "Int64", "Int", "MinKey"}
	types := []bsontype.Type{bsontype.ObjectID, bsontype.Symbol, bsontype.String, bsontype.Int32, bsontype.Int64, bsontype.Int32, bsontype.MinKey}
	values := []*extJSONValue{
		{t: bsontype.String, v: "57e193d7a9cc81b4027498b5"},
		{t: bsontype.String, v: "symbol"},
		{t: bsontype.String, v: "string"},
		{t: bsontype.String, v: "42"},
		{t: bsontype.String, v: "42"},
		{t: bsontype.Int32, v: int32(42)},
		{t: bsontype.Int32, v: int32(1)},
	}

	errFuncs := make([]expectedErrorFunc, 7)
	for i := 0; i < 7; i++ {
		errFuncs[i] = expectNoError
	}

	firstKeyError := func(desc, input string) readKeyValueTestCase {
		return readKeyValueTestCase{
			desc:   desc,
			input:  input,
			keys:   []string{""},
			typs:   []bsontype.Type{bsontype.Type(0)},
			vals:   []*extJSONValue{nil},
			keyEFs: []expectedErrorFunc{expectError},
			valEFs: []expectedErrorFunc{expectErrorNOOP},
		}
	}

	secondKeyError := func(desc, input, firstKey string, firstType bsontype.Type, firstValue *extJSONValue) readKeyValueTestCase {
		return readKeyValueTestCase{
			desc:   desc,
			input:  input,
			keys:   []string{firstKey, ""},
			typs:   []bsontype.Type{firstType, bsontype.Type(0)},
			vals:   []*extJSONValue{firstValue, nil},
			keyEFs: []expectedErrorFunc{expectNoError, expectError},
			valEFs: []expectedErrorFunc{expectNoError, expectErrorNOOP},
		}
	}

	cases := []readKeyValueTestCase{
		{
			desc: "normal spacing",
			input: `{
					"_id": { "$oid": "57e193d7a9cc81b4027498b5" },
					"Symbol": { "$symbol": "symbol" },
					"String": "string",
					"Int32": { "$numberInt": "42" },
					"Int64": { "$numberLong": "42" },
					"Int": 42,
					"MinKey": { "$minKey": 1 }
				}`,
			keys: keys, typs: types, vals: values,
			keyEFs: errFuncs, valEFs: errFuncs,
		},
		{
			desc: "new line before comma",
			input: `{ "_id": { "$oid": "57e193d7a9cc81b4027498b5" }
				 , "Symbol": { "$symbol": "symbol" }
				 , "String": "string"
				 , "Int32": { "$numberInt": "42" }
				 , "Int64": { "$numberLong": "42" }
				 , "Int": 42
				 , "MinKey": { "$minKey": 1 }
				 }`,
			keys: keys, typs: types, vals: values,
			keyEFs: errFuncs, valEFs: errFuncs,
		},
		{
			desc: "tabs around colons",
			input: `{
					"_id":    { "$oid"       : "57e193d7a9cc81b4027498b5" },
					"Symbol": { "$symbol"    : "symbol" },
					"String": "string",
					"Int32":  { "$numberInt" : "42" },
					"Int64":  { "$numberLong": "42" },
					"Int":    42,
					"MinKey": { "$minKey": 1 }
				}`,
			keys: keys, typs: types, vals: values,
			keyEFs: errFuncs, valEFs: errFuncs,
		},
		{
			desc:  "no whitespace",
			input: `{"_id":{"$oid":"57e193d7a9cc81b4027498b5"},"Symbol":{"$symbol":"symbol"},"String":"string","Int32":{"$numberInt":"42"},"Int64":{"$numberLong":"42"},"Int":42,"MinKey":{"$minKey":1}}`,
			keys:  keys, typs: types, vals: values,
			keyEFs: errFuncs, valEFs: errFuncs,
		},
		{
			desc: "mixed whitespace",
			input: `	{
					"_id"		: { "$oid": "57e193d7a9cc81b4027498b5" },
			        "Symbol"	: { "$symbol": "symbol" }	,
				    "String"	: "string",
					"Int32"		: { "$numberInt": "42" }    ,
					"Int64"		: {"$numberLong" : "42"},
					"Int"		: 42,
			      	"MinKey"	: { "$minKey": 1 } 	}	`,
			keys: keys, typs: types, vals: values,
			keyEFs: errFuncs, valEFs: errFuncs,
		},
		{
			desc:  "nested object",
			input: `{"k1": 1, "k2": { "k3": { "k4": 4 } }, "k5": 5}`,
			keys:  []string{"k1", "k2", "k3", "k4", "", "", "k5", ""},
			typs:  []bsontype.Type{bsontype.Int32, bsontype.EmbeddedDocument, bsontype.EmbeddedDocument, bsontype.Int32, bsontype.Type(0), bsontype.Type(0), bsontype.Int32, bsontype.Type(0)},
			vals: []*extJSONValue{
				{t: bsontype.Int32, v: int32(1)}, nil, nil, {t: bsontype.Int32, v: int32(4)}, nil, nil, {t: bsontype.Int32, v: int32(5)}, nil,
			},
			keyEFs: []expectedErrorFunc{
				expectNoError, expectNoError, expectNoError, expectNoError, expectErrEOD,
				expectErrEOD, expectNoError, expectErrEOD,
			},
			valEFs: []expectedErrorFunc{
				expectNoError, expectError, expectError, expectNoError, expectErrorNOOP,
				expectErrorNOOP, expectNoError, expectErrorNOOP,
			},
		},
		{
			desc:   "invalid input: invalid values for extended type",
			input:  `{"a": {"$numberInt": "1", "x"`,
			keys:   []string{"a"},
			typs:   []bsontype.Type{bsontype.Int32},
			vals:   []*extJSONValue{nil},
			keyEFs: []expectedErrorFunc{expectNoError},
			valEFs: []expectedErrorFunc{expectError},
		},
		firstKeyError("invalid input: missing key--EOF", "{"),
		firstKeyError("invalid input: missing key--colon first", "{:"),
		firstKeyError("invalid input: missing value", `{"a":`),
		firstKeyError("invalid input: missing colon", `{"a" 1`),
		firstKeyError("invalid input: extra colon", `{"a"::`),
		secondKeyError("invalid input: missing }", `{"a": 1`, "a", bsontype.Int32, &extJSONValue{t: bsontype.Int32, v: int32(1)}),
		secondKeyError("invalid input: missing comma", `{"a": 1 "b"`, "a", bsontype.Int32, &extJSONValue{t: bsontype.Int32, v: int32(1)}),
		secondKeyError("invalid input: extra comma", `{"a": 1,, "b"`, "a", bsontype.Int32, &extJSONValue{t: bsontype.Int32, v: int32(1)}),
		secondKeyError("invalid input: trailing comma in object", `{"a": 1,}`, "a", bsontype.Int32, &extJSONValue{t: bsontype.Int32, v: int32(1)}),
		{
			desc:   "invalid input: lone scope after a complete value",
			input:  `{"a": "", "b": {"$scope: ""}}`,
			keys:   []string{"a"},
			typs:   []bsontype.Type{bsontype.String},
			vals:   []*extJSONValue{{bsontype.String, ""}},
			keyEFs: []expectedErrorFunc{expectNoError, expectNoError},
			valEFs: []expectedErrorFunc{expectNoError, expectError},
		},
		{
			desc:   "invalid input: lone scope nested",
			input:  `{"a":{"b":{"$scope":{`,
			keys:   []string{},
			typs:   []bsontype.Type{},
			vals:   []*extJSONValue{nil},
			keyEFs: []expectedErrorFunc{expectNoError},
			valEFs: []expectedErrorFunc{expectError},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ejp := newExtJSONParser(strings.NewReader(tc.input), true)

			for i, eKey := range tc.keys {
				eTyp := tc.typs[i]
				eVal := tc.vals[i]

				keyErrF := tc.keyEFs[i]
				valErrF := tc.valEFs[i]

				k, typ, err := ejp.readKey()
				readKeyDiff(t, eKey, k, eTyp, typ, err, keyErrF, tc.desc)

				v, err := ejp.readValue(typ)
				readValueDiff(t, eVal, v, err, valErrF, tc.desc)
			}
		})
	}
}

type ejpExpectationTest func(t *testing.T, p *extJSONParser, expectedKey string, expectedType bsontype.Type, expectedValue interface{})

type ejpTestCase struct {
	f ejpExpectationTest
	p *extJSONParser
	k string
	t bsontype.Type
	v interface{}
}

// expectSingleValue is used for simple JSON types (strings, numbers, literals) and for extended JSON types that
// have single key-value pairs (i.e. { "$minKey": 1 }, { "$numberLong": "42.42" })
func expectSingleValue(t *testing.T, p *extJSONParser, expectedKey string, expectedType bsontype.Type, expectedValue interface{}) {
	eVal := expectedValue.(*extJSONValue)

	k, typ, err := p.readKey()
	readKeyDiff(t, expectedKey, k, expectedType, typ, err, expectNoError, expectedKey)

	v, err := p.readValue(typ)
	readValueDiff(t, eVal, v, err, expectNoError, expectedKey)
}

// expectMultipleValues is used for values that are subdocuments of known size and with known keys (such as extended
// JSON types { "$timestamp": {"t": 1, "i": 1} } and { "$regularExpression": {"pattern": "", options: ""} })
func expectMultipleValues(t *testing.T, p *extJSONParser, expectedKey string, expectedType bsontype.Type, expectedValue interface{}) {
	k, typ, err := p.readKey()
	readKeyDiff(t, expectedKey, k, expectedType, typ, err, expectNoError, expectedKey)

	v, err := p.readValue(typ)
	expectNoError(t, err, "")
	typDiff(t, bsontype.EmbeddedDocument, v.t, expectedKey)

	actObj := v.v.(*extJSONObject)
	expObj := expectedValue.(*extJSONObject)

	for i, actKey := range actObj.keys {
		expKey := expObj.keys[i]
		actVal := actObj.values[i]
		expVal := expObj.values[i]

		keyDiff(t, expKey, actKey, expectedKey)
		typDiff(t, expVal.t, actVal.t, expectedKey)
		valDiff(t, expVal.v, actVal.v, expectedKey)
	}
}

type ejpKeyTypValTriple struct {
	key string
	typ bsontype.Type
	val *extJSONValue
}

type ejpSubDocumentTestValue struct {
	code string               // code is only used for TypeCodeWithScope (and is ignored for TypeEmbeddedDocument
	ktvs []ejpKeyTypValTriple // list of (key, type, value) triples; this is "scope" for TypeCodeWithScope
}

// expectSubDocument is used for embedded documents and code with scope types; it reads all the keys and values
// in the embedded document (or scope for codeWithScope) and compares them to the expectedValue's list of (key, type,
// value) triples
func expectSubDocument(t *testing.T, p *extJSONParser, expectedKey string, expectedType bsontype.Type, expectedValue interface{}) {
	subdoc := expectedValue.(ejpSubDocumentTestValue)

	k, typ, err := p.readKey()
	readKeyDiff(t, expectedKey, k, expectedType, typ, err, expectNoError, expectedKey)

	if expectedType == bsontype.CodeWithScope {
		v, err := p.readValue(typ)
		readValueDiff(t, &extJSONValue{t: bsontype.String, v: subdoc.code}, v, err, expectNoError, expectedKey)
	}

	for _, ktv := range subdoc.ktvs {
		eKey := ktv.key
		eTyp := ktv.typ
		eVal := ktv.val

		k, typ, err = p.readKey()
		readKeyDiff(t, eKey, k, eTyp, typ, err, expectNoError, expectedKey)

		v, err := p.readValue(typ)
		readValueDiff(t, eVal, v, err, expectNoError, expectedKey)
	}

	if expectedType == bsontype.CodeWithScope {
		// expect scope doc to close
		k, typ, err = p.readKey()
		readKeyDiff(t, "", k, bsontype.Type(0), typ, err, expectErrEOD, expectedKey)
	}

	// expect subdoc to close
	k, typ, err = p.readKey()
	readKeyDiff(t, "", k, bsontype.Type(0), typ, err, expectErrEOD, expectedKey)
}

// expectArray takes the expectedKey, ignores the expectedType, and uses the expectedValue
// as a slice of (type Type, value *extJSONValue) pairs
func expectArray(t *testing.T, p *extJSONParser, expectedKey string, _ bsontype.Type, expectedValue interface{}) {
	ktvs := expectedValue.([]ejpKeyTypValTriple)

	k, typ, err := p.readKey()
	readKeyDiff(t, expectedKey, k, bsontype.Array, typ, err, expectNoError, expectedKey)

	for _, ktv := range ktvs {
		eTyp := ktv.typ
		eVal := ktv.val

		typ, err = p.peekType()
		typDiff(t, eTyp, typ, expectedKey)
		expectNoError(t, err, expectedKey)

		v, err := p.readValue(typ)
		readValueDiff(t, eVal, v, err, expectNoError, expectedKey)
	}

	// expect array to end
	typ, err = p.peekType()
	typDiff(t, bsontype.Type(0), typ, expectedKey)
	expectErrEOA(t, err, expectedKey)
}

func TestExtJSONParserAllTypes(t *testing.T) {
	in := ` { "_id"					: { "$oid": "57e193d7a9cc81b4027498b5"}
			, "Symbol"				: { "$symbol": "symbol"}
			, "String"				: "string"
			, "Int32"				: { "$numberInt": "42"}
			, "Int64"				: { "$numberLong": "42"}
			, "Double"				: { "$numberDouble": "42.42"}
			, "SpecialFloat"		: { "$numberDouble": "NaN" }
			, "Decimal"				: { "$numberDecimal": "1234" }
			, "Binary"			 	: { "$binary": { "base64": "o0w498Or7cijeBSpkquNtg==", "subType": "03" } }
			, "BinaryLegacy"  : { "$binary": "o0w498Or7cijeBSpkquNtg==", "$type": "03" }
			, "BinaryUserDefined"	: { "$binary": { "base64": "AQIDBAU=", "subType": "80" } }
			, "Code"				: { "$code": "function() {}" }
			, "CodeWithEmptyScope"	: { "$code": "function() {}", "$scope": {} }
			, "CodeWithScope"		: { "$code": "function() {}", "$scope": { "x": 1 } }
			, "EmptySubdocument"    : {}
			, "Subdocument"			: { "foo": "bar", "baz": { "$numberInt": "42" } }
			, "Array"				: [{"$numberInt": "1"}, {"$numberLong": "2"}, {"$numberDouble": "3"}, 4, "string", 5.0]
			, "Timestamp"			: { "$timestamp": { "t": 42, "i": 1 } }
			, "RegularExpression"	: { "$regularExpression": { "pattern": "foo*", "options": "ix" } }
			, "DatetimeEpoch"		: { "$date": { "$numberLong": "0" } }
			, "DatetimePositive"	: { "$date": { "$numberLong": "9223372036854775807" } }
			, "DatetimeNegative"	: { "$date": { "$numberLong": "-9223372036854775808" } }
			, "True"				: true
			, "False"				: false
			, "DBPointer"			: { "$dbPointer": { "$ref": "db.collection", "$id": { "$oid": "57e193d7a9cc81b4027498b1" } } }
			, "DBRef"				: { "$ref": "collection", "$id": { "$oid": "57fd71e96e32ab4225b723fb" }, "$db": "database" }
			, "DBRefNoDB"			: { "$ref": "collection", "$id": { "$oid": "57fd71e96e32ab4225b723fb" } }
			, "MinKey"				: { "$minKey": 1 }
			, "MaxKey"				: { "$maxKey": 1 }
			, "Null"				: null
			, "Undefined"			: { "$undefined": true }
			}`

	ejp := newExtJSONParser(strings.NewReader(in), true)

	cases := []ejpTestCase{
		{
			f: expectSingleValue, p: ejp,
			k: "_id", t: bsontype.ObjectID, v: &extJSONValue{t: bsontype.String, v: "57e193d7a9cc81b4027498b5"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Symbol", t: bsontype.Symbol, v: &extJSONValue{t: bsontype.String, v: "symbol"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "String", t: bsontype.String, v: &extJSONValue{t: bsontype.String, v: "string"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Int32", t: bsontype.Int32, v: &extJSONValue{t: bsontype.String, v: "42"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Int64", t: bsontype.Int64, v: &extJSONValue{t: bsontype.String, v: "42"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Double", t: bsontype.Double, v: &extJSONValue{t: bsontype.String, v: "42.42"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "SpecialFloat", t: bsontype.Double, v: &extJSONValue{t: bsontype.String, v: "NaN"},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Decimal", t: bsontype.Decimal128, v: &extJSONValue{t: bsontype.String, v: "1234"},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "Binary", t: bsontype.Binary,
			v: &extJSONObject{
				keys: []string{"base64", "subType"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "o0w498Or7cijeBSpkquNtg=="},
					{t: bsontype.String, v: "03"},
				},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "BinaryLegacy", t: bsontype.Binary,
			v: &extJSONObject{
				keys: []string{"base64", "subType"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "o0w498Or7cijeBSpkquNtg=="},
					{t: bsontype.String, v: "03"},
				},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "BinaryUserDefined", t: bsontype.Binary,
			v: &extJSONObject{
				keys: []string{"base64", "subType"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "AQIDBAU="},
					{t: bsontype.String, v: "80"},
				},
			},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Code", t: bsontype.JavaScript, v: &extJSONValue{t: bsontype.String, v: "function() {}"},
		},
		{
			f: expectSubDocument, p: ejp,
			k: "CodeWithEmptyScope", t: bsontype.CodeWithScope,
			v: ejpSubDocumentTestValue{
				code: "function() {}",
				ktvs: []ejpKeyTypValTriple{},
			},
		},
		{
			f: expectSubDocument, p: ejp,
			k: "CodeWithScope", t: bsontype.CodeWithScope,
			v: ejpSubDocumentTestValue{
				code: "function() {}",
				ktvs: []ejpKeyTypValTriple{
					{"x", bsontype.Int32, &extJSONValue{t: bsontype.Int32, v: int32(1)}},
				},
			},
		},
		{
			f: expectSubDocument, p: ejp,
			k: "EmptySubdocument", t: bsontype.EmbeddedDocument,
			v: ejpSubDocumentTestValue{
				ktvs: []ejpKeyTypValTriple{},
			},
		},
		{
			f: expectSubDocument, p: ejp,
			k: "Subdocument", t: bsontype.EmbeddedDocument,
			v: ejpSubDocumentTestValue{
				ktvs: []ejpKeyTypValTriple{
					{"foo", bsontype.String, &extJSONValue{t: bsontype.String, v: "bar"}},
					{"baz", bsontype.Int32, &extJSONValue{t: bsontype.String, v: "42"}},
				},
			},
		},
		{
			f: expectArray, p: ejp,
			k: "Array", t: bsontype.Array,
			v: []ejpKeyTypValTriple{
				{typ: bsontype.Int32, val: &extJSONValue{t: bsontype.String, v: "1"}},
				{typ: bsontype.Int64, val: &extJSONValue{t: bsontype.String, v: "2"}},
				{typ: bsontype.Double, val: &extJSONValue{t: bsontype.String, v: "3"}},
				{typ: bsontype.Int32, val: &extJSONValue{t: bsontype.Int32, v: int32(4)}},
				{typ: bsontype.String, val: &extJSONValue{t: bsontype.String, v: "string"}},
				{typ: bsontype.Double, val: &extJSONValue{t: bsontype.Double, v: 5.0}},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "Timestamp", t: bsontype.Timestamp,
			v: &extJSONObject{
				keys: []string{"t", "i"},
				values: []*extJSONValue{
					{t: bsontype.Int32, v: int32(42)},
					{t: bsontype.Int32, v: int32(1)},
				},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "RegularExpression", t: bsontype.Regex,
			v: &extJSONObject{
				keys: []string{"pattern", "options"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "foo*"},
					{t: bsontype.String, v: "ix"},
				},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "DatetimeEpoch", t: bsontype.DateTime,
			v: &extJSONObject{
				keys: []string{"$numberLong"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "0"},
				},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "DatetimePositive", t: bsontype.DateTime,
			v: &extJSONObject{
				keys: []string{"$numberLong"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "9223372036854775807"},
				},
			},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "DatetimeNegative", t: bsontype.DateTime,
			v: &extJSONObject{
				keys: []string{"$numberLong"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "-9223372036854775808"},
				},
			},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "True", t: bsontype.Boolean, v: &extJSONValue{t: bsontype.Boolean, v: true},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "False", t: bsontype.Boolean, v: &extJSONValue{t: bsontype.Boolean, v: false},
		},
		{
			f: expectMultipleValues, p: ejp,
			k: "DBPointer", t: bsontype.DBPointer,
			v: &extJSONObject{
				keys: []string{"$ref", "$id"},
				values: []*extJSONValue{
					{t: bsontype.String, v: "db.collection"},
					{t: bsontype.String, v: "57e193d7a9cc81b4027498b1"},
				},
			},
		},
		{
			f: expectSubDocument, p: ejp,
			k: "DBRef", t: bsontype.EmbeddedDocument,
			v: ejpSubDocumentTestValue{
				ktvs: []ejpKeyTypValTriple{
					{"$ref", bsontype.String, &extJSONValue{t: bsontype.String, v: "collection"}},
					{"$id", bsontype.ObjectID, &extJSONValue{t: bsontype.String, v: "57fd71e96e32ab4225b723fb"}},
					{"$db", bsontype.String, &extJSONValue{t: bsontype.String, v: "database"}},
				},
			},
		},
		{
			f: expectSubDocument, p: ejp,
			k: "DBRefNoDB", t: bsontype.EmbeddedDocument,
			v: ejpSubDocumentTestValue{
				ktvs: []ejpKeyTypValTriple{
					{"$ref", bsontype.String, &extJSONValue{t: bsontype.String, v: "collection"}},
					{"$id", bsontype.ObjectID, &extJSONValue{t: bsontype.String, v: "57fd71e96e32ab4225b723fb"}},
				},
			},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "MinKey", t: bsontype.MinKey, v: &extJSONValue{t: bsontype.Int32, v: int32(1)},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "MaxKey", t: bsontype.MaxKey, v: &extJSONValue{t: bsontype.Int32, v: int32(1)},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Null", t: bsontype.Null, v: &extJSONValue{t: bsontype.Null, v: nil},
		},
		{
			f: expectSingleValue, p: ejp,
			k: "Undefined", t: bsontype.Undefined, v: &extJSONValue{t: bsontype.Boolean, v: true},
		},
	}

	// run the test cases
	for _, tc := range cases {
		tc.f(t, tc.p, tc.k, tc.t, tc.v)
	}

	// expect end of whole document: read final }
	k, typ, err := ejp.readKey()
	readKeyDiff(t, "", k, bsontype.Type(0), typ, err, expectErrEOD, "")

	// expect end of whole document: read EOF
	k, typ, err = ejp.readKey()
	readKeyDiff(t, "", k, bsontype.Type(0), typ, err, expectErrEOF, "")
	if diff := cmp.Diff(jpsDoneState, ejp.s); diff != "" {
		t.Errorf("expected parser to be in done state but instead is in %v\n", ejp.s)
		t.FailNow()
	}
}

func TestExtJSONValue(t *testing.T) {
	t.Run("Large Date", func(t *testing.T) {
		val := &extJSONValue{
			t: bsontype.String,
			v: "3001-01-01T00:00:00Z",
		}

		intVal, err := val.parseDateTime()
		if err != nil {
			t.Fatalf("error parsing date time: %v", err)
		}

		if intVal <= 0 {
			t.Fatalf("expected value above 0, got %v", intVal)
		}
	})
	t.Run("fallback time format", func(t *testing.T) {
		val := &extJSONValue{
			t: bsontype.String,
			v: "2019-06-04T14:54:31.416+0000",
		}

		_, err := val.parseDateTime()
		if err != nil {
			t.Fatalf("error parsing date time: %v", err)
		}
	})
}
