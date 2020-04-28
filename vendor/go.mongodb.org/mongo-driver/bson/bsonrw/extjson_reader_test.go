// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

func TestExtJSONReader(t *testing.T) {
	t.Run("ReadDocument", func(t *testing.T) {
		t.Run("EmbeddedDocument", func(t *testing.T) {
			ejvr := &extJSONValueReader{
				stack: []ejvrState{
					{mode: mTopLevel},
					{mode: mElement, vType: bsontype.Boolean},
				},
				frame: 1,
			}

			ejvr.stack[1].mode = mArray
			wanterr := ejvr.invalidTransitionErr(mDocument, "ReadDocument", []mode{mTopLevel, mElement, mValue})
			_, err := ejvr.ReadDocument()
			if err == nil || err.Error() != wanterr.Error() {
				t.Errorf("Incorrect returned error. got %v; want %v", err, wanterr)
			}

		})
	})

	t.Run("invalid transition", func(t *testing.T) {
		t.Run("Skip", func(t *testing.T) {
			ejvr := &extJSONValueReader{stack: []ejvrState{{mode: mTopLevel}}}
			wanterr := (&extJSONValueReader{stack: []ejvrState{{mode: mTopLevel}}}).invalidTransitionErr(0, "Skip", []mode{mElement, mValue})
			goterr := ejvr.Skip()
			if !cmp.Equal(goterr, wanterr, cmp.Comparer(compareErrors)) {
				t.Errorf("Expected correct invalid transition error. got %v; want %v", goterr, wanterr)
			}
		})
	})
}

func TestReadMultipleTopLevelDocuments(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected [][]byte
	}{
		{
			"single top-level document",
			"{\"foo\":1}",
			[][]byte{
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			"single top-level document with leading and trailing whitespace",
			"\n\n   {\"foo\":1}   \n",
			[][]byte{
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			"two top-level documents",
			"{\"foo\":1}{\"foo\":2}",
			[][]byte{
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x02, 0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			"two top-level documents with leading and trailing whitespace and whitespace separation ",
			"\n\n  {\"foo\":1}\n{\"foo\":2}\n  ",
			[][]byte{
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x02, 0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			"top-level array with single document",
			"[{\"foo\":1}]",
			[][]byte{
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			"top-level array with 2 documents",
			"[{\"foo\":1},{\"foo\":2}]",
			[][]byte{
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
				{0x0E, 0x00, 0x00, 0x00, 0x10, 'f', 'o', 'o', 0x00, 0x02, 0x00, 0x00, 0x00, 0x00},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.input)
			vr, err := NewExtJSONValueReader(r, false)
			if err != nil {
				t.Fatalf("expected no error, but got %v", err)
			}

			actual, err := readAllDocuments(vr)
			if err != nil {
				t.Fatalf("expected no error, but got %v", err)
			}

			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Fatalf("expected does not match actual: %v", diff)
			}
		})
	}
}

func readAllDocuments(vr ValueReader) ([][]byte, error) {
	c := NewCopier()
	var actual [][]byte

	switch vr.Type() {
	case bsontype.EmbeddedDocument:
		for {
			result, err := c.CopyDocumentToBytes(vr)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}

			actual = append(actual, result)
		}
	case bsontype.Array:
		ar, err := vr.ReadArray()
		if err != nil {
			return nil, err
		}
		for {
			evr, err := ar.ReadValue()
			if err != nil {
				if err == ErrEOA {
					break
				}
				return nil, err
			}

			result, err := c.CopyDocumentToBytes(evr)
			if err != nil {
				return nil, err
			}

			actual = append(actual, result)
		}
	default:
		return nil, fmt.Errorf("expected an array or a document, but got %s", vr.Type())
	}

	return actual, nil
}
