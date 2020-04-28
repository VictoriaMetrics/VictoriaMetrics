// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonx

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestMDoc(t *testing.T) {
	t.Parallel()
	t.Run("ReadMDoc", func(t *testing.T) {
		t.Parallel()
		t.Run("UnmarshalingError", func(t *testing.T) {
			t.Parallel()
			invalid := []byte{0x01, 0x02}
			want := bsoncore.NewInsufficientBytesError(nil, nil)
			_, got := ReadMDoc(invalid)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("success", func(t *testing.T) {
			t.Parallel()
			valid := bsoncore.BuildDocument(nil, bsoncore.AppendNullElement(nil, "foobar"))
			var want error
			wantDoc := MDoc{"foobar": Null()}
			gotDoc, got := ReadMDoc(valid)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
			if !cmp.Equal(gotDoc, wantDoc) {
				t.Errorf("Expected returned documents to match. got %v; want %v", gotDoc, wantDoc)
			}
		})
	})
	t.Run("Copy", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name  string
			start MDoc
			copy  MDoc
		}{
			{"nil", nil, MDoc{}},
			{"not-nil", MDoc{"foobar": Null()}, MDoc{"foobar": Null()}},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				copy := tc.start.Copy()
				if !cmp.Equal(copy, tc.copy) {
					t.Errorf("Expected copies to be equal. got %v; want %v", copy, tc.copy)
				}
			})
		}
	})
	testCases := []struct {
		name   string
		fn     interface{}   // method to call
		params []interface{} // parameters
		rets   []interface{} // returns
	}{
		{
			"Lookup/err", MDoc{}.Lookup,
			[]interface{}{[]string{}},
			[]interface{}{Val{}},
		},
		{
			"Lookup/success", MDoc{"pi": Double(3.14159)}.Lookup,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Double(3.14159)},
		},
		{
			"LookupErr/err", MDoc{}.LookupErr,
			[]interface{}{[]string{}},
			[]interface{}{Val{}, KeyNotFound{Key: []string{}}},
		},
		{
			"LookupErr/success", MDoc{"pi": Double(3.14159)}.LookupErr,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Double(3.14159), error(nil)},
		},
		{
			"LookupElem/err", MDoc{}.LookupElement,
			[]interface{}{[]string{}},
			[]interface{}{Elem{}},
		},
		{
			"LookupElem/success", MDoc{"pi": Double(3.14159)}.LookupElement,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}},
		},
		{
			"LookupElementErr/zero length key", MDoc{}.LookupElementErr,
			[]interface{}{[]string{}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{}}},
		},
		{
			"LookupElementErr/key not found", MDoc{}.LookupElementErr,
			[]interface{}{[]string{"foo"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo"}}},
		},
		{
			"LookupElementErr/key not found/depth 2", MDoc{"foo": Document(Doc{})}.LookupElementErr,
			[]interface{}{[]string{"foo", "bar"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo", "bar"}, Depth: 1}},
		},
		{
			"LookupElementErr/invalid depth 2 type", MDoc{"foo": Document(MDoc{"pi": Double(3.14159)})}.LookupElementErr,
			[]interface{}{[]string{"foo", "pi", "baz"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo", "pi", "baz"}, Depth: 1, Type: bsontype.Double}},
		},
		{
			"LookupElementErr/invalid depth 2 type (Doc)", MDoc{"foo": Document(Doc{{"pi", Double(3.14159)}})}.LookupElementErr,
			[]interface{}{[]string{"foo", "pi", "baz"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo", "pi", "baz"}, Depth: 1, Type: bsontype.Double}},
		},
		{
			"LookupElementErr/success", MDoc{"pi": Double(3.14159)}.LookupElementErr,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}, error(nil)},
		},
		{
			"LookupElementErr/success/depth 2 (Doc)", MDoc{"foo": Document(Doc{{"pi", Double(3.14159)}})}.LookupElementErr,
			[]interface{}{[]string{"foo", "pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}, error(nil)},
		},
		{
			"LookupElementErr/success/depth 2", MDoc{"foo": Document(MDoc{"pi": Double(3.14159)})}.LookupElementErr,
			[]interface{}{[]string{"foo", "pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}, error(nil)},
		},
		{
			"MarshalBSONValue/nil", MDoc(nil).MarshalBSONValue,
			nil,
			[]interface{}{bsontype.Null, []byte(nil), error(nil)},
		},
		{
			"MarshalBSONValue/success", MDoc{"pi": Double(3.14159)}.MarshalBSONValue, nil,
			[]interface{}{
				bsontype.EmbeddedDocument,
				bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
				error(nil),
			},
		},
		{
			"MarshalBSON", MDoc{"pi": Double(3.14159)}.MarshalBSON, nil,
			[]interface{}{bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)), error(nil)},
		},
		{
			"MarshalBSON/empty", MDoc{}.MarshalBSON, nil,
			[]interface{}{bsoncore.BuildDocument(nil, nil), error(nil)},
		},
		{
			"AppendMarshalBSON", MDoc{"pi": Double(3.14159)}.AppendMarshalBSON, []interface{}{[]byte{0x01, 0x02, 0x03}},
			[]interface{}{bsoncore.BuildDocument([]byte{0x01, 0x02, 0x03}, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)), error(nil)},
		},
		{
			"AppendMarshalBSON/empty", MDoc{}.AppendMarshalBSON, []interface{}{[]byte{0x01, 0x02, 0x03}},
			[]interface{}{bsoncore.BuildDocument([]byte{0x01, 0x02, 0x03}, nil), error(nil)},
		},
		{"Equal/IDoc nil", MDoc(nil).Equal, []interface{}{IDoc(nil)}, []interface{}{true}},
		{"Equal/MDoc nil", MDoc(nil).Equal, []interface{}{Doc(nil)}, []interface{}{true}},
		{"Equal/Doc/different length", MDoc{"pi": Double(3.14159)}.Equal, []interface{}{Doc{}}, []interface{}{false}},
		{"Equal/Doc/elems not equal", MDoc{"pi": Double(3.14159)}.Equal, []interface{}{Doc{{"pi", Int32(1)}}}, []interface{}{false}},
		{"Equal/Doc/success", MDoc{"pi": Double(3.14159)}.Equal, []interface{}{Doc{{"pi", Double(3.14159)}}}, []interface{}{true}},
		{"Equal/MDoc/elems not equal", MDoc{"pi": Double(3.14159)}.Equal, []interface{}{MDoc{"pi": Int32(1)}}, []interface{}{false}},
		{"Equal/MDoc/elems not found", MDoc{"pi": Double(3.14159)}.Equal, []interface{}{MDoc{"foo": Int32(1)}}, []interface{}{false}},
		{
			"Equal/MDoc/duplicate",
			Doc{{"a", Int32(1)}, {"a", Int32(1)}}.Equal, []interface{}{MDoc{"a": Int32(1), "b": Int32(2)}},
			[]interface{}{false},
		},
		{"Equal/MDoc/success", Doc{{"pi", Double(3.14159)}}.Equal, []interface{}{MDoc{"pi": Double(3.14159)}}, []interface{}{true}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fn := reflect.ValueOf(tc.fn)
			if fn.Kind() != reflect.Func {
				t.Fatalf("property fn must be a function, but it is a %v", fn.Kind())
			}
			if fn.Type().NumIn() != len(tc.params) && !fn.Type().IsVariadic() {
				t.Fatalf("number of parameters does not match. fn takes %d, but was provided %d", fn.Type().NumIn(), len(tc.params))
			}
			params := make([]reflect.Value, 0, len(tc.params))
			for idx, param := range tc.params {
				if param == nil {
					params = append(params, reflect.New(fn.Type().In(idx)).Elem())
					continue
				}
				params = append(params, reflect.ValueOf(param))
			}
			var rets []reflect.Value
			if fn.Type().IsVariadic() {
				rets = fn.CallSlice(params)
			} else {
				rets = fn.Call(params)
			}
			if len(rets) != len(tc.rets) {
				t.Fatalf("mismatched number of returns. recieved %d; expected %d", len(rets), len(tc.rets))
			}
			for idx := range rets {
				got, want := rets[idx].Interface(), tc.rets[idx]
				if !cmp.Equal(got, want) {
					t.Errorf("Return %d does not match. got %v; want %v", idx, got, want)
				}
			}
		})
	}
}
