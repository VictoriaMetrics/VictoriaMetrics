// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncore

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValue(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		t.Run("invalid", func(t *testing.T) {
			v := Value{Type: bsontype.Double, Data: []byte{0x01, 0x02, 0x03, 0x04}}
			want := NewInsufficientBytesError(v.Data, v.Data)
			got := v.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("value", func(t *testing.T) {
			v := Value{Type: bsontype.Double, Data: AppendDouble(nil, 3.14159)}
			var want error
			got := v.Validate()
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
	})
	t.Run("IsNumber", func(t *testing.T) {
		testCases := []struct {
			name  string
			val   Value
			isnum bool
		}{
			{"double", Value{Type: bsontype.Double}, true},
			{"int32", Value{Type: bsontype.Int32}, true},
			{"int64", Value{Type: bsontype.Int64}, true},
			{"decimal128", Value{Type: bsontype.Decimal128}, true},
			{"string", Value{Type: bsontype.String}, false},
			{"regex", Value{Type: bsontype.Regex}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				isnum := tc.val.IsNumber()
				if isnum != tc.isnum {
					t.Errorf("IsNumber did not return the expected boolean. got %t; want %t", isnum, tc.isnum)
				}
			})
		}
	})

	now := time.Now().Truncate(time.Millisecond)
	oid := primitive.NewObjectID()

	testCases := []struct {
		name     string
		fn       interface{}
		val      Value
		panicErr error
		ret      []interface{}
	}{
		{
			"Double/Not Double", Value.Double, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Double", bsontype.String},
			nil,
		},
		{
			"Double/Insufficient Bytes", Value.Double, Value{Type: bsontype.Double, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}),
			nil,
		},
		{
			"Double/Success", Value.Double, Value{Type: bsontype.Double, Data: AppendDouble(nil, 3.14159)},
			nil,
			[]interface{}{float64(3.14159)},
		},
		{
			"DoubleOK/Not Double", Value.DoubleOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{float64(0), false},
		},
		{
			"DoubleOK/Insufficient Bytes", Value.DoubleOK, Value{Type: bsontype.Double, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			nil,
			[]interface{}{float64(0), false},
		},
		{
			"DoubleOK/Success", Value.DoubleOK, Value{Type: bsontype.Double, Data: AppendDouble(nil, 3.14159)},
			nil,
			[]interface{}{float64(3.14159), true},
		},
		{
			"StringValue/Not String", Value.StringValue, Value{Type: bsontype.Double},
			ElementTypeError{"bsoncore.Value.StringValue", bsontype.Double},
			nil,
		},
		{
			"StringValue/Insufficient Bytes", Value.StringValue, Value{Type: bsontype.String, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}),
			nil,
		},
		{
			"StringValue/Success", Value.StringValue, Value{Type: bsontype.String, Data: AppendString(nil, "hello, world!")},
			nil,
			[]interface{}{string("hello, world!")},
		},
		{
			"StringValueOK/Not String", Value.StringValueOK, Value{Type: bsontype.Double},
			nil,
			[]interface{}{string(""), false},
		},
		{
			"StringValueOK/Insufficient Bytes", Value.StringValueOK, Value{Type: bsontype.String, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			nil,
			[]interface{}{string(""), false},
		},
		{
			"StringValueOK/Success", Value.StringValueOK, Value{Type: bsontype.String, Data: AppendString(nil, "hello, world!")},
			nil,
			[]interface{}{string("hello, world!"), true},
		},
		{
			"Document/Not Document", Value.Document, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Document", bsontype.String},
			nil,
		},
		{
			"Document/Insufficient Bytes", Value.Document, Value{Type: bsontype.EmbeddedDocument, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}),
			nil,
		},
		{
			"Document/Success", Value.Document, Value{Type: bsontype.EmbeddedDocument, Data: []byte{0x05, 0x00, 0x00, 0x00, 0x00}},
			nil,
			[]interface{}{Document{0x05, 0x00, 0x00, 0x00, 0x00}},
		},
		{
			"DocumentOK/Not Document", Value.DocumentOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{Document(nil), false},
		},
		{
			"DocumentOK/Insufficient Bytes", Value.DocumentOK, Value{Type: bsontype.EmbeddedDocument, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			nil,
			[]interface{}{Document(nil), false},
		},
		{
			"DocumentOK/Success", Value.DocumentOK, Value{Type: bsontype.EmbeddedDocument, Data: []byte{0x05, 0x00, 0x00, 0x00, 0x00}},
			nil,
			[]interface{}{Document{0x05, 0x00, 0x00, 0x00, 0x00}, true},
		},
		{
			"Array/Not Array", Value.Array, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Array", bsontype.String},
			nil,
		},
		{
			"Array/Insufficient Bytes", Value.Array, Value{Type: bsontype.Array, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}),
			nil,
		},
		{
			"Array/Success", Value.Array, Value{Type: bsontype.Array, Data: []byte{0x05, 0x00, 0x00, 0x00, 0x00}},
			nil,
			[]interface{}{Document{0x05, 0x00, 0x00, 0x00, 0x00}},
		},
		{
			"ArrayOK/Not Array", Value.ArrayOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{Document(nil), false},
		},
		{
			"ArrayOK/Insufficient Bytes", Value.ArrayOK, Value{Type: bsontype.Array, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			nil,
			[]interface{}{Document(nil), false},
		},
		{
			"ArrayOK/Success", Value.ArrayOK, Value{Type: bsontype.Array, Data: []byte{0x05, 0x00, 0x00, 0x00, 0x00}},
			nil,
			[]interface{}{Document{0x05, 0x00, 0x00, 0x00, 0x00}, true},
		},
		{
			"Binary/Not Binary", Value.Binary, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Binary", bsontype.String},
			nil,
		},
		{
			"Binary/Insufficient Bytes", Value.Binary, Value{Type: bsontype.Binary, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}),
			nil,
		},
		{
			"Binary/Success", Value.Binary, Value{Type: bsontype.Binary, Data: AppendBinary(nil, 0xFF, []byte{0x01, 0x02, 0x03})},
			nil,
			[]interface{}{byte(0xFF), []byte{0x01, 0x02, 0x03}},
		},
		{
			"BinaryOK/Not Binary", Value.BinaryOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{byte(0x00), []byte(nil), false},
		},
		{
			"BinaryOK/Insufficient Bytes", Value.BinaryOK, Value{Type: bsontype.Binary, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			nil,
			[]interface{}{byte(0x00), []byte(nil), false},
		},
		{
			"BinaryOK/Success", Value.BinaryOK, Value{Type: bsontype.Binary, Data: AppendBinary(nil, 0xFF, []byte{0x01, 0x02, 0x03})},
			nil,
			[]interface{}{byte(0xFF), []byte{0x01, 0x02, 0x03}, true},
		},
		{
			"ObjectID/Not ObjectID", Value.ObjectID, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.ObjectID", bsontype.String},
			nil,
		},
		{
			"ObjectID/Insufficient Bytes", Value.ObjectID, Value{Type: bsontype.ObjectID, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}),
			nil,
		},
		{
			"ObjectID/Success", Value.ObjectID, Value{Type: bsontype.ObjectID, Data: AppendObjectID(nil, primitive.ObjectID{0x01, 0x02})},
			nil,
			[]interface{}{primitive.ObjectID{0x01, 0x02}},
		},
		{
			"ObjectIDOK/Not ObjectID", Value.ObjectIDOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{primitive.ObjectID{}, false},
		},
		{
			"ObjectIDOK/Insufficient Bytes", Value.ObjectIDOK, Value{Type: bsontype.ObjectID, Data: []byte{0x01, 0x02, 0x03, 0x04}},
			nil,
			[]interface{}{primitive.ObjectID{}, false},
		},
		{
			"ObjectIDOK/Success", Value.ObjectIDOK, Value{Type: bsontype.ObjectID, Data: AppendObjectID(nil, primitive.ObjectID{0x01, 0x02})},
			nil,
			[]interface{}{primitive.ObjectID{0x01, 0x02}, true},
		},
		{
			"Boolean/Not Boolean", Value.Boolean, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Boolean", bsontype.String},
			nil,
		},
		{
			"Boolean/Insufficient Bytes", Value.Boolean, Value{Type: bsontype.Boolean, Data: []byte{}},
			NewInsufficientBytesError([]byte{}, []byte{}),
			nil,
		},
		{
			"Boolean/Success", Value.Boolean, Value{Type: bsontype.Boolean, Data: AppendBoolean(nil, true)},
			nil,
			[]interface{}{true},
		},
		{
			"BooleanOK/Not Boolean", Value.BooleanOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{false, false},
		},
		{
			"BooleanOK/Insufficient Bytes", Value.BooleanOK, Value{Type: bsontype.Boolean, Data: []byte{}},
			nil,
			[]interface{}{false, false},
		},
		{
			"BooleanOK/Success", Value.BooleanOK, Value{Type: bsontype.Boolean, Data: AppendBoolean(nil, true)},
			nil,
			[]interface{}{true, true},
		},
		{
			"DateTime/Not DateTime", Value.DateTime, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.DateTime", bsontype.String},
			nil,
		},
		{
			"DateTime/Insufficient Bytes", Value.DateTime, Value{Type: bsontype.DateTime, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{}, []byte{}),
			nil,
		},
		{
			"DateTime/Success", Value.DateTime, Value{Type: bsontype.DateTime, Data: AppendDateTime(nil, 12345)},
			nil,
			[]interface{}{int64(12345)},
		},
		{
			"DateTimeOK/Not DateTime", Value.DateTimeOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{int64(0), false},
		},
		{
			"DateTimeOK/Insufficient Bytes", Value.DateTimeOK, Value{Type: bsontype.DateTime, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{int64(0), false},
		},
		{
			"DateTimeOK/Success", Value.DateTimeOK, Value{Type: bsontype.DateTime, Data: AppendDateTime(nil, 12345)},
			nil,
			[]interface{}{int64(12345), true},
		},
		{
			"Time/Not DateTime", Value.Time, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Time", bsontype.String},
			nil,
		},
		{
			"Time/Insufficient Bytes", Value.Time, Value{Type: bsontype.DateTime, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Time/Success", Value.Time, Value{Type: bsontype.DateTime, Data: AppendTime(nil, now)},
			nil,
			[]interface{}{time.Time(now)},
		},
		{
			"TimeOK/Not DateTime", Value.TimeOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{time.Time{}, false},
		},
		{
			"TimeOK/Insufficient Bytes", Value.TimeOK, Value{Type: bsontype.DateTime, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{time.Time{}, false},
		},
		{
			"TimeOK/Success", Value.TimeOK, Value{Type: bsontype.DateTime, Data: AppendTime(nil, now)},
			nil,
			[]interface{}{time.Time(now), true},
		},
		{
			"Regex/Not Regex", Value.Regex, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Regex", bsontype.String},
			nil,
		},
		{
			"Regex/Insufficient Bytes", Value.Regex, Value{Type: bsontype.Regex, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Regex/Success", Value.Regex, Value{Type: bsontype.Regex, Data: AppendRegex(nil, "/abcdefg/", "hijkl")},
			nil,
			[]interface{}{string("/abcdefg/"), string("hijkl")},
		},
		{
			"RegexOK/Not Regex", Value.RegexOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{string(""), string(""), false},
		},
		{
			"RegexOK/Insufficient Bytes", Value.RegexOK, Value{Type: bsontype.Regex, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{string(""), string(""), false},
		},
		{
			"RegexOK/Success", Value.RegexOK, Value{Type: bsontype.Regex, Data: AppendRegex(nil, "/abcdefg/", "hijkl")},
			nil,
			[]interface{}{string("/abcdefg/"), string("hijkl"), true},
		},
		{
			"DBPointer/Not DBPointer", Value.DBPointer, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.DBPointer", bsontype.String},
			nil,
		},
		{
			"DBPointer/Insufficient Bytes", Value.DBPointer, Value{Type: bsontype.DBPointer, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"DBPointer/Success", Value.DBPointer, Value{Type: bsontype.DBPointer, Data: AppendDBPointer(nil, "foobar", oid)},
			nil,
			[]interface{}{string("foobar"), primitive.ObjectID(oid)},
		},
		{
			"DBPointerOK/Not DBPointer", Value.DBPointerOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{string(""), primitive.ObjectID{}, false},
		},
		{
			"DBPointerOK/Insufficient Bytes", Value.DBPointerOK, Value{Type: bsontype.DBPointer, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{string(""), primitive.ObjectID{}, false},
		},
		{
			"DBPointerOK/Success", Value.DBPointerOK, Value{Type: bsontype.DBPointer, Data: AppendDBPointer(nil, "foobar", oid)},
			nil,
			[]interface{}{string("foobar"), primitive.ObjectID(oid), true},
		},
		{
			"JavaScript/Not JavaScript", Value.JavaScript, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.JavaScript", bsontype.String},
			nil,
		},
		{
			"JavaScript/Insufficient Bytes", Value.JavaScript, Value{Type: bsontype.JavaScript, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"JavaScript/Success", Value.JavaScript, Value{Type: bsontype.JavaScript, Data: AppendJavaScript(nil, "var hello = 'world';")},
			nil,
			[]interface{}{string("var hello = 'world';")},
		},
		{
			"JavaScriptOK/Not JavaScript", Value.JavaScriptOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{string(""), false},
		},
		{
			"JavaScriptOK/Insufficient Bytes", Value.JavaScriptOK, Value{Type: bsontype.JavaScript, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{string(""), false},
		},
		{
			"JavaScriptOK/Success", Value.JavaScriptOK, Value{Type: bsontype.JavaScript, Data: AppendJavaScript(nil, "var hello = 'world';")},
			nil,
			[]interface{}{string("var hello = 'world';"), true},
		},
		{
			"Symbol/Not Symbol", Value.Symbol, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Symbol", bsontype.String},
			nil,
		},
		{
			"Symbol/Insufficient Bytes", Value.Symbol, Value{Type: bsontype.Symbol, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Symbol/Success", Value.Symbol, Value{Type: bsontype.Symbol, Data: AppendSymbol(nil, "symbol123456")},
			nil,
			[]interface{}{string("symbol123456")},
		},
		{
			"SymbolOK/Not Symbol", Value.SymbolOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{string(""), false},
		},
		{
			"SymbolOK/Insufficient Bytes", Value.SymbolOK, Value{Type: bsontype.Symbol, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{string(""), false},
		},
		{
			"SymbolOK/Success", Value.SymbolOK, Value{Type: bsontype.Symbol, Data: AppendSymbol(nil, "symbol123456")},
			nil,
			[]interface{}{string("symbol123456"), true},
		},
		{
			"CodeWithScope/Not CodeWithScope", Value.CodeWithScope, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.CodeWithScope", bsontype.String},
			nil,
		},
		{
			"CodeWithScope/Insufficient Bytes", Value.CodeWithScope, Value{Type: bsontype.CodeWithScope, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"CodeWithScope/Success", Value.CodeWithScope, Value{Type: bsontype.CodeWithScope, Data: AppendCodeWithScope(nil, "var hello = 'world';", Document{0x05, 0x00, 0x00, 0x00, 0x00})},
			nil,
			[]interface{}{string("var hello = 'world';"), Document{0x05, 0x00, 0x00, 0x00, 0x00}},
		},
		{
			"CodeWithScopeOK/Not CodeWithScope", Value.CodeWithScopeOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{string(""), Document(nil), false},
		},
		{
			"CodeWithScopeOK/Insufficient Bytes", Value.CodeWithScopeOK, Value{Type: bsontype.CodeWithScope, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{string(""), Document(nil), false},
		},
		{
			"CodeWithScopeOK/Success", Value.CodeWithScopeOK, Value{Type: bsontype.CodeWithScope, Data: AppendCodeWithScope(nil, "var hello = 'world';", Document{0x05, 0x00, 0x00, 0x00, 0x00})},
			nil,
			[]interface{}{string("var hello = 'world';"), Document{0x05, 0x00, 0x00, 0x00, 0x00}, true},
		},
		{
			"Int32/Not Int32", Value.Int32, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Int32", bsontype.String},
			nil,
		},
		{
			"Int32/Insufficient Bytes", Value.Int32, Value{Type: bsontype.Int32, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Int32/Success", Value.Int32, Value{Type: bsontype.Int32, Data: AppendInt32(nil, 1234)},
			nil,
			[]interface{}{int32(1234)},
		},
		{
			"Int32OK/Not Int32", Value.Int32OK, Value{Type: bsontype.String},
			nil,
			[]interface{}{int32(0), false},
		},
		{
			"Int32OK/Insufficient Bytes", Value.Int32OK, Value{Type: bsontype.Int32, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{int32(0), false},
		},
		{
			"Int32OK/Success", Value.Int32OK, Value{Type: bsontype.Int32, Data: AppendInt32(nil, 1234)},
			nil,
			[]interface{}{int32(1234), true},
		},
		{
			"Timestamp/Not Timestamp", Value.Timestamp, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Timestamp", bsontype.String},
			nil,
		},
		{
			"Timestamp/Insufficient Bytes", Value.Timestamp, Value{Type: bsontype.Timestamp, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Timestamp/Success", Value.Timestamp, Value{Type: bsontype.Timestamp, Data: AppendTimestamp(nil, 12345, 67890)},
			nil,
			[]interface{}{uint32(12345), uint32(67890)},
		},
		{
			"TimestampOK/Not Timestamp", Value.TimestampOK, Value{Type: bsontype.String},
			nil,
			[]interface{}{uint32(0), uint32(0), false},
		},
		{
			"TimestampOK/Insufficient Bytes", Value.TimestampOK, Value{Type: bsontype.Timestamp, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{uint32(0), uint32(0), false},
		},
		{
			"TimestampOK/Success", Value.TimestampOK, Value{Type: bsontype.Timestamp, Data: AppendTimestamp(nil, 12345, 67890)},
			nil,
			[]interface{}{uint32(12345), uint32(67890), true},
		},
		{
			"Int64/Not Int64", Value.Int64, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Int64", bsontype.String},
			nil,
		},
		{
			"Int64/Insufficient Bytes", Value.Int64, Value{Type: bsontype.Int64, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Int64/Success", Value.Int64, Value{Type: bsontype.Int64, Data: AppendInt64(nil, 1234567890)},
			nil,
			[]interface{}{int64(1234567890)},
		},
		{
			"Int64OK/Not Int64", Value.Int64OK, Value{Type: bsontype.String},
			nil,
			[]interface{}{int64(0), false},
		},
		{
			"Int64OK/Insufficient Bytes", Value.Int64OK, Value{Type: bsontype.Int64, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{int64(0), false},
		},
		{
			"Int64OK/Success", Value.Int64OK, Value{Type: bsontype.Int64, Data: AppendInt64(nil, 1234567890)},
			nil,
			[]interface{}{int64(1234567890), true},
		},
		{
			"Decimal128/Not Decimal128", Value.Decimal128, Value{Type: bsontype.String},
			ElementTypeError{"bsoncore.Value.Decimal128", bsontype.String},
			nil,
		},
		{
			"Decimal128/Insufficient Bytes", Value.Decimal128, Value{Type: bsontype.Decimal128, Data: []byte{0x01, 0x02, 0x03}},
			NewInsufficientBytesError([]byte{0x01, 0x02, 0x03}, []byte{0x01, 0x02, 0x03}),
			nil,
		},
		{
			"Decimal128/Success", Value.Decimal128, Value{Type: bsontype.Decimal128, Data: AppendDecimal128(nil, primitive.NewDecimal128(12345, 67890))},
			nil,
			[]interface{}{primitive.NewDecimal128(12345, 67890)},
		},
		{
			"Decimal128OK/Not Decimal128", Value.Decimal128OK, Value{Type: bsontype.String},
			nil,
			[]interface{}{primitive.Decimal128{}, false},
		},
		{
			"Decimal128OK/Insufficient Bytes", Value.Decimal128OK, Value{Type: bsontype.Decimal128, Data: []byte{0x01, 0x02, 0x03}},
			nil,
			[]interface{}{primitive.Decimal128{}, false},
		},
		{
			"Decimal128OK/Success", Value.Decimal128OK, Value{Type: bsontype.Decimal128, Data: AppendDecimal128(nil, primitive.NewDecimal128(12345, 67890))},
			nil,
			[]interface{}{primitive.NewDecimal128(12345, 67890), true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				err := recover()
				if !cmp.Equal(err, tc.panicErr, cmp.Comparer(compareErrors)) {
					t.Errorf("Did not receive expected panic error. got %v; want %v", err, tc.panicErr)
				}
			}()

			fn := reflect.ValueOf(tc.fn)
			if fn.Kind() != reflect.Func || fn.Type().NumIn() != 1 || fn.Type().In(0) != reflect.TypeOf(Value{}) {
				t.Fatalf("test case field fn must be a function with 1 parameter that is a Value, but it is %v", fn.Type())
			}
			got := fn.Call([]reflect.Value{reflect.ValueOf(tc.val)})
			want := make([]reflect.Value, 0, len(tc.ret))
			for _, ret := range tc.ret {
				want = append(want, reflect.ValueOf(ret))
			}
			if len(got) != len(want) {
				t.Fatalf("incorrect number of values returned. got %d; want %d", len(got), len(want))
			}

			for idx := range got {
				gotv, wantv := got[idx].Interface(), want[idx].Interface()
				if !cmp.Equal(gotv, wantv, cmp.Comparer(compareDecimal128)) {
					t.Errorf("return values at index %d are not equal. got %v; want %v", idx, gotv, wantv)
				}
			}
		})
	}
}
