// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec_test

import (
	"fmt"
	"math"
	"reflect"

	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

func ExampleRegistry_customEncoder() {
	// Write a custom encoder for an integer type that is multiplied by -1 when encoding.

	// To register the default encoders and decoders in addition to this custom one, use bson.NewRegistryBuilder
	// instead.
	rb := bsoncodec.NewRegistryBuilder()
	type negatedInt int

	niType := reflect.TypeOf(negatedInt(0))
	encoder := func(ec bsoncodec.EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
		// All encoder implementations should check that val is valid and is of the correct type before proceeding.
		if !val.IsValid() || val.Type() != niType {
			return bsoncodec.ValueEncoderError{
				Name:     "negatedIntEncodeValue",
				Types:    []reflect.Type{niType},
				Received: val,
			}
		}

		// Negate val and encode as a BSON int32 if it can fit in 32 bits and a BSON int64 otherwise.
		negatedVal := val.Int() * -1
		if math.MinInt32 <= negatedVal && negatedVal <= math.MaxInt32 {
			return vw.WriteInt32(int32(negatedVal))
		}
		return vw.WriteInt64(negatedVal)
	}

	rb.RegisterTypeEncoder(niType, bsoncodec.ValueEncoderFunc(encoder))
}

func ExampleRegistry_customDecoder() {
	// Write a custom decoder for a boolean type that can be stored in the database as a BSON boolean, int32, int64,
	// double, or null. BSON int32, int64, and double values are considered "true" in this decoder if they are
	// non-zero. BSON null values are always considered false.

	// To register the default encoders and decoders in addition to this custom one, use bson.NewRegistryBuilder
	// instead.
	rb := bsoncodec.NewRegistryBuilder()
	type lenientBool bool

	decoder := func(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
		// All decoder implementations should check that val is valid and settable and is of the correct kind
		// before proceeding.
		if !val.IsValid() || !val.CanSet() || val.Kind() != reflect.Bool {
			return bsoncodec.ValueDecoderError{
				Name:     "lenientBoolDecodeValue",
				Kinds:    []reflect.Kind{reflect.Bool},
				Received: val,
			}
		}

		var result bool
		switch vr.Type() {
		case bsontype.Boolean:
			b, err := vr.ReadBoolean()
			if err != nil {
				return err
			}
			result = b
		case bsontype.Int32:
			i32, err := vr.ReadInt32()
			if err != nil {
				return err
			}
			result = i32 != 0
		case bsontype.Int64:
			i64, err := vr.ReadInt64()
			if err != nil {
				return err
			}
			result = i64 != 0
		case bsontype.Double:
			f64, err := vr.ReadDouble()
			if err != nil {
				return err
			}
			result = f64 != 0
		case bsontype.Null:
			if err := vr.ReadNull(); err != nil {
				return err
			}
			result = false
		default:
			return fmt.Errorf("received invalid BSON type to decode into lenientBool: %s", vr.Type())
		}

		val.SetBool(result)
		return nil
	}

	lbType := reflect.TypeOf(lenientBool(true))
	rb.RegisterTypeDecoder(lbType, bsoncodec.ValueDecoderFunc(decoder))
}
