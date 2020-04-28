// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonrw

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestExtJSONValueWriter(t *testing.T) {
	oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
	testCases := []struct {
		name   string
		fn     interface{}
		params []interface{}
	}{
		{
			"WriteBinary",
			(*extJSONValueWriter).WriteBinary,
			[]interface{}{[]byte{0x01, 0x02, 0x03}},
		},
		{
			"WriteBinaryWithSubtype (not 0x02)",
			(*extJSONValueWriter).WriteBinaryWithSubtype,
			[]interface{}{[]byte{0x01, 0x02, 0x03}, byte(0xFF)},
		},
		{
			"WriteBinaryWithSubtype (0x02)",
			(*extJSONValueWriter).WriteBinaryWithSubtype,
			[]interface{}{[]byte{0x01, 0x02, 0x03}, byte(0x02)},
		},
		{
			"WriteBoolean",
			(*extJSONValueWriter).WriteBoolean,
			[]interface{}{true},
		},
		{
			"WriteDBPointer",
			(*extJSONValueWriter).WriteDBPointer,
			[]interface{}{"bar", oid},
		},
		{
			"WriteDateTime",
			(*extJSONValueWriter).WriteDateTime,
			[]interface{}{int64(12345678)},
		},
		{
			"WriteDecimal128",
			(*extJSONValueWriter).WriteDecimal128,
			[]interface{}{primitive.NewDecimal128(10, 20)},
		},
		{
			"WriteDouble",
			(*extJSONValueWriter).WriteDouble,
			[]interface{}{float64(3.14159)},
		},
		{
			"WriteInt32",
			(*extJSONValueWriter).WriteInt32,
			[]interface{}{int32(123456)},
		},
		{
			"WriteInt64",
			(*extJSONValueWriter).WriteInt64,
			[]interface{}{int64(1234567890)},
		},
		{
			"WriteJavascript",
			(*extJSONValueWriter).WriteJavascript,
			[]interface{}{"var foo = 'bar';"},
		},
		{
			"WriteMaxKey",
			(*extJSONValueWriter).WriteMaxKey,
			[]interface{}{},
		},
		{
			"WriteMinKey",
			(*extJSONValueWriter).WriteMinKey,
			[]interface{}{},
		},
		{
			"WriteNull",
			(*extJSONValueWriter).WriteNull,
			[]interface{}{},
		},
		{
			"WriteObjectID",
			(*extJSONValueWriter).WriteObjectID,
			[]interface{}{oid},
		},
		{
			"WriteRegex",
			(*extJSONValueWriter).WriteRegex,
			[]interface{}{"bar", "baz"},
		},
		{
			"WriteString",
			(*extJSONValueWriter).WriteString,
			[]interface{}{"hello, world!"},
		},
		{
			"WriteSymbol",
			(*extJSONValueWriter).WriteSymbol,
			[]interface{}{"symbollolz"},
		},
		{
			"WriteTimestamp",
			(*extJSONValueWriter).WriteTimestamp,
			[]interface{}{uint32(10), uint32(20)},
		},
		{
			"WriteUndefined",
			(*extJSONValueWriter).WriteUndefined,
			[]interface{}{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn := reflect.ValueOf(tc.fn)
			if fn.Kind() != reflect.Func {
				t.Fatalf("fn must be of kind Func but it is a %v", fn.Kind())
			}
			if fn.Type().NumIn() != len(tc.params)+1 || fn.Type().In(0) != reflect.TypeOf((*extJSONValueWriter)(nil)) {
				t.Fatalf("fn must have at least one parameter and the first parameter must be a *valueWriter")
			}
			if fn.Type().NumOut() != 1 || fn.Type().Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
				t.Fatalf("fn must have one return value and it must be an error.")
			}
			params := make([]reflect.Value, 1, len(tc.params)+1)
			ejvw := newExtJSONWriter(ioutil.Discard, true, true)
			params[0] = reflect.ValueOf(ejvw)
			for _, param := range tc.params {
				params = append(params, reflect.ValueOf(param))
			}

			t.Run("incorrect transition", func(t *testing.T) {
				results := fn.Call(params)
				got := results[0].Interface().(error)
				fnName := tc.name
				if strings.Contains(fnName, "WriteBinary") {
					fnName = "WriteBinaryWithSubtype"
				}
				want := TransitionError{current: mTopLevel, name: fnName, modes: []mode{mElement, mValue},
					action: "write"}
				if !compareErrors(got, want) {
					t.Errorf("Errors do not match. got %v; want %v", got, want)
				}
			})
		})
	}

	t.Run("WriteArray", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mArray)
		want := TransitionError{current: mArray, destination: mArray, parent: mTopLevel,
			name: "WriteArray", modes: []mode{mElement, mValue}, action: "write"}
		_, got := ejvw.WriteArray()
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})
	t.Run("WriteCodeWithScope", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mArray)
		want := TransitionError{current: mArray, destination: mCodeWithScope, parent: mTopLevel,
			name: "WriteCodeWithScope", modes: []mode{mElement, mValue}, action: "write"}
		_, got := ejvw.WriteCodeWithScope("")
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})
	t.Run("WriteDocument", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mArray)
		want := TransitionError{current: mArray, destination: mDocument, parent: mTopLevel,
			name: "WriteDocument", modes: []mode{mElement, mValue, mTopLevel}, action: "write"}
		_, got := ejvw.WriteDocument()
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})
	t.Run("WriteDocumentElement", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mElement)
		want := TransitionError{current: mElement,
			destination: mElement,
			parent:      mTopLevel,
			name:        "WriteDocumentElement",
			modes:       []mode{mDocument, mTopLevel, mCodeWithScope},
			action:      "write"}
		_, got := ejvw.WriteDocumentElement("")
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})
	t.Run("WriteDocumentEnd", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mElement)
		want := fmt.Errorf("incorrect mode to end document: %s", mElement)
		got := ejvw.WriteDocumentEnd()
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})
	t.Run("WriteArrayElement", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mElement)
		want := TransitionError{current: mElement,
			destination: mValue,
			parent:      mTopLevel,
			name:        "WriteArrayElement",
			modes:       []mode{mArray},
			action:      "write"}
		_, got := ejvw.WriteArrayElement()
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})
	t.Run("WriteArrayEnd", func(t *testing.T) {
		ejvw := newExtJSONWriter(ioutil.Discard, true, true)
		ejvw.push(mElement)
		want := fmt.Errorf("incorrect mode to end array: %s", mElement)
		got := ejvw.WriteArrayEnd()
		if !compareErrors(got, want) {
			t.Errorf("Did not get expected error. got %v; want %v", got, want)
		}
	})

	t.Run("WriteBytes", func(t *testing.T) {
		t.Run("writeElementHeader error", func(t *testing.T) {
			ejvw := newExtJSONWriterFromSlice(nil, true, true)
			want := TransitionError{current: mTopLevel, destination: mode(0),
				name: "WriteBinaryWithSubtype", modes: []mode{mElement, mValue}, action: "write"}
			got := ejvw.WriteBinaryWithSubtype(nil, (byte)(bsontype.EmbeddedDocument))
			if !compareErrors(got, want) {
				t.Errorf("Did not received expected error. got %v; want %v", got, want)
			}
		})
	})

	t.Run("FormatDoubleWithExponent", func(t *testing.T) {
		want := "3E-12"
		got := formatDouble(float64(0.000000000003))
		if got != want {
			t.Errorf("Did not receive expected string. got %s: want %s", got, want)
		}
	})
}
