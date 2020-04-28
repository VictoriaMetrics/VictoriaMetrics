// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonx

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValue(t *testing.T) {
	longstr := "foobarbazqux, hello, world!"
	bytestr14 := "fourteen bytes"
	bin := primitive.Binary{Subtype: 0xFF, Data: []byte{0x01, 0x02, 0x03}}
	oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
	now := time.Now().Truncate(time.Millisecond)
	nowdt := now.Unix()*1e3 + int64(now.Nanosecond()/1e6)
	regex := primitive.Regex{Pattern: "/foobarbaz/", Options: "abr"}
	dbptr := primitive.DBPointer{DB: "foobar", Pointer: oid}
	js := "var hello ='world';"
	symbol := "foobarbaz"
	cws := primitive.CodeWithScope{Code: primitive.JavaScript(js), Scope: Doc{}}
	code, scope := js, Doc{}
	ts := primitive.Timestamp{I: 12345, T: 67890}
	d128 := primitive.NewDecimal128(12345, 67890)

	t.Parallel()
	testCases := []struct {
		name string
		fn   interface{}   // method to call
		ret  []interface{} // return value
		err  interface{}   // panic result or bool
	}{
		{"Interface/Double", Double(3.14159).Interface, []interface{}{float64(3.14159)}, nil},
		{"Interface/String", String("foo").Interface, []interface{}{string("foo")}, nil},
		{"Interface/Document", Document(Doc{}).Interface, []interface{}{Doc{}}, nil},
		{"Interface/Array", Array(Arr{}).Interface, []interface{}{Arr{}}, nil},
		{"Interface/Binary", Binary(bin.Subtype, bin.Data).Interface, []interface{}{bin}, nil},
		{"Interface/Undefined", Undefined().Interface, []interface{}{primitive.Undefined{}}, nil},
		{"Interface/Null", Null().Interface, []interface{}{primitive.Null{}}, nil},
		{"Interface/ObjectID", ObjectID(oid).Interface, []interface{}{oid}, nil},
		{"Interface/Boolean", Boolean(true).Interface, []interface{}{bool(true)}, nil},
		{"Interface/DateTime", DateTime(1234567890).Interface, []interface{}{int64(1234567890)}, nil},
		{"Interface/Time", Time(now).Interface, []interface{}{nowdt}, nil},
		{"Interface/Regex", Regex(regex.Pattern, regex.Options).Interface, []interface{}{regex}, nil},
		{"Interface/DBPointer", DBPointer(dbptr.DB, dbptr.Pointer).Interface, []interface{}{dbptr}, nil},
		{"Interface/JavaScript", JavaScript(js).Interface, []interface{}{js}, nil},
		{"Interface/Symbol", Symbol(symbol).Interface, []interface{}{symbol}, nil},
		{"Interface/CodeWithScope", CodeWithScope(string(cws.Code), cws.Scope.(Doc)).Interface, []interface{}{cws}, nil},
		{"Interface/Int32", Int32(12345).Interface, []interface{}{int32(12345)}, nil},
		{"Interface/Timestamp", Timestamp(ts.T, ts.I).Interface, []interface{}{ts}, nil},
		{"Interface/Int64", Int64(1234567890).Interface, []interface{}{int64(1234567890)}, nil},
		{"Interface/Decimal128", Decimal128(d128).Interface, []interface{}{d128}, nil},
		{"Interface/MinKey", MinKey().Interface, []interface{}{primitive.MinKey{}}, nil},
		{"Interface/MaxKey", MaxKey().Interface, []interface{}{primitive.MaxKey{}}, nil},
		{"Interface/Empty", Val{}.Interface, []interface{}{primitive.Null{}}, nil},
		{"IsNumber/Double", Double(0).IsNumber, []interface{}{bool(true)}, nil},
		{"IsNumber/Int32", Int32(0).IsNumber, []interface{}{bool(true)}, nil},
		{"IsNumber/Int64", Int64(0).IsNumber, []interface{}{bool(true)}, nil},
		{"IsNumber/Decimal128", Decimal128(primitive.Decimal128{}).IsNumber, []interface{}{bool(true)}, nil},
		{"IsNumber/String", String("").IsNumber, []interface{}{bool(false)}, nil},
		{"Double/panic", String("").Double, nil, ElementTypeError{"bson.Value.Double", bsontype.String}},
		{"Double/success", Double(3.14159).Double, []interface{}{float64(3.14159)}, nil},
		{"DoubleOK/error", String("").DoubleOK, []interface{}{float64(0), false}, nil},
		{"DoubleOK/success", Double(3.14159).DoubleOK, []interface{}{float64(3.14159), true}, nil},
		{"String/panic", Double(0).StringValue, nil, ElementTypeError{"bson.Value.StringValue", bsontype.Double}},
		{"String/success", String("bar").StringValue, []interface{}{string("bar")}, nil},
		{"String/14bytes", String(bytestr14).StringValue, []interface{}{string(bytestr14)}, nil},
		{"String/success(long)", String(longstr).StringValue, []interface{}{string(longstr)}, nil},
		{"StringOK/error", Double(0).StringValueOK, []interface{}{string(""), false}, nil},
		{"StringOK/success", String("bar").StringValueOK, []interface{}{string("bar"), true}, nil},
		{"Document/panic", Double(0).Document, nil, ElementTypeError{"bson.Value.Document", bsontype.Double}},
		{"Document/success", Document(Doc{}).Document, []interface{}{Doc{}}, nil},
		{"DocumentOK/error", Double(0).DocumentOK, []interface{}{(Doc)(nil), false}, nil},
		{"DocumentOK/success", Document(Doc{}).DocumentOK, []interface{}{Doc{}, true}, nil},
		{"MDocument/panic", Double(0).MDocument, nil, ElementTypeError{"bson.Value.MDocument", bsontype.Double}},
		{"MDocument/success", Document(MDoc{}).MDocument, []interface{}{MDoc{}}, nil},
		{"MDocumentOK/error", Double(0).MDocumentOK, []interface{}{(MDoc)(nil), false}, nil},
		{"MDocumentOK/success", Document(MDoc{}).MDocumentOK, []interface{}{MDoc{}, true}, nil},
		{"Document->MDocument/success", Document(Doc{}).MDocument, []interface{}{MDoc{}}, nil},
		{"MDocument->Document/success", Document(MDoc{}).Document, []interface{}{Doc{}}, nil},
		{"Document->MDocumentOK/success", Document(Doc{}).MDocumentOK, []interface{}{MDoc{}, true}, nil},
		{"MDocument->DocumentOK/success", Document(MDoc{}).DocumentOK, []interface{}{Doc{}, true}, nil},
		{"Array/panic", Double(0).Array, nil, ElementTypeError{"bson.Value.Array", bsontype.Double}},
		{"Array/success", Array(Arr{}).Array, []interface{}{Arr{}}, nil},
		{"ArrayOK/error", Double(0).ArrayOK, []interface{}{(Arr)(nil), false}, nil},
		{"ArrayOK/success", Array(Arr{}).ArrayOK, []interface{}{Arr{}, true}, nil},
		{"Document/NilDocument", Document((Doc)(nil)).Interface, []interface{}{primitive.Null{}}, nil},
		{"Array/NilArray", Array((Arr)(nil)).Interface, []interface{}{primitive.Null{}}, nil},
		{"Document/Nil", Document(nil).Interface, []interface{}{primitive.Null{}}, nil},
		{"Array/Nil", Array(nil).Interface, []interface{}{primitive.Null{}}, nil},
		{"Binary/panic", Double(0).Binary, nil, ElementTypeError{"bson.Value.Binary", bsontype.Double}},
		{"Binary/success", Binary(bin.Subtype, bin.Data).Binary, []interface{}{bin.Subtype, bin.Data}, nil},
		{"BinaryOK/error", Double(0).BinaryOK, []interface{}{byte(0x00), []byte(nil), false}, nil},
		{"BinaryOK/success", Binary(bin.Subtype, bin.Data).BinaryOK, []interface{}{bin.Subtype, bin.Data, true}, nil},
		{"Undefined/panic", Double(0).Undefined, nil, ElementTypeError{"bson.Value.Undefined", bsontype.Double}},
		{"Undefined/success", Undefined().Undefined, nil, nil},
		{"UndefinedOK/error", Double(0).UndefinedOK, []interface{}{false}, nil},
		{"UndefinedOK/success", Undefined().UndefinedOK, []interface{}{true}, nil},
		{"ObjectID/panic", Double(0).ObjectID, nil, ElementTypeError{"bson.Value.ObjectID", bsontype.Double}},
		{"ObjectID/success", ObjectID(oid).ObjectID, []interface{}{oid}, nil},
		{"ObjectIDOK/error", Double(0).ObjectIDOK, []interface{}{primitive.ObjectID{}, false}, nil},
		{"ObjectIDOK/success", ObjectID(oid).ObjectIDOK, []interface{}{oid, true}, nil},
		{"Boolean/panic", Double(0).Boolean, nil, ElementTypeError{"bson.Value.Boolean", bsontype.Double}},
		{"Boolean/success", Boolean(true).Boolean, []interface{}{bool(true)}, nil},
		{"BooleanOK/error", Double(0).BooleanOK, []interface{}{bool(false), false}, nil},
		{"BooleanOK/success", Boolean(false).BooleanOK, []interface{}{false, true}, nil},
		{"DateTime/panic", Double(0).DateTime, nil, ElementTypeError{"bson.Value.DateTime", bsontype.Double}},
		{"DateTime/success", DateTime(1234567890).DateTime, []interface{}{int64(1234567890)}, nil},
		{"DateTimeOK/error", Double(0).DateTimeOK, []interface{}{int64(0), false}, nil},
		{"DateTimeOK/success", DateTime(987654321).DateTimeOK, []interface{}{int64(987654321), true}, nil},
		{"Time/panic", Double(0).Time, nil, ElementTypeError{"bson.Value.Time", bsontype.Double}},
		{"Time/success", Time(now).Time, []interface{}{now}, nil},
		{"TimeOK/error", Double(0).TimeOK, []interface{}{time.Time{}, false}, nil},
		{"TimeOK/success", Time(now).TimeOK, []interface{}{now, true}, nil},
		{"Time->DateTime", Time(now).DateTime, []interface{}{nowdt}, nil},
		{"DateTime->Time", DateTime(nowdt).Time, []interface{}{now}, nil},
		{"Null/panic", Double(0).Null, nil, ElementTypeError{"bson.Value.Null", bsontype.Double}},
		{"Null/success", Null().Null, nil, nil},
		{"NullOK/error", Double(0).NullOK, []interface{}{false}, nil},
		{"NullOK/success", Null().NullOK, []interface{}{true}, nil},
		{"Regex/panic", Double(0).Regex, nil, ElementTypeError{"bson.Value.Regex", bsontype.Double}},
		{"Regex/success", Regex(regex.Pattern, regex.Options).Regex, []interface{}{regex.Pattern, regex.Options}, nil},
		{"RegexOK/error", Double(0).RegexOK, []interface{}{"", "", false}, nil},
		{"RegexOK/success", Regex(regex.Pattern, regex.Options).RegexOK, []interface{}{regex.Pattern, regex.Options, true}, nil},
		{"DBPointer/panic", Double(0).DBPointer, nil, ElementTypeError{"bson.Value.DBPointer", bsontype.Double}},
		{"DBPointer/success", DBPointer(dbptr.DB, dbptr.Pointer).DBPointer, []interface{}{dbptr.DB, dbptr.Pointer}, nil},
		{"DBPointerOK/error", Double(0).DBPointerOK, []interface{}{"", primitive.ObjectID{}, false}, nil},
		{"DBPointerOK/success", DBPointer(dbptr.DB, dbptr.Pointer).DBPointerOK, []interface{}{dbptr.DB, dbptr.Pointer, true}, nil},
		{"JavaScript/panic", Double(0).JavaScript, nil, ElementTypeError{"bson.Value.JavaScript", bsontype.Double}},
		{"JavaScript/success", JavaScript(js).JavaScript, []interface{}{js}, nil},
		{"JavaScriptOK/error", Double(0).JavaScriptOK, []interface{}{string(""), false}, nil},
		{"JavaScriptOK/success", JavaScript(js).JavaScriptOK, []interface{}{js, true}, nil},
		{"Symbol/panic", Double(0).Symbol, nil, ElementTypeError{"bson.Value.Symbol", bsontype.Double}},
		{"Symbol/success", Symbol(symbol).Symbol, []interface{}{symbol}, nil},
		{"SymbolOK/error", Double(0).SymbolOK, []interface{}{string(""), false}, nil},
		{"SymbolOK/success", Symbol(symbol).SymbolOK, []interface{}{symbol, true}, nil},
		{"CodeWithScope/panic", Double(0).CodeWithScope, nil, ElementTypeError{"bson.Value.CodeWithScope", bsontype.Double}},
		{"CodeWithScope/success", CodeWithScope(code, scope).CodeWithScope, []interface{}{code, scope}, nil},
		{"CodeWithScopeOK/error", Double(0).CodeWithScopeOK, []interface{}{"", (Doc)(nil), false}, nil},
		{"CodeWithScopeOK/success", CodeWithScope(code, scope).CodeWithScopeOK, []interface{}{code, scope, true}, nil},
		{"Int32/panic", Double(0).Int32, nil, ElementTypeError{"bson.Value.Int32", bsontype.Double}},
		{"Int32/success", Int32(12345).Int32, []interface{}{int32(12345)}, nil},
		{"Int32OK/error", Double(0).Int32OK, []interface{}{int32(0), false}, nil},
		{"Int32OK/success", Int32(54321).Int32OK, []interface{}{int32(54321), true}, nil},
		{"Timestamp/panic", Double(0).Timestamp, nil, ElementTypeError{"bson.Value.Timestamp", bsontype.Double}},
		{"Timestamp/success", Timestamp(ts.T, ts.I).Timestamp, []interface{}{ts.T, ts.I}, nil},
		{"TimestampOK/error", Double(0).TimestampOK, []interface{}{uint32(0), uint32(0), false}, nil},
		{"TimestampOK/success", Timestamp(ts.T, ts.I).TimestampOK, []interface{}{ts.T, ts.I, true}, nil},
		{"Int64/panic", Double(0).Int64, nil, ElementTypeError{"bson.Value.Int64", bsontype.Double}},
		{"Int64/success", Int64(1234567890).Int64, []interface{}{int64(1234567890)}, nil},
		{"Int64OK/error", Double(0).Int64OK, []interface{}{int64(0), false}, nil},
		{"Int64OK/success", Int64(9876543210).Int64OK, []interface{}{int64(9876543210), true}, nil},
		{"Decimal128/panic", Double(0).Decimal128, nil, ElementTypeError{"bson.Value.Decimal128", bsontype.Double}},
		{"Decimal128/success", Decimal128(d128).Decimal128, []interface{}{d128}, nil},
		{"Decimal128OK/error", Double(0).Decimal128OK, []interface{}{primitive.Decimal128{}, false}, nil},
		{"Decimal128OK/success", Decimal128(d128).Decimal128OK, []interface{}{d128, true}, nil},
		{"MinKey/panic", Double(0).MinKey, nil, ElementTypeError{"bson.Value.MinKey", bsontype.Double}},
		{"MinKey/success", MinKey().MinKey, nil, nil},
		{"MinKeyOK/error", Double(0).MinKeyOK, []interface{}{false}, nil},
		{"MinKeyOK/success", MinKey().MinKeyOK, []interface{}{true}, nil},
		{"MaxKey/panic", Double(0).MaxKey, nil, ElementTypeError{"bson.Value.MaxKey", bsontype.Double}},
		{"MaxKey/success", MaxKey().MaxKey, nil, nil},
		{"MaxKeyOK/error", Double(0).MaxKeyOK, []interface{}{false}, nil},
		{"MaxKeyOK/success", MaxKey().MaxKeyOK, []interface{}{true}, nil},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				err := recover()
				if err != nil && !cmp.Equal(err, tc.err) {
					t.Errorf("panic errors are not equal. got %v; want %v", err, tc.err)
					if tc.err == nil {
						panic(err)
					}
				}
			}()
			fn := reflect.ValueOf(tc.fn)
			if fn.Kind() != reflect.Func {
				t.Fatalf("fn must be a function, but is a %s", fn.Kind())
			}
			ret := fn.Call(nil)
			if len(ret) != len(tc.ret) {
				t.Fatalf("number of returned values does not match. got %d; want %d", len(ret), len(tc.ret))
			}

			for idx := range ret {
				got, want := ret[idx].Interface(), tc.ret[idx]
				if !cmp.Equal(got, want, cmp.Comparer(compareDecimal128)) {
					t.Errorf("Return %d does not match. got %v; want %v", idx, got, want)
				}
			}
		})
	}

	t.Run("Equal", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name string
			v1   Val
			v2   Val
			res  bool
		}{
			{"Different Types", String(""), Double(0), false},
			{"Unknown Types", Val{t: bsontype.Type(0x77)}, Val{t: bsontype.Type(0x77)}, false},
			{"Empty Types", Val{}, Val{}, true},
			{"Double/Equal", Double(3.14159), Double(3.14159), true},
			{"Double/Not Equal", Double(3.14159), Double(9.51413), false},
			{"DateTime/Equal", DateTime(nowdt), DateTime(nowdt), true},
			{"DateTime/Not Equal", DateTime(nowdt), DateTime(0), false},
			{"String/Equal", String("hello"), String("hello"), true},
			{"String/Not Equal", String("hello"), String("world"), false},
			{"Document/Equal", Document(Doc{}), Document(Doc{}), true},
			{"Document/Not Equal", Document(Doc{}), Document(Doc{{"", Null()}}), false},
			{"Array/Equal", Array(Arr{}), Array(Arr{}), true},
			{"Array/Not Equal", Array(Arr{}), Array(Arr{Null()}), false},
			{"Binary/Equal", Binary(bin.Subtype, bin.Data), Binary(bin.Subtype, bin.Data), true},
			{"Binary/Not Equal", Binary(bin.Subtype, bin.Data), Binary(0x00, nil), false},
			{"Undefined/Equal", Undefined(), Undefined(), true},
			{"ObjectID/Equal", ObjectID(oid), ObjectID(oid), true},
			{"ObjectID/Not Equal", ObjectID(oid), ObjectID(primitive.ObjectID{}), false},
			{"Boolean/Equal", Boolean(true), Boolean(true), true},
			{"Boolean/Not Equal", Boolean(true), Boolean(false), false},
			{"Null/Equal", Null(), Null(), true},
			{"Regex/Equal", Regex(regex.Pattern, regex.Options), Regex(regex.Pattern, regex.Options), true},
			{"Regex/Not Equal", Regex(regex.Pattern, regex.Options), Regex("", ""), false},
			{"DBPointer/Equal", DBPointer(dbptr.DB, dbptr.Pointer), DBPointer(dbptr.DB, dbptr.Pointer), true},
			{"DBPointer/Not Equal", DBPointer(dbptr.DB, dbptr.Pointer), DBPointer("", primitive.ObjectID{}), false},
			{"JavaScript/Equal", JavaScript(js), JavaScript(js), true},
			{"JavaScript/Not Equal", JavaScript(js), JavaScript(""), false},
			{"Symbol/Equal", Symbol(symbol), Symbol(symbol), true},
			{"Symbol/Not Equal", Symbol(symbol), Symbol(""), false},
			{"CodeWithScope/Equal", CodeWithScope(code, scope), CodeWithScope(code, scope), true},
			{"CodeWithScope/Equal (equal scope)", CodeWithScope(code, scope), CodeWithScope(code, Doc{}), true},
			{"CodeWithScope/Not Equal", CodeWithScope(code, scope), CodeWithScope("", nil), false},
			{"Int32/Equal", Int32(12345), Int32(12345), true},
			{"Int32/Not Equal", Int32(12345), Int32(54321), false},
			{"Timestamp/Equal", Timestamp(ts.T, ts.I), Timestamp(ts.T, ts.I), true},
			{"Timestamp/Not Equal", Timestamp(ts.T, ts.I), Timestamp(0, 0), false},
			{"Int64/Equal", Int64(1234567890), Int64(1234567890), true},
			{"Int64/Not Equal", Int64(1234567890), Int64(9876543210), false},
			{"Decimal128/Equal", Decimal128(d128), Decimal128(d128), true},
			{"Decimal128/Not Equal", Decimal128(d128), Decimal128(primitive.Decimal128{}), false},
			{"MinKey/Equal", MinKey(), MinKey(), true},
			{"MaxKey/Equal", MaxKey(), MaxKey(), true},
		}

		for _, tc := range testCases {
			tc := tc // capture range variable
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				res := tc.v1.Equal(tc.v2)
				if res != tc.res {
					t.Errorf("results do not match. got %v; want %v", res, tc.res)
				}
			})
		}
	})
}
