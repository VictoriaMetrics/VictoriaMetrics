// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bson

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/pretty"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type testCase struct {
	Description  string                `json:"description"`
	BsonType     string                `json:"bson_type"`
	TestKey      *string               `json:"test_key"`
	Valid        []validityTestCase    `json:"valid"`
	DecodeErrors []decodeErrorTestCase `json:"decodeErrors"`
	ParseErrors  []parseErrorTestCase  `json:"parseErrors"`
	Deprecated   *bool                 `json:"deprecated"`
}

type validityTestCase struct {
	Description       string  `json:"description"`
	CanonicalBson     string  `json:"canonical_bson"`
	CanonicalExtJSON  string  `json:"canonical_extjson"`
	RelaxedExtJSON    *string `json:"relaxed_extjson"`
	DegenerateBSON    *string `json:"degenerate_bson"`
	DegenerateExtJSON *string `json:"degenerate_extjson"`
	ConvertedBSON     *string `json:"converted_bson"`
	ConvertedExtJSON  *string `json:"converted_extjson"`
	Lossy             *bool   `json:"lossy"`
}

type decodeErrorTestCase struct {
	Description string `json:"description"`
	Bson        string `json:"bson"`
}

type parseErrorTestCase struct {
	Description string `json:"description"`
	String      string `json:"string"`
}

const dataDir = "../data"

var dvd bsoncodec.DefaultValueDecoders
var dve bsoncodec.DefaultValueEncoders

var dc = bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()}
var ec = bsoncodec.EncodeContext{Registry: NewRegistryBuilder().Build()}

func findJSONFilesInDir(t *testing.T, dir string) []string {
	files := make([]string, 0)

	entries, err := ioutil.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}

		files = append(files, entry.Name())
	}

	return files
}

func needsEscapedUnicode(bsonType string) bool {
	return bsonType == "0x02" || bsonType == "0x0D" || bsonType == "0x0E" || bsonType == "0x0F"
}

func unescapeUnicode(s, bsonType string) string {
	if !needsEscapedUnicode(bsonType) {
		return s
	}

	newS := ""

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			switch s[i+1] {
			case 'u':
				us := s[i : i+6]
				u, err := strconv.Unquote(strings.Replace(strconv.Quote(us), `\\u`, `\u`, 1))
				if err != nil {
					return ""
				}
				for _, r := range u {
					if r < ' ' {
						newS += fmt.Sprintf(`\u%04x`, r)
					} else {
						newS += string(r)
					}
				}
				i += 5
			default:
				newS += string(c)
			}
		default:
			if c > unicode.MaxASCII {
				r, size := utf8.DecodeRune([]byte(s[i:]))
				newS += string(r)
				i += size - 1
			} else {
				newS += string(c)
			}
		}
	}

	return newS
}

func formatDouble(f float64) string {
	var s string
	if math.IsInf(f, 1) {
		s = "Infinity"
	} else if math.IsInf(f, -1) {
		s = "-Infinity"
	} else if math.IsNaN(f) {
		s = "NaN"
	} else {
		// Print exactly one decimalType place for integers; otherwise, print as many are necessary to
		// perfectly represent it.
		s = strconv.FormatFloat(f, 'G', -1, 64)
		if !strings.ContainsRune(s, 'E') && !strings.ContainsRune(s, '.') {
			s += ".0"
		}
	}

	return s
}

func normalizeCanonicalDouble(t *testing.T, key string, cEJ string) string {
	// Unmarshal string into map
	cEJMap := make(map[string]map[string]string)
	err := json.Unmarshal([]byte(cEJ), &cEJMap)
	require.NoError(t, err)

	// Parse the float contained by the map.
	expectedString := cEJMap[key]["$numberDouble"]
	expectedFloat, err := strconv.ParseFloat(expectedString, 64)

	// Normalize the string
	return fmt.Sprintf(`{"%s":{"$numberDouble":"%s"}}`, key, formatDouble(expectedFloat))
}

func normalizeRelaxedDouble(t *testing.T, key string, rEJ string) string {
	// Unmarshal string into map
	rEJMap := make(map[string]float64)
	err := json.Unmarshal([]byte(rEJ), &rEJMap)
	if err != nil {
		return normalizeCanonicalDouble(t, key, rEJ)
	}

	// Parse the float contained by the map.
	expectedFloat := rEJMap[key]

	// Normalize the string
	return fmt.Sprintf(`{"%s":%s}`, key, formatDouble(expectedFloat))
}

// bsonToNative decodes the BSON bytes (b) into a native Document
func bsonToNative(t *testing.T, b []byte, bType, testDesc string) D {
	var doc D
	err := Unmarshal(b, &doc)
	expectNoError(t, err, fmt.Sprintf("%s: decoding %s BSON", testDesc, bType))
	return doc
}

// nativeToBSON encodes the native Document (doc) into canonical BSON and compares it to the expected
// canonical BSON (cB)
func nativeToBSON(t *testing.T, cB []byte, doc D, testDesc, bType, docSrcDesc string) {
	actual, err := Marshal(doc)
	expectNoError(t, err, fmt.Sprintf("%s: encoding %s BSON", testDesc, bType))

	if diff := cmp.Diff(cB, actual); diff != "" {
		t.Errorf("%s: 'native_to_bson(%s) = cB' failed (-want, +got):\n-%v\n+%v\n",
			testDesc, docSrcDesc, cB, actual)
		t.FailNow()
	}
}

// jsonToNative decodes the extended JSON string (ej) into a native Document
func jsonToNative(t *testing.T, ej, ejType, testDesc string) D {
	var doc D
	err := UnmarshalExtJSON([]byte(ej), ejType != "relaxed", &doc)
	expectNoError(t, err, fmt.Sprintf("%s: decoding %s extended JSON", testDesc, ejType))
	return doc
}

// nativeToJSON encodes the native Document (doc) into an extended JSON string
func nativeToJSON(t *testing.T, ej string, doc D, testDesc, ejType, ejShortName, docSrcDesc string) {
	actualEJ, err := MarshalExtJSON(doc, ejType != "relaxed", true)
	expectNoError(t, err, fmt.Sprintf("%s: encoding %s extended JSON", testDesc, ejType))

	if diff := cmp.Diff(ej, string(actualEJ)); diff != "" {
		t.Errorf("%s: 'native_to_%s_extended_json(%s) = %s' failed (-want, +got):\n%s\n",
			testDesc, ejType, docSrcDesc, ejShortName, diff)
		t.FailNow()
	}
}

func runTest(t *testing.T, file string) {
	filepath := path.Join(dataDir, file)
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	// Remove ".json" from filename.
	file = file[:len(file)-5]
	testName := "bson_corpus--" + file

	t.Run(testName, func(t *testing.T) {
		var test testCase
		require.NoError(t, json.Unmarshal(content, &test))

		t.Run("valid", func(t *testing.T) {
			for _, v := range test.Valid {
				t.Run(v.Description, func(t *testing.T) {
					// get canonical BSON
					cB, err := hex.DecodeString(v.CanonicalBson)
					expectNoError(t, err, fmt.Sprintf("%s: reading canonical BSON", v.Description))

					// get canonical extended JSON
					cEJ := unescapeUnicode(string(pretty.Ugly([]byte(v.CanonicalExtJSON))), test.BsonType)
					if test.BsonType == "0x01" {
						cEJ = normalizeCanonicalDouble(t, *test.TestKey, cEJ)
					}

					/*** canonical BSON round-trip tests ***/
					doc := bsonToNative(t, cB, "canonical", v.Description)

					// native_to_bson(bson_to_native(cB)) = cB
					nativeToBSON(t, cB, doc, v.Description, "canonical", "bson_to_native(cB)")

					// native_to_canonical_extended_json(bson_to_native(cB)) = cEJ
					nativeToJSON(t, cEJ, doc, v.Description, "canonical", "cEJ", "bson_to_native(cB)")

					// native_to_relaxed_extended_json(bson_to_native(cB)) = rEJ (if rEJ exists)
					if v.RelaxedExtJSON != nil {
						rEJ := unescapeUnicode(string(pretty.Ugly([]byte(*v.RelaxedExtJSON))), test.BsonType)
						if test.BsonType == "0x01" {
							rEJ = normalizeRelaxedDouble(t, *test.TestKey, rEJ)
						}

						nativeToJSON(t, rEJ, doc, v.Description, "relaxed", "rEJ", "bson_to_native(cB)")

						/*** relaxed extended JSON round-trip tests (if exists) ***/
						doc = jsonToNative(t, rEJ, "relaxed", v.Description)

						// native_to_relaxed_extended_json(json_to_native(rEJ)) = rEJ
						nativeToJSON(t, rEJ, doc, v.Description, "relaxed", "eJR", "json_to_native(rEJ)")
					}

					/*** canonical extended JSON round-trip tests ***/
					doc = jsonToNative(t, cEJ, "canonical", v.Description)

					// native_to_canonical_extended_json(json_to_native(cEJ)) = cEJ
					nativeToJSON(t, cEJ, doc, v.Description, "canonical", "cEJ", "json_to_native(cEJ)")

					// native_to_bson(json_to_native(cEJ)) = cb (unless lossy)
					if v.Lossy == nil || !*v.Lossy {
						nativeToBSON(t, cB, doc, v.Description, "canonical", "json_to_native(cEJ)")
					}

					/*** degenerate BSON round-trip tests (if exists) ***/
					if v.DegenerateBSON != nil {
						dB, err := hex.DecodeString(*v.DegenerateBSON)
						expectNoError(t, err, fmt.Sprintf("%s: reading degenerate BSON", v.Description))

						doc = bsonToNative(t, dB, "degenerate", v.Description)

						// native_to_bson(bson_to_native(dB)) = cB
						nativeToBSON(t, cB, doc, v.Description, "degenerate", "bson_to_native(dB)")
					}

					/*** degenerate JSON round-trip tests (if exists) ***/
					if v.DegenerateExtJSON != nil {
						dEJ := unescapeUnicode(string(pretty.Ugly([]byte(*v.DegenerateExtJSON))), test.BsonType)
						if test.BsonType == "0x01" {
							dEJ = normalizeCanonicalDouble(t, *test.TestKey, dEJ)
						}

						doc = jsonToNative(t, dEJ, "degenerate canonical", v.Description)

						// native_to_canonical_extended_json(json_to_native(dEJ)) = cEJ
						nativeToJSON(t, cEJ, doc, v.Description, "degenerate canonical", "cEJ", "json_to_native(dEJ)")

						// native_to_bson(json_to_native(dEJ)) = cB (unless lossy)
						if v.Lossy == nil || !*v.Lossy {
							nativeToBSON(t, cB, doc, v.Description, "canonical", "json_to_native(dEJ)")
						}
					}
				})
			}
		})

		t.Run("decode error", func(t *testing.T) {
			for _, d := range test.DecodeErrors {
				t.Run(d.Description, func(t *testing.T) {
					b, err := hex.DecodeString(d.Bson)
					expectNoError(t, err, d.Description)

					var doc D
					err = Unmarshal(b, &doc)

					// The driver unmarshals invalid UTF-8 strings without error. Loop over the unmarshalled elements
					// and assert that there was no error if any of the string or DBPointer values contain invalid UTF-8
					// characters.
					for _, elem := range doc {
						str, ok := elem.Value.(string)
						invalidString := ok && !utf8.ValidString(str)
						dbPtr, ok := elem.Value.(primitive.DBPointer)
						invalidDBPtr := ok && !utf8.ValidString(dbPtr.DB)

						if invalidString || invalidDBPtr {
							expectNoError(t, err, d.Description)
							return
						}
					}

					expectError(t, err, fmt.Sprintf("%s: expected decode error", d.Description))
				})
			}
		})

		t.Run("parse error", func(t *testing.T) {
			for _, p := range test.ParseErrors {
				t.Run(p.Description, func(t *testing.T) {
					// skip DBRef tests
					if strings.Contains(p.Description, "Bad DBRef") {
						t.Skip("skipping DBRef test")
					}

					s := unescapeUnicode(p.String, test.BsonType)
					if test.BsonType == "0x13" {
						s = fmt.Sprintf(`{"$numberDecimal": "%s"}`, s)
					}

					switch test.BsonType {
					case "0x00":
						var doc D
						err := UnmarshalExtJSON([]byte(s), true, &doc)
						expectError(t, err, fmt.Sprintf("%s: expected parse error", p.Description))
					case "0x13":
						ejvr, err := bsonrw.NewExtJSONValueReader(strings.NewReader(s), true)
						expectNoError(t, err, fmt.Sprintf("error creating value reader: %s", err))
						_, err = ejvr.ReadDecimal128()
						expectError(t, err, fmt.Sprintf("%s: expected parse error", p.Description))
					default:
						t.Errorf("Update test to check for parse errors for type %s", test.BsonType)
						t.Fail()
					}
				})
			}
		})
	})
}

func Test_BsonCorpus(t *testing.T) {
	for _, file := range findJSONFilesInDir(t, dataDir) {
		runTest(t, file)
	}
}

func expectNoError(t *testing.T, err error, desc string) {
	if err != nil {
		t.Helper()
		t.Errorf("%s: Unepexted error: %v", desc, err)
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
