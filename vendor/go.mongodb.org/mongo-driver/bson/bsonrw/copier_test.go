// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestCopier(t *testing.T) {
	t.Run("CopyDocument", func(t *testing.T) {
		t.Run("ReadDocument Error", func(t *testing.T) {
			want := errors.New("ReadDocumentError")
			src := &TestValueReaderWriter{t: t, err: want, errAfter: llvrwReadDocument}
			got := Copier{}.CopyDocument(nil, src)
			if !compareErrors(got, want) {
				t.Errorf("Did not receive correct error. got %v; want %v", got, want)
			}
		})
		t.Run("WriteDocument Error", func(t *testing.T) {
			want := errors.New("WriteDocumentError")
			src := &TestValueReaderWriter{}
			dst := &TestValueReaderWriter{t: t, err: want, errAfter: llvrwWriteDocument}
			got := Copier{}.CopyDocument(dst, src)
			if !compareErrors(got, want) {
				t.Errorf("Did not receive correct error. got %v; want %v", got, want)
			}
		})
		t.Run("success", func(t *testing.T) {
			idx, doc := bsoncore.AppendDocumentStart(nil)
			doc = bsoncore.AppendStringElement(doc, "Hello", "world")
			doc, err := bsoncore.AppendDocumentEnd(doc, idx)
			noerr(t, err)
			src := newValueReader(doc)
			dst := newValueWriterFromSlice(make([]byte, 0))
			want := doc
			err = Copier{}.CopyDocument(dst, src)
			noerr(t, err)
			got := dst.buf
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, want)
			}
		})
	})
	t.Run("copyArray", func(t *testing.T) {
		t.Run("ReadArray Error", func(t *testing.T) {
			want := errors.New("ReadArrayError")
			src := &TestValueReaderWriter{t: t, err: want, errAfter: llvrwReadArray}
			got := Copier{}.copyArray(nil, src)
			if !compareErrors(got, want) {
				t.Errorf("Did not receive correct error. got %v; want %v", got, want)
			}
		})
		t.Run("WriteArray Error", func(t *testing.T) {
			want := errors.New("WriteArrayError")
			src := &TestValueReaderWriter{}
			dst := &TestValueReaderWriter{t: t, err: want, errAfter: llvrwWriteArray}
			got := Copier{}.copyArray(dst, src)
			if !compareErrors(got, want) {
				t.Errorf("Did not receive correct error. got %v; want %v", got, want)
			}
		})
		t.Run("success", func(t *testing.T) {
			idx, doc := bsoncore.AppendDocumentStart(nil)
			aidx, doc := bsoncore.AppendArrayElementStart(doc, "foo")
			doc = bsoncore.AppendStringElement(doc, "0", "Hello, world!")
			doc, err := bsoncore.AppendArrayEnd(doc, aidx)
			noerr(t, err)
			doc, err = bsoncore.AppendDocumentEnd(doc, idx)
			noerr(t, err)
			src := newValueReader(doc)

			_, err = src.ReadDocument()
			noerr(t, err)
			_, _, err = src.ReadElement()
			noerr(t, err)

			dst := newValueWriterFromSlice(make([]byte, 0))
			_, err = dst.WriteDocument()
			noerr(t, err)
			_, err = dst.WriteDocumentElement("foo")
			noerr(t, err)
			want := doc

			err = Copier{}.copyArray(dst, src)
			noerr(t, err)

			err = dst.WriteDocumentEnd()
			noerr(t, err)

			got := dst.buf
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, want)
			}
		})
	})
	t.Run("CopyValue", func(t *testing.T) {
		testCases := []struct {
			name string
			dst  *TestValueReaderWriter
			src  *TestValueReaderWriter
			err  error
		}{
			{
				"Double/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Double, err: errors.New("1"), errAfter: llvrwReadDouble},
				errors.New("1"),
			},
			{
				"Double/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Double, err: errors.New("2"), errAfter: llvrwWriteDouble},
				&TestValueReaderWriter{bsontype: bsontype.Double, readval: float64(3.14159)},
				errors.New("2"),
			},
			{
				"String/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.String, err: errors.New("1"), errAfter: llvrwReadString},
				errors.New("1"),
			},
			{
				"String/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.String, err: errors.New("2"), errAfter: llvrwWriteString},
				&TestValueReaderWriter{bsontype: bsontype.String, readval: string("hello, world")},
				errors.New("2"),
			},
			{
				"Document/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.EmbeddedDocument, err: errors.New("1"), errAfter: llvrwReadDocument},
				errors.New("1"),
			},
			{
				"Array/dst/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Array, err: errors.New("2"), errAfter: llvrwReadArray},
				errors.New("2"),
			},
			{
				"Binary/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Binary, err: errors.New("1"), errAfter: llvrwReadBinary},
				errors.New("1"),
			},
			{
				"Binary/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Binary, err: errors.New("2"), errAfter: llvrwWriteBinaryWithSubtype},
				&TestValueReaderWriter{
					bsontype: bsontype.Binary,
					readval: bsoncore.Value{
						Type: bsontype.Binary,
						Data: []byte{0x03, 0x00, 0x00, 0x00, 0xFF, 0x01, 0x02, 0x03},
					},
				},
				errors.New("2"),
			},
			{
				"Undefined/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Undefined, err: errors.New("1"), errAfter: llvrwReadUndefined},
				errors.New("1"),
			},
			{
				"Undefined/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Undefined, err: errors.New("2"), errAfter: llvrwWriteUndefined},
				&TestValueReaderWriter{bsontype: bsontype.Undefined},
				errors.New("2"),
			},
			{
				"ObjectID/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.ObjectID, err: errors.New("1"), errAfter: llvrwReadObjectID},
				errors.New("1"),
			},
			{
				"ObjectID/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.ObjectID, err: errors.New("2"), errAfter: llvrwWriteObjectID},
				&TestValueReaderWriter{bsontype: bsontype.ObjectID, readval: primitive.ObjectID{0x01, 0x02, 0x03}},
				errors.New("2"),
			},
			{
				"Boolean/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Boolean, err: errors.New("1"), errAfter: llvrwReadBoolean},
				errors.New("1"),
			},
			{
				"Boolean/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Boolean, err: errors.New("2"), errAfter: llvrwWriteBoolean},
				&TestValueReaderWriter{bsontype: bsontype.Boolean, readval: bool(true)},
				errors.New("2"),
			},
			{
				"DateTime/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.DateTime, err: errors.New("1"), errAfter: llvrwReadDateTime},
				errors.New("1"),
			},
			{
				"DateTime/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.DateTime, err: errors.New("2"), errAfter: llvrwWriteDateTime},
				&TestValueReaderWriter{bsontype: bsontype.DateTime, readval: int64(1234567890)},
				errors.New("2"),
			},
			{
				"Null/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Null, err: errors.New("1"), errAfter: llvrwReadNull},
				errors.New("1"),
			},
			{
				"Null/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Null, err: errors.New("2"), errAfter: llvrwWriteNull},
				&TestValueReaderWriter{bsontype: bsontype.Null},
				errors.New("2"),
			},
			{
				"Regex/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Regex, err: errors.New("1"), errAfter: llvrwReadRegex},
				errors.New("1"),
			},
			{
				"Regex/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Regex, err: errors.New("2"), errAfter: llvrwWriteRegex},
				&TestValueReaderWriter{
					bsontype: bsontype.Regex,
					readval: bsoncore.Value{
						Type: bsontype.Regex,
						Data: bsoncore.AppendRegex(nil, "hello", "world"),
					},
				},
				errors.New("2"),
			},
			{
				"DBPointer/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.DBPointer, err: errors.New("1"), errAfter: llvrwReadDBPointer},
				errors.New("1"),
			},
			{
				"DBPointer/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.DBPointer, err: errors.New("2"), errAfter: llvrwWriteDBPointer},
				&TestValueReaderWriter{
					bsontype: bsontype.DBPointer,
					readval: bsoncore.Value{
						Type: bsontype.DBPointer,
						Data: bsoncore.AppendDBPointer(nil, "foo", primitive.ObjectID{0x01, 0x02, 0x03}),
					},
				},
				errors.New("2"),
			},
			{
				"Javascript/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.JavaScript, err: errors.New("1"), errAfter: llvrwReadJavascript},
				errors.New("1"),
			},
			{
				"Javascript/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.JavaScript, err: errors.New("2"), errAfter: llvrwWriteJavascript},
				&TestValueReaderWriter{bsontype: bsontype.JavaScript, readval: string("hello, world")},
				errors.New("2"),
			},
			{
				"Symbol/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Symbol, err: errors.New("1"), errAfter: llvrwReadSymbol},
				errors.New("1"),
			},
			{
				"Symbol/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Symbol, err: errors.New("2"), errAfter: llvrwWriteSymbol},
				&TestValueReaderWriter{
					bsontype: bsontype.Symbol,
					readval: bsoncore.Value{
						Type: bsontype.Symbol,
						Data: bsoncore.AppendSymbol(nil, "hello, world"),
					},
				},
				errors.New("2"),
			},
			{
				"CodeWithScope/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.CodeWithScope, err: errors.New("1"), errAfter: llvrwReadCodeWithScope},
				errors.New("1"),
			},
			{
				"CodeWithScope/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.CodeWithScope, err: errors.New("2"), errAfter: llvrwWriteCodeWithScope},
				&TestValueReaderWriter{bsontype: bsontype.CodeWithScope},
				errors.New("2"),
			},
			{
				"CodeWithScope/dst/copyDocumentCore error",
				&TestValueReaderWriter{err: errors.New("3"), errAfter: llvrwWriteDocumentElement},
				&TestValueReaderWriter{bsontype: bsontype.CodeWithScope},
				errors.New("3"),
			},
			{
				"Int32/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Int32, err: errors.New("1"), errAfter: llvrwReadInt32},
				errors.New("1"),
			},
			{
				"Int32/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Int32, err: errors.New("2"), errAfter: llvrwWriteInt32},
				&TestValueReaderWriter{bsontype: bsontype.Int32, readval: int32(12345)},
				errors.New("2"),
			},
			{
				"Timestamp/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Timestamp, err: errors.New("1"), errAfter: llvrwReadTimestamp},
				errors.New("1"),
			},
			{
				"Timestamp/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Timestamp, err: errors.New("2"), errAfter: llvrwWriteTimestamp},
				&TestValueReaderWriter{
					bsontype: bsontype.Timestamp,
					readval: bsoncore.Value{
						Type: bsontype.Timestamp,
						Data: bsoncore.AppendTimestamp(nil, 12345, 67890),
					},
				},
				errors.New("2"),
			},
			{
				"Int64/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Int64, err: errors.New("1"), errAfter: llvrwReadInt64},
				errors.New("1"),
			},
			{
				"Int64/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Int64, err: errors.New("2"), errAfter: llvrwWriteInt64},
				&TestValueReaderWriter{bsontype: bsontype.Int64, readval: int64(1234567890)},
				errors.New("2"),
			},
			{
				"Decimal128/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.Decimal128, err: errors.New("1"), errAfter: llvrwReadDecimal128},
				errors.New("1"),
			},
			{
				"Decimal128/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.Decimal128, err: errors.New("2"), errAfter: llvrwWriteDecimal128},
				&TestValueReaderWriter{bsontype: bsontype.Decimal128, readval: primitive.NewDecimal128(12345, 67890)},
				errors.New("2"),
			},
			{
				"MinKey/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.MinKey, err: errors.New("1"), errAfter: llvrwReadMinKey},
				errors.New("1"),
			},
			{
				"MinKey/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.MinKey, err: errors.New("2"), errAfter: llvrwWriteMinKey},
				&TestValueReaderWriter{bsontype: bsontype.MinKey},
				errors.New("2"),
			},
			{
				"MaxKey/src/error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{bsontype: bsontype.MaxKey, err: errors.New("1"), errAfter: llvrwReadMaxKey},
				errors.New("1"),
			},
			{
				"MaxKey/dst/error",
				&TestValueReaderWriter{bsontype: bsontype.MaxKey, err: errors.New("2"), errAfter: llvrwWriteMaxKey},
				&TestValueReaderWriter{bsontype: bsontype.MaxKey},
				errors.New("2"),
			},
			{
				"Unknown BSON type error",
				&TestValueReaderWriter{},
				&TestValueReaderWriter{},
				fmt.Errorf("Cannot copy unknown BSON type %s", bsontype.Type(0)),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				tc.dst.t, tc.src.t = t, t
				err := Copier{}.CopyValue(tc.dst, tc.src)
				if !compareErrors(err, tc.err) {
					t.Errorf("Did not receive expected error. got %v; want %v", err, tc.err)
				}
			})
		}
	})
	t.Run("CopyValueFromBytes", func(t *testing.T) {
		t.Run("BytesWriter", func(t *testing.T) {
			vw := newValueWriterFromSlice(make([]byte, 0))
			_, err := vw.WriteDocument()
			noerr(t, err)
			_, err = vw.WriteDocumentElement("foo")
			noerr(t, err)
			err = Copier{}.CopyValueFromBytes(vw, bsontype.String, bsoncore.AppendString(nil, "bar"))
			noerr(t, err)
			err = vw.WriteDocumentEnd()
			noerr(t, err)
			var idx int32
			want, err := bsoncore.AppendDocumentEnd(
				bsoncore.AppendStringElement(
					bsoncore.AppendDocumentStartInline(nil, &idx),
					"foo", "bar",
				),
				idx,
			)
			noerr(t, err)
			got := vw.buf
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes are not equal. got %v; want %v", got, want)
			}
		})
		t.Run("Non BytesWriter", func(t *testing.T) {
			llvrw := &TestValueReaderWriter{t: t}
			err := Copier{}.CopyValueFromBytes(llvrw, bsontype.String, bsoncore.AppendString(nil, "bar"))
			noerr(t, err)
			got, want := llvrw.invoked, llvrwWriteString
			if got != want {
				t.Errorf("Incorrect method invoked on llvrw. got %v; want %v", got, want)
			}
		})
	})
	t.Run("CopyValueToBytes", func(t *testing.T) {
		t.Run("BytesReader", func(t *testing.T) {
			var idx int32
			b, err := bsoncore.AppendDocumentEnd(
				bsoncore.AppendStringElement(
					bsoncore.AppendDocumentStartInline(nil, &idx),
					"hello", "world",
				),
				idx,
			)
			noerr(t, err)
			vr := newValueReader(b)
			_, err = vr.ReadDocument()
			noerr(t, err)
			_, _, err = vr.ReadElement()
			noerr(t, err)
			btype, got, err := Copier{}.CopyValueToBytes(vr)
			noerr(t, err)
			want := bsoncore.AppendString(nil, "world")
			if btype != bsontype.String {
				t.Errorf("Incorrect type returned. got %v; want %v", btype, bsontype.String)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes do not match. got %v; want %v", got, want)
			}
		})
		t.Run("Non BytesReader", func(t *testing.T) {
			llvrw := &TestValueReaderWriter{t: t, bsontype: bsontype.String, readval: string("Hello, world!")}
			btype, got, err := Copier{}.CopyValueToBytes(llvrw)
			noerr(t, err)
			want := bsoncore.AppendString(nil, "Hello, world!")
			if btype != bsontype.String {
				t.Errorf("Incorrect type returned. got %v; want %v", btype, bsontype.String)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes do not match. got %v; want %v", got, want)
			}
		})
	})
	t.Run("AppendValueBytes", func(t *testing.T) {
		t.Run("BytesReader", func(t *testing.T) {
			var idx int32
			b, err := bsoncore.AppendDocumentEnd(
				bsoncore.AppendStringElement(
					bsoncore.AppendDocumentStartInline(nil, &idx),
					"hello", "world",
				),
				idx,
			)
			noerr(t, err)
			vr := newValueReader(b)
			_, err = vr.ReadDocument()
			noerr(t, err)
			_, _, err = vr.ReadElement()
			noerr(t, err)
			btype, got, err := Copier{}.AppendValueBytes(nil, vr)
			noerr(t, err)
			want := bsoncore.AppendString(nil, "world")
			if btype != bsontype.String {
				t.Errorf("Incorrect type returned. got %v; want %v", btype, bsontype.String)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes do not match. got %v; want %v", got, want)
			}
		})
		t.Run("Non BytesReader", func(t *testing.T) {
			llvrw := &TestValueReaderWriter{t: t, bsontype: bsontype.String, readval: string("Hello, world!")}
			btype, got, err := Copier{}.AppendValueBytes(nil, llvrw)
			noerr(t, err)
			want := bsoncore.AppendString(nil, "Hello, world!")
			if btype != bsontype.String {
				t.Errorf("Incorrect type returned. got %v; want %v", btype, bsontype.String)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Bytes do not match. got %v; want %v", got, want)
			}
		})
		t.Run("CopyValue error", func(t *testing.T) {
			want := errors.New("CopyValue error")
			llvrw := &TestValueReaderWriter{t: t, bsontype: bsontype.String, err: want, errAfter: llvrwReadString}
			_, _, got := Copier{}.AppendValueBytes(make([]byte, 0), llvrw)
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
	})
}
