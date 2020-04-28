// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

func ExampleDocument_Validate() {
	doc := make(Document, 500)
	doc[250], doc[251], doc[252], doc[253], doc[254] = 0x05, 0x00, 0x00, 0x00, 0x00
	err := doc[250:].Validate()
	fmt.Println(err)

	// Output: <nil>
}

func BenchmarkDocumentValidate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		doc := make(Document, 500)
		doc[250], doc[251], doc[252], doc[253], doc[254] = 0x05, 0x00, 0x00, 0x00, 0x00
		_ = doc[250:].Validate()
	}
}

func TestDocument(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		t.Run("TooShort", func(t *testing.T) {
			want := NewInsufficientBytesError(nil, nil)
			got := Document{'\x00', '\x00'}.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("InvalidLength", func(t *testing.T) {
			want := Document{}.lengtherror(200, 5)
			r := make(Document, 5)
			binary.LittleEndian.PutUint32(r[0:4], 200)
			got := r.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("Invalid Element", func(t *testing.T) {
			want := NewInsufficientBytesError(nil, nil)
			r := make(Document, 9)
			binary.LittleEndian.PutUint32(r[0:4], 9)
			r[4], r[5], r[6], r[7], r[8] = 0x02, 'f', 'o', 'o', 0x00
			got := r.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		t.Run("Missing Null Terminator", func(t *testing.T) {
			want := ErrMissingNull
			r := make(Document, 8)
			binary.LittleEndian.PutUint32(r[0:4], 8)
			r[4], r[5], r[6], r[7] = 0x0A, 'f', 'o', 'o'
			got := r.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Did not get expected error. got %v; want %v", got, want)
			}
		})
		testCases := []struct {
			name string
			r    Document
			want error
		}{
			{"null", Document{'\x08', '\x00', '\x00', '\x00', '\x0A', 'x', '\x00', '\x00'}, nil},
			{"subdocument",
				Document{
					'\x15', '\x00', '\x00', '\x00',
					'\x03',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', 'a', '\x00',
					'\x0A', 'b', '\x00', '\x00', '\x00',
				},
				nil,
			},
			{"array",
				Document{
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
				got := tc.r.Validate()
				if !compareErrors(got, tc.want) {
					t.Errorf("Returned error does not match. got %v; want %v", got, tc.want)
				}
			})
		}
	})
	t.Run("Lookup", func(t *testing.T) {
		t.Run("empty-key", func(t *testing.T) {
			rdr := Document{'\x05', '\x00', '\x00', '\x00', '\x00'}
			_, err := rdr.LookupErr()
			if err != ErrEmptyKey {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", err, ErrEmptyKey)
			}
		})
		t.Run("corrupted-subdocument", func(t *testing.T) {
			rdr := Document{
				'\x0D', '\x00', '\x00', '\x00',
				'\x03', 'x', '\x00',
				'\x06', '\x00', '\x00', '\x00',
				'\x01',
				'\x00',
				'\x00',
			}
			_, got := rdr.LookupErr("x", "y")
			want := NewInsufficientBytesError(nil, nil)
			if !cmp.Equal(got, want) {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", got, want)
			}
		})
		t.Run("corrupted-array", func(t *testing.T) {
			rdr := Document{
				'\x0D', '\x00', '\x00', '\x00',
				'\x04', 'x', '\x00',
				'\x06', '\x00', '\x00', '\x00',
				'\x01',
				'\x00',
				'\x00',
			}
			_, got := rdr.LookupErr("x", "y")
			want := NewInsufficientBytesError(nil, nil)
			if !cmp.Equal(got, want) {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", got, want)
			}
		})
		t.Run("invalid-traversal", func(t *testing.T) {
			rdr := Document{'\x08', '\x00', '\x00', '\x00', '\x0A', 'x', '\x00', '\x00'}
			_, got := rdr.LookupErr("x", "y")
			want := InvalidDepthTraversalError{Key: "x", Type: bsontype.Null}
			if !compareErrors(got, want) {
				t.Errorf("Empty key lookup did not return expected result. got %v; want %v", got, want)
			}
		})
		testCases := []struct {
			name string
			r    Document
			key  []string
			want Value
			err  error
		}{
			{"first",
				Document{
					'\x08', '\x00', '\x00', '\x00', '\x0A', 'x', '\x00', '\x00',
				},
				[]string{"x"},
				Value{Type: bsontype.Null, Data: []byte{}},
				nil,
			},
			{"first-second",
				Document{
					'\x15', '\x00', '\x00', '\x00',
					'\x03',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', 'a', '\x00',
					'\x0A', 'b', '\x00', '\x00', '\x00',
				},
				[]string{"foo", "b"},
				Value{Type: bsontype.Null, Data: []byte{}},
				nil,
			},
			{"first-second-array",
				Document{
					'\x15', '\x00', '\x00', '\x00',
					'\x04',
					'f', 'o', 'o', '\x00',
					'\x0B', '\x00', '\x00', '\x00', '\x0A', '1', '\x00',
					'\x0A', '2', '\x00', '\x00', '\x00',
				},
				[]string{"foo", "2"},
				Value{Type: bsontype.Null, Data: []byte{}},
				nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("Lookup", func(t *testing.T) {
					got := tc.r.Lookup(tc.key...)
					if !cmp.Equal(got, tc.want) {
						t.Errorf("Returned value does not match expected element. got %v; want %v", got, tc.want)
					}
				})
				t.Run("LookupErr", func(t *testing.T) {
					got, err := tc.r.LookupErr(tc.key...)
					if err != tc.err {
						t.Errorf("Returned error does not match. got %v; want %v", err, tc.err)
					}
					if !cmp.Equal(got, tc.want) {
						t.Errorf("Returned value does not match expected element. got %v; want %v", got, tc.want)
					}
				})
			})
		}
	})
	t.Run("Index", func(t *testing.T) {
		t.Run("Out of bounds", func(t *testing.T) {
			rdr := Document{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0}
			_, err := rdr.IndexErr(3)
			if err != ErrOutOfBounds {
				t.Errorf("Out of bounds should be returned when accessing element beyond end of document. got %v; want %v", err, ErrOutOfBounds)
			}
		})
		t.Run("Validation Error", func(t *testing.T) {
			rdr := Document{0x07, 0x00, 0x00, 0x00, 0x00}
			_, got := rdr.IndexErr(1)
			want := NewInsufficientBytesError(nil, nil)
			if !compareErrors(got, want) {
				t.Errorf("Did not receive expected error. got %v; want %v", got, want)
			}
		})
		testCases := []struct {
			name  string
			rdr   Document
			index uint
			want  Element
		}{
			{"first",
				Document{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0},
				0, Element{0x0a, 0x78, 0x00},
			},
			{"second",
				Document{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0},
				1, Element{0x0a, 0x79, 0x00},
			},
			{"third",
				Document{0xe, 0x0, 0x0, 0x0, 0xa, 0x78, 0x0, 0xa, 0x79, 0x0, 0xa, 0x7a, 0x0, 0x0},
				2, Element{0x0a, 0x7a, 0x00},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("IndexErr", func(t *testing.T) {
					got, err := tc.rdr.IndexErr(tc.index)
					if err != nil {
						t.Errorf("Unexpected error from IndexErr: %s", err)
					}
					if diff := cmp.Diff(got, tc.want); diff != "" {
						t.Errorf("Documents differ: (-got +want)\n%s", diff)
					}
				})
				t.Run("Index", func(t *testing.T) {
					defer func() {
						if err := recover(); err != nil {
							t.Errorf("Unexpected error: %v", err)
						}
					}()
					got := tc.rdr.Index(tc.index)
					if diff := cmp.Diff(got, tc.want); diff != "" {
						t.Errorf("Documents differ: (-got +want)\n%s", diff)
					}
				})
			})
		}
	})
	t.Run("NewDocumentFromReader", func(t *testing.T) {
		testCases := []struct {
			name     string
			ioReader io.Reader
			doc      Document
			err      error
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
				doc, err := NewDocumentFromReader(tc.ioReader)
				if !compareErrors(err, tc.err) {
					t.Errorf("errors do not match. got %v; want %v", err, tc.err)
				}
				if !bytes.Equal(tc.doc, doc) {
					t.Errorf("documents differ. got %v; want %v", tc.doc, doc)
				}
			})
		}
	})
	t.Run("Elements", func(t *testing.T) {
		invalidElem := BuildDocument(nil, AppendHeader(nil, bsontype.Double, "foo"))
		invalidTwoElem := BuildDocument(nil,
			AppendHeader(
				AppendDoubleElement(nil, "pi", 3.14159),
				bsontype.Double, "foo",
			),
		)
		oneElem := BuildDocument(nil, AppendDoubleElement(nil, "pi", 3.14159))
		twoElems := BuildDocument(nil,
			AppendStringElement(
				AppendDoubleElement(nil, "pi", 3.14159),
				"hello", "world!",
			),
		)
		testCases := []struct {
			name  string
			doc   Document
			elems []Element
			err   error
		}{
			{"Insufficient Bytes Length", Document{0x03, 0x00, 0x00}, nil, NewInsufficientBytesError(nil, nil)},
			{"Insufficient Bytes First Element", invalidElem, nil, NewInsufficientBytesError(nil, nil)},
			{"Insufficient Bytes Second Element", invalidTwoElem, []Element{AppendDoubleElement(nil, "pi", 3.14159)}, NewInsufficientBytesError(nil, nil)},
			{"Success One Element", oneElem, []Element{AppendDoubleElement(nil, "pi", 3.14159)}, nil},
			{"Success Two Elements", twoElems, []Element{AppendDoubleElement(nil, "pi", 3.14159), AppendStringElement(nil, "hello", "world!")}, nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				elems, err := tc.doc.Elements()
				if !compareErrors(err, tc.err) {
					t.Errorf("errors do not match. got %v; want %v", err, tc.err)
				}
				if len(elems) != len(tc.elems) {
					t.Fatalf("number of elements returned does not match. got %d; want %d", len(elems), len(tc.elems))
				}

				for idx := range elems {
					got, want := elems[idx], tc.elems[idx]
					if !bytes.Equal(got, want) {
						t.Errorf("Elements at index %d differ. got %v; want %v", idx, got.DebugString(), want.DebugString())
					}
				}
			})
		}
	})
}
