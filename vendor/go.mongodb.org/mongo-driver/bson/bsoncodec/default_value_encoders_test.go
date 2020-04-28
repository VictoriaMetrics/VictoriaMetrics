// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsonrw/bsonrwtest"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

type myInterface interface {
	Foo() int
}

type myStruct struct {
	Val int
}

func (ms myStruct) Foo() int {
	return ms.Val
}

func TestDefaultValueEncoders(t *testing.T) {
	var dve DefaultValueEncoders
	var wrong = func(string, string) string { return "wrong" }

	type mybool bool
	type myint8 int8
	type myint16 int16
	type myint32 int32
	type myint64 int64
	type myint int
	type myuint8 uint8
	type myuint16 uint16
	type myuint32 uint32
	type myuint64 uint64
	type myuint uint
	type myfloat32 float32
	type myfloat64 float64
	type mystring string

	now := time.Now().Truncate(time.Millisecond)
	pjsnum := new(json.Number)
	*pjsnum = json.Number("3.14159")
	d128 := primitive.NewDecimal128(12345, 67890)
	var nilValueMarshaler *testValueMarshaler
	var nilMarshaler *testMarshaler
	var nilProxy *testProxy

	vmStruct := struct{ V testValueMarshalPtr }{testValueMarshalPtr{t: bsontype.String, buf: []byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00}}}
	mStruct := struct{ V testMarshalPtr }{testMarshalPtr{buf: bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))}}
	pStruct := struct{ V testProxyPtr }{testProxyPtr{ret: int64(1234567890)}}

	type subtest struct {
		name   string
		val    interface{}
		ectx   *EncodeContext
		llvrw  *bsonrwtest.ValueReaderWriter
		invoke bsonrwtest.Invoked
		err    error
	}

	testCases := []struct {
		name     string
		ve       ValueEncoder
		subtests []subtest
	}{
		{
			"BooleanEncodeValue",
			ValueEncoderFunc(dve.BooleanEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "BooleanEncodeValue", Kinds: []reflect.Kind{reflect.Bool}, Received: reflect.ValueOf(wrong)},
				},
				{"fast path", bool(true), nil, nil, bsonrwtest.WriteBoolean, nil},
				{"reflection path", mybool(true), nil, nil, bsonrwtest.WriteBoolean, nil},
			},
		},
		{
			"IntEncodeValue",
			ValueEncoderFunc(dve.IntEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "IntEncodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf(wrong),
					},
				},
				{"int8/fast path", int8(127), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int16/fast path", int16(32767), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int32/fast path", int32(2147483647), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int64/fast path", int64(1234567890987), nil, nil, bsonrwtest.WriteInt64, nil},
				{"int64/fast path - minsize", int64(math.MaxInt32), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"int64/fast path - minsize too large", int64(math.MaxInt32 + 1), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"int64/fast path - minsize too small", int64(math.MinInt32 - 1), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"int/fast path - positive int32", int(math.MaxInt32 - 1), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/fast path - negative int32", int(math.MinInt32 + 1), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/fast path - MaxInt32", int(math.MaxInt32), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/fast path - MinInt32", int(math.MinInt32), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/fast path - larger than MaxInt32", int(math.MaxInt32 + 1), nil, nil, bsonrwtest.WriteInt64, nil},
				{"int/fast path - smaller than MinInt32", int(math.MinInt32 - 1), nil, nil, bsonrwtest.WriteInt64, nil},
				{"int8/reflection path", myint8(127), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int16/reflection path", myint16(32767), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int32/reflection path", myint32(2147483647), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int64/reflection path", myint64(1234567890987), nil, nil, bsonrwtest.WriteInt64, nil},
				{"int64/reflection path - minsize", myint64(math.MaxInt32), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"int64/reflection path - minsize too large", myint64(math.MaxInt32 + 1), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"int64/reflection path - minsize too small", myint64(math.MinInt32 - 1), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"int/reflection path - positive int32", myint(math.MaxInt32 - 1), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/reflection path - negative int32", myint(math.MinInt32 + 1), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/reflection path - MaxInt32", myint(math.MaxInt32), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/reflection path - MinInt32", myint(math.MinInt32), nil, nil, bsonrwtest.WriteInt32, nil},
				{"int/reflection path - larger than MaxInt32", myint(math.MaxInt32 + 1), nil, nil, bsonrwtest.WriteInt64, nil},
				{"int/reflection path - smaller than MinInt32", myint(math.MinInt32 - 1), nil, nil, bsonrwtest.WriteInt64, nil},
			},
		},
		{
			"UintEncodeValue",
			defaultUIntCodec,
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "UintEncodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf(wrong),
					},
				},
				{"uint8/fast path", uint8(127), nil, nil, bsonrwtest.WriteInt32, nil},
				{"uint16/fast path", uint16(32767), nil, nil, bsonrwtest.WriteInt32, nil},
				{"uint32/fast path", uint32(2147483647), nil, nil, bsonrwtest.WriteInt64, nil},
				{"uint64/fast path", uint64(1234567890987), nil, nil, bsonrwtest.WriteInt64, nil},
				{"uint/fast path", uint(1234567), nil, nil, bsonrwtest.WriteInt64, nil},
				{"uint32/fast path - minsize", uint32(2147483647), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"uint64/fast path - minsize", uint64(2147483647), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"uint/fast path - minsize", uint(2147483647), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"uint32/fast path - minsize too large", uint32(2147483648), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"uint64/fast path - minsize too large", uint64(2147483648), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"uint/fast path - minsize too large", uint(2147483648), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"uint64/fast path - overflow", uint64(1 << 63), nil, nil, bsonrwtest.Nothing, fmt.Errorf("%d overflows int64", uint(1<<63))},
				{"uint/fast path - overflow", uint(1 << 63), nil, nil, bsonrwtest.Nothing, fmt.Errorf("%d overflows int64", uint(1<<63))},
				{"uint8/reflection path", myuint8(127), nil, nil, bsonrwtest.WriteInt32, nil},
				{"uint16/reflection path", myuint16(32767), nil, nil, bsonrwtest.WriteInt32, nil},
				{"uint32/reflection path", myuint32(2147483647), nil, nil, bsonrwtest.WriteInt64, nil},
				{"uint64/reflection path", myuint64(1234567890987), nil, nil, bsonrwtest.WriteInt64, nil},
				{"uint/reflection path", myuint(1234567890987), nil, nil, bsonrwtest.WriteInt64, nil},
				{"uint32/reflection path - minsize", myuint32(2147483647), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"uint64/reflection path - minsize", myuint64(2147483647), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"uint/reflection path - minsize", myuint(2147483647), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt32, nil},
				{"uint32/reflection path - minsize too large", myuint(1 << 31), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"uint64/reflection path - minsize too large", myuint64(1 << 31), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"uint/reflection path - minsize too large", myuint(2147483648), &EncodeContext{MinSize: true}, nil, bsonrwtest.WriteInt64, nil},
				{"uint64/reflection path - overflow", myuint64(1 << 63), nil, nil, bsonrwtest.Nothing, fmt.Errorf("%d overflows int64", uint(1<<63))},
				{"uint/reflection path - overflow", myuint(1 << 63), nil, nil, bsonrwtest.Nothing, fmt.Errorf("%d overflows int64", uint(1<<63))},
			},
		},
		{
			"FloatEncodeValue",
			ValueEncoderFunc(dve.FloatEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "FloatEncodeValue",
						Kinds:    []reflect.Kind{reflect.Float32, reflect.Float64},
						Received: reflect.ValueOf(wrong),
					},
				},
				{"float32/fast path", float32(3.14159), nil, nil, bsonrwtest.WriteDouble, nil},
				{"float64/fast path", float64(3.14159), nil, nil, bsonrwtest.WriteDouble, nil},
				{"float32/reflection path", myfloat32(3.14159), nil, nil, bsonrwtest.WriteDouble, nil},
				{"float64/reflection path", myfloat64(3.14159), nil, nil, bsonrwtest.WriteDouble, nil},
			},
		},
		{
			"TimeEncodeValue",
			defaultTimeCodec,
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "TimeEncodeValue", Types: []reflect.Type{tTime}, Received: reflect.ValueOf(wrong)},
				},
				{"time.Time", now, nil, nil, bsonrwtest.WriteDateTime, nil},
			},
		},
		{
			"MapEncodeValue",
			defaultMapCodec,
			[]subtest{
				{
					"wrong kind",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "MapEncodeValue", Kinds: []reflect.Kind{reflect.Map}, Received: reflect.ValueOf(wrong)},
				},
				{
					"WriteDocument Error",
					map[string]interface{}{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wd error"), ErrAfter: bsonrwtest.WriteDocument},
					bsonrwtest.WriteDocument,
					errors.New("wd error"),
				},
				{
					"Lookup Error",
					map[string]int{"foo": 1},
					&EncodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteDocument,
					fmt.Errorf("no encoder found for int"),
				},
				{
					"WriteDocumentElement Error",
					map[string]interface{}{"foo": "bar"},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wde error"), ErrAfter: bsonrwtest.WriteDocumentElement},
					bsonrwtest.WriteDocumentElement,
					errors.New("wde error"),
				},
				{
					"EncodeValue Error",
					map[string]interface{}{"foo": "bar"},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("ev error"), ErrAfter: bsonrwtest.WriteString},
					bsonrwtest.WriteString,
					errors.New("ev error"),
				},
				{
					"empty map/success",
					map[string]interface{}{},
					&EncodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"with interface/success",
					map[string]myInterface{"foo": myStruct{1}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"with interface/nil/success",
					map[string]myInterface{"foo": nil},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"non-string key success",
					map[int]interface{}{
						1: "foobar",
					},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
			},
		},
		{
			"ArrayEncodeValue",
			ValueEncoderFunc(dve.ArrayEncodeValue),
			[]subtest{
				{
					"wrong kind",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "ArrayEncodeValue", Kinds: []reflect.Kind{reflect.Array}, Received: reflect.ValueOf(wrong)},
				},
				{
					"WriteArray Error",
					[1]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wa error"), ErrAfter: bsonrwtest.WriteArray},
					bsonrwtest.WriteArray,
					errors.New("wa error"),
				},
				{
					"Lookup Error",
					[1]int{1},
					&EncodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteArray,
					fmt.Errorf("no encoder found for int"),
				},
				{
					"WriteArrayElement Error",
					[1]string{"foo"},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wae error"), ErrAfter: bsonrwtest.WriteArrayElement},
					bsonrwtest.WriteArrayElement,
					errors.New("wae error"),
				},
				{
					"EncodeValue Error",
					[1]string{"foo"},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("ev error"), ErrAfter: bsonrwtest.WriteString},
					bsonrwtest.WriteString,
					errors.New("ev error"),
				},
				{
					"[1]primitive.E/success",
					[1]primitive.E{{"hello", "world"}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"[1]primitive.E/success",
					[1]primitive.E{{"hello", nil}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"[1]interface/success",
					[1]myInterface{myStruct{1}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteArrayEnd,
					nil,
				},
				{
					"[1]interface/nil/success",
					[1]myInterface{nil},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteArrayEnd,
					nil,
				},
			},
		},
		{
			"SliceEncodeValue",
			defaultSliceCodec,
			[]subtest{
				{
					"wrong kind",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "SliceEncodeValue", Kinds: []reflect.Kind{reflect.Slice}, Received: reflect.ValueOf(wrong)},
				},
				{
					"WriteArray Error",
					[]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wa error"), ErrAfter: bsonrwtest.WriteArray},
					bsonrwtest.WriteArray,
					errors.New("wa error"),
				},
				{
					"Lookup Error",
					[]int{1},
					&EncodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteArray,
					fmt.Errorf("no encoder found for int"),
				},
				{
					"WriteArrayElement Error",
					[]string{"foo"},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wae error"), ErrAfter: bsonrwtest.WriteArrayElement},
					bsonrwtest.WriteArrayElement,
					errors.New("wae error"),
				},
				{
					"EncodeValue Error",
					[]string{"foo"},
					&EncodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("ev error"), ErrAfter: bsonrwtest.WriteString},
					bsonrwtest.WriteString,
					errors.New("ev error"),
				},
				{
					"D/success",
					primitive.D{{"hello", "world"}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"D/success",
					primitive.D{{"hello", nil}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"empty slice/success",
					[]interface{}{},
					&EncodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteArrayEnd,
					nil,
				},
				{
					"interface/success",
					[]myInterface{myStruct{1}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteArrayEnd,
					nil,
				},
				{
					"interface/success",
					[]myInterface{nil},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteArrayEnd,
					nil,
				},
			},
		},
		{
			"ObjectIDEncodeValue",
			ValueEncoderFunc(dve.ObjectIDEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "ObjectIDEncodeValue", Types: []reflect.Type{tOID}, Received: reflect.ValueOf(wrong)},
				},
				{
					"primitive.ObjectID/success",
					primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					nil, nil, bsonrwtest.WriteObjectID, nil,
				},
			},
		},
		{
			"Decimal128EncodeValue",
			ValueEncoderFunc(dve.Decimal128EncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "Decimal128EncodeValue", Types: []reflect.Type{tDecimal}, Received: reflect.ValueOf(wrong)},
				},
				{"Decimal128/success", d128, nil, nil, bsonrwtest.WriteDecimal128, nil},
			},
		},
		{
			"JSONNumberEncodeValue",
			ValueEncoderFunc(dve.JSONNumberEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "JSONNumberEncodeValue", Types: []reflect.Type{tJSONNumber}, Received: reflect.ValueOf(wrong)},
				},
				{
					"json.Number/invalid",
					json.Number("hello world"),
					nil, nil, bsonrwtest.Nothing, errors.New(`strconv.ParseFloat: parsing "hello world": invalid syntax`),
				},
				{
					"json.Number/int64/success",
					json.Number("1234567890"),
					nil, nil, bsonrwtest.WriteInt64, nil,
				},
				{
					"json.Number/float64/success",
					json.Number("3.14159"),
					nil, nil, bsonrwtest.WriteDouble, nil,
				},
			},
		},
		{
			"URLEncodeValue",
			ValueEncoderFunc(dve.URLEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "URLEncodeValue", Types: []reflect.Type{tURL}, Received: reflect.ValueOf(wrong)},
				},
				{"url.URL", url.URL{Scheme: "http", Host: "example.com"}, nil, nil, bsonrwtest.WriteString, nil},
			},
		},
		{
			"ByteSliceEncodeValue",
			defaultByteSliceCodec,
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "ByteSliceEncodeValue", Types: []reflect.Type{tByteSlice}, Received: reflect.ValueOf(wrong)},
				},
				{"[]byte", []byte{0x01, 0x02, 0x03}, nil, nil, bsonrwtest.WriteBinary, nil},
				{"[]byte/nil", []byte(nil), nil, nil, bsonrwtest.WriteNull, nil},
			},
		},
		{
			"EmptyInterfaceEncodeValue",
			defaultEmptyInterfaceCodec,
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "EmptyInterfaceEncodeValue", Types: []reflect.Type{tEmpty}, Received: reflect.ValueOf(wrong)},
				},
			},
		},
		{
			"ValueMarshalerEncodeValue",
			ValueEncoderFunc(dve.ValueMarshalerEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "ValueMarshalerEncodeValue",
						Types:    []reflect.Type{tValueMarshaler},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"MarshalBSONValue error",
					testValueMarshaler{err: errors.New("mbsonv error")},
					nil,
					nil,
					bsonrwtest.Nothing,
					errors.New("mbsonv error"),
				},
				{
					"Copy error",
					testValueMarshaler{},
					nil,
					nil,
					bsonrwtest.Nothing,
					fmt.Errorf("Cannot copy unknown BSON type %s", bsontype.Type(0)),
				},
				{
					"success struct implementation",
					testValueMarshaler{t: bsontype.String, buf: []byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00}},
					nil,
					nil,
					bsonrwtest.WriteString,
					nil,
				},
				{
					"success ptr to struct implementation",
					&testValueMarshaler{t: bsontype.String, buf: []byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00}},
					nil,
					nil,
					bsonrwtest.WriteString,
					nil,
				},
				{
					"success nil ptr to struct implementation",
					nilValueMarshaler,
					nil,
					nil,
					bsonrwtest.WriteNull,
					nil,
				},
				{
					"success ptr to ptr implementation",
					&testValueMarshalPtr{t: bsontype.String, buf: []byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00}},
					nil,
					nil,
					bsonrwtest.WriteString,
					nil,
				},
				{
					"unaddressable ptr implementation",
					testValueMarshalPtr{t: bsontype.String, buf: []byte{0x04, 0x00, 0x00, 0x00, 'f', 'o', 'o', 0x00}},
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "ValueMarshalerEncodeValue",
						Types:    []reflect.Type{tValueMarshaler},
						Received: reflect.ValueOf(testValueMarshalPtr{}),
					},
				},
			},
		},
		{
			"MarshalerEncodeValue",
			ValueEncoderFunc(dve.MarshalerEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "MarshalerEncodeValue", Types: []reflect.Type{tMarshaler}, Received: reflect.ValueOf(wrong)},
				},
				{
					"MarshalBSON error",
					testMarshaler{err: errors.New("mbson error")},
					nil,
					nil,
					bsonrwtest.Nothing,
					errors.New("mbson error"),
				},
				{
					"success struct implementation",
					testMarshaler{buf: bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))},
					nil,
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"success ptr to struct implementation",
					&testMarshaler{buf: bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))},
					nil,
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"success nil ptr to struct implementation",
					nilMarshaler,
					nil,
					nil,
					bsonrwtest.WriteNull,
					nil,
				},
				{
					"success ptr to ptr implementation",
					&testMarshalPtr{buf: bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))},
					nil,
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"unaddressable ptr implementation",
					testMarshalPtr{buf: bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))},
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "MarshalerEncodeValue", Types: []reflect.Type{tMarshaler}, Received: reflect.ValueOf(testMarshalPtr{})},
				},
			},
		},
		{
			"ProxyEncodeValue",
			ValueEncoderFunc(dve.ProxyEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "ProxyEncodeValue", Types: []reflect.Type{tProxy}, Received: reflect.ValueOf(wrong)},
				},
				{
					"Proxy error",
					testProxy{err: errors.New("proxy error")},
					nil,
					nil,
					bsonrwtest.Nothing,
					errors.New("proxy error"),
				},
				{
					"Lookup error",
					testProxy{ret: nil},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.Nothing,
					ErrNoEncoder{Type: nil},
				},
				{
					"success struct implementation",
					testProxy{ret: int64(1234567890)},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteInt64,
					nil,
				},
				{
					"success ptr to struct implementation",
					&testProxy{ret: int64(1234567890)},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteInt64,
					nil,
				},
				{
					"success nil ptr to struct implementation",
					nilProxy,
					nil,
					nil,
					bsonrwtest.WriteNull,
					nil,
				},
				{
					"success ptr to ptr implementation",
					&testProxyPtr{ret: int64(1234567890)},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteInt64,
					nil,
				},
				{
					"unaddressable ptr implementation",
					testProxyPtr{ret: int64(1234567890)},
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "ProxyEncodeValue", Types: []reflect.Type{tProxy}, Received: reflect.ValueOf(testProxyPtr{})},
				},
			},
		},
		{
			"PointerCodec.EncodeValue",
			NewPointerCodec(),
			[]subtest{
				{
					"nil",
					nil,
					nil,
					nil,
					bsonrwtest.WriteNull,
					nil,
				},
				{
					"not pointer",
					int32(123456),
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "PointerCodec.EncodeValue", Kinds: []reflect.Kind{reflect.Ptr}, Received: reflect.ValueOf(int32(123456))},
				},
				{
					"typed nil",
					(*int32)(nil),
					nil,
					nil,
					bsonrwtest.WriteNull,
					nil,
				},
				{
					"no encoder",
					&wrong,
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.Nothing,
					ErrNoEncoder{Type: reflect.TypeOf(wrong)},
				},
			},
		},
		{
			"pointer implementation addressable interface",
			NewPointerCodec(),
			[]subtest{
				{
					"ValueMarshaler",
					&vmStruct,
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"Marshaler",
					&mStruct,
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"Proxy",
					&pStruct,
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
			},
		},
		{
			"JavaScriptEncodeValue",
			ValueEncoderFunc(dve.JavaScriptEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "JavaScriptEncodeValue", Types: []reflect.Type{tJavaScript}, Received: reflect.ValueOf(wrong)},
				},
				{"JavaScript", primitive.JavaScript("foobar"), nil, nil, bsonrwtest.WriteJavascript, nil},
			},
		},
		{
			"SymbolEncodeValue",
			ValueEncoderFunc(dve.SymbolEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "SymbolEncodeValue", Types: []reflect.Type{tSymbol}, Received: reflect.ValueOf(wrong)},
				},
				{"Symbol", primitive.Symbol("foobar"), nil, nil, bsonrwtest.WriteSymbol, nil},
			},
		},
		{
			"BinaryEncodeValue",
			ValueEncoderFunc(dve.BinaryEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "BinaryEncodeValue", Types: []reflect.Type{tBinary}, Received: reflect.ValueOf(wrong)},
				},
				{"Binary/success", primitive.Binary{Data: []byte{0x01, 0x02}, Subtype: 0xFF}, nil, nil, bsonrwtest.WriteBinaryWithSubtype, nil},
			},
		},
		{
			"UndefinedEncodeValue",
			ValueEncoderFunc(dve.UndefinedEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "UndefinedEncodeValue", Types: []reflect.Type{tUndefined}, Received: reflect.ValueOf(wrong)},
				},
				{"Undefined/success", primitive.Undefined{}, nil, nil, bsonrwtest.WriteUndefined, nil},
			},
		},
		{
			"DateTimeEncodeValue",
			ValueEncoderFunc(dve.DateTimeEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "DateTimeEncodeValue", Types: []reflect.Type{tDateTime}, Received: reflect.ValueOf(wrong)},
				},
				{"DateTime/success", primitive.DateTime(1234567890), nil, nil, bsonrwtest.WriteDateTime, nil},
			},
		},
		{
			"NullEncodeValue",
			ValueEncoderFunc(dve.NullEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "NullEncodeValue", Types: []reflect.Type{tNull}, Received: reflect.ValueOf(wrong)},
				},
				{"Null/success", primitive.Null{}, nil, nil, bsonrwtest.WriteNull, nil},
			},
		},
		{
			"RegexEncodeValue",
			ValueEncoderFunc(dve.RegexEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "RegexEncodeValue", Types: []reflect.Type{tRegex}, Received: reflect.ValueOf(wrong)},
				},
				{"Regex/success", primitive.Regex{Pattern: "foo", Options: "bar"}, nil, nil, bsonrwtest.WriteRegex, nil},
			},
		},
		{
			"DBPointerEncodeValue",
			ValueEncoderFunc(dve.DBPointerEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "DBPointerEncodeValue", Types: []reflect.Type{tDBPointer}, Received: reflect.ValueOf(wrong)},
				},
				{
					"DBPointer/success",
					primitive.DBPointer{
						DB:      "foobar",
						Pointer: primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					},
					nil, nil, bsonrwtest.WriteDBPointer, nil,
				},
			},
		},
		{
			"TimestampEncodeValue",
			ValueEncoderFunc(dve.TimestampEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "TimestampEncodeValue", Types: []reflect.Type{tTimestamp}, Received: reflect.ValueOf(wrong)},
				},
				{"Timestamp/success", primitive.Timestamp{T: 12345, I: 67890}, nil, nil, bsonrwtest.WriteTimestamp, nil},
			},
		},
		{
			"MinKeyEncodeValue",
			ValueEncoderFunc(dve.MinKeyEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "MinKeyEncodeValue", Types: []reflect.Type{tMinKey}, Received: reflect.ValueOf(wrong)},
				},
				{"MinKey/success", primitive.MinKey{}, nil, nil, bsonrwtest.WriteMinKey, nil},
			},
		},
		{
			"MaxKeyEncodeValue",
			ValueEncoderFunc(dve.MaxKeyEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{Name: "MaxKeyEncodeValue", Types: []reflect.Type{tMaxKey}, Received: reflect.ValueOf(wrong)},
				},
				{"MaxKey/success", primitive.MaxKey{}, nil, nil, bsonrwtest.WriteMaxKey, nil},
			},
		},
		{
			"CoreDocumentEncodeValue",
			ValueEncoderFunc(dve.CoreDocumentEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "CoreDocumentEncodeValue",
						Types:    []reflect.Type{tCoreDocument},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"WriteDocument Error",
					bsoncore.Document{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wd error"), ErrAfter: bsonrwtest.WriteDocument},
					bsonrwtest.WriteDocument,
					errors.New("wd error"),
				},
				{
					"bsoncore.Document.Elements Error",
					bsoncore.Document{0xFF, 0x00, 0x00, 0x00, 0x00},
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteDocument,
					errors.New("length read exceeds number of bytes available. length=5 bytes=255"),
				},
				{
					"WriteDocumentElement Error",
					bsoncore.Document(buildDocument(bsoncore.AppendNullElement(nil, "foo"))),
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wde error"), ErrAfter: bsonrwtest.WriteDocumentElement},
					bsonrwtest.WriteDocumentElement,
					errors.New("wde error"),
				},
				{
					"encodeValue error",
					bsoncore.Document(buildDocument(bsoncore.AppendNullElement(nil, "foo"))),
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("ev error"), ErrAfter: bsonrwtest.WriteNull},
					bsonrwtest.WriteNull,
					errors.New("ev error"),
				},
				{
					"iterator error",
					bsoncore.Document{0x0C, 0x00, 0x00, 0x00, 0x01, 'f', 'o', 'o', 0x00, 0x01, 0x02, 0x03},
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteDocumentElement,
					errors.New("not enough bytes available to read type. bytes=3 type=double"),
				},
			},
		},
		{
			"StructEncodeValue",
			defaultStructCodec,
			[]subtest{
				{
					"interface value",
					struct{ Foo myInterface }{Foo: myStruct{1}},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
				{
					"nil interface value",
					struct{ Foo myInterface }{Foo: nil},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil,
					bsonrwtest.WriteDocumentEnd,
					nil,
				},
			},
		},
		{
			"CodeWithScopeEncodeValue",
			ValueEncoderFunc(dve.CodeWithScopeEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueEncoderError{
						Name:     "CodeWithScopeEncodeValue",
						Types:    []reflect.Type{tCodeWithScope},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"WriteCodeWithScope error",
					primitive.CodeWithScope{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("wcws error"), ErrAfter: bsonrwtest.WriteCodeWithScope},
					bsonrwtest.WriteCodeWithScope,
					errors.New("wcws error"),
				},
				{
					"CodeWithScope/success",
					primitive.CodeWithScope{
						Code:  "var hello = 'world';",
						Scope: primitive.D{},
					},
					&EncodeContext{Registry: buildDefaultRegistry()},
					nil, bsonrwtest.WriteDocumentEnd, nil,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, subtest := range tc.subtests {
				t.Run(subtest.name, func(t *testing.T) {
					var ec EncodeContext
					if subtest.ectx != nil {
						ec = *subtest.ectx
					}
					llvrw := new(bsonrwtest.ValueReaderWriter)
					if subtest.llvrw != nil {
						llvrw = subtest.llvrw
					}
					llvrw.T = t
					err := tc.ve.EncodeValue(ec, llvrw, reflect.ValueOf(subtest.val))
					if !compareErrors(err, subtest.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, subtest.err)
					}
					invoked := llvrw.Invoked
					if !cmp.Equal(invoked, subtest.invoke) {
						t.Errorf("Incorrect method invoked. got %v; want %v", invoked, subtest.invoke)
					}
				})
			}
		})
	}

	t.Run("success path", func(t *testing.T) {
		oid := primitive.NewObjectID()
		oids := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()}
		var str = new(string)
		*str = "bar"
		now := time.Now().Truncate(time.Millisecond)
		murl, err := url.Parse("https://mongodb.com/random-url?hello=world")
		if err != nil {
			t.Errorf("Error parsing URL: %v", err)
			t.FailNow()
		}
		decimal128, err := primitive.ParseDecimal128("1.5e10")
		if err != nil {
			t.Errorf("Error parsing decimal128: %v", err)
			t.FailNow()
		}

		testCases := []struct {
			name  string
			value interface{}
			b     []byte
			err   error
		}{
			{
				"map[string]int",
				map[string]int32{"foo": 1},
				[]byte{
					0x0E, 0x00, 0x00, 0x00,
					0x10, 'f', 'o', 'o', 0x00,
					0x01, 0x00, 0x00, 0x00,
					0x00,
				},
				nil,
			},
			{
				"map[string]primitive.ObjectID",
				map[string]primitive.ObjectID{"foo": oid},
				buildDocument(bsoncore.AppendObjectIDElement(nil, "foo", oid)),
				nil,
			},
			{
				"map[string][]int32",
				map[string][]int32{"Z": {1, 2, 3}},
				buildDocumentArray(func(doc []byte) []byte {
					doc = bsoncore.AppendInt32Element(doc, "0", 1)
					doc = bsoncore.AppendInt32Element(doc, "1", 2)
					return bsoncore.AppendInt32Element(doc, "2", 3)
				}),
				nil,
			},
			{
				"map[string][]primitive.ObjectID",
				map[string][]primitive.ObjectID{"Z": oids},
				buildDocumentArray(func(doc []byte) []byte {
					doc = bsoncore.AppendObjectIDElement(doc, "0", oids[0])
					doc = bsoncore.AppendObjectIDElement(doc, "1", oids[1])
					return bsoncore.AppendObjectIDElement(doc, "2", oids[2])
				}),
				nil,
			},
			{
				"map[string][]json.Number(int64)",
				map[string][]json.Number{"Z": {json.Number("5"), json.Number("10")}},
				buildDocumentArray(func(doc []byte) []byte {
					doc = bsoncore.AppendInt64Element(doc, "0", 5)
					return bsoncore.AppendInt64Element(doc, "1", 10)
				}),
				nil,
			},
			{
				"map[string][]json.Number(float64)",
				map[string][]json.Number{"Z": {json.Number("5"), json.Number("10.1")}},
				buildDocumentArray(func(doc []byte) []byte {
					doc = bsoncore.AppendInt64Element(doc, "0", 5)
					return bsoncore.AppendDoubleElement(doc, "1", 10.1)
				}),
				nil,
			},
			{
				"map[string][]*url.URL",
				map[string][]*url.URL{"Z": {murl}},
				buildDocumentArray(func(doc []byte) []byte {
					return bsoncore.AppendStringElement(doc, "0", murl.String())
				}),
				nil,
			},
			{
				"map[string][]primitive.Decimal128",
				map[string][]primitive.Decimal128{"Z": {decimal128}},
				buildDocumentArray(func(doc []byte) []byte {
					return bsoncore.AppendDecimal128Element(doc, "0", decimal128)
				}),
				nil,
			},
			{
				"-",
				struct {
					A string `bson:"-"`
				}{
					A: "",
				},
				[]byte{0x05, 0x00, 0x00, 0x00, 0x00},
				nil,
			},
			{
				"omitempty",
				struct {
					A string `bson:",omitempty"`
				}{
					A: "",
				},
				[]byte{0x05, 0x00, 0x00, 0x00, 0x00},
				nil,
			},
			{
				"omitempty, empty time",
				struct {
					A time.Time `bson:",omitempty"`
				}{
					A: time.Time{},
				},
				[]byte{0x05, 0x00, 0x00, 0x00, 0x00},
				nil,
			},
			{
				"no private fields",
				noPrivateFields{a: "should be empty"},
				[]byte{0x05, 0x00, 0x00, 0x00, 0x00},
				nil,
			},
			{
				"minsize",
				struct {
					A int64 `bson:",minsize"`
				}{
					A: 12345,
				},
				buildDocument(bsoncore.AppendInt32Element(nil, "a", 12345)),
				nil,
			},
			{
				"inline",
				struct {
					Foo struct {
						A int64 `bson:",minsize"`
					} `bson:",inline"`
				}{
					Foo: struct {
						A int64 `bson:",minsize"`
					}{
						A: 12345,
					},
				},
				buildDocument(bsoncore.AppendInt32Element(nil, "a", 12345)),
				nil,
			},
			{
				"inline struct pointer",
				struct {
					Foo *struct {
						A int64 `bson:",minsize"`
					} `bson:",inline"`
					Bar *struct {
						B int64
					} `bson:",inline"`
				}{
					Foo: &struct {
						A int64 `bson:",minsize"`
					}{
						A: 12345,
					},
					Bar: nil,
				},
				buildDocument(bsoncore.AppendInt32Element(nil, "a", 12345)),
				nil,
			},
			{
				"nested inline struct pointer",
				struct {
					Foo *struct {
						Bar *struct {
							A int64 `bson:",minsize"`
						} `bson:",inline"`
					} `bson:",inline"`
				}{
					Foo: &struct {
						Bar *struct {
							A int64 `bson:",minsize"`
						} `bson:",inline"`
					}{
						Bar: &struct {
							A int64 `bson:",minsize"`
						}{
							A: 12345,
						},
					},
				},
				buildDocument(bsoncore.AppendInt32Element(nil, "a", 12345)),
				nil,
			},
			{
				"inline nil struct pointer",
				struct {
					Foo *struct {
						A int64 `bson:",minsize"`
					} `bson:",inline"`
				}{
					Foo: nil,
				},
				buildDocument([]byte{}),
				nil,
			},
			{
				"inline map",
				struct {
					Foo map[string]string `bson:",inline"`
				}{
					Foo: map[string]string{"foo": "bar"},
				},
				buildDocument(bsoncore.AppendStringElement(nil, "foo", "bar")),
				nil,
			},
			{
				"alternate name bson:name",
				struct {
					A string `bson:"foo"`
				}{
					A: "bar",
				},
				buildDocument(bsoncore.AppendStringElement(nil, "foo", "bar")),
				nil,
			},
			{
				"alternate name",
				struct {
					A string `bson:"foo"`
				}{
					A: "bar",
				},
				buildDocument(bsoncore.AppendStringElement(nil, "foo", "bar")),
				nil,
			},
			{
				"inline, omitempty",
				struct {
					A   string
					Foo zeroTest `bson:"omitempty,inline"`
				}{
					A:   "bar",
					Foo: zeroTest{true},
				},
				buildDocument(bsoncore.AppendStringElement(nil, "a", "bar")),
				nil,
			},
			{
				"struct{}",
				struct {
					A bool
					B int32
					C int64
					D uint16
					E uint64
					F float64
					G string
					H map[string]string
					I []byte
					K [2]string
					L struct {
						M string
					}
					Q  primitive.ObjectID
					T  []struct{}
					Y  json.Number
					Z  time.Time
					AA json.Number
					AB *url.URL
					AC primitive.Decimal128
					AD *time.Time
					AE testValueMarshaler
					AF Proxy
					AG testProxy
					AH map[string]interface{}
					AI primitive.CodeWithScope
				}{
					A: true,
					B: 123,
					C: 456,
					D: 789,
					E: 101112,
					F: 3.14159,
					G: "Hello, world",
					H: map[string]string{"foo": "bar"},
					I: []byte{0x01, 0x02, 0x03},
					K: [2]string{"baz", "qux"},
					L: struct {
						M string
					}{
						M: "foobar",
					},
					Q:  oid,
					T:  nil,
					Y:  json.Number("5"),
					Z:  now,
					AA: json.Number("10.1"),
					AB: murl,
					AC: decimal128,
					AD: &now,
					AE: testValueMarshaler{t: bsontype.String, buf: bsoncore.AppendString(nil, "hello, world")},
					AF: testProxy{ret: struct{ Hello string }{Hello: "world!"}},
					AG: testProxy{ret: struct{ Pi float64 }{Pi: 3.14159}},
					AH: nil,
					AI: primitive.CodeWithScope{Code: "var hello = 'world';", Scope: primitive.D{{"pi", 3.14159}}},
				},
				buildDocument(func(doc []byte) []byte {
					doc = bsoncore.AppendBooleanElement(doc, "a", true)
					doc = bsoncore.AppendInt32Element(doc, "b", 123)
					doc = bsoncore.AppendInt64Element(doc, "c", 456)
					doc = bsoncore.AppendInt32Element(doc, "d", 789)
					doc = bsoncore.AppendInt64Element(doc, "e", 101112)
					doc = bsoncore.AppendDoubleElement(doc, "f", 3.14159)
					doc = bsoncore.AppendStringElement(doc, "g", "Hello, world")
					doc = bsoncore.AppendDocumentElement(doc, "h", buildDocument(bsoncore.AppendStringElement(nil, "foo", "bar")))
					doc = bsoncore.AppendBinaryElement(doc, "i", 0x00, []byte{0x01, 0x02, 0x03})
					doc = bsoncore.AppendArrayElement(doc, "k",
						buildArray(bsoncore.AppendStringElement(bsoncore.AppendStringElement(nil, "0", "baz"), "1", "qux")),
					)
					doc = bsoncore.AppendDocumentElement(doc, "l", buildDocument(bsoncore.AppendStringElement(nil, "m", "foobar")))
					doc = bsoncore.AppendObjectIDElement(doc, "q", oid)
					doc = bsoncore.AppendNullElement(doc, "t")
					doc = bsoncore.AppendInt64Element(doc, "y", 5)
					doc = bsoncore.AppendDateTimeElement(doc, "z", now.UnixNano()/int64(time.Millisecond))
					doc = bsoncore.AppendDoubleElement(doc, "aa", 10.1)
					doc = bsoncore.AppendStringElement(doc, "ab", murl.String())
					doc = bsoncore.AppendDecimal128Element(doc, "ac", decimal128)
					doc = bsoncore.AppendDateTimeElement(doc, "ad", now.UnixNano()/int64(time.Millisecond))
					doc = bsoncore.AppendStringElement(doc, "ae", "hello, world")
					doc = bsoncore.AppendDocumentElement(doc, "af", buildDocument(bsoncore.AppendStringElement(nil, "hello", "world!")))
					doc = bsoncore.AppendDocumentElement(doc, "ag", buildDocument(bsoncore.AppendDoubleElement(nil, "pi", 3.14159)))
					doc = bsoncore.AppendNullElement(doc, "ah")
					doc = bsoncore.AppendCodeWithScopeElement(doc, "ai",
						"var hello = 'world';", buildDocument(bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
					)
					return doc
				}(nil)),
				nil,
			},
			{
				"struct{[]interface{}}",
				struct {
					A []bool
					B []int32
					C []int64
					D []uint16
					E []uint64
					F []float64
					G []string
					H []map[string]string
					I [][]byte
					K [1][2]string
					L []struct {
						M string
					}
					N  [][]string
					R  []primitive.ObjectID
					T  []struct{}
					W  []map[string]struct{}
					X  []map[string]struct{}
					Y  []map[string]struct{}
					Z  []time.Time
					AA []json.Number
					AB []*url.URL
					AC []primitive.Decimal128
					AD []*time.Time
					AE []testValueMarshaler
					AF []Proxy
					AG []testProxy
				}{
					A: []bool{true},
					B: []int32{123},
					C: []int64{456},
					D: []uint16{789},
					E: []uint64{101112},
					F: []float64{3.14159},
					G: []string{"Hello, world"},
					H: []map[string]string{{"foo": "bar"}},
					I: [][]byte{{0x01, 0x02, 0x03}},
					K: [1][2]string{{"baz", "qux"}},
					L: []struct {
						M string
					}{
						{
							M: "foobar",
						},
					},
					N:  [][]string{{"foo", "bar"}},
					R:  oids,
					T:  nil,
					W:  nil,
					X:  []map[string]struct{}{},   // Should be empty BSON Array
					Y:  []map[string]struct{}{{}}, // Should be BSON array with one element, an empty BSON SubDocument
					Z:  []time.Time{now, now},
					AA: []json.Number{json.Number("5"), json.Number("10.1")},
					AB: []*url.URL{murl},
					AC: []primitive.Decimal128{decimal128},
					AD: []*time.Time{&now, &now},
					AE: []testValueMarshaler{
						{t: bsontype.String, buf: bsoncore.AppendString(nil, "hello")},
						{t: bsontype.String, buf: bsoncore.AppendString(nil, "world")},
					},
					AF: []Proxy{
						testProxy{ret: struct{ Hello string }{Hello: "world!"}},
						testProxy{ret: struct{ Foo string }{Foo: "bar"}},
					},
					AG: []testProxy{
						{ret: struct{ One int64 }{One: 1234567890}},
						{ret: struct{ Pi float64 }{Pi: 3.14159}},
					},
				},
				buildDocument(func(doc []byte) []byte {
					doc = appendArrayElement(doc, "a", bsoncore.AppendBooleanElement(nil, "0", true))
					doc = appendArrayElement(doc, "b", bsoncore.AppendInt32Element(nil, "0", 123))
					doc = appendArrayElement(doc, "c", bsoncore.AppendInt64Element(nil, "0", 456))
					doc = appendArrayElement(doc, "d", bsoncore.AppendInt32Element(nil, "0", 789))
					doc = appendArrayElement(doc, "e", bsoncore.AppendInt64Element(nil, "0", 101112))
					doc = appendArrayElement(doc, "f", bsoncore.AppendDoubleElement(nil, "0", 3.14159))
					doc = appendArrayElement(doc, "g", bsoncore.AppendStringElement(nil, "0", "Hello, world"))
					doc = appendArrayElement(doc, "h", buildDocumentElement("0", bsoncore.AppendStringElement(nil, "foo", "bar")))
					doc = appendArrayElement(doc, "i", bsoncore.AppendBinaryElement(nil, "0", 0x00, []byte{0x01, 0x02, 0x03}))
					doc = appendArrayElement(doc, "k",
						buildArrayElement("0",
							bsoncore.AppendStringElement(bsoncore.AppendStringElement(nil, "0", "baz"), "1", "qux")),
					)
					doc = appendArrayElement(doc, "l", buildDocumentElement("0", bsoncore.AppendStringElement(nil, "m", "foobar")))
					doc = appendArrayElement(doc, "n",
						buildArrayElement("0",
							bsoncore.AppendStringElement(bsoncore.AppendStringElement(nil, "0", "foo"), "1", "bar")),
					)
					doc = appendArrayElement(doc, "r",
						bsoncore.AppendObjectIDElement(
							bsoncore.AppendObjectIDElement(
								bsoncore.AppendObjectIDElement(nil,
									"0", oids[0]),
								"1", oids[1]),
							"2", oids[2]),
					)
					doc = bsoncore.AppendNullElement(doc, "t")
					doc = bsoncore.AppendNullElement(doc, "w")
					doc = appendArrayElement(doc, "x", nil)
					doc = appendArrayElement(doc, "y", buildDocumentElement("0", nil))
					doc = appendArrayElement(doc, "z",
						bsoncore.AppendDateTimeElement(
							bsoncore.AppendDateTimeElement(
								nil, "0", now.UnixNano()/int64(time.Millisecond)),
							"1", now.UnixNano()/int64(time.Millisecond)),
					)
					doc = appendArrayElement(doc, "aa", bsoncore.AppendDoubleElement(bsoncore.AppendInt64Element(nil, "0", 5), "1", 10.10))
					doc = appendArrayElement(doc, "ab", bsoncore.AppendStringElement(nil, "0", murl.String()))
					doc = appendArrayElement(doc, "ac", bsoncore.AppendDecimal128Element(nil, "0", decimal128))
					doc = appendArrayElement(doc, "ad",
						bsoncore.AppendDateTimeElement(
							bsoncore.AppendDateTimeElement(nil, "0", now.UnixNano()/int64(time.Millisecond)),
							"1", now.UnixNano()/int64(time.Millisecond)),
					)
					doc = appendArrayElement(doc, "ae",
						bsoncore.AppendStringElement(bsoncore.AppendStringElement(nil, "0", "hello"), "1", "world"),
					)
					doc = appendArrayElement(doc, "af",
						bsoncore.AppendDocumentElement(
							bsoncore.AppendDocumentElement(nil, "0",
								bsoncore.BuildDocument(nil, bsoncore.AppendStringElement(nil, "hello", "world!")),
							), "1",
							bsoncore.BuildDocument(nil, bsoncore.AppendStringElement(nil, "foo", "bar")),
						),
					)
					doc = appendArrayElement(doc, "ag",
						bsoncore.AppendDocumentElement(
							bsoncore.AppendDocumentElement(nil, "0",
								bsoncore.BuildDocument(nil, bsoncore.AppendInt64Element(nil, "one", 1234567890)),
							), "1",
							bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
						),
					)
					return doc
				}(nil)),
				nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				b := make(bsonrw.SliceWriter, 0, 512)
				vw, err := bsonrw.NewBSONValueWriter(&b)
				noerr(t, err)
				reg := buildDefaultRegistry()
				enc, err := reg.LookupEncoder(reflect.TypeOf(tc.value))
				noerr(t, err)
				err = enc.EncodeValue(EncodeContext{Registry: reg}, vw, reflect.ValueOf(tc.value))
				if err != tc.err {
					t.Errorf("Did not receive expected error. got %v; want %v", err, tc.err)
				}
				if diff := cmp.Diff([]byte(b), tc.b); diff != "" {
					t.Errorf("Bytes written differ: (-got +want)\n%s", diff)
					t.Errorf("Bytes\ngot: %v\nwant:%v\n", b, tc.b)
					t.Errorf("Readers\ngot: %v\nwant:%v\n", bsoncore.Document(b), bsoncore.Document(tc.b))
				}
			})
		}
	})

	t.Run("EmptyInterfaceEncodeValue/nil", func(t *testing.T) {
		val := reflect.New(tEmpty).Elem()
		llvrw := new(bsonrwtest.ValueReaderWriter)
		err := dve.EmptyInterfaceEncodeValue(EncodeContext{Registry: NewRegistryBuilder().Build()}, llvrw, val)
		noerr(t, err)
		if llvrw.Invoked != bsonrwtest.WriteNull {
			t.Errorf("Incorrect method called. got %v; want %v", llvrw.Invoked, bsonrwtest.WriteNull)
		}
	})

	t.Run("EmptyInterfaceEncodeValue/LookupEncoder error", func(t *testing.T) {
		val := reflect.New(tEmpty).Elem()
		val.Set(reflect.ValueOf(int64(1234567890)))
		llvrw := new(bsonrwtest.ValueReaderWriter)
		got := dve.EmptyInterfaceEncodeValue(EncodeContext{Registry: NewRegistryBuilder().Build()}, llvrw, val)
		want := ErrNoEncoder{Type: tInt64}
		if !compareErrors(got, want) {
			t.Errorf("Did not recieve expected error. got %v; want %v", got, want)
		}
	})
}

type testValueMarshaler struct {
	t   bsontype.Type
	buf []byte
	err error
}

func (tvm testValueMarshaler) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return tvm.t, tvm.buf, tvm.err
}

type testValueMarshalPtr struct {
	t   bsontype.Type
	buf []byte
	err error
}

func (tvm *testValueMarshalPtr) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return tvm.t, tvm.buf, tvm.err
}

type testMarshaler struct {
	buf []byte
	err error
}

func (tvm testMarshaler) MarshalBSON() ([]byte, error) {
	return tvm.buf, tvm.err
}

type testMarshalPtr struct {
	buf []byte
	err error
}

func (tvm *testMarshalPtr) MarshalBSON() ([]byte, error) {
	return tvm.buf, tvm.err
}

type testProxy struct {
	ret interface{}
	err error
}

func (tp testProxy) ProxyBSON() (interface{}, error) { return tp.ret, tp.err }

type testProxyPtr struct {
	ret interface{}
	err error
}

func (tp *testProxyPtr) ProxyBSON() (interface{}, error) { return tp.ret, tp.err }
