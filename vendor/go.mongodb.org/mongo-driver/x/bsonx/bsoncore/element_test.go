// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncore

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

func TestElement(t *testing.T) {
	t.Run("KeyErr", func(t *testing.T) {
		testCases := []struct {
			name string
			elem Element
			str  string
			err  error
		}{
			{"No Type", Element{}, "", ErrElementMissingType},
			{"No Key", Element{0x01, 'f', 'o', 'o'}, "", ErrElementMissingKey},
			{"Success", AppendHeader(nil, bsontype.Double, "foo"), "foo", nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("Key", func(t *testing.T) {
					str := tc.elem.Key()
					if str != tc.str {
						t.Errorf("returned strings do not match. got %s; want %s", str, tc.str)
					}
				})
				t.Run("KeyErr", func(t *testing.T) {
					str, err := tc.elem.KeyErr()
					if !compareErrors(err, tc.err) {
						t.Errorf("errors do not match. got %v; want %v", err, tc.err)
					}
					if str != tc.str {
						t.Errorf("returned strings do not match. got %s; want %s", str, tc.str)
					}
				})
			})
		}
	})
	t.Run("Validate", func(t *testing.T) {
		testCases := []struct {
			name string
			elem Element
			err  error
		}{
			{"No Type", Element{}, ErrElementMissingType},
			{"No Key", Element{0x01, 'f', 'o', 'o'}, ErrElementMissingKey},
			{"Insufficient Bytes", AppendHeader(nil, bsontype.Double, "foo"), NewInsufficientBytesError(nil, nil)},
			{"Success", AppendDoubleElement(nil, "foo", 3.14159), nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.elem.Validate()
				if !compareErrors(err, tc.err) {
					t.Errorf("errors do not match. got %v; want %v", err, tc.err)
				}
			})
		}
	})
	t.Run("CompareKey", func(t *testing.T) {
		testCases := []struct {
			name  string
			elem  Element
			key   []byte
			equal bool
		}{
			{"Element Too Short", Element{0x02}, nil, false},
			{"Element Invalid Key", Element{0x02, 'f', 'o', 'o'}, nil, false},
			{"Key With Null Byte", AppendHeader(nil, bsontype.Double, "foo"), []byte{'f', 'o', 'o', 0x00}, true},
			{"Key Without Null Byte", AppendHeader(nil, bsontype.Double, "pi"), []byte{'p', 'i'}, true},
			{"Key With Null Byte With Extra", AppendHeader(nil, bsontype.Double, "foo"), []byte{'f', 'o', 'o', 0x00, 'b', 'a', 'r'}, true},
			{"Prefix Key No Match", AppendHeader(nil, bsontype.Double, "foo"), []byte{'f', 'o', 'o', 'b', 'a', 'r'}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				equal := tc.elem.CompareKey(tc.key)
				if equal != tc.equal {
					t.Errorf("Did not get expected equality result. got %t; want %t", equal, tc.equal)
				}
			})
		}
	})
	t.Run("Value & ValueErr", func(t *testing.T) {
		testCases := []struct {
			name string
			elem Element
			val  Value
			err  error
		}{
			{"No Type", Element{}, Value{}, ErrElementMissingType},
			{"No Key", Element{0x01, 'f', 'o', 'o'}, Value{}, ErrElementMissingKey},
			{"Insufficient Bytes", AppendHeader(nil, bsontype.Double, "foo"), Value{}, NewInsufficientBytesError(nil, nil)},
			{"Success", AppendDoubleElement(nil, "foo", 3.14159), Value{Type: bsontype.Double, Data: AppendDouble(nil, 3.14159)}, nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("Value", func(t *testing.T) {
					val := tc.elem.Value()
					if !cmp.Equal(val, tc.val) {
						t.Errorf("Values do not match. got %v; want %v", val, tc.val)
					}
				})
				t.Run("ValueErr", func(t *testing.T) {
					val, err := tc.elem.ValueErr()
					if !compareErrors(err, tc.err) {
						t.Errorf("errors do not match. got %v; want %v", err, tc.err)
					}
					if !cmp.Equal(val, tc.val) {
						t.Errorf("Values do not match. got %v; want %v", val, tc.val)
					}
				})
			})
		}
	})
}
