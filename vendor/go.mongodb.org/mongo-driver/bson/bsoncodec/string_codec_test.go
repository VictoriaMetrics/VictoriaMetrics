// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsonoptions"
	"go.mongodb.org/mongo-driver/bson/bsonrw/bsonrwtest"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

func TestStringCodec(t *testing.T) {
	t.Run("ObjectIDAsHex", func(t *testing.T) {
		oid := primitive.NewObjectID()
		byteArray := [12]byte(oid)
		reader := &bsonrwtest.ValueReaderWriter{BSONType: bsontype.ObjectID, Return: oid}
		testCases := []struct {
			name   string
			opts   *bsonoptions.StringCodecOptions
			hex    bool
			result string
		}{
			{"default", bsonoptions.StringCodec(), true, oid.Hex()},
			{"true", bsonoptions.StringCodec().SetDecodeObjectIDAsHex(true), true, oid.Hex()},
			{"false", bsonoptions.StringCodec().SetDecodeObjectIDAsHex(false), false, string(byteArray[:])},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				stringCodec := NewStringCodec(tc.opts)

				actual := reflect.New(reflect.TypeOf("")).Elem()
				err := stringCodec.DecodeValue(DecodeContext{}, reader, actual)
				assert.Nil(t, err, "StringCodec.DecodeValue error: %v", err)

				actualString := actual.Interface().(string)
				assert.Equal(t, tc.result, actualString, "Expected string %v, got %v", tc.result, actualString)
			})
		}
	})
}
