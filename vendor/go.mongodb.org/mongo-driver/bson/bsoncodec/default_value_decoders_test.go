// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"bytes"
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

func TestDefaultValueDecoders(t *testing.T) {
	var dvd DefaultValueDecoders
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
	type mystruct struct{}

	const cansetreflectiontest = "cansetreflectiontest"
	const cansettest = "cansettest"

	now := time.Now().Truncate(time.Millisecond)
	d128 := primitive.NewDecimal128(12345, 67890)
	var pbool = func(b bool) *bool { return &b }
	var pi32 = func(i32 int32) *int32 { return &i32 }
	var pi64 = func(i64 int64) *int64 { return &i64 }

	type subtest struct {
		name   string
		val    interface{}
		dctx   *DecodeContext
		llvrw  *bsonrwtest.ValueReaderWriter
		invoke bsonrwtest.Invoked
		err    error
	}

	testCases := []struct {
		name     string
		vd       ValueDecoder
		subtests []subtest
	}{
		{
			"BooleanDecodeValue",
			ValueDecoderFunc(dvd.BooleanDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Boolean},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "BooleanDecodeValue", Kinds: []reflect.Kind{reflect.Bool}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not boolean",
					bool(false),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a boolean", bsontype.String),
				},
				{
					"fast path",
					bool(true),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Boolean, Return: bool(true)},
					bsonrwtest.ReadBoolean,
					nil,
				},
				{
					"reflection path",
					mybool(true),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Boolean, Return: bool(true)},
					bsonrwtest.ReadBoolean,
					nil,
				},
				{
					"reflection path error",
					mybool(true),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Boolean, Return: bool(true), Err: errors.New("ReadBoolean Error"), ErrAfter: bsonrwtest.ReadBoolean},
					bsonrwtest.ReadBoolean, errors.New("ReadBoolean Error"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Boolean},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "BooleanDecodeValue", Kinds: []reflect.Kind{reflect.Bool}},
				},
				{
					"decode null",
					mybool(false),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"IntDecodeValue",
			ValueDecoderFunc(dvd.IntDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)},
					bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "IntDecodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"type not int32/int64",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into an integer type", bsontype.String),
				},
				{
					"ReadInt32 error",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0), Err: errors.New("ReadInt32 error"), ErrAfter: bsonrwtest.ReadInt32},
					bsonrwtest.ReadInt32,
					errors.New("ReadInt32 error"),
				},
				{
					"ReadInt64 error",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(0), Err: errors.New("ReadInt64 error"), ErrAfter: bsonrwtest.ReadInt64},
					bsonrwtest.ReadInt64,
					errors.New("ReadInt64 error"),
				},
				{
					"ReadDouble error",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0), Err: errors.New("ReadDouble error"), ErrAfter: bsonrwtest.ReadDouble},
					bsonrwtest.ReadDouble,
					errors.New("ReadDouble error"),
				},
				{
					"ReadDouble", int64(3), &DecodeContext{},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.00)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"ReadDouble (truncate)", int64(3), &DecodeContext{Truncate: true},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"ReadDouble (no truncate)", int64(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					errors.New("IntDecodeValue can only truncate float64 to an integer type when truncation is enabled"),
				},
				{
					"ReadDouble overflows int64", int64(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: math.MaxFloat64}, bsonrwtest.ReadDouble,
					fmt.Errorf("%g overflows int64", math.MaxFloat64),
				},
				{"int8/fast path", int8(127), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(127)}, bsonrwtest.ReadInt32, nil},
				{"int16/fast path", int16(32676), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(32676)}, bsonrwtest.ReadInt32, nil},
				{"int32/fast path", int32(1234), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1234)}, bsonrwtest.ReadInt32, nil},
				{"int64/fast path", int64(1234), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1234)}, bsonrwtest.ReadInt64, nil},
				{"int/fast path", int(1234), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1234)}, bsonrwtest.ReadInt64, nil},
				{
					"int8/fast path - nil", (*int8)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "IntDecodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf((*int8)(nil)),
					},
				},
				{
					"int16/fast path - nil", (*int16)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "IntDecodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf((*int16)(nil)),
					},
				},
				{
					"int32/fast path - nil", (*int32)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "IntDecodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf((*int32)(nil)),
					},
				},
				{
					"int64/fast path - nil", (*int64)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "IntDecodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf((*int64)(nil)),
					},
				},
				{
					"int/fast path - nil", (*int)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "IntDecodeValue",
						Kinds:    []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
						Received: reflect.ValueOf((*int)(nil)),
					},
				},
				{
					"int8/fast path - overflow", int8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(129)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int8", 129),
				},
				{
					"int16/fast path - overflow", int16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(32768)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int16", 32768),
				},
				{
					"int32/fast path - overflow", int32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(2147483648)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows int32", 2147483648),
				},
				{
					"int8/fast path - overflow (negative)", int8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-129)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int8", -129),
				},
				{
					"int16/fast path - overflow (negative)", int16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-32769)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int16", -32769),
				},
				{
					"int32/fast path - overflow (negative)", int32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-2147483649)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows int32", -2147483649),
				},
				{
					"int8/reflection path", myint8(127), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(127)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"int16/reflection path", myint16(255), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(255)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"int32/reflection path", myint32(511), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(511)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"int64/reflection path", myint64(1023), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1023)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"int/reflection path", myint(2047), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(2047)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"int8/reflection path - overflow", myint8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(129)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int8", 129),
				},
				{
					"int16/reflection path - overflow", myint16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(32768)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int16", 32768),
				},
				{
					"int32/reflection path - overflow", myint32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(2147483648)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows int32", 2147483648),
				},
				{
					"int8/reflection path - overflow (negative)", myint8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-129)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int8", -129),
				},
				{
					"int16/reflection path - overflow (negative)", myint16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-32769)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows int16", -32769),
				},
				{
					"int32/reflection path - overflow (negative)", myint32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-2147483649)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows int32", -2147483649),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)},
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:  "IntDecodeValue",
						Kinds: []reflect.Kind{reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int},
					},
				},
				{
					"decode null",
					myint(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"defaultUIntCodec.DecodeValue",
			defaultUIntCodec,
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)},
					bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "UintDecodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"type not int32/int64",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into an integer type", bsontype.String),
				},
				{
					"ReadInt32 error",
					uint(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0), Err: errors.New("ReadInt32 error"), ErrAfter: bsonrwtest.ReadInt32},
					bsonrwtest.ReadInt32,
					errors.New("ReadInt32 error"),
				},
				{
					"ReadInt64 error",
					uint(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(0), Err: errors.New("ReadInt64 error"), ErrAfter: bsonrwtest.ReadInt64},
					bsonrwtest.ReadInt64,
					errors.New("ReadInt64 error"),
				},
				{
					"ReadDouble error",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0), Err: errors.New("ReadDouble error"), ErrAfter: bsonrwtest.ReadDouble},
					bsonrwtest.ReadDouble,
					errors.New("ReadDouble error"),
				},
				{
					"ReadDouble", uint64(3), &DecodeContext{},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.00)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"ReadDouble (truncate)", uint64(3), &DecodeContext{Truncate: true},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"ReadDouble (no truncate)", uint64(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					errors.New("UintDecodeValue can only truncate float64 to an integer type when truncation is enabled"),
				},
				{
					"ReadDouble overflows int64", uint64(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: math.MaxFloat64}, bsonrwtest.ReadDouble,
					fmt.Errorf("%g overflows int64", math.MaxFloat64),
				},
				{"uint8/fast path", uint8(127), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(127)}, bsonrwtest.ReadInt32, nil},
				{"uint16/fast path", uint16(255), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(255)}, bsonrwtest.ReadInt32, nil},
				{"uint32/fast path", uint32(1234), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1234)}, bsonrwtest.ReadInt32, nil},
				{"uint64/fast path", uint64(1234), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1234)}, bsonrwtest.ReadInt64, nil},
				{"uint/fast path", uint(1234), nil, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1234)}, bsonrwtest.ReadInt64, nil},
				{
					"uint8/fast path - nil", (*uint8)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "UintDecodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf((*uint8)(nil)),
					},
				},
				{
					"uint16/fast path - nil", (*uint16)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "UintDecodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf((*uint16)(nil)),
					},
				},
				{
					"uint32/fast path - nil", (*uint32)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "UintDecodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf((*uint32)(nil)),
					},
				},
				{
					"uint64/fast path - nil", (*uint64)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "UintDecodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf((*uint64)(nil)),
					},
				},
				{
					"uint/fast path - nil", (*uint)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)}, bsonrwtest.ReadInt32,
					ValueDecoderError{
						Name:     "UintDecodeValue",
						Kinds:    []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
						Received: reflect.ValueOf((*uint)(nil)),
					},
				},
				{
					"uint8/fast path - overflow", uint8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1 << 8)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint8", 1<<8),
				},
				{
					"uint16/fast path - overflow", uint16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1 << 16)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint16", 1<<16),
				},
				{
					"uint32/fast path - overflow", uint32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1 << 32)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint32", 1<<32),
				},
				{
					"uint8/fast path - overflow (negative)", uint8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-1)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint8", -1),
				},
				{
					"uint16/fast path - overflow (negative)", uint16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-1)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint16", -1),
				},
				{
					"uint32/fast path - overflow (negative)", uint32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-1)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint32", -1),
				},
				{
					"uint64/fast path - overflow (negative)", uint64(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-1)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint64", -1),
				},
				{
					"uint/fast path - overflow (negative)", uint(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-1)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint", -1),
				},
				{
					"uint8/reflection path", myuint8(127), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(127)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"uint16/reflection path", myuint16(255), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(255)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"uint32/reflection path", myuint32(511), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(511)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"uint64/reflection path", myuint64(1023), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1023)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"uint/reflection path", myuint(2047), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(2047)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"uint8/reflection path - overflow", myuint8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1 << 8)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint8", 1<<8),
				},
				{
					"uint16/reflection path - overflow", myuint16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(1 << 16)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint16", 1<<16),
				},
				{
					"uint32/reflection path - overflow", myuint32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1 << 32)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint32", 1<<32),
				},
				{
					"uint8/reflection path - overflow (negative)", myuint8(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-1)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint8", -1),
				},
				{
					"uint16/reflection path - overflow (negative)", myuint16(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(-1)}, bsonrwtest.ReadInt32,
					fmt.Errorf("%d overflows uint16", -1),
				},
				{
					"uint32/reflection path - overflow (negative)", myuint32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-1)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint32", -1),
				},
				{
					"uint64/reflection path - overflow (negative)", myuint64(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-1)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint64", -1),
				},
				{
					"uint/reflection path - overflow (negative)", myuint(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(-1)}, bsonrwtest.ReadInt64,
					fmt.Errorf("%d overflows uint", -1),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0)},
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:  "UintDecodeValue",
						Kinds: []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint},
					},
				},
			},
		},
		{
			"FloatDecodeValue",
			ValueDecoderFunc(dvd.FloatDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0)},
					bsonrwtest.ReadDouble,
					ValueDecoderError{
						Name:     "FloatDecodeValue",
						Kinds:    []reflect.Kind{reflect.Float32, reflect.Float64},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"type not double",
					0,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a float32 or float64 type", bsontype.String),
				},
				{
					"ReadDouble error",
					float64(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0), Err: errors.New("ReadDouble error"), ErrAfter: bsonrwtest.ReadDouble},
					bsonrwtest.ReadDouble,
					errors.New("ReadDouble error"),
				},
				{
					"ReadInt32 error",
					float64(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(0), Err: errors.New("ReadInt32 error"), ErrAfter: bsonrwtest.ReadInt32},
					bsonrwtest.ReadInt32,
					errors.New("ReadInt32 error"),
				},
				{
					"ReadInt64 error",
					float64(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(0), Err: errors.New("ReadInt64 error"), ErrAfter: bsonrwtest.ReadInt64},
					bsonrwtest.ReadInt64,
					errors.New("ReadInt64 error"),
				},
				{
					"float64/int32", float32(32.0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(32)}, bsonrwtest.ReadInt32,
					nil,
				},
				{
					"float64/int64", float32(64.0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(64)}, bsonrwtest.ReadInt64,
					nil,
				},
				{
					"float32/fast path (equal)", float32(3.0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.0)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"float64/fast path", float64(3.14159), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14159)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"float32/fast path (truncate)", float32(3.14), &DecodeContext{Truncate: true},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"float32/fast path (no truncate)", float32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					errors.New("FloatDecodeValue can only convert float64 to float32 when truncation is allowed"),
				},
				{
					"float32/fast path - nil", (*float32)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0)}, bsonrwtest.ReadDouble,
					ValueDecoderError{
						Name:     "FloatDecodeValue",
						Kinds:    []reflect.Kind{reflect.Float32, reflect.Float64},
						Received: reflect.ValueOf((*float32)(nil)),
					},
				},
				{
					"float64/fast path - nil", (*float64)(nil), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0)}, bsonrwtest.ReadDouble,
					ValueDecoderError{
						Name:     "FloatDecodeValue",
						Kinds:    []reflect.Kind{reflect.Float32, reflect.Float64},
						Received: reflect.ValueOf((*float64)(nil)),
					},
				},
				{
					"float32/reflection path (equal)", myfloat32(3.0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.0)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"float64/reflection path", myfloat64(3.14159), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14159)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"float32/reflection path (truncate)", myfloat32(3.14), &DecodeContext{Truncate: true},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					nil,
				},
				{
					"float32/reflection path (no truncate)", myfloat32(0), nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14)}, bsonrwtest.ReadDouble,
					errors.New("FloatDecodeValue can only convert float64 to float32 when truncation is allowed"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(0)},
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:  "FloatDecodeValue",
						Kinds: []reflect.Kind{reflect.Float32, reflect.Float64},
					},
				},
			},
		},
		{
			"defaultTimeCodec.DecodeValue",
			defaultTimeCodec,
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Return: int64(0)},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "TimeDecodeValue", Types: []reflect.Type{tTime}, Received: reflect.ValueOf(wrong)},
				},
				{
					"ReadDateTime error",
					time.Time{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Return: int64(0), Err: errors.New("ReadDateTime error"), ErrAfter: bsonrwtest.ReadDateTime},
					bsonrwtest.ReadDateTime,
					errors.New("ReadDateTime error"),
				},
				{
					"time.Time",
					now,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Return: int64(now.UnixNano() / int64(time.Millisecond))},
					bsonrwtest.ReadDateTime,
					nil,
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Return: int64(0)},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "TimeDecodeValue", Types: []reflect.Type{tTime}},
				},
				{
					"decode null",
					time.Time{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"defaultMapCodec.DecodeValue",
			defaultMapCodec,
			[]subtest{
				{
					"wrong kind",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "MapDecodeValue", Kinds: []reflect.Kind{reflect.Map}, Received: reflect.ValueOf(wrong)},
				},
				{
					"wrong kind (non-string key)",
					map[bool]interface{}{},
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.ReadElement,
					fmt.Errorf("BSON map must have string or decimal keys. Got:%v", reflect.ValueOf(map[bool]interface{}{}).Type()),
				},
				{
					"ReadDocument Error",
					make(map[string]interface{}),
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("rd error"), ErrAfter: bsonrwtest.ReadDocument},
					bsonrwtest.ReadDocument,
					errors.New("rd error"),
				},
				{
					"Lookup Error",
					map[string]string{},
					&DecodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.ReadDocument,
					ErrNoDecoder{Type: reflect.TypeOf(string(""))},
				},
				{
					"ReadElement Error",
					make(map[string]interface{}),
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("re error"), ErrAfter: bsonrwtest.ReadElement},
					bsonrwtest.ReadElement,
					errors.New("re error"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "MapDecodeValue", Kinds: []reflect.Kind{reflect.Map}},
				},
				{
					"wrong BSON type",
					map[string]interface{}{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					errors.New("cannot decode string into a map[string]interface {}"),
				},
				{
					"decode null",
					(map[string]interface{})(nil),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"ArrayDecodeValue",
			ValueDecoderFunc(dvd.ArrayDecodeValue),
			[]subtest{
				{
					"wrong kind",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "ArrayDecodeValue", Kinds: []reflect.Kind{reflect.Array}, Received: reflect.ValueOf(wrong)},
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "ArrayDecodeValue", Kinds: []reflect.Kind{reflect.Array}},
				},
				{
					"Not Type Array",
					[1]interface{}{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					errors.New("cannot decode string into an array"),
				},
				{
					"ReadArray Error",
					[1]interface{}{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("ra error"), ErrAfter: bsonrwtest.ReadArray, BSONType: bsontype.Array},
					bsonrwtest.ReadArray,
					errors.New("ra error"),
				},
				{
					"Lookup Error",
					[1]string{},
					&DecodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Array},
					bsonrwtest.ReadArray,
					ErrNoDecoder{Type: reflect.TypeOf(string(""))},
				},
				{
					"ReadValue Error",
					[1]string{},
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("rv error"), ErrAfter: bsonrwtest.ReadValue, BSONType: bsontype.Array},
					bsonrwtest.ReadValue,
					errors.New("rv error"),
				},
				{
					"DecodeValue Error",
					[1]string{},
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Array},
					bsonrwtest.ReadValue,
					errors.New("cannot decode array into a string type"),
				},
				{
					"Document but not D",
					[1]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Type(0)},
					bsonrwtest.Nothing,
					errors.New("cannot decode document into [1]string"),
				},
				{
					"EmbeddedDocument but not D",
					[1]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.EmbeddedDocument},
					bsonrwtest.Nothing,
					errors.New("cannot decode document into [1]string"),
				},
				{
					"decode null",
					[1]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"defaultSliceCodec.DecodeValue",
			defaultSliceCodec,
			[]subtest{
				{
					"wrong kind",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "SliceDecodeValue", Kinds: []reflect.Kind{reflect.Slice}, Received: reflect.ValueOf(wrong)},
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "SliceDecodeValue", Kinds: []reflect.Kind{reflect.Slice}},
				},
				{
					"Not Type Array",
					[]interface{}{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32},
					bsonrwtest.Nothing,
					errors.New("cannot decode 32-bit integer into a slice"),
				},
				{
					"ReadArray Error",
					[]interface{}{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("ra error"), ErrAfter: bsonrwtest.ReadArray, BSONType: bsontype.Array},
					bsonrwtest.ReadArray,
					errors.New("ra error"),
				},
				{
					"Lookup Error",
					[]string{},
					&DecodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Array},
					bsonrwtest.ReadArray,
					ErrNoDecoder{Type: reflect.TypeOf(string(""))},
				},
				{
					"ReadValue Error",
					[]string{},
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{Err: errors.New("rv error"), ErrAfter: bsonrwtest.ReadValue, BSONType: bsontype.Array},
					bsonrwtest.ReadValue,
					errors.New("rv error"),
				},
				{
					"DecodeValue Error",
					[]string{},
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Array},
					bsonrwtest.ReadValue,
					errors.New("cannot decode array into a string type"),
				},
				{
					"Document but not D",
					[]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Type(0)},
					bsonrwtest.Nothing,
					errors.New("cannot decode document into []string"),
				},
				{
					"EmbeddedDocument but not D",
					[]string{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.EmbeddedDocument},
					bsonrwtest.Nothing,
					errors.New("cannot decode document into []string"),
				},
				{
					"decode null",
					([]string)(nil),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"ObjectIDDecodeValue",
			ValueDecoderFunc(dvd.ObjectIDDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.ObjectID},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "ObjectIDDecodeValue", Types: []reflect.Type{tOID}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not objectID",
					primitive.ObjectID{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into an ObjectID", bsontype.Int32),
				},
				{
					"ReadObjectID Error",
					primitive.ObjectID{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.ObjectID, Err: errors.New("roid error"), ErrAfter: bsonrwtest.ReadObjectID},
					bsonrwtest.ReadObjectID,
					errors.New("roid error"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.ObjectID, Return: primitive.ObjectID{}},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "ObjectIDDecodeValue", Types: []reflect.Type{tOID}},
				},
				{
					"success",
					primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.ObjectID,
						Return:   primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					},
					bsonrwtest.ReadObjectID,
					nil,
				},
				{
					"success/string",
					primitive.ObjectID{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x61, 0x62},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.String,
						Return:   "0123456789ab",
					},
					bsonrwtest.ReadString,
					nil,
				},
				{
					"decode null",
					primitive.ObjectID{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"Decimal128DecodeValue",
			ValueDecoderFunc(dvd.Decimal128DecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Decimal128},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "Decimal128DecodeValue", Types: []reflect.Type{tDecimal}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not decimal128",
					primitive.Decimal128{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a primitive.Decimal128", bsontype.String),
				},
				{
					"ReadDecimal128 Error",
					primitive.Decimal128{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Decimal128, Err: errors.New("rd128 error"), ErrAfter: bsonrwtest.ReadDecimal128},
					bsonrwtest.ReadDecimal128,
					errors.New("rd128 error"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Decimal128, Return: d128},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "Decimal128DecodeValue", Types: []reflect.Type{tDecimal}},
				},
				{
					"success",
					d128,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Decimal128, Return: d128},
					bsonrwtest.ReadDecimal128,
					nil,
				},
				{
					"decode null",
					primitive.Decimal128{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"JSONNumberDecodeValue",
			ValueDecoderFunc(dvd.JSONNumberDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.ObjectID},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "JSONNumberDecodeValue", Types: []reflect.Type{tJSONNumber}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not double/int32/int64",
					json.Number(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a json.Number", bsontype.String),
				},
				{
					"ReadDouble Error",
					json.Number(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Err: errors.New("rd error"), ErrAfter: bsonrwtest.ReadDouble},
					bsonrwtest.ReadDouble,
					errors.New("rd error"),
				},
				{
					"ReadInt32 Error",
					json.Number(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Err: errors.New("ri32 error"), ErrAfter: bsonrwtest.ReadInt32},
					bsonrwtest.ReadInt32,
					errors.New("ri32 error"),
				},
				{
					"ReadInt64 Error",
					json.Number(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Err: errors.New("ri64 error"), ErrAfter: bsonrwtest.ReadInt64},
					bsonrwtest.ReadInt64,
					errors.New("ri64 error"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.ObjectID, Return: primitive.ObjectID{}},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "JSONNumberDecodeValue", Types: []reflect.Type{tJSONNumber}},
				},
				{
					"success/double",
					json.Number("3.14159"),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14159)},
					bsonrwtest.ReadDouble,
					nil,
				},
				{
					"success/int32",
					json.Number("12345"),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32, Return: int32(12345)},
					bsonrwtest.ReadInt32,
					nil,
				},
				{
					"success/int64",
					json.Number("1234567890"),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: int64(1234567890)},
					bsonrwtest.ReadInt64,
					nil,
				},
				{
					"decode null",
					json.Number(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"URLDecodeValue",
			ValueDecoderFunc(dvd.URLDecodeValue),
			[]subtest{
				{
					"wrong type",
					url.URL{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a *url.URL", bsontype.Int32),
				},
				{
					"type not *url.URL",
					int64(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("http://example.com")},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "URLDecodeValue", Types: []reflect.Type{tURL}, Received: reflect.ValueOf(int64(0))},
				},
				{
					"ReadString error",
					url.URL{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Err: errors.New("rs error"), ErrAfter: bsonrwtest.ReadString},
					bsonrwtest.ReadString,
					errors.New("rs error"),
				},
				{
					"url.Parse error",
					url.URL{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("not-valid-%%%%://")},
					bsonrwtest.ReadString,
					errors.New("parse not-valid-%%%%://: first path segment in URL cannot contain colon"),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("http://example.com")},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "URLDecodeValue", Types: []reflect.Type{tURL}},
				},
				{
					"url.URL",
					url.URL{Scheme: "http", Host: "example.com"},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("http://example.com")},
					bsonrwtest.ReadString,
					nil,
				},
				{
					"decode null",
					url.URL{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"defaultByteSliceCodec.DecodeValue",
			defaultByteSliceCodec,
			[]subtest{
				{
					"wrong type",
					[]byte{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a []byte", bsontype.Int32),
				},
				{
					"type not []byte",
					int64(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Binary, Return: bsoncore.Value{Type: bsontype.Binary}},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "ByteSliceDecodeValue", Types: []reflect.Type{tByteSlice}, Received: reflect.ValueOf(int64(0))},
				},
				{
					"ReadBinary error",
					[]byte{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Binary, Err: errors.New("rb error"), ErrAfter: bsonrwtest.ReadBinary},
					bsonrwtest.ReadBinary,
					errors.New("rb error"),
				},
				{
					"incorrect subtype",
					[]byte{},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.Binary,
						Return: bsoncore.Value{
							Type: bsontype.Binary,
							Data: bsoncore.AppendBinary(nil, 0xFF, []byte{0x01, 0x02, 0x03}),
						},
					},
					bsonrwtest.ReadBinary,
					fmt.Errorf("ByteSliceDecodeValue can only be used to decode subtype 0x00 or 0x02 for %s, got %v", bsontype.Binary, byte(0xFF)),
				},
				{
					"can set false",
					cansettest,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Binary, Return: bsoncore.AppendBinary(nil, 0x00, []byte{0x01, 0x02, 0x03})},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "ByteSliceDecodeValue", Types: []reflect.Type{tByteSlice}},
				},
				{
					"decode null",
					([]byte)(nil),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"defaultStringCodec.DecodeValue",
			defaultStringCodec,
			[]subtest{
				{
					"symbol",
					"var hello = 'world';",
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Symbol, Return: "var hello = 'world';"},
					bsonrwtest.ReadSymbol,
					nil,
				},
				{
					"decode null",
					"",
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"ValueUnmarshalerDecodeValue",
			ValueDecoderFunc(dvd.ValueUnmarshalerDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:     "ValueUnmarshalerDecodeValue",
						Types:    []reflect.Type{tValueUnmarshaler},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"copy error",
					&testValueUnmarshaler{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Err: errors.New("copy error"), ErrAfter: bsonrwtest.ReadString},
					bsonrwtest.ReadString,
					errors.New("copy error"),
				},
				{
					"ValueUnmarshaler",
					&testValueUnmarshaler{t: bsontype.String, val: bsoncore.AppendString(nil, "hello, world")},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("hello, world")},
					bsonrwtest.ReadString,
					nil,
				},
			},
		},
		{
			"UnmarshalerDecodeValue",
			ValueDecoderFunc(dvd.UnmarshalerDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "UnmarshalerDecodeValue", Types: []reflect.Type{tUnmarshaler}, Received: reflect.ValueOf(wrong)},
				},
				{
					"copy error",
					&testUnmarshaler{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Err: errors.New("copy error"), ErrAfter: bsonrwtest.ReadString},
					bsonrwtest.ReadString,
					errors.New("copy error"),
				},
				{
					"Unmarshaler",
					testUnmarshaler{Val: bsoncore.AppendDouble(nil, 3.14159)},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14159)},
					bsonrwtest.ReadDouble,
					nil,
				},
			},
		},
		{
			"PointerCodec.DecodeValue",
			NewPointerCodec(),
			[]subtest{
				{
					"not valid", nil, nil, nil, bsonrwtest.Nothing,
					ValueDecoderError{Name: "PointerCodec.DecodeValue", Kinds: []reflect.Kind{reflect.Ptr}, Received: reflect.Value{}},
				},
				{
					"can set", cansettest, nil, nil, bsonrwtest.Nothing,
					ValueDecoderError{Name: "PointerCodec.DecodeValue", Kinds: []reflect.Kind{reflect.Ptr}},
				},
				{
					"No Decoder", &wrong, &DecodeContext{Registry: buildDefaultRegistry()}, nil, bsonrwtest.Nothing,
					ErrNoDecoder{Type: reflect.TypeOf(wrong)},
				},
				{
					"decode null",
					(*mystruct)(nil),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"BinaryDecodeValue",
			ValueDecoderFunc(dvd.BinaryDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "BinaryDecodeValue", Types: []reflect.Type{tBinary}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not binary",
					primitive.Binary{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a Binary", bsontype.String),
				},
				{
					"ReadBinary Error",
					primitive.Binary{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Binary, Err: errors.New("rb error"), ErrAfter: bsonrwtest.ReadBinary},
					bsonrwtest.ReadBinary,
					errors.New("rb error"),
				},
				{
					"Binary/success",
					primitive.Binary{Data: []byte{0x01, 0x02, 0x03}, Subtype: 0xFF},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.Binary,
						Return: bsoncore.Value{
							Type: bsontype.Binary,
							Data: bsoncore.AppendBinary(nil, 0xFF, []byte{0x01, 0x02, 0x03}),
						},
					},
					bsonrwtest.ReadBinary,
					nil,
				},
				{
					"decode null",
					primitive.Binary{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"UndefinedDecodeValue",
			ValueDecoderFunc(dvd.UndefinedDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Undefined},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "UndefinedDecodeValue", Types: []reflect.Type{tUndefined}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not undefined",
					primitive.Undefined{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into an Undefined", bsontype.String),
				},
				{
					"ReadUndefined Error",
					primitive.Undefined{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Undefined, Err: errors.New("ru error"), ErrAfter: bsonrwtest.ReadUndefined},
					bsonrwtest.ReadUndefined,
					errors.New("ru error"),
				},
				{
					"ReadUndefined/success",
					primitive.Undefined{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Undefined},
					bsonrwtest.ReadUndefined,
					nil,
				},
				{
					"decode null",
					primitive.Undefined{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"DateTimeDecodeValue",
			ValueDecoderFunc(dvd.DateTimeDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "DateTimeDecodeValue", Types: []reflect.Type{tDateTime}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not datetime",
					primitive.DateTime(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a DateTime", bsontype.String),
				},
				{
					"ReadDateTime Error",
					primitive.DateTime(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Err: errors.New("rdt error"), ErrAfter: bsonrwtest.ReadDateTime},
					bsonrwtest.ReadDateTime,
					errors.New("rdt error"),
				},
				{
					"success",
					primitive.DateTime(1234567890),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Return: int64(1234567890)},
					bsonrwtest.ReadDateTime,
					nil,
				},
				{
					"decode null",
					primitive.DateTime(0),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"NullDecodeValue",
			ValueDecoderFunc(dvd.NullDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "NullDecodeValue", Types: []reflect.Type{tNull}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not null",
					primitive.Null{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a Null", bsontype.String),
				},
				{
					"ReadNull Error",
					primitive.Null{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null, Err: errors.New("rn error"), ErrAfter: bsonrwtest.ReadNull},
					bsonrwtest.ReadNull,
					errors.New("rn error"),
				},
				{
					"success",
					primitive.Null{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"RegexDecodeValue",
			ValueDecoderFunc(dvd.RegexDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Regex},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "RegexDecodeValue", Types: []reflect.Type{tRegex}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not regex",
					primitive.Regex{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a Regex", bsontype.String),
				},
				{
					"ReadRegex Error",
					primitive.Regex{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Regex, Err: errors.New("rr error"), ErrAfter: bsonrwtest.ReadRegex},
					bsonrwtest.ReadRegex,
					errors.New("rr error"),
				},
				{
					"success",
					primitive.Regex{Pattern: "foo", Options: "bar"},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.Regex,
						Return: bsoncore.Value{
							Type: bsontype.Regex,
							Data: bsoncore.AppendRegex(nil, "foo", "bar"),
						},
					},
					bsonrwtest.ReadRegex,
					nil,
				},
				{
					"decode null",
					primitive.Regex{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"DBPointerDecodeValue",
			ValueDecoderFunc(dvd.DBPointerDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DBPointer},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "DBPointerDecodeValue", Types: []reflect.Type{tDBPointer}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not dbpointer",
					primitive.DBPointer{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a DBPointer", bsontype.String),
				},
				{
					"ReadDBPointer Error",
					primitive.DBPointer{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.DBPointer, Err: errors.New("rdbp error"), ErrAfter: bsonrwtest.ReadDBPointer},
					bsonrwtest.ReadDBPointer,
					errors.New("rdbp error"),
				},
				{
					"success",
					primitive.DBPointer{
						DB:      "foobar",
						Pointer: primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.DBPointer,
						Return: bsoncore.Value{
							Type: bsontype.DBPointer,
							Data: bsoncore.AppendDBPointer(
								nil, "foobar", primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
							),
						},
					},
					bsonrwtest.ReadDBPointer,
					nil,
				},
				{
					"decode null",
					primitive.DBPointer{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"TimestampDecodeValue",
			ValueDecoderFunc(dvd.TimestampDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Timestamp},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "TimestampDecodeValue", Types: []reflect.Type{tTimestamp}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not timestamp",
					primitive.Timestamp{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a Timestamp", bsontype.String),
				},
				{
					"ReadTimestamp Error",
					primitive.Timestamp{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Timestamp, Err: errors.New("rt error"), ErrAfter: bsonrwtest.ReadTimestamp},
					bsonrwtest.ReadTimestamp,
					errors.New("rt error"),
				},
				{
					"success",
					primitive.Timestamp{T: 12345, I: 67890},
					nil,
					&bsonrwtest.ValueReaderWriter{
						BSONType: bsontype.Timestamp,
						Return: bsoncore.Value{
							Type: bsontype.Timestamp,
							Data: bsoncore.AppendTimestamp(nil, 12345, 67890),
						},
					},
					bsonrwtest.ReadTimestamp,
					nil,
				},
				{
					"decode null",
					primitive.Timestamp{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"MinKeyDecodeValue",
			ValueDecoderFunc(dvd.MinKeyDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.MinKey},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "MinKeyDecodeValue", Types: []reflect.Type{tMinKey}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not null",
					primitive.MinKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a MinKey", bsontype.String),
				},
				{
					"ReadMinKey Error",
					primitive.MinKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.MinKey, Err: errors.New("rn error"), ErrAfter: bsonrwtest.ReadMinKey},
					bsonrwtest.ReadMinKey,
					errors.New("rn error"),
				},
				{
					"success",
					primitive.MinKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.MinKey},
					bsonrwtest.ReadMinKey,
					nil,
				},
				{
					"decode null",
					primitive.MinKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"MaxKeyDecodeValue",
			ValueDecoderFunc(dvd.MaxKeyDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.MaxKey},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "MaxKeyDecodeValue", Types: []reflect.Type{tMaxKey}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not null",
					primitive.MaxKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a MaxKey", bsontype.String),
				},
				{
					"ReadMaxKey Error",
					primitive.MaxKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.MaxKey, Err: errors.New("rn error"), ErrAfter: bsonrwtest.ReadMaxKey},
					bsonrwtest.ReadMaxKey,
					errors.New("rn error"),
				},
				{
					"success",
					primitive.MaxKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.MaxKey},
					bsonrwtest.ReadMaxKey,
					nil,
				},
				{
					"decode null",
					primitive.MaxKey{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"JavaScriptDecodeValue",
			ValueDecoderFunc(dvd.JavaScriptDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.JavaScript, Return: ""},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "JavaScriptDecodeValue", Types: []reflect.Type{tJavaScript}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not Javascript",
					primitive.JavaScript(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a primitive.JavaScript", bsontype.String),
				},
				{
					"ReadJavascript Error",
					primitive.JavaScript(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.JavaScript, Err: errors.New("rjs error"), ErrAfter: bsonrwtest.ReadJavascript},
					bsonrwtest.ReadJavascript,
					errors.New("rjs error"),
				},
				{
					"JavaScript/success",
					primitive.JavaScript("var hello = 'world';"),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.JavaScript, Return: "var hello = 'world';"},
					bsonrwtest.ReadJavascript,
					nil,
				},
				{
					"decode null",
					primitive.JavaScript(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"SymbolDecodeValue",
			ValueDecoderFunc(dvd.SymbolDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Symbol, Return: ""},
					bsonrwtest.Nothing,
					ValueDecoderError{Name: "SymbolDecodeValue", Types: []reflect.Type{tSymbol}, Received: reflect.ValueOf(wrong)},
				},
				{
					"type not Symbol",
					primitive.Symbol(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int32},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a primitive.Symbol", bsontype.Int32),
				},
				{
					"ReadSymbol Error",
					primitive.Symbol(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Symbol, Err: errors.New("rjs error"), ErrAfter: bsonrwtest.ReadSymbol},
					bsonrwtest.ReadSymbol,
					errors.New("rjs error"),
				},
				{
					"Symbol/success",
					primitive.Symbol("var hello = 'world';"),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Symbol, Return: "var hello = 'world';"},
					bsonrwtest.ReadSymbol,
					nil,
				},
				{
					"decode null",
					primitive.Symbol(""),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"CoreDocumentDecodeValue",
			ValueDecoderFunc(dvd.CoreDocumentDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:     "CoreDocumentDecodeValue",
						Types:    []reflect.Type{tCoreDocument},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"*bsoncore.Document is nil",
					(*bsoncore.Document)(nil),
					nil,
					nil,
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:     "CoreDocumentDecodeValue",
						Types:    []reflect.Type{tCoreDocument},
						Received: reflect.ValueOf((*bsoncore.Document)(nil)),
					},
				},
				{
					"Copy error",
					bsoncore.Document{},
					nil,
					&bsonrwtest.ValueReaderWriter{Err: errors.New("copy error"), ErrAfter: bsonrwtest.ReadDocument},
					bsonrwtest.ReadDocument,
					errors.New("copy error"),
				},
			},
		},
		{
			"StructCodec.DecodeValue",
			defaultStructCodec,
			[]subtest{
				{
					"Not struct",
					reflect.New(reflect.TypeOf(struct{ Foo string }{})).Elem().Interface(),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					errors.New("cannot decode string into a struct { Foo string }"),
				},
				{
					"decode null",
					reflect.New(reflect.TypeOf(struct{ Foo string }{})).Elem().Interface(),
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
		{
			"CodeWithScopeDecodeValue",
			ValueDecoderFunc(dvd.CodeWithScopeDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.CodeWithScope},
					bsonrwtest.Nothing,
					ValueDecoderError{
						Name:     "CodeWithScopeDecodeValue",
						Types:    []reflect.Type{tCodeWithScope},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"type not codewithscope",
					primitive.CodeWithScope{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.String},
					bsonrwtest.Nothing,
					fmt.Errorf("cannot decode %v into a primitive.CodeWithScope", bsontype.String),
				},
				{
					"ReadCodeWithScope Error",
					primitive.CodeWithScope{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.CodeWithScope, Err: errors.New("rcws error"), ErrAfter: bsonrwtest.ReadCodeWithScope},
					bsonrwtest.ReadCodeWithScope,
					errors.New("rcws error"),
				},
				{
					"decodeDocument Error",
					primitive.CodeWithScope{
						Code:  "var hello = 'world';",
						Scope: primitive.D{{"foo", nil}},
					},
					&DecodeContext{Registry: buildDefaultRegistry()},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.CodeWithScope, Err: errors.New("dd error"), ErrAfter: bsonrwtest.ReadElement},
					bsonrwtest.ReadElement,
					errors.New("dd error"),
				},
				{
					"decode null",
					primitive.CodeWithScope{},
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Null},
					bsonrwtest.ReadNull,
					nil,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rc := range tc.subtests {
				t.Run(rc.name, func(t *testing.T) {
					var dc DecodeContext
					if rc.dctx != nil {
						dc = *rc.dctx
					}
					llvrw := new(bsonrwtest.ValueReaderWriter)
					if rc.llvrw != nil {
						llvrw = rc.llvrw
					}
					llvrw.T = t
					// var got interface{}
					if rc.val == cansetreflectiontest { // We're doing a CanSet reflection test
						err := tc.vd.DecodeValue(dc, llvrw, reflect.Value{})
						if !compareErrors(err, rc.err) {
							t.Errorf("Errors do not match. got %v; want %v", err, rc.err)
						}

						val := reflect.New(reflect.TypeOf(rc.val)).Elem()
						err = tc.vd.DecodeValue(dc, llvrw, val)
						if !compareErrors(err, rc.err) {
							t.Errorf("Errors do not match. got %v; want %v", err, rc.err)
						}
						return
					}
					if rc.val == cansettest { // We're doing an IsValid and CanSet test
						wanterr, ok := rc.err.(ValueDecoderError)
						if !ok {
							t.Fatalf("Error must be a DecodeValueError, but got a %T", rc.err)
						}

						err := tc.vd.DecodeValue(dc, llvrw, reflect.Value{})
						wanterr.Received = reflect.ValueOf(nil)
						if !compareErrors(err, wanterr) {
							t.Errorf("Errors do not match. got %v; want %v", err, wanterr)
						}

						err = tc.vd.DecodeValue(dc, llvrw, reflect.ValueOf(int(12345)))
						wanterr.Received = reflect.ValueOf(int(12345))
						if !compareErrors(err, wanterr) {
							t.Errorf("Errors do not match. got %v; want %v", err, wanterr)
						}
						return
					}
					var val reflect.Value
					if rtype := reflect.TypeOf(rc.val); rtype != nil {
						val = reflect.New(rtype).Elem()
					}
					want := rc.val
					defer func() {
						if err := recover(); err != nil {
							fmt.Println(t.Name())
							panic(err)
						}
					}()
					err := tc.vd.DecodeValue(dc, llvrw, val)
					if !compareErrors(err, rc.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, rc.err)
					}
					invoked := llvrw.Invoked
					if !cmp.Equal(invoked, rc.invoke) {
						t.Errorf("Incorrect method invoked. got %v; want %v", invoked, rc.invoke)
					}
					var got interface{}
					if val.IsValid() && val.CanInterface() {
						got = val.Interface()
					}
					if rc.err == nil && !cmp.Equal(got, want, cmp.Comparer(compareDecimal128)) {
						t.Errorf("Values do not match. got (%T)%v; want (%T)%v", got, got, want, want)
					}
				})
			}
		})
	}

	t.Run("CodeWithScopeCodec/DecodeValue/success", func(t *testing.T) {
		dc := DecodeContext{Registry: buildDefaultRegistry()}
		b := bsoncore.BuildDocument(nil,
			bsoncore.AppendCodeWithScopeElement(
				nil, "foo", "var hello = 'world';",
				buildDocument(bsoncore.AppendNullElement(nil, "bar")),
			),
		)
		dvr := bsonrw.NewBSONDocumentReader(b)
		dr, err := dvr.ReadDocument()
		noerr(t, err)
		_, vr, err := dr.ReadElement()
		noerr(t, err)

		want := primitive.CodeWithScope{
			Code:  "var hello = 'world';",
			Scope: primitive.D{{"bar", nil}},
		}
		val := reflect.New(tCodeWithScope).Elem()
		err = dvd.CodeWithScopeDecodeValue(dc, vr, val)
		noerr(t, err)

		got := val.Interface().(primitive.CodeWithScope)
		if got.Code != want.Code && !cmp.Equal(got.Scope, want.Scope) {
			t.Errorf("CodeWithScopes do not match. got %v; want %v", got, want)
		}
	})
	t.Run("ValueUnmarshalerDecodeValue/UnmarshalBSONValue error", func(t *testing.T) {
		var dc DecodeContext
		llvrw := &bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("hello, world!")}
		llvrw.T = t

		want := errors.New("ubsonv error")
		valUnmarshaler := &testValueUnmarshaler{err: want}
		got := dvd.ValueUnmarshalerDecodeValue(dc, llvrw, reflect.ValueOf(valUnmarshaler))
		if !compareErrors(got, want) {
			t.Errorf("Errors do not match. got %v; want %v", got, want)
		}
	})
	t.Run("ValueUnmarshalerDecodeValue/Unaddressable value", func(t *testing.T) {
		var dc DecodeContext
		llvrw := &bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: string("hello, world!")}
		llvrw.T = t

		val := reflect.ValueOf(testValueUnmarshaler{})
		want := ValueDecoderError{Name: "ValueUnmarshalerDecodeValue", Types: []reflect.Type{tValueUnmarshaler}, Received: val}
		got := dvd.ValueUnmarshalerDecodeValue(dc, llvrw, val)
		if !compareErrors(got, want) {
			t.Errorf("Errors do not match. got %v; want %v", got, want)
		}
	})

	t.Run("SliceCodec/DecodeValue/can't set slice", func(t *testing.T) {
		var val []string
		want := ValueDecoderError{Name: "SliceDecodeValue", Kinds: []reflect.Kind{reflect.Slice}, Received: reflect.ValueOf(val)}
		got := dvd.SliceDecodeValue(DecodeContext{}, nil, reflect.ValueOf(val))
		if !compareErrors(got, want) {
			t.Errorf("Errors do not match. got %v; want %v", got, want)
		}
	})
	t.Run("SliceCodec/DecodeValue/too many elements", func(t *testing.T) {
		idx, doc := bsoncore.AppendDocumentStart(nil)
		aidx, doc := bsoncore.AppendArrayElementStart(doc, "foo")
		doc = bsoncore.AppendStringElement(doc, "0", "foo")
		doc = bsoncore.AppendStringElement(doc, "1", "bar")
		doc, err := bsoncore.AppendArrayEnd(doc, aidx)
		noerr(t, err)
		doc, err = bsoncore.AppendDocumentEnd(doc, idx)
		noerr(t, err)
		dvr := bsonrw.NewBSONDocumentReader(doc)
		noerr(t, err)
		dr, err := dvr.ReadDocument()
		noerr(t, err)
		_, vr, err := dr.ReadElement()
		noerr(t, err)
		var val [1]string
		want := fmt.Errorf("more elements returned in array than can fit inside %T, got 2 elements", val)

		dc := DecodeContext{Registry: buildDefaultRegistry()}
		got := dvd.ArrayDecodeValue(dc, vr, reflect.ValueOf(val))
		if !compareErrors(got, want) {
			t.Errorf("Errors do not match. got %v; want %v", got, want)
		}
	})

	t.Run("success path", func(t *testing.T) {
		oid := primitive.NewObjectID()
		oids := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()}
		var str = new(string)
		*str = "bar"
		now := time.Now().Truncate(time.Millisecond).UTC()
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
				func() []byte {
					idx, doc := bsoncore.AppendDocumentStart(nil)
					doc = bsoncore.AppendObjectIDElement(doc, "foo", oid)
					doc, _ = bsoncore.AppendDocumentEnd(doc, idx)
					return doc
				}(),
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
				"map[mystring]interface{}",
				map[mystring]interface{}{"pi": 3.14159},
				buildDocument(bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
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
					AE *testValueUnmarshaler
					AF *bool
					AG *bool
					AH *int32
					AI *int64
					AJ *primitive.ObjectID
					AK *primitive.ObjectID
					AL testValueUnmarshaler
					AM interface{}
					AN interface{}
					AO interface{}
					AP primitive.D
					AQ primitive.A
					AR [2]primitive.E
					AS []byte
					AT map[string]interface{}
					AU primitive.CodeWithScope
					AV primitive.M
					AW primitive.D
					AX map[string]interface{}
					AY []primitive.E
					AZ interface{}
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
					AE: &testValueUnmarshaler{t: bsontype.String, val: bsoncore.AppendString(nil, "hello, world!")},
					AF: func(b bool) *bool { return &b }(true),
					AG: nil,
					AH: func(i32 int32) *int32 { return &i32 }(12345),
					AI: func(i64 int64) *int64 { return &i64 }(1234567890),
					AJ: &oid,
					AK: nil,
					AL: testValueUnmarshaler{t: bsontype.String, val: bsoncore.AppendString(nil, "hello, world!")},
					AM: "hello, world",
					AN: int32(12345),
					AO: oid,
					AP: primitive.D{{"foo", "bar"}},
					AQ: primitive.A{"foo", "bar"},
					AR: [2]primitive.E{{"hello", "world"}, {"pi", 3.14159}},
					AS: nil,
					AT: nil,
					AU: primitive.CodeWithScope{Code: "var hello = 'world';", Scope: primitive.D{{"pi", 3.14159}}},
					AV: primitive.M{"foo": primitive.M{"bar": "baz"}},
					AW: primitive.D{{"foo", primitive.D{{"bar", "baz"}}}},
					AX: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
					AY: []primitive.E{{"foo", []primitive.E{{"bar", "baz"}}}},
					AZ: primitive.D{{"foo", primitive.D{{"bar", "baz"}}}},
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
					doc = bsoncore.AppendStringElement(doc, "ae", "hello, world!")
					doc = bsoncore.AppendBooleanElement(doc, "af", true)
					doc = bsoncore.AppendNullElement(doc, "ag")
					doc = bsoncore.AppendInt32Element(doc, "ah", 12345)
					doc = bsoncore.AppendInt32Element(doc, "ai", 1234567890)
					doc = bsoncore.AppendObjectIDElement(doc, "aj", oid)
					doc = bsoncore.AppendNullElement(doc, "ak")
					doc = bsoncore.AppendStringElement(doc, "al", "hello, world!")
					doc = bsoncore.AppendStringElement(doc, "am", "hello, world")
					doc = bsoncore.AppendInt32Element(doc, "an", 12345)
					doc = bsoncore.AppendObjectIDElement(doc, "ao", oid)
					doc = bsoncore.AppendDocumentElement(doc, "ap", buildDocument(bsoncore.AppendStringElement(nil, "foo", "bar")))
					doc = bsoncore.AppendArrayElement(doc, "aq",
						buildArray(bsoncore.AppendStringElement(bsoncore.AppendStringElement(nil, "0", "foo"), "1", "bar")),
					)
					doc = bsoncore.AppendDocumentElement(doc, "ar",
						buildDocument(bsoncore.AppendDoubleElement(bsoncore.AppendStringElement(nil, "hello", "world"), "pi", 3.14159)),
					)
					doc = bsoncore.AppendNullElement(doc, "as")
					doc = bsoncore.AppendNullElement(doc, "at")
					doc = bsoncore.AppendCodeWithScopeElement(doc, "au",
						"var hello = 'world';", buildDocument(bsoncore.AppendDoubleElement(nil, "pi", 3.14159)),
					)
					for _, name := range [5]string{"av", "aw", "ax", "ay", "az"} {
						doc = bsoncore.AppendDocumentElement(doc, name, buildDocument(
							bsoncore.AppendDocumentElement(nil, "foo", buildDocument(
								bsoncore.AppendStringElement(nil, "bar", "baz"),
							)),
						))
					}
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
					AE []*testValueUnmarshaler
					AF []*bool
					AG []*int32
					AH []*int64
					AI []*primitive.ObjectID
					AJ []primitive.D
					AK []primitive.A
					AL [][2]primitive.E
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
					AE: []*testValueUnmarshaler{
						{t: bsontype.String, val: bsoncore.AppendString(nil, "hello")},
						{t: bsontype.String, val: bsoncore.AppendString(nil, "world")},
					},
					AF: []*bool{pbool(true), nil},
					AG: []*int32{pi32(12345), nil},
					AH: []*int64{pi64(1234567890), nil, pi64(9012345678)},
					AI: []*primitive.ObjectID{&oid, nil},
					AJ: []primitive.D{{{"foo", "bar"}}, nil},
					AK: []primitive.A{{"foo", "bar"}, nil},
					AL: [][2]primitive.E{{{"hello", "world"}, {"pi", 3.14159}}},
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
						bsoncore.AppendNullElement(bsoncore.AppendBooleanElement(nil, "0", true), "1"),
					)
					doc = appendArrayElement(doc, "ag",
						bsoncore.AppendNullElement(bsoncore.AppendInt32Element(nil, "0", 12345), "1"),
					)
					doc = appendArrayElement(doc, "ah",
						bsoncore.AppendInt64Element(
							bsoncore.AppendNullElement(bsoncore.AppendInt64Element(nil, "0", 1234567890), "1"),
							"2", 9012345678,
						),
					)
					doc = appendArrayElement(doc, "ai",
						bsoncore.AppendNullElement(bsoncore.AppendObjectIDElement(nil, "0", oid), "1"),
					)
					doc = appendArrayElement(doc, "aj",
						bsoncore.AppendNullElement(
							bsoncore.AppendDocumentElement(nil, "0", buildDocument(bsoncore.AppendStringElement(nil, "foo", "bar"))),
							"1",
						),
					)
					doc = appendArrayElement(doc, "ak",
						bsoncore.AppendNullElement(
							buildArrayElement("0",
								bsoncore.AppendStringElement(bsoncore.AppendStringElement(nil, "0", "foo"), "1", "bar"),
							),
							"1",
						),
					)
					doc = appendArrayElement(doc, "al",
						buildDocumentElement(
							"0",
							bsoncore.AppendDoubleElement(bsoncore.AppendStringElement(nil, "hello", "world"), "pi", 3.14159),
						),
					)
					return doc
				}(nil)),
				nil,
			},
		}

		t.Run("Decode", func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					vr := bsonrw.NewBSONDocumentReader(tc.b)
					reg := buildDefaultRegistry()
					vtype := reflect.TypeOf(tc.value)
					dec, err := reg.LookupDecoder(vtype)
					noerr(t, err)

					gotVal := reflect.New(reflect.TypeOf(tc.value)).Elem()
					err = dec.DecodeValue(DecodeContext{Registry: reg}, vr, gotVal)
					noerr(t, err)

					got := gotVal.Interface()
					want := tc.value
					if diff := cmp.Diff(
						got, want,
						cmp.Comparer(compareDecimal128),
						cmp.Comparer(compareNoPrivateFields),
						cmp.Comparer(compareZeroTest),
						cmp.Comparer(compareTime),
					); diff != "" {
						t.Errorf("difference:\n%s", diff)
						t.Errorf("Values are not equal.\ngot: %#v\nwant:%#v", got, want)
					}
				})
			}
		})
	})

	t.Run("defaultEmptyInterfaceCodec.DecodeValue", func(t *testing.T) {
		t.Run("DecodeValue", func(t *testing.T) {
			testCases := []struct {
				name     string
				val      interface{}
				bsontype bsontype.Type
			}{
				{
					"Double - float64",
					float64(3.14159),
					bsontype.Double,
				},
				{
					"String - string",
					string("foo bar baz"),
					bsontype.String,
				},
				{
					"Array - primitive.A",
					primitive.A{3.14159},
					bsontype.Array,
				},
				{
					"Binary - Binary",
					primitive.Binary{Subtype: 0xFF, Data: []byte{0x01, 0x02, 0x03}},
					bsontype.Binary,
				},
				{
					"Undefined - Undefined",
					primitive.Undefined{},
					bsontype.Undefined,
				},
				{
					"ObjectID - primitive.ObjectID",
					primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					bsontype.ObjectID,
				},
				{
					"Boolean - bool",
					bool(true),
					bsontype.Boolean,
				},
				{
					"DateTime - DateTime",
					primitive.DateTime(1234567890),
					bsontype.DateTime,
				},
				{
					"Null - Null",
					nil,
					bsontype.Null,
				},
				{
					"Regex - Regex",
					primitive.Regex{Pattern: "foo", Options: "bar"},
					bsontype.Regex,
				},
				{
					"DBPointer - DBPointer",
					primitive.DBPointer{
						DB:      "foobar",
						Pointer: primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
					},
					bsontype.DBPointer,
				},
				{
					"JavaScript - JavaScript",
					primitive.JavaScript("var foo = 'bar';"),
					bsontype.JavaScript,
				},
				{
					"Symbol - Symbol",
					primitive.Symbol("foobarbazlolz"),
					bsontype.Symbol,
				},
				{
					"Int32 - int32",
					int32(123456),
					bsontype.Int32,
				},
				{
					"Int64 - int64",
					int64(1234567890),
					bsontype.Int64,
				},
				{
					"Timestamp - Timestamp",
					primitive.Timestamp{T: 12345, I: 67890},
					bsontype.Timestamp,
				},
				{
					"Decimal128 - decimal.Decimal128",
					primitive.NewDecimal128(12345, 67890),
					bsontype.Decimal128,
				},
				{
					"MinKey - MinKey",
					primitive.MinKey{},
					bsontype.MinKey,
				},
				{
					"MaxKey - MaxKey",
					primitive.MaxKey{},
					bsontype.MaxKey,
				},
			}
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					llvr := &bsonrwtest.ValueReaderWriter{BSONType: tc.bsontype}

					t.Run("Type Map failure", func(t *testing.T) {
						if tc.bsontype == bsontype.Null {
							t.Skip()
						}
						val := reflect.New(tEmpty).Elem()
						dc := DecodeContext{Registry: NewRegistryBuilder().Build()}
						want := ErrNoTypeMapEntry{Type: tc.bsontype}
						got := defaultEmptyInterfaceCodec.DecodeValue(dc, llvr, val)
						if !compareErrors(got, want) {
							t.Errorf("Errors are not equal. got %v; want %v", got, want)
						}
					})

					t.Run("Lookup failure", func(t *testing.T) {
						if tc.bsontype == bsontype.Null {
							t.Skip()
						}
						val := reflect.New(tEmpty).Elem()
						dc := DecodeContext{
							Registry: NewRegistryBuilder().
								RegisterTypeMapEntry(tc.bsontype, reflect.TypeOf(tc.val)).
								Build(),
						}
						want := ErrNoDecoder{Type: reflect.TypeOf(tc.val)}
						got := defaultEmptyInterfaceCodec.DecodeValue(dc, llvr, val)
						if !compareErrors(got, want) {
							t.Errorf("Errors are not equal. got %v; want %v", got, want)
						}
					})

					t.Run("DecodeValue failure", func(t *testing.T) {
						if tc.bsontype == bsontype.Null {
							t.Skip()
						}
						want := errors.New("DecodeValue failure error")
						llc := &llCodec{t: t, err: want}
						dc := DecodeContext{
							Registry: NewRegistryBuilder().
								RegisterTypeDecoder(reflect.TypeOf(tc.val), llc).
								RegisterTypeMapEntry(tc.bsontype, reflect.TypeOf(tc.val)).
								Build(),
						}
						got := defaultEmptyInterfaceCodec.DecodeValue(dc, llvr, reflect.New(tEmpty).Elem())
						if !compareErrors(got, want) {
							t.Errorf("Errors are not equal. got %v; want %v", got, want)
						}
					})

					t.Run("Success", func(t *testing.T) {
						want := tc.val
						llc := &llCodec{t: t, decodeval: tc.val}
						dc := DecodeContext{
							Registry: NewRegistryBuilder().
								RegisterTypeDecoder(reflect.TypeOf(tc.val), llc).
								RegisterTypeMapEntry(tc.bsontype, reflect.TypeOf(tc.val)).
								Build(),
						}
						got := reflect.New(tEmpty).Elem()
						err := defaultEmptyInterfaceCodec.DecodeValue(dc, llvr, got)
						noerr(t, err)
						if !cmp.Equal(got.Interface(), want, cmp.Comparer(compareDecimal128)) {
							t.Errorf("Did not receive expected value. got %v; want %v", got.Interface(), want)
						}
					})
				})
			}
		})

		t.Run("non-interface{}", func(t *testing.T) {
			val := uint64(1234567890)
			want := ValueDecoderError{Name: "EmptyInterfaceDecodeValue", Types: []reflect.Type{tEmpty}, Received: reflect.ValueOf(val)}
			got := defaultEmptyInterfaceCodec.DecodeValue(DecodeContext{}, nil, reflect.ValueOf(val))
			if !compareErrors(got, want) {
				t.Errorf("Errors are not equal. got %v; want %v", got, want)
			}
		})

		t.Run("nil *interface{}", func(t *testing.T) {
			var val interface{}
			want := ValueDecoderError{Name: "EmptyInterfaceDecodeValue", Types: []reflect.Type{tEmpty}, Received: reflect.ValueOf(val)}
			got := defaultEmptyInterfaceCodec.DecodeValue(DecodeContext{}, nil, reflect.ValueOf(val))
			if !compareErrors(got, want) {
				t.Errorf("Errors are not equal. got %v; want %v", got, want)
			}
		})

		t.Run("no type registered", func(t *testing.T) {
			llvr := &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double}
			want := ErrNoTypeMapEntry{Type: bsontype.Double}
			val := reflect.New(tEmpty).Elem()
			got := defaultEmptyInterfaceCodec.DecodeValue(DecodeContext{Registry: NewRegistryBuilder().Build()}, llvr, val)
			if !compareErrors(got, want) {
				t.Errorf("Errors are not equal. got %v; want %v", got, want)
			}
		})
		t.Run("top level document", func(t *testing.T) {
			data := bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))
			vr := bsonrw.NewBSONDocumentReader(data)
			want := primitive.D{{"pi", 3.14159}}
			var got interface{}
			val := reflect.ValueOf(&got).Elem()
			err := defaultEmptyInterfaceCodec.DecodeValue(DecodeContext{Registry: buildDefaultRegistry()}, vr, val)
			noerr(t, err)
			if !cmp.Equal(got, want) {
				t.Errorf("Did not get correct result. got %v; want %v", got, want)
			}
		})
		t.Run("custom type map entry", func(t *testing.T) {
			// registering a custom type map entry for both bsontype.Type(0) anad bsontype.EmbeddedDocument should cause
			// both top-level and embedded documents to decode to registered type when unmarshalling to interface{}

			topLevelRb := NewRegistryBuilder()
			defaultValueEncoders.RegisterDefaultEncoders(topLevelRb)
			defaultValueDecoders.RegisterDefaultDecoders(topLevelRb)
			topLevelRb.RegisterTypeMapEntry(bsontype.Type(0), reflect.TypeOf(primitive.M{}))

			embeddedRb := NewRegistryBuilder()
			defaultValueEncoders.RegisterDefaultEncoders(embeddedRb)
			defaultValueDecoders.RegisterDefaultDecoders(embeddedRb)
			embeddedRb.RegisterTypeMapEntry(bsontype.Type(0), reflect.TypeOf(primitive.M{}))

			// create doc {"nested": {"foo": 1}}
			innerDoc := bsoncore.BuildDocument(
				nil,
				bsoncore.AppendInt32Element(nil, "foo", 1),
			)
			doc := bsoncore.BuildDocument(
				nil,
				bsoncore.AppendDocumentElement(nil, "nested", innerDoc),
			)
			want := primitive.M{
				"nested": primitive.M{
					"foo": int32(1),
				},
			}

			testCases := []struct {
				name     string
				registry *Registry
			}{
				{"top level", topLevelRb.Build()},
				{"embedded", embeddedRb.Build()},
			}
			for _, tc := range testCases {
				var got interface{}
				vr := bsonrw.NewBSONDocumentReader(doc)
				val := reflect.ValueOf(&got).Elem()

				err := defaultEmptyInterfaceCodec.DecodeValue(DecodeContext{Registry: tc.registry}, vr, val)
				noerr(t, err)
				if !cmp.Equal(got, want) {
					t.Fatalf("got %v, want %v", got, want)
				}
			}
		})
		t.Run("ancestor info is used over custom type map entry", func(t *testing.T) {
			// If a type map entry is registered for bsontype.EmbeddedDocument, the decoder should use ancestor
			// information if available instead of the registered entry.

			rb := NewRegistryBuilder()
			defaultValueEncoders.RegisterDefaultEncoders(rb)
			defaultValueDecoders.RegisterDefaultDecoders(rb)
			rb.RegisterTypeMapEntry(bsontype.EmbeddedDocument, reflect.TypeOf(primitive.M{}))
			reg := rb.Build()

			// build document {"nested": {"foo": 10}}
			inner := bsoncore.BuildDocument(
				nil,
				bsoncore.AppendInt32Element(nil, "foo", 10),
			)
			doc := bsoncore.BuildDocument(
				nil,
				bsoncore.AppendDocumentElement(nil, "nested", inner),
			)
			want := primitive.D{
				{"nested", primitive.D{
					{"foo", int32(10)},
				}},
			}

			var got primitive.D
			vr := bsonrw.NewBSONDocumentReader(doc)
			val := reflect.ValueOf(&got).Elem()
			err := defaultSliceCodec.DecodeValue(DecodeContext{Registry: reg}, vr, val)
			noerr(t, err)
			if !cmp.Equal(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	})
}

type testValueUnmarshaler struct {
	t   bsontype.Type
	val []byte
	err error
}

func (tvu *testValueUnmarshaler) UnmarshalBSONValue(t bsontype.Type, val []byte) error {
	tvu.t, tvu.val = t, val
	return tvu.err
}

type testUnmarshaler struct {
	Val []byte
	Err error
}

func (tvu *testUnmarshaler) UnmarshalBSON(val []byte) error {
	tvu.Val = val
	return tvu.Err
}

func (tvu testValueUnmarshaler) Equal(tvu2 testValueUnmarshaler) bool {
	return tvu.t == tvu2.t && bytes.Equal(tvu.val, tvu2.val)
}

// buildDocumentArray inserts vals inside of an array inside of a document.
func buildDocumentArray(fn func([]byte) []byte) []byte {
	aix, doc := bsoncore.AppendArrayElementStart(nil, "Z")
	doc = fn(doc)
	doc, _ = bsoncore.AppendArrayEnd(doc, aix)
	return buildDocument(doc)
}

func buildArray(vals []byte) []byte {
	aix, doc := bsoncore.AppendArrayStart(nil)
	doc = append(doc, vals...)
	doc, _ = bsoncore.AppendArrayEnd(doc, aix)
	return doc
}

func buildArrayElement(key string, vals []byte) []byte {
	return appendArrayElement(nil, key, vals)
}

func appendArrayElement(dst []byte, key string, vals []byte) []byte {
	aix, doc := bsoncore.AppendArrayElementStart(dst, key)
	doc = append(doc, vals...)
	doc, _ = bsoncore.AppendArrayEnd(doc, aix)
	return doc
}

// buildDocument inserts elems inside of a document.
func buildDocument(elems []byte) []byte {
	idx, doc := bsoncore.AppendDocumentStart(nil)
	doc = append(doc, elems...)
	doc, _ = bsoncore.AppendDocumentEnd(doc, idx)
	return doc
}

func buildDocumentElement(key string, elems []byte) []byte {
	idx, doc := bsoncore.AppendDocumentElementStart(nil, key)
	doc = append(doc, elems...)
	doc, _ = bsoncore.AppendDocumentEnd(doc, idx)
	return doc
}
