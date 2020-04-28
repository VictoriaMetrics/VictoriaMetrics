// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bson

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func ExampleRaw_Validate() {
	rdr := make(Raw, 500)
	rdr[250], rdr[251], rdr[252], rdr[253], rdr[254] = '\x05', '\x00', '\x00', '\x00', '\x00'
	err := rdr[250:].Validate()
	fmt.Println(err)

	// Output: <nil>
}

func BenchmarkRawValidate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rdr := make(Raw, 500)
		rdr[250], rdr[251], rdr[252], rdr[253], rdr[254] = '\x05', '\x00', '\x00', '\x00', '\x00'
		_ = rdr[250:].Validate()
	}

}

func TestRaw(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		t.Run("TooShort", func(t *testing.T) {
			want := bsoncore.NewInsufficientBytesError(nil, nil)
			got := Raw{'\x00', '\x00'}.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("InvalidLength", func(t *testing.T) {
			want := bsoncore.DocumentValidationError("document length exceeds available bytes. length=200 remainingBytes=5")
			r := make(Raw, 5)
			binary.LittleEndian.PutUint32(r[0:4], 200)
			got := r.Validate()
			if got != want {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("keyLength-error", func(t *testing.T) {
			want := bsoncore.ErrMissingNull
			r := make(Raw, 8)
			binary.LittleEndian.PutUint32(r[0:4], 8)
			r[4], r[5], r[6], r[7] = '\x02', 'f', 'o', 'o'
			got := r.Validate()
			if got != want {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("Missing-Null-Terminator", func(t *testing.T) {
			want := bsoncore.ErrMissingNull
			r := make(Raw, 9)
			binary.LittleEndian.PutUint32(r[0:4], 9)
			r[4], r[5], r[6], r[7], r[8] = '\x0A', 'f', 'o', 'o', '\x00'
			got := r.Validate()
			if got != want {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("validateValue-error", func(t *testing.T) {
			want := bsoncore.ErrMissingNull
			r := make(Raw, 11)
			binary.LittleEndian.PutUint32(r[0:4], 11)
			r[4], r[5], r[6], r[7], r[8], r[9], r[10] = '\x01', 'f', 'o', 'o', '\x00', '\x01', '\x02'
			got := r.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		testCases := []struct {
			name string
			r    Raw
			err  error
		}{
			{"null", Raw{'\x08', '\x00', '\x00', '\x00', '\x0A', 'x', '\x00', '\x00'}, nil},
			{"subdocument",
				Raw{
					'\x15', '\x00', '\x00', '\x00',
					'\x03',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', 'a', '\x00',
					'\x0A', 'b', '\x00', '\x00', '\x00',
				},
				nil,
			},
			{"array",
				Raw{
					'\x15', '\x00', '\x00', '\x00',
					'\x04',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', '1', '\x00',
					'\x0A', '2', '\x00', '\x00', '\x00',
				},
				nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.r.Validate()
				if err != tc.err {
					t.Errorf("Returned error does not match. got %v; want %v", err, tc.err)
				}
			})
		}
	})
	t.Run("Lookup", func(t *testing.T) {
		t.Run("empty-key", func(t *testing.T) {
			rdr := Raw{'\x05', '\x00', '\x00', '\x00', '\x00'}
			_, err := rdr.LookupErr()
			if err != bsoncore.ErrEmptyKey {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", err, bsoncore.ErrEmptyKey)
			}
		})
		t.Run("corrupted-subdocument", func(t *testing.T) {
			rdr := Raw{
				'\x0D', '\x00', '\x00', '\x00',
				'\x03', 'x', '\x00',
				'\x06', '\x00', '\x00', '\x00',
				'\x01',
				'\x00',
				'\x00',
			}
			_, err := rdr.LookupErr("x", "y")
			want := bsoncore.NewInsufficientBytesError(nil, nil)
			if !compareErrors(err, want) {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", err, want)
			}
		})
		t.Run("corrupted-array", func(t *testing.T) {
			rdr := Raw{
				'\x0D', '\x00', '\x00', '\x00',
				'\x04', 'x', '\x00',
				'\x06', '\x00', '\x00', '\x00',
				'\x01',
				'\x00',
				'\x00',
			}
			_, err := rdr.LookupErr("x", "y")
			want := bsoncore.NewInsufficientBytesError(nil, nil)
			if !compareErrors(err, want) {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", err, want)
			}
		})
		t.Run("invalid-traversal", func(t *testing.T) {
			rdr := Raw{'\x08', '\x00', '\x00', '\x00', '\x0A', 'x', '\x00', '\x00'}
			_, err := rdr.LookupErr("x", "y")
			want := bsoncore.InvalidDepthTraversalError{Key: "x", Type: bsontype.Null}
			if !compareErrors(err, want) {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", err, want)
			}
		})
		testCases := []struct {
			name string
			r    Raw
			key  []string
			want RawValue
			err  error
		}{
			{"first",
				Raw{
					'\x08', '\x00', '\x00', '\x00', '\x0A', 'x', '\x00', '\x00',
				},
				[]string{"x"},
				RawValue{Type: bsontype.Null}, nil,
			},
			{"first-second",
				Raw{
					'\x15', '\x00', '\x00', '\x00',
					'\x03',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', 'a', '\x00',
					'\x0A', 'b', '\x00', '\x00', '\x00',
				},
				[]string{"foo", "b"},
				RawValue{Type: bsontype.Null}, nil,
			},
			{"first-second-array",
				Raw{
					'\x15', '\x00', '\x00', '\x00',
					'\x04',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', '1', '\x00',
					'\x0A', '2', '\x00', '\x00', '\x00',
				},
				[]string{"foo", "2"},
				RawValue{Type: bsontype.Null}, nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := tc.r.LookupErr(tc.key...)
				if err != tc.err {
					t.Errorf("Returned error does not match. got %v; want %v", err, tc.err)
				}
				if !cmp.Equal(got, tc.want) {
					t.Errorf("Returned element does not match expected element. got %v; want %v", got, tc.want)
				}
			})
		}
	})
	t.Run("ElementAt", func(t *testing.T) {
		t.Run("Out of bounds", func(t *testing.T) {
			rdr := Raw{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0}
			_, err := rdr.IndexErr(3)
			if err != bsoncore.ErrOutOfBounds {
				t.Errorf("Out of bounds should be returned when accessing element beyond end of document. got %v; want %v", err, bsoncore.ErrOutOfBounds)
			}
		})
		t.Run("Validation Error", func(t *testing.T) {
			rdr := Raw{0x07, 0x00, 0x00, 0x00, 0x00}
			_, err := rdr.IndexErr(1)
			want := bsoncore.NewInsufficientBytesError(nil, nil)
			if !compareErrors(err, want) {
				t.Errorf("Did not receive expected error. got %v; want %v", err, want)
			}
		})
		testCases := []struct {
			name  string
			rdr   Raw
			index uint
			want  RawElement
		}{
			{"first",
				Raw{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0},
				0, bsoncore.AppendNullElement(nil, "x")},
			{"second",
				Raw{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0},
				1, bsoncore.AppendNullElement(nil, "y")},
			{"third",
				Raw{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0},
				2, bsoncore.AppendNullElement(nil, "z")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := tc.rdr.IndexErr(tc.index)
				if err != nil {
					t.Errorf("Unexpected error from ElementAt: %s", err)
				}
				if diff := cmp.Diff(got, tc.want); diff != "" {
					t.Errorf("Documents differ: (-got +want)\n%s", diff)
				}
			})
		}
	})
	t.Run("NewFromIOReader", func(t *testing.T) {
		testCases := []struct {
			name       string
			ioReader   io.Reader
			bsonReader Raw
			err        error
		}{
			{
				"nil reader",
				nil,
				nil,
				ErrNilReader,
			},
			{
				"premature end of reader",
				bytes.NewBuffer([]byte{}),
				nil,
				io.EOF,
			},
			{
				"empty document",
				bytes.NewBuffer([]byte{5, 0, 0, 0, 0}),
				[]byte{5, 0, 0, 0, 0},
				nil,
			},
			{
				"non-empty document",
				bytes.NewBuffer([]byte{
					// length
					0x17, 0x0, 0x0, 0x0,

					// type - string
					0x2,
					// key - "foo"
					0x66, 0x6f, 0x6f, 0x0,
					// value - string length
					0x4, 0x0, 0x0, 0x0,
					// value - string "bar"
					0x62, 0x61, 0x72, 0x0,

					// type - null
					0xa,
					// key - "baz"
					0x62, 0x61, 0x7a, 0x0,

					// null terminator
					0x0,
				}),
				[]byte{
					// length
					0x17, 0x0, 0x0, 0x0,

					// type - string
					0x2,
					// key - "foo"
					0x66, 0x6f, 0x6f, 0x0,
					// value - string length
					0x4, 0x0, 0x0, 0x0,
					// value - string "bar"
					0x62, 0x61, 0x72, 0x0,

					// type - null
					0xa,
					// key - "baz"
					0x62, 0x61, 0x7a, 0x0,

					// null terminator
					0x0,
				},
				nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reader, err := NewFromIOReader(tc.ioReader)
				require.Equal(t, err, tc.err)
				require.True(t, bytes.Equal(tc.bsonReader, reader))
			})
		}
	})
}
