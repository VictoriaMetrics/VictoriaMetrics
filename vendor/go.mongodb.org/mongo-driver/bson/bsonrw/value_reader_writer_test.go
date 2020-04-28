// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

type VRWInvoked byte

const (
	llvrwNothing VRWInvoked = iota
	llvrwReadArray
	llvrwReadBinary
	llvrwReadBoolean
	llvrwReadDocument
	llvrwReadCodeWithScope
	llvrwReadDBPointer
	llvrwReadDateTime
	llvrwReadDecimal128
	llvrwReadDouble
	llvrwReadInt32
	llvrwReadInt64
	llvrwReadJavascript
	llvrwReadMaxKey
	llvrwReadMinKey
	llvrwReadNull
	llvrwReadObjectID
	llvrwReadRegex
	llvrwReadString
	llvrwReadSymbol
	llvrwReadTimestamp
	llvrwReadUndefined
	llvrwReadElement
	llvrwReadValue
	llvrwWriteArray
	llvrwWriteBinary
	llvrwWriteBinaryWithSubtype
	llvrwWriteBoolean
	llvrwWriteCodeWithScope
	llvrwWriteDBPointer
	llvrwWriteDateTime
	llvrwWriteDecimal128
	llvrwWriteDouble
	llvrwWriteInt32
	llvrwWriteInt64
	llvrwWriteJavascript
	llvrwWriteMaxKey
	llvrwWriteMinKey
	llvrwWriteNull
	llvrwWriteObjectID
	llvrwWriteRegex
	llvrwWriteString
	llvrwWriteDocument
	llvrwWriteSymbol
	llvrwWriteTimestamp
	llvrwWriteUndefined
	llvrwWriteDocumentElement
	llvrwWriteDocumentEnd
	llvrwWriteArrayElement
	llvrwWriteArrayEnd
)

type TestValueReaderWriter struct {
	t        *testing.T
	invoked  VRWInvoked
	readval  interface{}
	bsontype bsontype.Type
	err      error
	errAfter VRWInvoked // error after this method is called
}

func (llvrw *TestValueReaderWriter) Type() bsontype.Type {
	return llvrw.bsontype
}

func (llvrw *TestValueReaderWriter) Skip() error {
	panic("not implemented")
}

func (llvrw *TestValueReaderWriter) ReadArray() (ArrayReader, error) {
	llvrw.invoked = llvrwReadArray
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}

	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) ReadBinary() (b []byte, btype byte, err error) {
	llvrw.invoked = llvrwReadBinary
	if llvrw.errAfter == llvrw.invoked {
		return nil, 0x00, llvrw.err
	}

	switch tt := llvrw.readval.(type) {
	case bsoncore.Value:
		subtype, data, _, ok := bsoncore.ReadBinary(tt.Data)
		if !ok {
			llvrw.t.Error("Invalid Value provided for return value of ReadBinary.")
			return nil, 0x00, nil
		}
		return data, subtype, nil
	default:
		llvrw.t.Errorf("Incorrect type provided for return value of ReadBinary: %T", llvrw.readval)
		return nil, 0x00, nil
	}
}

func (llvrw *TestValueReaderWriter) ReadBoolean() (bool, error) {
	llvrw.invoked = llvrwReadBoolean
	if llvrw.errAfter == llvrw.invoked {
		return false, llvrw.err
	}

	b, ok := llvrw.readval.(bool)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadBoolean: %T", llvrw.readval)
		return false, nil
	}

	return b, llvrw.err
}

func (llvrw *TestValueReaderWriter) ReadDocument() (DocumentReader, error) {
	llvrw.invoked = llvrwReadDocument
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}

	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) ReadCodeWithScope() (code string, dr DocumentReader, err error) {
	llvrw.invoked = llvrwReadCodeWithScope
	if llvrw.errAfter == llvrw.invoked {
		return "", nil, llvrw.err
	}

	return "", llvrw, nil
}

func (llvrw *TestValueReaderWriter) ReadDBPointer() (ns string, oid primitive.ObjectID, err error) {
	llvrw.invoked = llvrwReadDBPointer
	if llvrw.errAfter == llvrw.invoked {
		return "", primitive.ObjectID{}, llvrw.err
	}

	switch tt := llvrw.readval.(type) {
	case bsoncore.Value:
		ns, oid, _, ok := bsoncore.ReadDBPointer(tt.Data)
		if !ok {
			llvrw.t.Error("Invalid Value instance provided for return value of ReadDBPointer")
			return "", primitive.ObjectID{}, nil
		}
		return ns, oid, nil
	default:
		llvrw.t.Errorf("Incorrect type provided for return value of ReadDBPointer: %T", llvrw.readval)
		return "", primitive.ObjectID{}, nil
	}
}

func (llvrw *TestValueReaderWriter) ReadDateTime() (int64, error) {
	llvrw.invoked = llvrwReadDateTime
	if llvrw.errAfter == llvrw.invoked {
		return 0, llvrw.err
	}

	dt, ok := llvrw.readval.(int64)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadDateTime: %T", llvrw.readval)
		return 0, nil
	}

	return dt, nil
}

func (llvrw *TestValueReaderWriter) ReadDecimal128() (primitive.Decimal128, error) {
	llvrw.invoked = llvrwReadDecimal128
	if llvrw.errAfter == llvrw.invoked {
		return primitive.Decimal128{}, llvrw.err
	}

	d128, ok := llvrw.readval.(primitive.Decimal128)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadDecimal128: %T", llvrw.readval)
		return primitive.Decimal128{}, nil
	}

	return d128, nil
}

func (llvrw *TestValueReaderWriter) ReadDouble() (float64, error) {
	llvrw.invoked = llvrwReadDouble
	if llvrw.errAfter == llvrw.invoked {
		return 0, llvrw.err
	}

	f64, ok := llvrw.readval.(float64)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadDouble: %T", llvrw.readval)
		return 0, nil
	}

	return f64, nil
}

func (llvrw *TestValueReaderWriter) ReadInt32() (int32, error) {
	llvrw.invoked = llvrwReadInt32
	if llvrw.errAfter == llvrw.invoked {
		return 0, llvrw.err
	}

	i32, ok := llvrw.readval.(int32)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadInt32: %T", llvrw.readval)
		return 0, nil
	}

	return i32, nil
}

func (llvrw *TestValueReaderWriter) ReadInt64() (int64, error) {
	llvrw.invoked = llvrwReadInt64
	if llvrw.errAfter == llvrw.invoked {
		return 0, llvrw.err
	}
	i64, ok := llvrw.readval.(int64)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadInt64: %T", llvrw.readval)
		return 0, nil
	}

	return i64, nil
}

func (llvrw *TestValueReaderWriter) ReadJavascript() (code string, err error) {
	llvrw.invoked = llvrwReadJavascript
	if llvrw.errAfter == llvrw.invoked {
		return "", llvrw.err
	}
	js, ok := llvrw.readval.(string)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadJavascript: %T", llvrw.readval)
		return "", nil
	}

	return js, nil
}

func (llvrw *TestValueReaderWriter) ReadMaxKey() error {
	llvrw.invoked = llvrwReadMaxKey
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}

	return nil
}

func (llvrw *TestValueReaderWriter) ReadMinKey() error {
	llvrw.invoked = llvrwReadMinKey
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}

	return nil
}

func (llvrw *TestValueReaderWriter) ReadNull() error {
	llvrw.invoked = llvrwReadNull
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}

	return nil
}

func (llvrw *TestValueReaderWriter) ReadObjectID() (primitive.ObjectID, error) {
	llvrw.invoked = llvrwReadObjectID
	if llvrw.errAfter == llvrw.invoked {
		return primitive.ObjectID{}, llvrw.err
	}
	oid, ok := llvrw.readval.(primitive.ObjectID)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadObjectID: %T", llvrw.readval)
		return primitive.ObjectID{}, nil
	}

	return oid, nil
}

func (llvrw *TestValueReaderWriter) ReadRegex() (pattern string, options string, err error) {
	llvrw.invoked = llvrwReadRegex
	if llvrw.errAfter == llvrw.invoked {
		return "", "", llvrw.err
	}
	switch tt := llvrw.readval.(type) {
	case bsoncore.Value:
		pattern, options, _, ok := bsoncore.ReadRegex(tt.Data)
		if !ok {
			llvrw.t.Error("Invalid Value instance provided for ReadRegex")
			return "", "", nil
		}
		return pattern, options, nil
	default:
		llvrw.t.Errorf("Incorrect type provided for return value of ReadRegex: %T", llvrw.readval)
		return "", "", nil
	}
}

func (llvrw *TestValueReaderWriter) ReadString() (string, error) {
	llvrw.invoked = llvrwReadString
	if llvrw.errAfter == llvrw.invoked {
		return "", llvrw.err
	}
	str, ok := llvrw.readval.(string)
	if !ok {
		llvrw.t.Errorf("Incorrect type provided for return value of ReadString: %T", llvrw.readval)
		return "", nil
	}

	return str, nil
}

func (llvrw *TestValueReaderWriter) ReadSymbol() (symbol string, err error) {
	llvrw.invoked = llvrwReadSymbol
	if llvrw.errAfter == llvrw.invoked {
		return "", llvrw.err
	}
	switch tt := llvrw.readval.(type) {
	case bsoncore.Value:
		symbol, _, ok := bsoncore.ReadSymbol(tt.Data)
		if !ok {
			llvrw.t.Error("Invalid Value instance provided for ReadSymbol")
			return "", nil
		}
		return symbol, nil
	default:
		llvrw.t.Errorf("Incorrect type provided for return value of ReadSymbol: %T", llvrw.readval)
		return "", nil
	}
}

func (llvrw *TestValueReaderWriter) ReadTimestamp() (t uint32, i uint32, err error) {
	llvrw.invoked = llvrwReadTimestamp
	if llvrw.errAfter == llvrw.invoked {
		return 0, 0, llvrw.err
	}
	switch tt := llvrw.readval.(type) {
	case bsoncore.Value:
		t, i, _, ok := bsoncore.ReadTimestamp(tt.Data)
		if !ok {
			llvrw.t.Errorf("Invalid Value instance provided for return value of ReadTimestamp")
			return 0, 0, nil
		}
		return t, i, nil
	default:
		llvrw.t.Errorf("Incorrect type provided for return value of ReadTimestamp: %T", llvrw.readval)
		return 0, 0, nil
	}
}

func (llvrw *TestValueReaderWriter) ReadUndefined() error {
	llvrw.invoked = llvrwReadUndefined
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}

	return nil
}

func (llvrw *TestValueReaderWriter) WriteArray() (ArrayWriter, error) {
	llvrw.invoked = llvrwWriteArray
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}
	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteBinary(b []byte) error {
	llvrw.invoked = llvrwWriteBinary
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteBinaryWithSubtype(b []byte, btype byte) error {
	llvrw.invoked = llvrwWriteBinaryWithSubtype
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteBoolean(bool) error {
	llvrw.invoked = llvrwWriteBoolean
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteCodeWithScope(code string) (DocumentWriter, error) {
	llvrw.invoked = llvrwWriteCodeWithScope
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}
	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteDBPointer(ns string, oid primitive.ObjectID) error {
	llvrw.invoked = llvrwWriteDBPointer
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteDateTime(dt int64) error {
	llvrw.invoked = llvrwWriteDateTime
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteDecimal128(primitive.Decimal128) error {
	llvrw.invoked = llvrwWriteDecimal128
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteDouble(float64) error {
	llvrw.invoked = llvrwWriteDouble
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteInt32(int32) error {
	llvrw.invoked = llvrwWriteInt32
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteInt64(int64) error {
	llvrw.invoked = llvrwWriteInt64
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteJavascript(code string) error {
	llvrw.invoked = llvrwWriteJavascript
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteMaxKey() error {
	llvrw.invoked = llvrwWriteMaxKey
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteMinKey() error {
	llvrw.invoked = llvrwWriteMinKey
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteNull() error {
	llvrw.invoked = llvrwWriteNull
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteObjectID(primitive.ObjectID) error {
	llvrw.invoked = llvrwWriteObjectID
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteRegex(pattern string, options string) error {
	llvrw.invoked = llvrwWriteRegex
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteString(string) error {
	llvrw.invoked = llvrwWriteString
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteDocument() (DocumentWriter, error) {
	llvrw.invoked = llvrwWriteDocument
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}
	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteSymbol(symbol string) error {
	llvrw.invoked = llvrwWriteSymbol
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteTimestamp(t uint32, i uint32) error {
	llvrw.invoked = llvrwWriteTimestamp
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) WriteUndefined() error {
	llvrw.invoked = llvrwWriteUndefined
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}
	return nil
}

func (llvrw *TestValueReaderWriter) ReadElement() (string, ValueReader, error) {
	llvrw.invoked = llvrwReadElement
	if llvrw.errAfter == llvrw.invoked {
		return "", nil, llvrw.err
	}

	return "", llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteDocumentElement(string) (ValueWriter, error) {
	llvrw.invoked = llvrwWriteDocumentElement
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}

	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteDocumentEnd() error {
	llvrw.invoked = llvrwWriteDocumentEnd
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}

	return nil
}

func (llvrw *TestValueReaderWriter) ReadValue() (ValueReader, error) {
	llvrw.invoked = llvrwReadValue
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}

	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteArrayElement() (ValueWriter, error) {
	llvrw.invoked = llvrwWriteArrayElement
	if llvrw.errAfter == llvrw.invoked {
		return nil, llvrw.err
	}

	return llvrw, nil
}

func (llvrw *TestValueReaderWriter) WriteArrayEnd() error {
	llvrw.invoked = llvrwWriteArrayEnd
	if llvrw.errAfter == llvrw.invoked {
		return llvrw.err
	}

	return nil
}
