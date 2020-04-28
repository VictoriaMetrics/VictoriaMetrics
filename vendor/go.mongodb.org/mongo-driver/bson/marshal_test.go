// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bson

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMarshalAppendWithRegistry(t *testing.T) {
	for _, tc := range marshalingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			dst := make([]byte, 0, 1024)
			var reg *bsoncodec.Registry
			if tc.reg != nil {
				reg = tc.reg
			} else {
				reg = DefaultRegistry
			}
			got, err := MarshalAppendWithRegistry(reg, dst, tc.val)
			noerr(t, err)

			if !bytes.Equal(got, tc.want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, tc.want)
				t.Errorf("Bytes:\n%v\n%v", got, tc.want)
			}
		})
	}
}

func TestMarshalAppendWithContext(t *testing.T) {
	for _, tc := range marshalingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			dst := make([]byte, 0, 1024)
			var reg *bsoncodec.Registry
			if tc.reg != nil {
				reg = tc.reg
			} else {
				reg = DefaultRegistry
			}
			ec := bsoncodec.EncodeContext{Registry: reg}
			got, err := MarshalAppendWithContext(ec, dst, tc.val)
			noerr(t, err)

			if !bytes.Equal(got, tc.want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, tc.want)
				t.Errorf("Bytes:\n%v\n%v", got, tc.want)
			}
		})
	}
}

func TestMarshalWithRegistry(t *testing.T) {
	for _, tc := range marshalingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			var reg *bsoncodec.Registry
			if tc.reg != nil {
				reg = tc.reg
			} else {
				reg = DefaultRegistry
			}
			got, err := MarshalWithRegistry(reg, tc.val)
			noerr(t, err)

			if !bytes.Equal(got, tc.want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, tc.want)
				t.Errorf("Bytes:\n%v\n%v", got, tc.want)
			}
		})
	}
}

func TestMarshalWithContext(t *testing.T) {
	for _, tc := range marshalingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			var reg *bsoncodec.Registry
			if tc.reg != nil {
				reg = tc.reg
			} else {
				reg = DefaultRegistry
			}
			ec := bsoncodec.EncodeContext{Registry: reg}
			got, err := MarshalWithContext(ec, tc.val)
			noerr(t, err)

			if !bytes.Equal(got, tc.want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, tc.want)
				t.Errorf("Bytes:\n%v\n%v", got, tc.want)
			}
		})
	}
}

func TestMarshalAppend(t *testing.T) {
	for _, tc := range marshalingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.reg != nil {
				t.Skip() // test requires custom registry
			}
			dst := make([]byte, 0, 1024)
			got, err := MarshalAppend(dst, tc.val)
			noerr(t, err)

			if !bytes.Equal(got, tc.want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, tc.want)
				t.Errorf("Bytes:\n%v\n%v", got, tc.want)
			}
		})
	}
}

func TestMarshalExtJSONAppendWithContext(t *testing.T) {
	t.Run("MarshalExtJSONAppendWithContext", func(t *testing.T) {
		dst := make([]byte, 0, 1024)
		type teststruct struct{ Foo int }
		val := teststruct{1}
		ec := bsoncodec.EncodeContext{Registry: DefaultRegistry}
		got, err := MarshalExtJSONAppendWithContext(ec, dst, val, true, false)
		noerr(t, err)
		want := []byte(`{"foo":{"$numberInt":"1"}}`)
		if !bytes.Equal(got, want) {
			t.Errorf("Bytes are not equal. got %v; want %v", got, want)
			t.Errorf("Bytes:\n%s\n%s", got, want)
		}
	})
}

func TestMarshalExtJSONWithContext(t *testing.T) {
	t.Run("MarshalExtJSONWithContext", func(t *testing.T) {
		type teststruct struct{ Foo int }
		val := teststruct{1}
		ec := bsoncodec.EncodeContext{Registry: DefaultRegistry}
		got, err := MarshalExtJSONWithContext(ec, val, true, false)
		noerr(t, err)
		want := []byte(`{"foo":{"$numberInt":"1"}}`)
		if !bytes.Equal(got, want) {
			t.Errorf("Bytes are not equal. got %v; want %v", got, want)
			t.Errorf("Bytes:\n%s\n%s", got, want)
		}
	})
}

func TestMarshal_roundtripFromBytes(t *testing.T) {
	before := []byte{
		// length
		0x1c, 0x0, 0x0, 0x0,

		// --- begin array ---

		// type - document
		0x3,
		// key - "foo"
		0x66, 0x6f, 0x6f, 0x0,

		// length
		0x12, 0x0, 0x0, 0x0,
		// type - string
		0x2,
		// key - "bar"
		0x62, 0x61, 0x72, 0x0,
		// value - string length
		0x4, 0x0, 0x0, 0x0,
		// value - "baz"
		0x62, 0x61, 0x7a, 0x0,

		// null terminator
		0x0,

		// --- end array ---

		// null terminator
		0x0,
	}

	var doc D
	require.NoError(t, Unmarshal(before, &doc))

	after, err := Marshal(doc)
	require.NoError(t, err)

	require.True(t, bytes.Equal(before, after))
}

func TestMarshal_roundtripFromDoc(t *testing.T) {
	before := D{
		{"foo", "bar"},
		{"baz", int64(-27)},
		{"bing", A{nil, primitive.Regex{Pattern: "word", Options: "i"}}},
	}

	b, err := Marshal(before)
	require.NoError(t, err)

	var after D
	require.NoError(t, Unmarshal(b, &after))

	if !cmp.Equal(after, before) {
		t.Errorf("Documents to not match. got %v; want %v", after, before)
	}
}
