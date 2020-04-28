// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestValueReader(t *testing.T) {
	t.Run("ReadBinary", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			btype  byte
			b      []byte
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				0,
				nil,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Binary),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				0,
				nil,
				io.EOF,
				bsontype.Binary,
			},
			{
				"no byte available",
				[]byte{0x00, 0x00, 0x00, 0x00},
				0,
				0,
				nil,
				io.EOF,
				bsontype.Binary,
			},
			{
				"not enough bytes for binary",
				[]byte{0x05, 0x00, 0x00, 0x00, 0x00},
				0,
				0,
				nil,
				io.EOF,
				bsontype.Binary,
			},
			{
				"success",
				[]byte{0x03, 0x00, 0x00, 0x00, 0xEA, 0x01, 0x02, 0x03},
				0,
				0xEA,
				[]byte{0x01, 0x02, 0x03},
				nil,
				bsontype.Binary,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				b, btype, err := vr.ReadBinary()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if btype != tc.btype {
					t.Errorf("Incorrect binary type returned. got %v; want %v", btype, tc.btype)
				}
				if !bytes.Equal(b, tc.b) {
					t.Errorf("Binary data does not match. got %v; want %v", b, tc.b)
				}
			})
		}
	})
	t.Run("ReadBoolean", func(t *testing.T) {
		testCases := []struct {
			name    string
			data    []byte
			offset  int64
			boolean bool
			err     error
			vType   bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				false,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Boolean),
				bsontype.EmbeddedDocument,
			},
			{
				"no byte available",
				[]byte{},
				0,
				false,
				io.EOF,
				bsontype.Boolean,
			},
			{
				"invalid byte for boolean",
				[]byte{0x03},
				0,
				false,
				fmt.Errorf("invalid byte for boolean, %b", 0x03),
				bsontype.Boolean,
			},
			{
				"success",
				[]byte{0x01},
				0,
				true,
				nil,
				bsontype.Boolean,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				boolean, err := vr.ReadBoolean()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if boolean != tc.boolean {
					t.Errorf("Incorrect boolean returned. got %v; want %v", boolean, tc.boolean)
				}
			})
		}
	})
	t.Run("ReadDocument", func(t *testing.T) {
		t.Run("TopLevel", func(t *testing.T) {
			doc := []byte{0x05, 0x00, 0x00, 0x00, 0x00}
			vr := &valueReader{
				offset: 0,
				stack:  []vrState{{mode: mTopLevel}},
				frame:  0,
			}

			// invalid length
			vr.d = []byte{0x00, 0x00}
			_, err := vr.ReadDocument()
			if err != io.EOF {
				t.Errorf("Expected io.EOF with document length too small. got %v; want %v", err, io.EOF)
			}

			vr.d = doc
			_, err = vr.ReadDocument()
			noerr(t, err)
			if vr.stack[vr.frame].end != 5 {
				t.Errorf("Incorrect end for document. got %d; want %d", vr.stack[vr.frame].end, 5)
			}
		})
		t.Run("EmbeddedDocument", func(t *testing.T) {
			vr := &valueReader{
				offset: 0,
				stack: []vrState{
					{mode: mTopLevel},
					{mode: mElement, vType: bsontype.Boolean},
				},
				frame: 1,
			}

			var wanterr = (&valueReader{stack: []vrState{{mode: mElement, vType: bsontype.Boolean}}}).typeError(bsontype.EmbeddedDocument)
			_, err := vr.ReadDocument()
			if err == nil || err.Error() != wanterr.Error() {
				t.Errorf("Incorrect returned error. got %v; want %v", err, wanterr)
			}

			vr.stack[1].mode = mArray
			wanterr = vr.invalidTransitionErr(mDocument, "ReadDocument", []mode{mTopLevel, mElement, mValue})
			_, err = vr.ReadDocument()
			if err == nil || err.Error() != wanterr.Error() {
				t.Errorf("Incorrect returned error. got %v; want %v", err, wanterr)
			}

			vr.stack[1].mode, vr.stack[1].vType = mElement, bsontype.EmbeddedDocument
			vr.d = []byte{0x0A, 0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00}
			vr.offset = 4
			_, err = vr.ReadDocument()
			noerr(t, err)
			if len(vr.stack) != 3 {
				t.Errorf("Incorrect number of stack frames. got %d; want %d", len(vr.stack), 3)
			}
			if vr.stack[2].mode != mDocument {
				t.Errorf("Incorrect mode set. got %v; want %v", vr.stack[2].mode, mDocument)
			}
			if vr.stack[2].end != 9 {
				t.Errorf("End of embedded document is not correct. got %d; want %d", vr.stack[2].end, 9)
			}
			if vr.offset != 8 {
				t.Errorf("Offset not incremented correctly. got %d; want %d", vr.offset, 8)
			}

			vr.frame--
			_, err = vr.ReadDocument()
			if err != io.EOF {
				t.Errorf("Should return error when attempting to read length with not enough bytes. got %v; want %v", err, io.EOF)
			}
		})
	})
	t.Run("ReadBinary", func(t *testing.T) {
		codeWithScope := []byte{
			0x11, 0x00, 0x00, 0x00, // total length
			0x4, 0x00, 0x00, 0x00, // string length
			'f', 'o', 'o', 0x00, // string
			0x05, 0x00, 0x00, 0x00, 0x00, // document
		}
		mismatchCodeWithScope := []byte{
			0x11, 0x00, 0x00, 0x00, // total length
			0x4, 0x00, 0x00, 0x00, // string length
			'f', 'o', 'o', 0x00, // string
			0x07, 0x00, 0x00, 0x00, // document
			0x0A, 0x00, // null element, empty key
			0x00, // document end
		}
		invalidCodeWithScope := []byte{
			0x7, 0x00, 0x00, 0x00, // total length
			0x0, 0x00, 0x00, 0x00, // string length = 0
			0x05, 0x00, 0x00, 0x00, 0x00, // document
		}
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.CodeWithScope),
				bsontype.EmbeddedDocument,
			},
			{
				"total length not enough bytes",
				[]byte{},
				0,
				io.EOF,
				bsontype.CodeWithScope,
			},
			{
				"string length not enough bytes",
				codeWithScope[:4],
				0,
				io.EOF,
				bsontype.CodeWithScope,
			},
			{
				"not enough string bytes",
				codeWithScope[:8],
				0,
				io.EOF,
				bsontype.CodeWithScope,
			},
			{
				"document length not enough bytes",
				codeWithScope[:12],
				0,
				io.EOF,
				bsontype.CodeWithScope,
			},
			{
				"length mismatch",
				mismatchCodeWithScope,
				0,
				fmt.Errorf("length of CodeWithScope does not match lengths of components; total: %d; components: %d", 17, 19),
				bsontype.CodeWithScope,
			},
			{
				"invalid strLength",
				invalidCodeWithScope,
				0,
				fmt.Errorf("invalid string length: %d", 0),
				bsontype.CodeWithScope,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				_, _, err := vr.ReadCodeWithScope()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
			})
		}

		t.Run("success", func(t *testing.T) {
			doc := []byte{0x00, 0x00, 0x00, 0x00}
			doc = append(doc, codeWithScope...)
			doc = append(doc, 0x00)
			vr := &valueReader{
				offset: 4,
				d:      doc,
				stack: []vrState{
					{mode: mTopLevel},
					{mode: mElement, vType: bsontype.CodeWithScope},
				},
				frame: 1,
			}

			code, _, err := vr.ReadCodeWithScope()
			noerr(t, err)
			if code != "foo" {
				t.Errorf("Code does not match. got %s; want %s", code, "foo")
			}
			if len(vr.stack) != 3 {
				t.Errorf("Incorrect number of stack frames. got %d; want %d", len(vr.stack), 3)
			}
			if vr.stack[2].mode != mCodeWithScope {
				t.Errorf("Incorrect mode set. got %v; want %v", vr.stack[2].mode, mDocument)
			}
			if vr.stack[2].end != 21 {
				t.Errorf("End of scope is not correct. got %d; want %d", vr.stack[2].end, 21)
			}
			if vr.offset != 20 {
				t.Errorf("Offset not incremented correctly. got %d; want %d", vr.offset, 20)
			}
		})
	})
	t.Run("ReadDBPointer", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			ns     string
			oid    primitive.ObjectID
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				"",
				primitive.ObjectID{},
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.DBPointer),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				"",
				primitive.ObjectID{},
				io.EOF,
				bsontype.DBPointer,
			},
			{
				"not enough bytes for namespace",
				[]byte{0x04, 0x00, 0x00, 0x00},
				0,
				"",
				primitive.ObjectID{},
				io.EOF,
				bsontype.DBPointer,
			},
			{
				"not enough bytes for objectID",
				[]byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00},
				0,
				"",
				primitive.ObjectID{},
				io.EOF,
				bsontype.DBPointer,
			},
			{
				"success",
				[]byte{
					0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00,
					0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C,
				},
				0,
				"foo",
				primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
				nil,
				bsontype.DBPointer,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				ns, oid, err := vr.ReadDBPointer()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if ns != tc.ns {
					t.Errorf("Incorrect namespace returned. got %v; want %v", ns, tc.ns)
				}
				if !bytes.Equal(oid[:], tc.oid[:]) {
					t.Errorf("ObjectIDs did not match. got %v; want %v", oid, tc.oid)
				}
			})
		}
	})
	t.Run("ReadDateTime", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			dt     int64
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				0,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.DateTime),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				0,
				io.EOF,
				bsontype.DateTime,
			},
			{
				"success",
				[]byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				0,
				255,
				nil,
				bsontype.DateTime,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				dt, err := vr.ReadDateTime()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if dt != tc.dt {
					t.Errorf("Incorrect datetime returned. got %d; want %d", dt, tc.dt)
				}
			})
		}
	})
	t.Run("ReadDecimal128", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			dc128  primitive.Decimal128
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				primitive.Decimal128{},
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Decimal128),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				primitive.Decimal128{},
				io.EOF,
				bsontype.Decimal128,
			},
			{
				"success",
				[]byte{
					0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Low
					0x00, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // High
				},
				0,
				primitive.NewDecimal128(65280, 255),
				nil,
				bsontype.Decimal128,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				dc128, err := vr.ReadDecimal128()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				gotHigh, gotLow := dc128.GetBytes()
				wantHigh, wantLow := tc.dc128.GetBytes()
				if gotHigh != wantHigh {
					t.Errorf("Retuired high byte does not match. got %d; want %d", gotHigh, wantHigh)
				}
				if gotLow != wantLow {
					t.Errorf("Returned low byte does not match. got %d; want %d", gotLow, wantLow)
				}
			})
		}
	})
	t.Run("ReadDouble", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			double float64
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				0,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Double),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				0,
				io.EOF,
				bsontype.Double,
			},
			{
				"success",
				[]byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				0,
				math.Float64frombits(255),
				nil,
				bsontype.Double,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				double, err := vr.ReadDouble()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if double != tc.double {
					t.Errorf("Incorrect double returned. got %f; want %f", double, tc.double)
				}
			})
		}
	})
	t.Run("ReadInt32", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			i32    int32
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				0,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Int32),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				0,
				io.EOF,
				bsontype.Int32,
			},
			{
				"success",
				[]byte{0xFF, 0x00, 0x00, 0x00},
				0,
				255,
				nil,
				bsontype.Int32,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				i32, err := vr.ReadInt32()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if i32 != tc.i32 {
					t.Errorf("Incorrect int32 returned. got %d; want %d", i32, tc.i32)
				}
			})
		}
	})
	t.Run("ReadInt32", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			i64    int64
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				0,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Int64),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				0,
				io.EOF,
				bsontype.Int64,
			},
			{
				"success",
				[]byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				0,
				255,
				nil,
				bsontype.Int64,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				i64, err := vr.ReadInt64()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if i64 != tc.i64 {
					t.Errorf("Incorrect int64 returned. got %d; want %d", i64, tc.i64)
				}
			})
		}
	})
	t.Run("ReadJavascript/ReadString/ReadSymbol", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			fn     func(*valueReader) (string, error)
			css    string // code, string, symbol :P
			err    error
			vType  bsontype.Type
		}{
			{
				"ReadJavascript/incorrect type",
				[]byte{},
				0,
				(*valueReader).ReadJavascript,
				"",
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.JavaScript),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadString/incorrect type",
				[]byte{},
				0,
				(*valueReader).ReadString,
				"",
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.String),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadSymbol/incorrect type",
				[]byte{},
				0,
				(*valueReader).ReadSymbol,
				"",
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Symbol),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadJavascript/length too short",
				[]byte{},
				0,
				(*valueReader).ReadJavascript,
				"",
				io.EOF,
				bsontype.JavaScript,
			},
			{
				"ReadString/length too short",
				[]byte{},
				0,
				(*valueReader).ReadString,
				"",
				io.EOF,
				bsontype.String,
			},
			{
				"ReadSymbol/length too short",
				[]byte{},
				0,
				(*valueReader).ReadSymbol,
				"",
				io.EOF,
				bsontype.Symbol,
			},
			{
				"ReadJavascript/incorrect end byte",
				[]byte{0x01, 0x00, 0x00, 0x00, 0x05},
				0,
				(*valueReader).ReadJavascript,
				"",
				fmt.Errorf("string does not end with null byte, but with %v", 0x05),
				bsontype.JavaScript,
			},
			{
				"ReadString/incorrect end byte",
				[]byte{0x01, 0x00, 0x00, 0x00, 0x05},
				0,
				(*valueReader).ReadString,
				"",
				fmt.Errorf("string does not end with null byte, but with %v", 0x05),
				bsontype.String,
			},
			{
				"ReadSymbol/incorrect end byte",
				[]byte{0x01, 0x00, 0x00, 0x00, 0x05},
				0,
				(*valueReader).ReadSymbol,
				"",
				fmt.Errorf("string does not end with null byte, but with %v", 0x05),
				bsontype.Symbol,
			},
			{
				"ReadJavascript/success",
				[]byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00},
				0,
				(*valueReader).ReadJavascript,
				"foo",
				nil,
				bsontype.JavaScript,
			},
			{
				"ReadString/success",
				[]byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00},
				0,
				(*valueReader).ReadString,
				"foo",
				nil,
				bsontype.String,
			},
			{
				"ReadSymbol/success",
				[]byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00},
				0,
				(*valueReader).ReadSymbol,
				"foo",
				nil,
				bsontype.Symbol,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				css, err := tc.fn(vr)
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if css != tc.css {
					t.Errorf("Incorrect (JavaScript,String,Symbol) returned. got %s; want %s", css, tc.css)
				}
			})
		}
	})
	t.Run("ReadMaxKey/ReadMinKey/ReadNull/ReadUndefined", func(t *testing.T) {
		testCases := []struct {
			name  string
			fn    func(*valueReader) error
			err   error
			vType bsontype.Type
		}{
			{
				"ReadMaxKey/incorrect type",
				(*valueReader).ReadMaxKey,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.MaxKey),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadMaxKey/success",
				(*valueReader).ReadMaxKey,
				nil,
				bsontype.MaxKey,
			},
			{
				"ReadMinKey/incorrect type",
				(*valueReader).ReadMinKey,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.MinKey),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadMinKey/success",
				(*valueReader).ReadMinKey,
				nil,
				bsontype.MinKey,
			},
			{
				"ReadNull/incorrect type",
				(*valueReader).ReadNull,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Null),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadNull/success",
				(*valueReader).ReadNull,
				nil,
				bsontype.Null,
			},
			{
				"ReadUndefined/incorrect type",
				(*valueReader).ReadUndefined,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Undefined),
				bsontype.EmbeddedDocument,
			},
			{
				"ReadUndefined/success",
				(*valueReader).ReadUndefined,
				nil,
				bsontype.Undefined,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				err := tc.fn(vr)
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
			})
		}
	})
	t.Run("ReadObjectID", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			oid    primitive.ObjectID
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				primitive.ObjectID{},
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.ObjectID),
				bsontype.EmbeddedDocument,
			},
			{
				"not enough bytes for objectID",
				[]byte{},
				0,
				primitive.ObjectID{},
				io.EOF,
				bsontype.ObjectID,
			},
			{
				"success",
				[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
				0,
				primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
				nil,
				bsontype.ObjectID,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				oid, err := vr.ReadObjectID()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if !bytes.Equal(oid[:], tc.oid[:]) {
					t.Errorf("ObjectIDs did not match. got %v; want %v", oid, tc.oid)
				}
			})
		}
	})
	t.Run("ReadRegex", func(t *testing.T) {
		testCases := []struct {
			name    string
			data    []byte
			offset  int64
			pattern string
			options string
			err     error
			vType   bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				"",
				"",
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Regex),
				bsontype.EmbeddedDocument,
			},
			{
				"length too short",
				[]byte{},
				0,
				"",
				"",
				io.EOF,
				bsontype.Regex,
			},
			{
				"not enough bytes for options",
				[]byte{'f', 'o', 'o', 0x00},
				0,
				"",
				"",
				io.EOF,
				bsontype.Regex,
			},
			{
				"success",
				[]byte{'f', 'o', 'o', 0x00, 'b', 'a', 'r', 0x00},
				0,
				"foo",
				"bar",
				nil,
				bsontype.Regex,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				pattern, options, err := vr.ReadRegex()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if pattern != tc.pattern {
					t.Errorf("Incorrect pattern returned. got %s; want %s", pattern, tc.pattern)
				}
				if options != tc.options {
					t.Errorf("Incorrect options returned. got %s; want %s", options, tc.options)
				}
			})
		}
	})
	t.Run("ReadTimestamp", func(t *testing.T) {
		testCases := []struct {
			name   string
			data   []byte
			offset int64
			ts     uint32
			incr   uint32
			err    error
			vType  bsontype.Type
		}{
			{
				"incorrect type",
				[]byte{},
				0,
				0,
				0,
				(&valueReader{stack: []vrState{{vType: bsontype.EmbeddedDocument}}, frame: 0}).typeError(bsontype.Timestamp),
				bsontype.EmbeddedDocument,
			},
			{
				"not enough bytes for increment",
				[]byte{},
				0,
				0,
				0,
				io.EOF,
				bsontype.Timestamp,
			},
			{
				"not enough bytes for timestamp",
				[]byte{0x01, 0x02, 0x03, 0x04},
				0,
				0,
				0,
				io.EOF,
				bsontype.Timestamp,
			},
			{
				"success",
				[]byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00},
				0,
				256,
				255,
				nil,
				bsontype.Timestamp,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vr := &valueReader{
					offset: tc.offset,
					d:      tc.data,
					stack: []vrState{
						{mode: mTopLevel},
						{
							mode:  mElement,
							vType: tc.vType,
						},
					},
					frame: 1,
				}

				ts, incr, err := vr.ReadTimestamp()
				if !errequal(t, err, tc.err) {
					t.Errorf("Returned errors do not match. got %v; want %v", err, tc.err)
				}
				if ts != tc.ts {
					t.Errorf("Incorrect timestamp returned. got %d; want %d", ts, tc.ts)
				}
				if incr != tc.incr {
					t.Errorf("Incorrect increment returned. got %d; want %d", incr, tc.incr)
				}
			})
		}
	})

	t.Run("ReadBytes & Skip", func(t *testing.T) {
		index, docb := bsoncore.ReserveLength(nil)
		docb = bsoncore.AppendNullElement(docb, "foobar")
		docb = append(docb, 0x00)
		docb = bsoncore.UpdateLength(docb, index, int32(len(docb)))
		cwsbytes := bsoncore.AppendCodeWithScope(nil, "var hellow = world;", docb)
		strbytes := []byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00}
		testCases := []struct {
			name           string
			t              bsontype.Type
			data           []byte
			err            error
			offset         int64
			startingOffset int64
		}{
			{
				"Array/invalid length",
				bsontype.Array,
				[]byte{0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"Array/not enough bytes",
				bsontype.Array,
				[]byte{0x0F, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"Array/success",
				bsontype.Array,
				[]byte{0x08, 0x00, 0x00, 0x00, 0x0A, '1', 0x00, 0x00},
				nil, 8, 0,
			},
			{
				"EmbeddedDocument/invalid length",
				bsontype.EmbeddedDocument,
				[]byte{0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"EmbeddedDocument/not enough bytes",
				bsontype.EmbeddedDocument,
				[]byte{0x0F, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"EmbeddedDocument/success",
				bsontype.EmbeddedDocument,
				[]byte{0x08, 0x00, 0x00, 0x00, 0x0A, 'A', 0x00, 0x00},
				nil, 8, 0,
			},
			{
				"CodeWithScope/invalid length",
				bsontype.CodeWithScope,
				[]byte{0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"CodeWithScope/not enough bytes",
				bsontype.CodeWithScope,
				[]byte{0x0F, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"CodeWithScope/success",
				bsontype.CodeWithScope,
				cwsbytes,
				nil, 41, 0,
			},
			{
				"Binary/invalid length",
				bsontype.Binary,
				[]byte{0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"Binary/not enough bytes",
				bsontype.Binary,
				[]byte{0x0F, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"Binary/success",
				bsontype.Binary,
				[]byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				nil, 8, 0,
			},
			{
				"Boolean/invalid length",
				bsontype.Boolean,
				[]byte{},
				io.EOF, 0, 0,
			},
			{
				"Boolean/success",
				bsontype.Boolean,
				[]byte{0x01},
				nil, 1, 0,
			},
			{
				"DBPointer/invalid length",
				bsontype.DBPointer,
				[]byte{0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"DBPointer/not enough bytes",
				bsontype.DBPointer,
				[]byte{0x0F, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				io.EOF, 0, 0,
			},
			{
				"DBPointer/success",
				bsontype.DBPointer,
				[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
				nil, 17, 0,
			},
			{"DBPointer/not enough bytes", bsontype.DateTime, []byte{0x01, 0x02, 0x03, 0x04}, io.EOF, 0, 0},
			{"DBPointer/success", bsontype.DateTime, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, nil, 8, 0},
			{"Double/not enough bytes", bsontype.Double, []byte{0x01, 0x02, 0x03, 0x04}, io.EOF, 0, 0},
			{"Double/success", bsontype.Double, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, nil, 8, 0},
			{"Int64/not enough bytes", bsontype.Int64, []byte{0x01, 0x02, 0x03, 0x04}, io.EOF, 0, 0},
			{"Int64/success", bsontype.Int64, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, nil, 8, 0},
			{"Timestamp/not enough bytes", bsontype.Timestamp, []byte{0x01, 0x02, 0x03, 0x04}, io.EOF, 0, 0},
			{"Timestamp/success", bsontype.Timestamp, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, nil, 8, 0},
			{
				"Decimal128/not enough bytes",
				bsontype.Decimal128,
				[]byte{0x01, 0x02, 0x03, 0x04},
				io.EOF, 0, 0,
			},
			{
				"Decimal128/success",
				bsontype.Decimal128,
				[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
				nil, 16, 0,
			},
			{"Int32/not enough bytes", bsontype.Int32, []byte{0x01, 0x02}, io.EOF, 0, 0},
			{"Int32/success", bsontype.Int32, []byte{0x01, 0x02, 0x03, 0x04}, nil, 4, 0},
			{"Javascript/invalid length", bsontype.JavaScript, strbytes[:2], io.EOF, 0, 0},
			{"Javascript/not enough bytes", bsontype.JavaScript, strbytes[:5], io.EOF, 0, 0},
			{"Javascript/success", bsontype.JavaScript, strbytes, nil, 8, 0},
			{"String/invalid length", bsontype.String, strbytes[:2], io.EOF, 0, 0},
			{"String/not enough bytes", bsontype.String, strbytes[:5], io.EOF, 0, 0},
			{"String/success", bsontype.String, strbytes, nil, 8, 0},
			{"Symbol/invalid length", bsontype.Symbol, strbytes[:2], io.EOF, 0, 0},
			{"Symbol/not enough bytes", bsontype.Symbol, strbytes[:5], io.EOF, 0, 0},
			{"Symbol/success", bsontype.Symbol, strbytes, nil, 8, 0},
			{"MaxKey/success", bsontype.MaxKey, []byte{}, nil, 0, 0},
			{"MinKey/success", bsontype.MinKey, []byte{}, nil, 0, 0},
			{"Null/success", bsontype.Null, []byte{}, nil, 0, 0},
			{"Undefined/success", bsontype.Undefined, []byte{}, nil, 0, 0},
			{
				"ObjectID/not enough bytes",
				bsontype.ObjectID,
				[]byte{0x01, 0x02, 0x03, 0x04},
				io.EOF, 0, 0,
			},
			{
				"ObjectID/success",
				bsontype.ObjectID,
				[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
				nil, 12, 0,
			},
			{
				"Regex/not enough bytes (first string)",
				bsontype.Regex,
				[]byte{'f', 'o', 'o'},
				io.EOF, 0, 0,
			},
			{
				"Regex/not enough bytes (second string)",
				bsontype.Regex,
				[]byte{'f', 'o', 'o', 0x00, 'b', 'a', 'r'},
				io.EOF, 0, 0,
			},
			{
				"Regex/success",
				bsontype.Regex,
				[]byte{0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00, 'i', 0x00},
				nil, 9, 3,
			},
			{
				"Unknown Type",
				bsontype.Type(0),
				nil,
				fmt.Errorf("attempted to read bytes of unknown BSON type %v", bsontype.Type(0)), 0, 0,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("Skip", func(t *testing.T) {
					vr := &valueReader{
						d: tc.data,
						stack: []vrState{
							{mode: mTopLevel},
							{mode: mElement, vType: tc.t},
						},
						frame:  1,
						offset: tc.startingOffset,
					}

					err := vr.Skip()
					if !errequal(t, err, tc.err) {
						t.Errorf("Did not receive expected error; got %v; want %v", err, tc.err)
					}
					if tc.err == nil && vr.offset != tc.offset {
						t.Errorf("Offset not set at correct position; got %d; want %d", vr.offset, tc.offset)
					}
				})
				t.Run("ReadBytes", func(t *testing.T) {
					vr := &valueReader{
						d: tc.data,
						stack: []vrState{
							{mode: mTopLevel},
							{mode: mElement, vType: tc.t},
						},
						frame:  1,
						offset: tc.startingOffset,
					}

					_, got, err := vr.ReadValueBytes(nil)
					if !errequal(t, err, tc.err) {
						t.Errorf("Did not receive expected error; got %v; want %v", err, tc.err)
					}
					if tc.err == nil && vr.offset != tc.offset {
						t.Errorf("Offset not set at correct position; got %d; want %d", vr.offset, tc.offset)
					}
					if tc.err == nil && !bytes.Equal(got, tc.data[tc.startingOffset:]) {
						t.Errorf("Did not receive expected bytes. got %v; want %v", got, tc.data[tc.startingOffset:])
					}
				})
			})
		}
		t.Run("ReadValueBytes/Top Level Doc", func(t *testing.T) {
			testCases := []struct {
				name     string
				want     []byte
				wantType bsontype.Type
				wantErr  error
			}{
				{
					"success",
					bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
					bsontype.Type(0),
					nil,
				},
				{
					"wrong length",
					[]byte{0x01, 0x02, 0x03},
					bsontype.Type(0),
					io.EOF,
				},
				{
					"append bytes",
					[]byte{0x01, 0x02, 0x03, 0x04},
					bsontype.Type(0),
					io.EOF,
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					vr := &valueReader{
						d: tc.want,
						stack: []vrState{
							{mode: mTopLevel},
						},
						frame: 0,
					}
					gotType, got, gotErr := vr.ReadValueBytes(nil)
					if gotErr != tc.wantErr {
						t.Errorf("Did not receive expected error. got %v; want %v", gotErr, tc.wantErr)
					}
					if tc.wantErr == nil && gotType != tc.wantType {
						t.Errorf("Did not receive expected type. got %v; want %v", gotType, tc.wantType)
					}
					if tc.wantErr == nil && !bytes.Equal(got, tc.want) {
						t.Errorf("Did not receive expected bytes. got %v; want %v", got, tc.want)
					}
				})
			}
		})
	})

	t.Run("invalid transition", func(t *testing.T) {
		t.Run("Skip", func(t *testing.T) {
			vr := &valueReader{stack: []vrState{{mode: mTopLevel}}}
			wanterr := (&valueReader{stack: []vrState{{mode: mTopLevel}}}).invalidTransitionErr(0, "Skip", []mode{mElement, mValue})
			goterr := vr.Skip()
			if !cmp.Equal(goterr, wanterr, cmp.Comparer(compareErrors)) {
				t.Errorf("Expected correct invalid transition error. got %v; want %v", goterr, wanterr)
			}
		})
	})
	t.Run("ReadBytes", func(t *testing.T) {
		vr := &valueReader{stack: []vrState{{mode: mTopLevel}, {mode: mDocument}}, frame: 1}
		wanterr := (&valueReader{stack: []vrState{{mode: mTopLevel}, {mode: mDocument}}, frame: 1}).
			invalidTransitionErr(0, "ReadValueBytes", []mode{mElement, mValue})
		_, _, goterr := vr.ReadValueBytes(nil)
		if !cmp.Equal(goterr, wanterr, cmp.Comparer(compareErrors)) {
			t.Errorf("Expected correct invalid transition error. got %v; want %v", goterr, wanterr)
		}
	})
}

func errequal(t *testing.T, err1, err2 error) bool {
	t.Helper()
	if err1 == nil && err2 == nil { // If they are both nil, they are equal
		return true
	}
	if err1 == nil || err2 == nil { // If only one is nil, they are not equal
		return false
	}

	if err1 == err2 { // They are the same error, they are equal
		return true
	}

	if err1.Error() == err2.Error() { // They string formats match, they are equal
		return true
	}

	return false
}
