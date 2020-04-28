// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonx

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func ExampleDocument() {
	internalVersion := "1234567"

	f := func(appName string) Doc {
		doc := Doc{
			{"driver", Document(Doc{{"name", String("mongo-go-driver")}, {"version", String(internalVersion)}})},
			{"os", Document(Doc{{"type", String("darwin")}, {"architecture", String("amd64")}})},
			{"platform", String("go1.11.1")},
		}
		if appName != "" {
			doc = append(doc, Elem{"application", Document(MDoc{"name": String(appName)})})
		}

		return doc
	}
	buf, err := f("hello-world").MarshalBSON()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(buf)

	// Output: [178 0 0 0 3 100 114 105 118 101 114 0 52 0 0 0 2 110 97 109 101 0 16 0 0 0 109 111 110 103 111 45 103 111 45 100 114 105 118 101 114 0 2 118 101 114 115 105 111 110 0 8 0 0 0 49 50 51 52 53 54 55 0 0 3 111 115 0 46 0 0 0 2 116 121 112 101 0 7 0 0 0 100 97 114 119 105 110 0 2 97 114 99 104 105 116 101 99 116 117 114 101 0 6 0 0 0 97 109 100 54 52 0 0 2 112 108 97 116 102 111 114 109 0 9 0 0 0 103 111 49 46 49 49 46 49 0 3 97 112 112 108 105 99 97 116 105 111 110 0 27 0 0 0 2 110 97 109 101 0 12 0 0 0 104 101 108 108 111 45 119 111 114 108 100 0 0 0]
}

func BenchmarkDocument(b *testing.B) {
	b.ReportAllocs()
	internalVersion := "1234567"
	for i := 0; i < b.N; i++ {
		doc := Doc{
			{"driver", Document(Doc{{"name", String("mongo-go-driver")}, {"version", String(internalVersion)}})},
			{"os", Document(Doc{{"type", String("darwin")}, {"architecture", String("amd64")}})},
			{"platform", String("go1.11.1")},
		}
		_, _ = doc.MarshalBSON()
	}
}

func valueEqual(v1, v2 Val) bool { return v1.Equal(v2) }

func elementEqual(e1, e2 Elem) bool { return e1.Equal(e2) }

func documentComparer(d1, d2 Doc) bool { return d1.Equal(d2) }

func TestDocument(t *testing.T) {
	t.Parallel()
	t.Run("ReadDocument", func(t *testing.T) {
		t.Parallel()
		t.Run("UnmarshalingError", func(t *testing.T) {
			t.Parallel()
			testCases := []struct {
				name    string
				invalid []byte
			}{
				{"base", []byte{0x01, 0x02}},
				{"fuzzed1", []byte("0\x990\xc4")}, // fuzzed
				{"fuzzed2", []byte("\x10\x00\x00\x00\x10\x000000\x0600\x00\x05\x00\xff\xff\xff\u007f")}, // fuzzed
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					want := bsoncore.NewInsufficientBytesError(nil, nil)
					_, got := ReadDoc(tc.invalid)
					if !compareErrors(got, want) {
						t.Errorf("Expected errors to match. got %v; want %v", got, want)
					}
				})
			}
		})
		t.Run("success", func(t *testing.T) {
			t.Parallel()
			valid := bsoncore.BuildDocument(nil, bsoncore.AppendNullElement(nil, "foobar"))
			var want error
			wantDoc := Doc{{"foobar", Null()}}
			gotDoc, got := ReadDoc(valid)
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
			start Doc
			copy  Doc
		}{
			{"nil", nil, Doc{}},
			{"not-nil", Doc{{"foobar", Null()}}, Doc{{"foobar", Null()}}},
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
			"Append", Doc{}.Append,
			[]interface{}{"foo", Null()},
			[]interface{}{Doc{{"foo", Null()}}},
		},
		{
			"Prepend", Doc{}.Prepend,
			[]interface{}{"foo", Null()},
			[]interface{}{Doc{{"foo", Null()}}},
		},
		{
			"Set/append", Doc{{"foo", Null()}}.Set,
			[]interface{}{"bar", Null()},
			[]interface{}{Doc{{"foo", Null()}, {"bar", Null()}}},
		},
		{
			"Set/replace", Doc{{"foo", Null()}, {"bar", Null()}, {"baz", Double(3.14159)}}.Set,
			[]interface{}{"bar", Int64(1234567890)},
			[]interface{}{Doc{{"foo", Null()}, {"bar", Int64(1234567890)}, {"baz", Double(3.14159)}}},
		},
		{
			"Delete/doesn't exist", Doc{{"foo", Null()}, {"bar", Null()}, {"baz", Double(3.14159)}}.Delete,
			[]interface{}{"qux"},
			[]interface{}{Doc{{"foo", Null()}, {"bar", Null()}, {"baz", Double(3.14159)}}},
		},
		{
			"Delete/exists", Doc{{"foo", Null()}, {"bar", Null()}, {"baz", Double(3.14159)}}.Delete,
			[]interface{}{"bar"},
			[]interface{}{Doc{{"foo", Null()}, {"baz", Double(3.14159)}}},
		},
		{
			"Lookup/err", Doc{}.Lookup,
			[]interface{}{[]string{}},
			[]interface{}{Val{}},
		},
		{
			"Lookup/success", Doc{{"pi", Double(3.14159)}}.Lookup,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Double(3.14159)},
		},
		{
			"LookupErr/err", Doc{}.LookupErr,
			[]interface{}{[]string{}},
			[]interface{}{Val{}, KeyNotFound{Key: []string{}}},
		},
		{
			"LookupErr/success", Doc{{"pi", Double(3.14159)}}.LookupErr,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Double(3.14159), error(nil)},
		},
		{
			"LookupElem/err", Doc{}.LookupElement,
			[]interface{}{[]string{}},
			[]interface{}{Elem{}},
		},
		{
			"LookupElem/success", Doc{{"pi", Double(3.14159)}}.LookupElement,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}},
		},
		{
			"LookupElementErr/zero length key", Doc{}.LookupElementErr,
			[]interface{}{[]string{}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{}}},
		},
		{
			"LookupElementErr/key not found", Doc{}.LookupElementErr,
			[]interface{}{[]string{"foo"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo"}}},
		},
		{
			"LookupElementErr/key not found/depth 2", Doc{{"foo", Document(Doc{})}}.LookupElementErr,
			[]interface{}{[]string{"foo", "bar"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo", "bar"}, Depth: 1}},
		},
		{
			"LookupElementErr/invalid depth 2 type", Doc{{"foo", Document(Doc{{"pi", Double(3.14159)}})}}.LookupElementErr,
			[]interface{}{[]string{"foo", "pi", "baz"}},
			[]interface{}{Elem{}, KeyNotFound{Key: []string{"foo", "pi", "baz"}, Depth: 1, Type: bsontype.Double}},
		},
		{
			"LookupElementErr/success", Doc{{"pi", Double(3.14159)}}.LookupElementErr,
			[]interface{}{[]string{"pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}, error(nil)},
		},
		{
			"LookupElementErr/success/depth 2", Doc{{"foo", Document(Doc{{"pi", Double(3.14159)}})}}.LookupElementErr,
			[]interface{}{[]string{"foo", "pi"}},
			[]interface{}{Elem{"pi", Double(3.14159)}, error(nil)},
		},
		{
			"MarshalBSONValue/nil", Doc(nil).MarshalBSONValue,
			nil,
			[]interface{}{bsontype.Null, []byte(nil), error(nil)},
		},
		{
			"MarshalBSONValue/success", Doc{{"pi", Double(3.14159)}}.MarshalBSONValue, nil,
			[]interface{}{
				bsontype.EmbeddedDocument,
				bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
				error(nil),
			},
		},
		{
			"MarshalBSON", Doc{{"pi", Double(3.14159)}}.MarshalBSON, nil,
			[]interface{}{bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)), error(nil)},
		},
		{
			"MarshalBSON/empty", Doc{}.MarshalBSON, nil,
			[]interface{}{bsoncore.BuildDocument(nil, nil), error(nil)},
		},
		{
			"AppendMarshalBSON", Doc{{"pi", Double(3.14159)}}.AppendMarshalBSON, []interface{}{[]byte{0x01, 0x02, 0x03}},
			[]interface{}{bsoncore.BuildDocument([]byte{0x01, 0x02, 0x03}, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)), error(nil)},
		},
		{
			"AppendMarshalBSON/empty", Doc{}.AppendMarshalBSON, []interface{}{[]byte{0x01, 0x02, 0x03}},
			[]interface{}{bsoncore.BuildDocument([]byte{0x01, 0x02, 0x03}, nil), error(nil)},
		},
		{"Equal/IDoc nil", Doc(nil).Equal, []interface{}{IDoc(nil)}, []interface{}{true}},
		{"Equal/MDoc nil", Doc(nil).Equal, []interface{}{MDoc(nil)}, []interface{}{true}},
		{"Equal/Doc/different length", Doc{{"pi", Double(3.14159)}}.Equal, []interface{}{Doc{}}, []interface{}{false}},
		{"Equal/Doc/elems not equal", Doc{{"pi", Double(3.14159)}}.Equal, []interface{}{Doc{{"pi", Int32(1)}}}, []interface{}{false}},
		{"Equal/Doc/success", Doc{{"pi", Double(3.14159)}}.Equal, []interface{}{Doc{{"pi", Double(3.14159)}}}, []interface{}{true}},
		{"Equal/MDoc/elems not equal", Doc{{"pi", Double(3.14159)}}.Equal, []interface{}{MDoc{"pi": Int32(1)}}, []interface{}{false}},
		{"Equal/MDoc/elems not found", Doc{{"pi", Double(3.14159)}}.Equal, []interface{}{MDoc{"foo": Int32(1)}}, []interface{}{false}},
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
