// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Based on gopkg.in/mgo.v2/bson by Gustavo Niemeyer
// See THIRD-PARTY-NOTICES for original license terms.

package mgocompat

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

// Wrap up the document elements contained in data, prepending the int32
// length of the data, and appending the '\x00' value closing the document.
func wrapInDoc(data string) string {
	result := make([]byte, len(data)+5)
	binary.LittleEndian.PutUint32(result, uint32(len(result)))
	copy(result[4:], []byte(data))
	return string(result)
}

func makeZeroDoc(value interface{}) (zero interface{}) {
	v := reflect.ValueOf(value)
	t := v.Type()
	switch t.Kind() {
	case reflect.Map:
		mv := reflect.MakeMap(t)
		zero = mv.Interface()
	case reflect.Ptr:
		pv := reflect.New(v.Type().Elem())
		zero = pv.Interface()
	case reflect.Slice, reflect.Int, reflect.Int64, reflect.Struct:
		zero = reflect.New(t).Interface()
	default:
		panic("unsupported doc type: " + t.Name())
	}
	return zero
}

func testUnmarshal(t *testing.T, data string, obj interface{}) {
	zero := makeZeroDoc(obj)
	err := bson.UnmarshalWithRegistry(Registry, []byte(data), zero)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.True(t, reflect.DeepEqual(zero, obj), "expected: %v, got: %v", obj, zero)
}

type testItemType struct {
	obj  interface{}
	data string
}

// --------------------------------------------------------------------------
// Samples from bsonspec.org:

var sampleItems = []testItemType{
	{bson.M{"hello": "world"},
		"\x16\x00\x00\x00\x02hello\x00\x06\x00\x00\x00world\x00\x00"},
	{bson.M{"BSON": []interface{}{"awesome", float64(5.05), 1986}},
		"1\x00\x00\x00\x04BSON\x00&\x00\x00\x00\x020\x00\x08\x00\x00\x00" +
			"awesome\x00\x011\x00333333\x14@\x102\x00\xc2\x07\x00\x00\x00\x00"},
	{bson.M{"slice": []uint8{1, 2}},
		"\x13\x00\x00\x00\x05slice\x00\x02\x00\x00\x00\x00\x01\x02\x00"},
	{bson.M{"slice": []byte{1, 2}},
		"\x13\x00\x00\x00\x05slice\x00\x02\x00\x00\x00\x00\x01\x02\x00"},
}

func TestMarshalSampleItems(t *testing.T) {
	for i, item := range sampleItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.Equal(t, string(data), item.data, "expected: %v, got: %v", item.data, string(data))
		})
	}
}

func TestUnmarshalSampleItems(t *testing.T) {
	for i, item := range sampleItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			value := bson.M{}
			err := bson.UnmarshalWithRegistry(Registry, []byte(item.data), &value)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.True(t, reflect.DeepEqual(value, item.obj), "expected: %v, got: %v", item.obj, value)
		})
	}
}

// --------------------------------------------------------------------------
// Every type, ordered by the type flag. These are not wrapped with the
// length and last \x00 from the document. wrapInDoc() computes them.
// Note that all of them should be supported as two-way conversions.

var allItems = []testItemType{
	{bson.M{},
		""},
	{bson.M{"_": float64(5.05)},
		"\x01_\x00333333\x14@"},
	{bson.M{"_": "yo"},
		"\x02_\x00\x03\x00\x00\x00yo\x00"},
	{bson.M{"_": bson.M{"a": true}},
		"\x03_\x00\x09\x00\x00\x00\x08a\x00\x01\x00"},
	{bson.M{"_": []interface{}{true, false}},
		"\x04_\x00\r\x00\x00\x00\x080\x00\x01\x081\x00\x00\x00"},
	{bson.M{"_": []byte("yo")},
		"\x05_\x00\x02\x00\x00\x00\x00yo"},
	{bson.M{"_": primitive.Binary{Subtype: 0x80, Data: []byte("udef")}},
		"\x05_\x00\x04\x00\x00\x00\x80udef"},
	{bson.M{"_": primitive.Undefined{}}, // Obsolete, but still seen in the wild.
		"\x06_\x00"},
	{bson.M{"_": primitive.ObjectID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}},
		"\x07_\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B"}, //technically this is not the same as the original mgo test
	{bson.M{"_": primitive.DBPointer{DB: "testnamespace", Pointer: primitive.ObjectID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}}},
		"\x0C_\x00\x0e\x00\x00\x00testnamespace\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B"},
	{bson.M{"_": false},
		"\x08_\x00\x00"},
	{bson.M{"_": true},
		"\x08_\x00\x01"},
	{bson.M{"_": time.Unix(0, 258e6).UTC()}, // Note the NS <=> MS conversion.
		"\x09_\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"_": nil},
		"\x0A_\x00"},
	{bson.M{"_": primitive.Regex{Pattern: "ab", Options: "cd"}},
		"\x0B_\x00ab\x00cd\x00"},
	{bson.M{"_": primitive.JavaScript("code")},
		"\x0D_\x00\x05\x00\x00\x00code\x00"},
	{bson.M{"_": primitive.Symbol("sym")},
		"\x0E_\x00\x04\x00\x00\x00sym\x00"},
	{bson.M{"_": primitive.CodeWithScope{Code: "code", Scope: primitive.D{{"", nil}}}},
		"\x0F_\x00\x14\x00\x00\x00\x05\x00\x00\x00code\x00" +
			"\x07\x00\x00\x00\x0A\x00\x00"},
	{bson.M{"_": 258},
		"\x10_\x00\x02\x01\x00\x00"},
	{bson.M{"_": primitive.Timestamp{0, 258}},
		"\x11_\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"_": int64(258)},
		"\x12_\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"_": int64(258 << 32)},
		"\x12_\x00\x00\x00\x00\x00\x02\x01\x00\x00"},
	{bson.M{"_": primitive.MaxKey{}},
		"\x7F_\x00"},
	{bson.M{"_": primitive.MinKey{}},
		"\xFF_\x00"},
}

func TestMarshalAllItems(t *testing.T) {
	for i, item := range allItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.Equal(t, string(data), wrapInDoc(item.data), "expected: %v, got: %v", wrapInDoc(item.data), string(data))
		})
	}
}

func TestUnmarshalAllItems(t *testing.T) {
	for i, item := range allItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			value := bson.M{}
			err := bson.UnmarshalWithRegistry(Registry, []byte(wrapInDoc(item.data)), &value)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.True(t, reflect.DeepEqual(value, item.obj), "expected: %v, got: %v", item.obj, value)
		})
	}
}

func TestUnmarshalRawAllItems(t *testing.T) {
	for i, item := range allItems {
		if len(item.data) == 0 {
			continue
		}
		value := item.obj.(bson.M)["_"]
		if value == nil {
			continue
		}
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			pv := reflect.New(reflect.ValueOf(value).Type())
			raw := bson.RawValue{Type: bsontype.Type(item.data[0]), Value: []byte(item.data[3:])}
			err := raw.UnmarshalWithRegistry(Registry, pv.Interface())
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.True(t, reflect.DeepEqual(value, pv.Elem().Interface()), "expected: %v, got: %v", value, pv.Elem().Interface())
		})
	}
}

func TestUnmarshalRawIncompatible(t *testing.T) {
	raw := bson.RawValue{Type: 0x08, Value: []byte{0x01}} // true
	err := raw.UnmarshalWithRegistry(Registry, &struct{}{})
	assert.NotNil(t, err, "expected an error")
}

func TestUnmarshalZeroesStruct(t *testing.T) {
	data, err := bson.MarshalWithRegistry(Registry, bson.M{"b": 2})
	assert.Nil(t, err, "expected nil error, got: %v", err)
	type T struct{ A, B int }
	v := T{A: 1}
	err = bson.UnmarshalWithRegistry(Registry, data, &v)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.Equal(t, 0, v.A, "expected: 0, got: %v", v.A)
	assert.Equal(t, 2, v.B, "expected: 2, got: %v", v.B)
}

func TestUnmarshalZeroesMap(t *testing.T) {
	data, err := bson.MarshalWithRegistry(Registry, bson.M{"b": 2})
	assert.Nil(t, err, "expected nil error, got: %v", err)
	m := bson.M{"a": 1}
	err = bson.UnmarshalWithRegistry(Registry, data, &m)
	assert.Nil(t, err, "expected nil error, got: %v", err)

	want := bson.M{"b": 2}
	assert.True(t, reflect.DeepEqual(want, m), "expected: %v, got: %v", want, m)
}

func TestUnmarshalNonNilInterface(t *testing.T) {
	data, err := bson.MarshalWithRegistry(Registry, bson.M{"b": 2})
	assert.Nil(t, err, "expected nil error, got: %v", err)
	m := bson.M{"a": 1}
	var i interface{}
	i = m
	err = bson.UnmarshalWithRegistry(Registry, data, &i)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.True(t, reflect.DeepEqual(bson.M{"b": 2}, i), "expected: %v, got: %v", bson.M{"b": 2}, i)
	assert.True(t, reflect.DeepEqual(bson.M{"a": 1}, m), "expected: %v, got: %v", bson.M{"a": 1}, m)
}

func TestPtrInline(t *testing.T) {
	cases := []struct {
		In  interface{}
		Out bson.M
	}{
		{
			In:  InlinePtrStruct{A: 1, MStruct: &MStruct{M: 3}},
			Out: bson.M{"a": 1, "m": 3},
		},
		{ // go deeper
			In:  inlinePtrPtrStruct{B: 10, InlinePtrStruct: &InlinePtrStruct{A: 20, MStruct: &MStruct{M: 30}}},
			Out: bson.M{"b": 10, "a": 20, "m": 30},
		},
		{
			// nil embed struct
			In:  &InlinePtrStruct{A: 3},
			Out: bson.M{"a": 3},
		},
		{
			// nil embed struct
			In:  &inlinePtrPtrStruct{B: 5},
			Out: bson.M{"b": 5},
		},
	}

	for i, cs := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, cs.In)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			var dataBSON bson.M
			err = bson.UnmarshalWithRegistry(Registry, data, &dataBSON)
			assert.Nil(t, err, "expected nil error, got: %v", err)

			assert.True(t, reflect.DeepEqual(cs.Out, dataBSON), "expected: %v, got: %v", cs.Out, dataBSON)
		})
	}
}

// --------------------------------------------------------------------------
// Some one way marshaling operations which would unmarshal differently.

var js = primitive.JavaScript("code")

var oneWayMarshalItems = []testItemType{
	// These are being passed as pointers, and will unmarshal as values.
	{bson.M{"": &primitive.Binary{Subtype: 0x02, Data: []byte("old")}},
		"\x05\x00\x07\x00\x00\x00\x02\x03\x00\x00\x00old"},
	{bson.M{"": &primitive.Binary{Subtype: 0x80, Data: []byte("udef")}},
		"\x05\x00\x04\x00\x00\x00\x80udef"},
	{bson.M{"": &primitive.Regex{Pattern: "ab", Options: "cd"}},
		"\x0B\x00ab\x00cd\x00"},
	{bson.M{"": &js},
		"\x0D\x00\x05\x00\x00\x00code\x00"},
	{bson.M{"": &primitive.CodeWithScope{Code: "code", Scope: bson.M{"": nil}}},
		"\x0F\x00\x14\x00\x00\x00\x05\x00\x00\x00code\x00" +
			"\x07\x00\x00\x00\x0A\x00\x00"},

	// There's no float32 type in BSON.  Will encode as a float64.
	{bson.M{"": float32(5.05)},
		"\x01\x00\x00\x00\x00@33\x14@"},

	// The array will be unmarshaled as a slice instead.
	{bson.M{"": [2]bool{true, false}},
		"\x04\x00\r\x00\x00\x00\x080\x00\x01\x081\x00\x00\x00"},

	// The typed slice will be unmarshaled as []interface{}.
	{bson.M{"": []bool{true, false}},
		"\x04\x00\r\x00\x00\x00\x080\x00\x01\x081\x00\x00\x00"},

	// Will unmarshal as a []byte.
	{bson.M{"": primitive.Binary{Subtype: 0x00, Data: []byte("yo")}},
		"\x05\x00\x02\x00\x00\x00\x00yo"},
	{bson.M{"": primitive.Binary{Subtype: 0x02, Data: []byte("old")}},
		"\x05\x00\x07\x00\x00\x00\x02\x03\x00\x00\x00old"},

	// No way to preserve the type information here. We might encode as a zero
	// value, but this would mean that pointer values in structs wouldn't be
	// able to correctly distinguish between unset and set to the zero value.
	{bson.M{"": (*byte)(nil)},
		"\x0A\x00"},

	// No int types smaller than int32 in BSON. Could encode this as a char,
	// but it would still be ambiguous, take more, and be awkward in Go when
	// loaded without typing information.
	{bson.M{"": byte(8)},
		"\x10\x00\x08\x00\x00\x00"},

	// There are no unsigned types in BSON.  Will unmarshal as int32 or int64.
	{bson.M{"": uint32(258)},
		"\x10\x00\x02\x01\x00\x00"},
	{bson.M{"": uint64(258)},
		"\x12\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"": uint64(258 << 32)},
		"\x12\x00\x00\x00\x00\x00\x02\x01\x00\x00"},

	// This will unmarshal as int.
	{bson.M{"": int32(258)},
		"\x10\x00\x02\x01\x00\x00"},

	// That's a special case. The unsigned value is too large for an int32,
	// so an int64 is used instead.
	{bson.M{"": uint32(1<<32 - 1)},
		"\x12\x00\xFF\xFF\xFF\xFF\x00\x00\x00\x00"},
	{bson.M{"": uint(1<<32 - 1)},
		"\x12\x00\xFF\xFF\xFF\xFF\x00\x00\x00\x00"},
}

func TestOneWayMarshalItems(t *testing.T) {
	for i, item := range oneWayMarshalItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)

			assert.Equal(t, wrapInDoc(item.data), string(data), "expected: %v, got: %v", bson.Raw(wrapInDoc(item.data)), bson.Raw(data))
		})
	}
}

// --------------------------------------------------------------------------
// Two-way tests for user-defined structures using the samples
// from bsonspec.org.

type specSample1 struct {
	Hello string
}

type specSample2 struct {
	BSON []interface{} `bson:"BSON"`
}

var structSampleItems = []testItemType{
	{&specSample1{"world"},
		"\x16\x00\x00\x00\x02hello\x00\x06\x00\x00\x00world\x00\x00"},
	{&specSample2{[]interface{}{"awesome", float64(5.05), 1986}},
		"1\x00\x00\x00\x04BSON\x00&\x00\x00\x00\x020\x00\x08\x00\x00\x00" +
			"awesome\x00\x011\x00333333\x14@\x102\x00\xc2\x07\x00\x00\x00\x00"},
}

func TestMarshalStructSampleItems(t *testing.T) {
	for i, item := range structSampleItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.Equal(t, item.data, string(data), "expected: %v, got: %v", item.data, string(data))
		})
	}
}

func TestUnmarshalStructSampleItems(t *testing.T) {
	for i, item := range structSampleItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			testUnmarshal(t, item.data, item.obj)
		})
	}
}

func Test64bitInt(t *testing.T) {
	var i int64 = (1 << 31)
	if int(i) > 0 {
		data, err := bson.MarshalWithRegistry(Registry, bson.M{"i": int(i)})
		assert.Nil(t, err, "expected nil error, got: %v", err)
		want := wrapInDoc("\x12i\x00\x00\x00\x00\x80\x00\x00\x00\x00")
		assert.Equal(t, want, string(data), "expected: %v, got: %v", want, string(data))

		var result struct{ I int }
		err = bson.UnmarshalWithRegistry(Registry, data, &result)
		assert.Nil(t, err, "expected nil error, got: %v", err)
		assert.Equal(t, i, int64(result.I), "expected: %v, got: %v", i, int64(result.I))
	}
}

// --------------------------------------------------------------------------
// Generic two-way struct marshaling tests.

type prefixPtr string
type prefixVal string

func (t *prefixPtr) GetBSON() (interface{}, error) {
	if t == nil {
		return nil, nil
	}
	return "foo-" + string(*t), nil
}

func (t *prefixPtr) SetBSON(raw bson.RawValue) error {
	var s string
	if raw.Type == 0x0A {
		return ErrSetZero
	}
	rval := reflect.ValueOf(&s).Elem()
	decoder, err := Registry.LookupDecoder(rval.Type())
	if err != nil {
		return err
	}
	vr := bsonrw.NewBSONValueReader(raw.Type, raw.Value)
	err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: Registry}, vr, rval)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(s, "foo-") {
		return errors.New("Prefix not found: " + s)
	}
	*t = prefixPtr(s[4:])
	return nil
}

func (t prefixVal) GetBSON() (interface{}, error) {
	return "foo-" + string(t), nil
}

func (t *prefixVal) SetBSON(raw bson.RawValue) error {
	var s string
	if raw.Type == 0x0A {
		return ErrSetZero
	}
	rval := reflect.ValueOf(&s).Elem()
	decoder, err := Registry.LookupDecoder(rval.Type())
	if err != nil {
		return err
	}
	vr := bsonrw.NewBSONValueReader(raw.Type, raw.Value)
	err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: Registry}, vr, rval)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(s, "foo-") {
		return errors.New("Prefix not found: " + s)
	}
	*t = prefixVal(s[4:])
	return nil
}

var bytevar = byte(8)
var byteptr = &bytevar
var prefixptr = prefixPtr("bar")
var prefixval = prefixVal("bar")

var structItems = []testItemType{
	{&struct{ Ptr *byte }{nil},
		"\x0Aptr\x00"},
	{&struct{ Ptr *byte }{&bytevar},
		"\x10ptr\x00\x08\x00\x00\x00"},
	{&struct{ Ptr **byte }{&byteptr},
		"\x10ptr\x00\x08\x00\x00\x00"},
	{&struct{ Byte byte }{8},
		"\x10byte\x00\x08\x00\x00\x00"},
	{&struct{ Byte byte }{0},
		"\x10byte\x00\x00\x00\x00\x00"},
	{&struct {
		V byte `bson:"Tag"`
	}{8},
		"\x10Tag\x00\x08\x00\x00\x00"},
	{&struct {
		V *struct {
			Byte byte
		}
	}{&struct{ Byte byte }{8}},
		"\x03v\x00" + "\x0f\x00\x00\x00\x10byte\x00\b\x00\x00\x00\x00"},
	{&struct{ priv byte }{}, ""},

	// The order of the dumped fields should be the same in the struct.
	{&struct{ A, C, B, D, F, E *byte }{},
		"\x0Aa\x00\x0Ac\x00\x0Ab\x00\x0Ad\x00\x0Af\x00\x0Ae\x00"},

	{&struct{ V bson.RawValue }{bson.RawValue{Type: 0x03, Value: []byte("\x0f\x00\x00\x00\x10byte\x00\b\x00\x00\x00\x00")}},
		"\x03v\x00" + "\x0f\x00\x00\x00\x10byte\x00\b\x00\x00\x00\x00"},
	{&struct{ V bson.RawValue }{bson.RawValue{Type: 0x10, Value: []byte("\x00\x00\x00\x00")}},
		"\x10v\x00" + "\x00\x00\x00\x00"},

	// Byte arrays.
	{&struct{ V [2]byte }{[2]byte{'y', 'o'}},
		"\x05v\x00\x02\x00\x00\x00\x00yo"},

	{&struct{ V prefixPtr }{prefixPtr("buzz")},
		"\x02v\x00\x09\x00\x00\x00foo-buzz\x00"},

	{&struct{ V *prefixPtr }{&prefixptr},
		"\x02v\x00\x08\x00\x00\x00foo-bar\x00"},

	{&struct{ V *prefixPtr }{nil},
		"\x0Av\x00"},

	{&struct{ V prefixVal }{prefixVal("buzz")},
		"\x02v\x00\x09\x00\x00\x00foo-buzz\x00"},

	{&struct{ V *prefixVal }{&prefixval},
		"\x02v\x00\x08\x00\x00\x00foo-bar\x00"},

	{&struct{ V *prefixVal }{nil},
		"\x0Av\x00"},
}

func TestMarshalStructItems(t *testing.T) {
	for i, item := range structItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.Equal(t, wrapInDoc(item.data), string(data), "expected: %v, got: %v", wrapInDoc(item.data), string(data))
		})
	}
}

func TestUnmarshalStructItems(t *testing.T) {
	for i, item := range structItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			testUnmarshal(t, wrapInDoc(item.data), item.obj)
		})
	}
}

func TestUnmarshalRawStructItems(t *testing.T) {
	for i, item := range structItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			raw := bson.Raw(wrapInDoc(item.data))
			zero := makeZeroDoc(item.obj)
			err := bson.UnmarshalWithRegistry(Registry, raw, zero)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.True(t, reflect.DeepEqual(item.obj, zero), "expected: %v, got: %v", item.obj, zero)
		})
	}
}

// func TestUnmarshalRawNil(t *testing.T) {
// 	// Regression test: shouldn't try to nil out the pointer itself,
// 	// as it's not settable.
// 	raw := bson.RawValue{Type: 0x0A, Value: []byte{}}
// 	err := raw.UnmarshalWithRegistry(Registry, &struct{}{})
// 	assert.Nil(t, err, "expected nil error, got: %v", err)
// }

// --------------------------------------------------------------------------
// One-way marshaling tests.

type dOnIface struct {
	D interface{}
}

type ignoreField struct {
	Before string
	Ignore string `bson:"-"`
	After  string
}

var marshalItems = []testItemType{
	// Ordered document dump.  Will unmarshal as a dictionary by default.
	{bson.D{{"a", nil}, {"c", nil}, {"b", nil}, {"d", nil}, {"f", nil}, {"e", true}},
		"\x0Aa\x00\x0Ac\x00\x0Ab\x00\x0Ad\x00\x0Af\x00\x08e\x00\x01"},
	{MyD{{"a", nil}, {"c", nil}, {"b", nil}, {"d", nil}, {"f", nil}, {"e", true}},
		"\x0Aa\x00\x0Ac\x00\x0Ab\x00\x0Ad\x00\x0Af\x00\x08e\x00\x01"},
	{&dOnIface{bson.D{{"a", nil}, {"c", nil}, {"b", nil}, {"d", true}}},
		"\x03d\x00" + wrapInDoc("\x0Aa\x00\x0Ac\x00\x0Ab\x00\x08d\x00\x01")},

	{&ignoreField{"before", "ignore", "after"},
		"\x02before\x00\a\x00\x00\x00before\x00\x02after\x00\x06\x00\x00\x00after\x00"},

	// Marshalling a Raw document does nothing.
	// {bson.RawValue{Type: 0x03, Value: []byte(wrapInDoc("anything"))},
	// 	"anything"},
	// {bson.RawValue{Value: []byte(wrapInDoc("anything"))},
	// 	"anything"},
}

func TestMarshalOneWayItems(t *testing.T) {
	for i, item := range marshalItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.Equal(t, wrapInDoc(item.data), string(data), "expected: %v, got: %v", wrapInDoc(item.data), string(data))
		})
	}
}

// --------------------------------------------------------------------------
// One-way unmarshaling tests.

type intAlias int

var unmarshalItems = []testItemType{
	// Field is private.  Should not attempt to unmarshal it.
	{&struct{ priv byte }{},
		"\x10priv\x00\x08\x00\x00\x00"},

	// Ignore non-existing field.
	{&struct{ Byte byte }{9},
		"\x10boot\x00\x08\x00\x00\x00" + "\x10byte\x00\x09\x00\x00\x00"},

	// Do not unmarshal on ignored field.
	{&ignoreField{"before", "", "after"},
		"\x02before\x00\a\x00\x00\x00before\x00" +
			"\x02-\x00\a\x00\x00\x00ignore\x00" +
			"\x02after\x00\x06\x00\x00\x00after\x00"},

	// Ordered document.
	{&struct{ bson.D }{bson.D{{"a", nil}, {"c", nil}, {"b", nil}, {"d", true}}},
		"\x03d\x00" + wrapInDoc("\x0Aa\x00\x0Ac\x00\x0Ab\x00\x08d\x00\x01")},

	// Raw document.
	// {&bson.RawValue{Type: 0x03, Value: []byte(wrapInDoc("\x10byte\x00\x08\x00\x00\x00"))},
	// 	"\x10byte\x00\x08\x00\x00\x00"},

	// Decode old binary.
	{bson.M{"_": []byte("old")},
		"\x05_\x00\x07\x00\x00\x00\x02\x03\x00\x00\x00old"},

	// Decode old binary without length. According to the spec, this shouldn't happen.
	{bson.M{"_": []byte("old")},
		"\x05_\x00\x03\x00\x00\x00\x02old"},

	// int key maps
	{map[int]string{10: "s"},
		"\x0210\x00\x02\x00\x00\x00s\x00"},

	//// event if type is alias to int
	{map[intAlias]string{10: "s"},
		"\x0210\x00\x02\x00\x00\x00s\x00"},
}

func TestUnmarshalOneWayItems(t *testing.T) {
	for i, item := range unmarshalItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			testUnmarshal(t, wrapInDoc(item.data), item.obj)
		})
	}
}

func TestUnmarshalNilInStruct(t *testing.T) {
	// Nil is the default value, so we need to ensure it's indeed being set.
	b := byte(1)
	v := &struct{ Ptr *byte }{&b}
	err := bson.UnmarshalWithRegistry(Registry, []byte(wrapInDoc("\x0Aptr\x00")), v)
	assert.Nil(t, err, "expected nil error, got: %v", err)

	want := &struct{ Ptr *byte }{nil}
	assert.Equal(t, *want, *v, "expected: %v, got: %v", *want, *v)
}

// --------------------------------------------------------------------------
// Marshalling error cases.

type structWithDupKeys struct {
	Name  byte
	Other byte `bson:"name"` // Tag should precede.
}

var marshalErrorItems = []testItemType{
	{bson.M{"": uint64(1 << 63)},
		"BSON has no uint64 type, and value is too large to fit correctly in an int64"},
	{int64(123),
		"Can't marshal int64 as a BSON document"},
	{bson.M{"": 1i},
		"Can't marshal complex128 in a BSON document"},
	{&structWithDupKeys{},
		"Duplicated key 'name' in struct bson_test.structWithDupKeys"},
	{bson.RawValue{Type: 0xA, Value: []byte{}},
		"Attempted to marshal Raw kind 10 as a document"},
	{bson.Raw{},
		"Attempted to marshal empty Raw document"},
	{bson.M{"w": bson.Raw{}},
		"Attempted to marshal empty Raw document"},
	{&inlineDupName{1, struct{ A, B int }{2, 3}},
		"Duplicated key 'a' in struct bson_test.inlineDupName"},
	{&inlineDupMap{},
		"Multiple ,inline maps in struct bson_test.inlineDupMap"},
	{&inlineBadKeyMap{},
		"Option ,inline needs a map with string keys in struct bson_test.inlineBadKeyMap"},
	{&inlineMap{A: 1, M: map[string]interface{}{"a": 1}},
		`Can't have key "a" in inlined map; conflicts with struct field`},
}

func TestMarshalErrorItems(t *testing.T) {
	for i, item := range marshalErrorItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data, err := bson.MarshalWithRegistry(Registry, item.obj)

			assert.NotNil(t, err, "expected error")
			assert.Nil(t, data, " expected nil data, got: %v", data)
		})
	}
}

// --------------------------------------------------------------------------
// Unmarshalling error cases.

type unmarshalErrorType struct {
	obj   interface{}
	data  string
	error string
}

var unmarshalErrorItems = []unmarshalErrorType{
	// Tag name conflicts with existing parameter.
	{&structWithDupKeys{},
		"\x10name\x00\x08\x00\x00\x00",
		"Duplicated key 'name' in struct bson_test.structWithDupKeys"},

	{nil,
		"\xEEname\x00",
		"Unknown element kind \\(0xEE\\)"},

	{struct{ Name bool }{},
		"\x10name\x00\x08\x00\x00\x00",
		"unmarshal can't deal with struct values. Use a pointer"},

	{123,
		"\x10name\x00\x08\x00\x00\x00",
		"unmarshal needs a map or a pointer to a struct"},

	{nil,
		"\x08\x62\x00\x02",
		"encoded boolean must be 1 or 0, found 2"},

	// Non-string and not numeric map key.
	{map[bool]interface{}{true: 1},
		"\x10true\x00\x01\x00\x00\x00",
		"BSON map must have string or decimal keys. Got: map\\[bool\\]interface \\{\\}"},
}

func TestUnmarshalErrorItems(t *testing.T) {
	for i, item := range unmarshalErrorItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			data := []byte(wrapInDoc(item.data))
			var value interface{}
			switch reflect.ValueOf(item.obj).Kind() {
			case reflect.Map, reflect.Ptr:
				value = makeZeroDoc(item.obj)
			case reflect.Invalid:
				value = bson.M{}
			default:
				value = item.obj
			}
			err := bson.UnmarshalWithRegistry(Registry, data, value)
			assert.NotNil(t, err, "expected error")
		})
	}
}

type unmarshalRawErrorType struct {
	obj   interface{}
	raw   bson.RawValue
	error string
}

var unmarshalRawErrorItems = []unmarshalRawErrorType{
	// Tag name conflicts with existing parameter.
	{&structWithDupKeys{},
		bson.RawValue{Type: 0x03, Value: []byte("\x10byte\x00\x08\x00\x00\x00")},
		"Duplicated key 'name' in struct bson_test.structWithDupKeys"},

	{&struct{}{},
		bson.RawValue{Type: 0xEE, Value: []byte{}},
		"Unknown element kind \\(0xEE\\)"},

	{struct{ Name bool }{},
		bson.RawValue{Type: 0x10, Value: []byte("\x08\x00\x00\x00")},
		"raw Unmarshal can't deal with struct values. Use a pointer"},

	{123,
		bson.RawValue{Type: 0x10, Value: []byte("\x08\x00\x00\x00")},
		"raw Unmarshal needs a map or a valid pointer"},
}

func TestUnmarshalRawErrorItems(t *testing.T) {
	for i, item := range unmarshalRawErrorItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := item.raw.UnmarshalWithRegistry(Registry, item.obj)
			assert.NotNil(t, err, "expected error")
		})
	}
}

var corruptedData = []string{
	"\x04\x00\x00\x00\x00",         // Document shorter than minimum
	"\x06\x00\x00\x00\x00",         // Not enough data
	"\x05\x00\x00",                 // Broken length
	"\x05\x00\x00\x00\xff",         // Corrupted termination
	"\x0A\x00\x00\x00\x0Aooop\x00", // Unfinished C string

	// Array end past end of string (s[2]=0x07 is correct)
	wrapInDoc("\x04\x00\x09\x00\x00\x00\x0A\x00\x00"),

	// Array end within string, but past acceptable.
	wrapInDoc("\x04\x00\x08\x00\x00\x00\x0A\x00\x00"),

	// Document end within string, but past acceptable.
	wrapInDoc("\x03\x00\x08\x00\x00\x00\x0A\x00\x00"),

	// // String with corrupted end.
	// wrapInDoc("\x02\x00\x03\x00\x00\x00yo\xFF"),

	// String with negative length (issue #116).
	"\x0c\x00\x00\x00\x02x\x00\xff\xff\xff\xff\x00",

	// // String with zero length (must include trailing '\x00')
	// "\x0c\x00\x00\x00\x02x\x00\x00\x00\x00\x00\x00",

	// Binary with negative length.
	"\r\x00\x00\x00\x05x\x00\xff\xff\xff\xff\x00\x00",
}

func TestUnmarshalMapDocumentTooShort(t *testing.T) {
	for i, data := range corruptedData {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := bson.UnmarshalWithRegistry(Registry, []byte(data), bson.M{})
			assert.NotNil(t, err, "expected error, got nil")

			err = bson.UnmarshalWithRegistry(Registry, []byte(data), &struct{}{})
			assert.NotNil(t, err, "expected error, got nil")
		})
	}
}

// --------------------------------------------------------------------------
// Setter test cases.

var setterResult = map[string]error{}

type setterType struct {
	Received interface{}
}

func (o *setterType) SetBSON(raw bson.RawValue) error {
	rval := reflect.ValueOf(o).Elem().Field(0)
	decoder, err := Registry.LookupDecoder(rval.Type())
	if err != nil {
		return err
	}
	if raw.Type == 0x00 {
		raw.Type = bsontype.EmbeddedDocument
	}
	vr := bsonrw.NewBSONValueReader(raw.Type, raw.Value)
	err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: Registry}, vr, rval)
	if err != nil {
		return err
	}

	if s, ok := o.Received.(string); ok {
		if result, ok := setterResult[s]; ok {
			return result
		}
	}
	return nil
}

type ptrSetterDoc struct {
	Field *setterType `bson:"_"`
}

type valSetterDoc struct {
	Field setterType `bson:"_"`
}

func TestUnmarshalAllItemsWithPtrSetter(t *testing.T) {
	for ind, item := range allItems {
		if ind == 3 {
			continue
		}
		t.Run(strconv.Itoa(ind), func(t *testing.T) {
			for i := 0; i != 2; i++ {
				var field *setterType
				if i == 0 {
					obj := &ptrSetterDoc{}
					err := bson.UnmarshalWithRegistry(Registry, []byte(wrapInDoc(item.data)), obj)
					assert.Nil(t, err, "expected nil error, got: %v", err)
					field = obj.Field
				} else {
					obj := &valSetterDoc{}
					err := bson.UnmarshalWithRegistry(Registry, []byte(wrapInDoc(item.data)), obj)
					assert.Nil(t, err, "expected nil error, got: %v", err)
					field = &obj.Field
				}
				if item.data == "" {
					// Nothing to unmarshal. Should be untouched.
					if i == 0 {
						assert.Nil(t, field, "expected field to be nil, got: %v", field)
					} else {
						assert.Nil(t, field.Received, "expected field.recieved to be nil, got: %v", field.Received)
					}
				} else {
					expected := item.obj.(bson.M)["_"]
					assert.NotNil(t, field, "Pointer not initialized (%#v)", expected)

					assert.True(t, reflect.DeepEqual(expected, field.Received), "expected field.recieved to be: %v, got: %v", expected, field.Received)
				}
			}
		})
	}
}

func TestUnmarshalWholeDocumentWithSetter(t *testing.T) {
	obj := &setterType{}
	err := bson.UnmarshalWithRegistry(Registry, []byte(sampleItems[0].data), obj)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.True(t, reflect.DeepEqual(bson.M{"hello": "world"}, obj.Received), "expected obj.recieved to be: %v, got: %v", bson.M{"hello": "world"}, obj.Received)
}

func TestUnmarshalSetterErrors(t *testing.T) {
	boom := errors.New("BOOM")
	setterResult["2"] = boom
	defer delete(setterResult, "2")

	m := map[string]*setterType{}
	data := wrapInDoc("\x02abc\x00\x02\x00\x00\x001\x00" +
		"\x02def\x00\x02\x00\x00\x002\x00" +
		"\x02ghi\x00\x02\x00\x00\x003\x00")
	err := bson.UnmarshalWithRegistry(Registry, []byte(data), m)
	assert.Equal(t, boom, err, "expected error to be: %v, got: %v", boom, err)

	assert.NotNil(t, m["abc"], "expected value not to be nil")
	assert.Nil(t, m["def"], "expected value to be nil, got: %v", m["def"])
	assert.Nil(t, m["ghi"], "expected value to be nil, got: %v", m["ghi"])

	assert.Equal(t, "1", m["abc"].Received, "expected m[\"abc\"].recieved to be: %v, got: %v", "1", m["abc"].Received)
}

func TestDMap(t *testing.T) {
	d := bson.D{{"a", 1}, {"b", 2}}
	want := bson.M{"a": 1, "b": 2}
	assert.True(t, reflect.DeepEqual(want, d.Map()), "expected: %v, got: %v", want, d.Map())
}

func TestUnmarshalSetterErrSetZero(t *testing.T) {
	setterResult["foo"] = ErrSetZero
	defer delete(setterResult, "field")

	data, err := bson.MarshalWithRegistry(Registry, bson.M{"field": "foo"})
	assert.Nil(t, err, "expected nil error, got: %v", err)

	m := map[string]*setterType{}
	err = bson.UnmarshalWithRegistry(Registry, []byte(data), m)
	assert.Nil(t, err, "expected nil error, got: %v", err)

	value, ok := m["field"]
	assert.True(t, reflect.DeepEqual(true, ok), "expected ok to be: %v, got: %v", true, ok)
	assert.Nil(t, value, "expected nil value, got: %v", value)
}

// --------------------------------------------------------------------------
// Getter test cases.

type typeWithGetter struct {
	result interface{}
	err    error
}

func (t *typeWithGetter) GetBSON() (interface{}, error) {
	if t == nil {
		return "<value is nil>", nil
	}
	return t.result, t.err
}

type docWithGetterField struct {
	Field *typeWithGetter `bson:"_"`
}

func TestMarshalAllItemsWithGetter(t *testing.T) {
	for i, item := range allItems {
		if item.data == "" {
			continue
		}
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			obj := &docWithGetterField{}
			obj.Field = &typeWithGetter{result: item.obj.(bson.M)["_"]}
			data, err := bson.MarshalWithRegistry(Registry, obj)
			assert.Nil(t, err, "expected nil error, got: %v", err)
			assert.Equal(t, wrapInDoc(item.data), string(data),
				"expected value at %v to be: %v, got: %v", i, wrapInDoc(item.data), string(data))
		})
	}
}

func TestMarshalWholeDocumentWithGetter(t *testing.T) {
	obj := &typeWithGetter{result: sampleItems[0].obj}
	data, err := bson.MarshalWithRegistry(Registry, obj)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.Equal(t, sampleItems[0].data, string(data),
		"expected: %v, got: %v", sampleItems[0].data, string(data))
}

func TestGetterErrors(t *testing.T) {
	e := errors.New("oops")

	obj1 := &docWithGetterField{}
	obj1.Field = &typeWithGetter{sampleItems[0].obj, e}
	data, err := bson.MarshalWithRegistry(Registry, obj1)
	assert.Equal(t, e, err, "expected error: %v, got: %v", e, err)
	assert.Nil(t, data, "expected nil data, got: %v", data)

	obj2 := &typeWithGetter{sampleItems[0].obj, e}
	data, err = bson.MarshalWithRegistry(Registry, obj2)
	assert.Equal(t, e, err, "expected error: %v, got: %v", e, err)
	assert.Nil(t, data, "expected nil data, got: %v", data)
}

type intGetter int64

func (t intGetter) GetBSON() (interface{}, error) {
	return int64(t), nil
}

type typeWithIntGetter struct {
	V intGetter `bson:",minsize"`
}

func TestMarshalShortWithGetter(t *testing.T) {
	obj := typeWithIntGetter{42}
	data, err := bson.MarshalWithRegistry(Registry, obj)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	m := bson.M{}
	err = bson.UnmarshalWithRegistry(Registry, data, &m)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.Equal(t, 42, m["v"], "expected m[\"v\"] to be: %v, got: %v", 42, m["v"])
}

func TestMarshalWithGetterNil(t *testing.T) {
	obj := docWithGetterField{}
	data, err := bson.MarshalWithRegistry(Registry, obj)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	m := bson.M{}
	err = bson.UnmarshalWithRegistry(Registry, data, &m)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	want := bson.M{"_": "<value is nil>"}
	assert.Equal(t, want, m, "expected m[\"v\"] to be: %v, got: %v", want, m)
}

// --------------------------------------------------------------------------
// Cross-type conversion tests.

type crossTypeItem struct {
	obj1 interface{}
	obj2 interface{}
}

type condStr struct {
	V string `bson:",omitempty"`
}
type condStrNS struct {
	V string `a:"A" bson:",omitempty" b:"B"`
}
type condBool struct {
	V bool `bson:",omitempty"`
}
type condInt struct {
	V int `bson:",omitempty"`
}
type condUInt struct {
	V uint `bson:",omitempty"`
}
type condFloat struct {
	V float64 `bson:",omitempty"`
}
type condIface struct {
	V interface{} `bson:",omitempty"`
}
type condPtr struct {
	V *bool `bson:",omitempty"`
}
type condSlice struct {
	V []string `bson:",omitempty"`
}
type condMap struct {
	V map[string]int `bson:",omitempty"`
}
type namedCondStr struct {
	V string `bson:"myv,omitempty"`
}
type condTime struct {
	V time.Time `bson:",omitempty"`
}
type condStruct struct {
	V struct{ A []int } `bson:",omitempty"`
}
type condRaw struct {
	V bson.RawValue `bson:",omitempty"`
}

type shortInt struct {
	V int64 `bson:",minsize"`
}
type shortUint struct {
	V uint64 `bson:",minsize"`
}
type shortIface struct {
	V interface{} `bson:",minsize"`
}
type shortPtr struct {
	V *int64 `bson:",minsize"`
}
type shortNonEmptyInt struct {
	V int64 `bson:",minsize,omitempty"`
}

type inlineInt struct {
	V struct{ A, B int } `bson:",inline"`
}
type inlineCantPtr struct {
	V *struct{ A, B int } `bson:",inline"`
}
type inlineDupName struct {
	A int
	V struct{ A, B int } `bson:",inline"`
}
type inlineMap struct {
	A int
	M map[string]interface{} `bson:",inline"`
}
type inlineMapInt struct {
	A int
	M map[string]int `bson:",inline"`
}
type inlineMapMyM struct {
	A int
	M MyM `bson:",inline"`
}
type inlineDupMap struct {
	M1 map[string]interface{} `bson:",inline"`
	M2 map[string]interface{} `bson:",inline"`
}
type inlineBadKeyMap struct {
	M map[int]int `bson:",inline"`
}
type inlineUnexported struct {
	M          map[string]interface{} `bson:",inline"`
	unexported `bson:",inline"`
}
type MStruct struct {
	M int `bson:"m,omitempty"`
}
type InlinePtrStruct struct {
	A        int
	*MStruct `bson:",inline"`
}
type inlinePtrPtrStruct struct {
	B                int
	*InlinePtrStruct `bson:",inline"`
}
type unexported struct {
	A int
}

type getterSetterD bson.D

func (s getterSetterD) GetBSON() (interface{}, error) {
	if len(s) == 0 {
		return bson.D{}, nil
	}
	return bson.D(s[:len(s)-1]), nil
}

func (s *getterSetterD) SetBSON(raw bson.RawValue) error {
	var doc bson.D
	rval := reflect.ValueOf(&doc).Elem()
	decoder, err := Registry.LookupDecoder(rval.Type())
	if err != nil {
		return err
	}
	if raw.Type == 0x00 {
		raw.Type = bsontype.EmbeddedDocument
	}
	vr := bsonrw.NewBSONValueReader(raw.Type, raw.Value)
	err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: Registry}, vr, rval)
	if err != nil {
		return err
	}
	doc = append(doc, bson.E{"suffix", true})
	*s = getterSetterD(doc)
	return err
}

type getterSetterInt int

func (i getterSetterInt) GetBSON() (interface{}, error) {
	return bson.D{{"a", int(i)}}, nil
}

func (i *getterSetterInt) SetBSON(raw bson.RawValue) error {
	var doc struct{ A int }
	rval := reflect.ValueOf(&doc).Elem()
	decoder, err := Registry.LookupDecoder(rval.Type())
	if err != nil {
		return err
	}
	if raw.Type == 0x00 {
		raw.Type = bsontype.EmbeddedDocument
	}
	vr := bsonrw.NewBSONValueReader(raw.Type, raw.Value)
	err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: Registry}, vr, rval)
	if err != nil {
		return err
	}
	*i = getterSetterInt(doc.A)
	return err
}

type ifaceType interface {
	Hello()
}

type ifaceSlice []ifaceType

func (s *ifaceSlice) SetBSON(raw bson.RawValue) error {
	var ns []int
	rval := reflect.ValueOf(&ns).Elem()
	decoder, err := Registry.LookupDecoder(rval.Type())
	if err != nil {
		return err
	}
	vr := bsonrw.NewBSONValueReader(raw.Type, raw.Value)
	err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: Registry}, vr, rval)
	if err != nil {
		return err
	}
	*s = make(ifaceSlice, ns[0])
	return nil
}

func (s ifaceSlice) GetBSON() (interface{}, error) {
	return []int{len(s)}, nil
}

type (
	MyString string
	MyBytes  []byte
	MyBool   bool
	MyD      bson.D
	MyRawD   bson.Raw
	MyM      map[string]interface{}
)

var (
	truevar  = true
	falsevar = false

	int64var = int64(42)
	int64ptr = &int64var
	intvar   = int(42)
	intptr   = &intvar

	gsintvar = getterSetterInt(42)
)

func parseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// That's a pretty fun test.  It will dump the first item, generate a zero
// value equivalent to the second one, load the dumped data onto it, and then
// verify that the resulting value is deep-equal to the untouched second value.
// Then, it will do the same in the *opposite* direction!
var twoWayCrossItems = []crossTypeItem{
	// int<=>int
	{&struct{ I int }{42}, &struct{ I int8 }{42}},
	{&struct{ I int }{42}, &struct{ I int32 }{42}},
	{&struct{ I int }{42}, &struct{ I int64 }{42}},
	{&struct{ I int8 }{42}, &struct{ I int32 }{42}},
	{&struct{ I int8 }{42}, &struct{ I int64 }{42}},
	{&struct{ I int32 }{42}, &struct{ I int64 }{42}},

	// uint<=>uint
	{&struct{ I uint }{42}, &struct{ I uint8 }{42}},
	{&struct{ I uint }{42}, &struct{ I uint32 }{42}},
	{&struct{ I uint }{42}, &struct{ I uint64 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I uint32 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I uint64 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I uint64 }{42}},

	// float32<=>float64
	{&struct{ I float32 }{42}, &struct{ I float64 }{42}},

	// int<=>uint
	{&struct{ I uint }{42}, &struct{ I int }{42}},
	{&struct{ I uint }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint }{42}, &struct{ I int64 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int64 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int64 }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int64 }{42}},

	// int <=> float
	{&struct{ I int }{42}, &struct{ I float64 }{42}},

	// int <=> bool
	{&struct{ I int }{1}, &struct{ I bool }{true}},
	{&struct{ I int }{0}, &struct{ I bool }{false}},

	// uint <=> float64
	{&struct{ I uint }{42}, &struct{ I float64 }{42}},

	// uint <=> bool
	{&struct{ I uint }{1}, &struct{ I bool }{true}},
	{&struct{ I uint }{0}, &struct{ I bool }{false}},

	// float64 <=> bool
	{&struct{ I float64 }{1}, &struct{ I bool }{true}},
	{&struct{ I float64 }{0}, &struct{ I bool }{false}},

	// string <=> string and string <=> []byte
	{&struct{ S []byte }{[]byte("abc")}, &struct{ S string }{"abc"}},
	{&struct{ S []byte }{[]byte("def")}, &struct{ S primitive.Symbol }{"def"}},
	{&struct{ S string }{"ghi"}, &struct{ S primitive.Symbol }{"ghi"}},

	{&struct{ S string }{"0123456789ab"},
		&struct{ S primitive.ObjectID }{primitive.ObjectID{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x61, 0x62}}},

	// map <=> struct
	{&struct {
		A struct {
			B, C int
		}
	}{struct{ B, C int }{1, 2}},
		map[string]map[string]int{"a": {"b": 1, "c": 2}}},

	{&struct{ A primitive.Symbol }{"abc"}, map[string]string{"a": "abc"}},
	{&struct{ A primitive.Symbol }{"abc"}, map[string][]byte{"a": []byte("abc")}},
	{&struct{ A []byte }{[]byte("abc")}, map[string]string{"a": "abc"}},
	{&struct{ A uint }{42}, map[string]int{"a": 42}},
	{&struct{ A uint }{42}, map[string]float64{"a": 42}},
	{&struct{ A uint }{1}, map[string]bool{"a": true}},
	{&struct{ A int }{42}, map[string]uint{"a": 42}},
	{&struct{ A int }{42}, map[string]float64{"a": 42}},
	{&struct{ A int }{1}, map[string]bool{"a": true}},
	{&struct{ A float64 }{42}, map[string]float32{"a": 42}},
	{&struct{ A float64 }{42}, map[string]int{"a": 42}},
	{&struct{ A float64 }{42}, map[string]uint{"a": 42}},
	{&struct{ A float64 }{1}, map[string]bool{"a": true}},
	{&struct{ A bool }{true}, map[string]int{"a": 1}},
	{&struct{ A bool }{true}, map[string]uint{"a": 1}},
	{&struct{ A bool }{true}, map[string]float64{"a": 1}},
	{&struct{ A **byte }{&byteptr}, map[string]byte{"a": 8}},

	// url.URL <=> string
	{&struct{ URL *url.URL }{parseURL("h://e.c/p")}, map[string]string{"url": "h://e.c/p"}},
	{&struct{ URL url.URL }{*parseURL("h://e.c/p")}, map[string]string{"url": "h://e.c/p"}},

	// Slices
	{&struct{ S []int }{[]int{1, 2, 3}}, map[string][]int{"s": {1, 2, 3}}},
	{&struct{ S *[]int }{&[]int{1, 2, 3}}, map[string][]int{"s": {1, 2, 3}}},

	// Conditionals
	{&condBool{true}, map[string]bool{"v": true}},
	{&condBool{}, map[string]bool{}},
	{&condInt{1}, map[string]int{"v": 1}},
	{&condInt{}, map[string]int{}},
	{&condUInt{1}, map[string]uint{"v": 1}},
	{&condUInt{}, map[string]uint{}},
	{&condFloat{}, map[string]int{}},
	{&condStr{"yo"}, map[string]string{"v": "yo"}},
	{&condStr{}, map[string]string{}},
	{&condStrNS{"yo"}, map[string]string{"v": "yo"}},
	{&condStrNS{}, map[string]string{}},
	{&condSlice{[]string{"yo"}}, map[string][]string{"v": {"yo"}}},
	{&condSlice{}, map[string][]string{}},
	{&condMap{map[string]int{"k": 1}}, bson.M{"v": bson.M{"k": 1}}},
	{&condMap{}, map[string][]string{}},
	{&condIface{"yo"}, map[string]string{"v": "yo"}},
	{&condIface{""}, map[string]string{"v": ""}},
	{&condIface{}, map[string]string{}},
	{&condPtr{&truevar}, map[string]bool{"v": true}},
	{&condPtr{&falsevar}, map[string]bool{"v": false}},
	{&condPtr{}, map[string]string{}},

	{&condTime{time.Unix(123456789, 123e6).UTC()}, map[string]time.Time{"v": time.Unix(123456789, 123e6).UTC()}},
	{&condTime{}, map[string]string{}},

	{&condStruct{struct{ A []int }{[]int{1}}}, bson.M{"v": bson.M{"a": []interface{}{1}}}},
	{&condStruct{struct{ A []int }{}}, bson.M{}},

	// {&condRaw{bson.RawValue{Type: 0x0A, Value: []byte{}}},bson.M{"v": nil}},
	// {&condRaw{bson.RawValue{Type: 0x00}}, bson.M{}},

	{&namedCondStr{"yo"}, map[string]string{"myv": "yo"}},
	{&namedCondStr{}, map[string]string{}},

	{&shortInt{1}, map[string]interface{}{"v": 1}},
	{&shortInt{1 << 30}, map[string]interface{}{"v": 1 << 30}},
	{&shortInt{1 << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortUint{1 << 30}, map[string]interface{}{"v": 1 << 30}},
	{&shortUint{1 << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortIface{int64(1) << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortPtr{int64ptr}, map[string]interface{}{"v": intvar}},

	{&shortNonEmptyInt{1}, map[string]interface{}{"v": 1}},
	{&shortNonEmptyInt{1 << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortNonEmptyInt{}, map[string]interface{}{}},

	{&inlineInt{struct{ A, B int }{1, 2}}, map[string]interface{}{"a": 1, "b": 2}},
	{&inlineMap{A: 1, M: map[string]interface{}{"b": 2}}, map[string]interface{}{"a": 1, "b": 2}},
	{&inlineMap{A: 1, M: nil}, map[string]interface{}{"a": 1}},
	{&inlineMapInt{A: 1, M: map[string]int{"b": 2}}, map[string]int{"a": 1, "b": 2}},
	{&inlineMapInt{A: 1, M: nil}, map[string]int{"a": 1}},
	{&inlineMapMyM{A: 1, M: MyM{"b": MyM{"c": 3}}}, map[string]interface{}{"a": 1, "b": map[string]interface{}{"c": 3}}},
	{&inlineUnexported{M: map[string]interface{}{"b": 1}, unexported: unexported{A: 2}}, map[string]interface{}{"b": 1, "a": 2}},

	// []byte <=> Binary
	{&struct{ B []byte }{[]byte("abc")}, map[string]primitive.Binary{"b": {Data: []byte("abc")}}},

	// []byte <=> MyBytes
	{&struct{ B MyBytes }{[]byte("abc")}, &map[string]string{"b": "abc"}},
	{&struct{ B MyBytes }{[]byte{}}, &map[string]string{"b": ""}},
	// {&struct{ B MyBytes }{}, &map[string]bool{}},
	{&struct{ B []byte }{[]byte("abc")}, &map[string]MyBytes{"b": []byte("abc")}},

	// bool <=> MyBool
	{&struct{ B MyBool }{true}, map[string]bool{"b": true}},
	{&struct{ B MyBool }{}, map[string]bool{"b": false}},
	// {&struct{ B MyBool }{}, map[string]string{}},
	{&struct{ B bool }{}, map[string]MyBool{"b": false}},

	// arrays
	{&struct{ V [2]int }{[...]int{1, 2}}, map[string][2]int{"v": {1, 2}}},
	{&struct{ V [2]byte }{[...]byte{1, 2}}, map[string][2]byte{"v": {1, 2}}},

	// zero time
	{&struct{ V time.Time }{}, map[string]interface{}{"v": time.Time{}}},

	// zero time + 1 second + 1 millisecond; overflows int64 as nanoseconds
	{&struct{ V time.Time }{time.Unix(-62135596799, 1e6).UTC()},
		map[string]interface{}{"v": time.Unix(-62135596799, 1e6).UTC()}},

	// bson.D <=> []DocElem
	{&bson.D{{"a", bson.D{{"b", 1}, {"c", 2}}}}, &bson.D{{"a", bson.D{{"b", 1}, {"c", 2}}}}},
	{&bson.D{{"a", bson.D{{"b", 1}, {"c", 2}}}}, &MyD{{"a", MyD{{"b", 1}, {"c", 2}}}}},
	{&struct{ V MyD }{MyD{{"a", 1}}}, &bson.D{{"v", bson.D{{"a", 1}}}}},

	// bson.M <=> map
	{&bson.M{"a": bson.M{"b": 1, "c": 2}}, MyM{"a": MyM{"b": 1, "c": 2}}},
	{&bson.M{"a": bson.M{"b": 1, "c": 2}}, map[string]interface{}{"a": map[string]interface{}{"b": 1, "c": 2}}},

	// bson.M <=> map[MyString]
	{&bson.M{"a": bson.M{"b": 1, "c": 2}}, map[MyString]interface{}{"a": map[MyString]interface{}{"b": 1, "c": 2}}},

	// json.Number <=> int64, float64
	{&struct{ N json.Number }{"5"}, map[string]interface{}{"n": int64(5)}},
	{&struct{ N json.Number }{"5.05"}, map[string]interface{}{"n": 5.05}},
	{&struct{ N json.Number }{"9223372036854776000"}, map[string]interface{}{"n": float64(1 << 63)}},

	// bson.D <=> non-struct getter/setter
	{&bson.D{{"a", 1}}, &getterSetterD{{"a", 1}, {"suffix", true}}},
	{&bson.D{{"a", 42}}, &gsintvar},

	// Interface slice setter.
	{&struct{ V ifaceSlice }{ifaceSlice{nil, nil, nil}}, bson.M{"v": []interface{}{3}}},
}

// Same thing, but only one way (obj1 => obj2).
var oneWayCrossItems = []crossTypeItem{
	// Would get decoded into a int32 too in the opposite direction.
	{&shortIface{int64(1) << 30}, map[string]interface{}{"v": 1 << 30}},

	// Ensure omitempty on struct with private fields works properly.
	{&struct {
		V struct{ v time.Time } `bson:",omitempty"`
	}{}, map[string]interface{}{}},
}

func testCrossPair(t *testing.T, dump interface{}, load interface{}) {
	zero := makeZeroDoc(load)
	data, err := bson.MarshalWithRegistry(Registry, dump)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	err = bson.UnmarshalWithRegistry(Registry, data, zero)
	assert.Nil(t, err, "expected nil error, got: %v", err)

	assert.True(t, reflect.DeepEqual(load, zero), "expected: %v, got: %v", load, zero)
}

func TestTwoWayCrossPairs(t *testing.T) {
	for i, item := range twoWayCrossItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			testCrossPair(t, item.obj1, item.obj2)
			testCrossPair(t, item.obj2, item.obj1)
		})
	}
}

func TestOneWayCrossPairs(t *testing.T) {
	for i, item := range oneWayCrossItems {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			testCrossPair(t, item.obj1, item.obj2)
		})
	}
}

// --------------------------------------------------------------------------
// ObjectId JSON marshalling.

type jsonType struct {
	ID primitive.ObjectID
}

func objectIDHex(s string) primitive.ObjectID {
	oid, _ := primitive.ObjectIDFromHex(s)
	return oid
}

var jsonIDTests = []struct {
	value     jsonType
	json      string
	marshal   bool
	unmarshal bool
	error     string
}{{
	value:     jsonType{ID: objectIDHex("4d88e15b60f486e428412dc9")},
	json:      `{"ID":"4d88e15b60f486e428412dc9"}`,
	marshal:   true,
	unmarshal: true,
}, {
	// 	value:     jsonType{},
	// 	json:      `{"Id":""}`,
	// 	marshal:   true,
	// 	unmarshal: true,
	// }, {
	// 	value:     jsonType{},
	// 	json:      `{"Id":null}`,
	// 	marshal:   false,
	// 	unmarshal: true,
	// }, {
	json:      `{"Id":"4d88e15b60f486e428412dc9A"}`,
	error:     `invalid ObjectId in JSON: "4d88e15b60f486e428412dc9A"`,
	marshal:   false,
	unmarshal: true,
}, {
	json:      `{"Id":"4d88e15b60f486e428412dcZ"}`,
	error:     `invalid ObjectId in JSON: "4d88e15b60f486e428412dcZ" .*`,
	marshal:   false,
	unmarshal: true,
}}

func TestObjectIdJSONMarshaling(t *testing.T) {
	for i, test := range jsonIDTests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if test.marshal {
				data, err := json.Marshal(&test.value)
				if test.error == "" {
					assert.Nil(t, err, "expected nil error, got: %v", err)
					assert.Equal(t, test.json, string(data), "expected: %v, got: %v", test.json, string(data))
				} else {
					assert.NotNil(t, err, "expected a marshal error")
				}
			}

			if test.unmarshal {
				var value jsonType
				err := json.Unmarshal([]byte(test.json), &value)
				if test.error == "" {
					assert.Nil(t, err, "expected nil error, got: %v", err)
					assert.True(t, reflect.DeepEqual(test.value, value), "expected: %v, got: %v", test.value, value)
				} else {
					assert.NotNil(t, err, "expected a unmarshal error")
				}
			}
		})
	}
}

func TestMarshalNotRespectNil(t *testing.T) {
	type T struct {
		Slice  []int
		BSlice []byte
		Map    map[string]interface{}
	}

	testStruct1 := T{}

	assert.Nil(t, testStruct1.Slice, "expected nil slice, got: %v", testStruct1.Slice)
	assert.Nil(t, testStruct1.BSlice, "expected nil byte slice, got: %v", testStruct1.BSlice)
	assert.Nil(t, testStruct1.Map, "expected nil map, got: %v", testStruct1.Map)

	b, _ := bson.MarshalWithRegistry(Registry, testStruct1)

	testStruct2 := T{}

	_ = bson.UnmarshalWithRegistry(Registry, b, &testStruct2)

	assert.NotNil(t, testStruct2.Slice, "expected non-nil slice")
	assert.NotNil(t, testStruct2.BSlice, "expected non-nil byte slice")
	assert.NotNil(t, testStruct2.Map, "expected non-nil map")
}

func TestMarshalRespectNil(t *testing.T) {
	type T struct {
		Slice    []int
		SlicePtr *[]int
		Ptr      *int
		Map      map[string]interface{}
		MapPtr   *map[string]interface{}
	}

	testStruct1 := T{}

	assert.Nil(t, testStruct1.Slice, "expected nil slice, got: %v", testStruct1.Slice)
	assert.Nil(t, testStruct1.SlicePtr, "expected nil slice ptr, got: %v", testStruct1.SlicePtr)
	assert.Nil(t, testStruct1.Map, "expected nil map, got: %v", testStruct1.Map)
	assert.Nil(t, testStruct1.MapPtr, "expected nil map ptr, got: %v", testStruct1.MapPtr)
	assert.Nil(t, testStruct1.Ptr, "expected nil ptr, got: %v", testStruct1.Ptr)

	b, _ := bson.MarshalWithRegistry(RegistryRespectNilValues, testStruct1)

	testStruct2 := T{}

	_ = bson.UnmarshalWithRegistry(RegistryRespectNilValues, b, &testStruct2)

	assert.Nil(t, testStruct2.Slice, "expected nil slice, got: %v", testStruct2.Slice)
	assert.Nil(t, testStruct2.SlicePtr, "expected nil slice ptr, got: %v", testStruct2.SlicePtr)
	assert.Nil(t, testStruct2.Map, "expected nil map, got: %v", testStruct2.Map)
	assert.Nil(t, testStruct2.MapPtr, "expected nil map ptr, got: %v", testStruct2.MapPtr)
	assert.Nil(t, testStruct2.Ptr, "expected nil ptr, got: %v", testStruct2.Ptr)

	testStruct1 = T{
		Slice:    []int{},
		SlicePtr: &[]int{},
		Map:      map[string]interface{}{},
		MapPtr:   &map[string]interface{}{},
	}

	assert.NotNil(t, testStruct1.Slice, "expected non-nil slice")
	assert.NotNil(t, testStruct1.SlicePtr, "expected non-nil slice ptr")
	assert.NotNil(t, testStruct1.Map, "expected non-nil map")
	assert.NotNil(t, testStruct1.MapPtr, "expected non-nil map ptr")

	b, _ = bson.MarshalWithRegistry(RegistryRespectNilValues, testStruct1)

	testStruct2 = T{}

	_ = bson.UnmarshalWithRegistry(RegistryRespectNilValues, b, &testStruct2)

	assert.NotNil(t, testStruct2.Slice, "expected non-nil slice")
	assert.NotNil(t, testStruct2.SlicePtr, "expected non-nil slice ptr")
	assert.NotNil(t, testStruct2.Map, "expected non-nil map")
	assert.NotNil(t, testStruct2.MapPtr, "expected non-nil map ptr")
}

// Our mgocompat.Registry tests
type Inner struct {
	ID string
}

type InlineLoop struct {
	Inner `bson:",inline"`
	Value string
	Draft *InlineLoop `bson:",omitempty"`
}

func TestInlineWithPointerToSelf(t *testing.T) {
	x1 := InlineLoop{
		Inner: Inner{
			ID: "1",
		},
		Value: "",
	}

	bytes, err := bson.MarshalWithRegistry(Registry, x1)
	assert.Nil(t, err, "expected nil error, got: %v", err)

	var x2 InlineLoop
	err = bson.UnmarshalWithRegistry(Registry, bytes, &x2)
	assert.Nil(t, err, "expected nil error, got: %v", err)
	assert.Equal(t, x1, x2, "Expected %v, got %v", x1, x2)
}
