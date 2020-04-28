// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/bsonoptions"
	"go.mongodb.org/mongo-driver/bson/bsonrw/bsonrwtest"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestTimeCodec(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)

	t.Run("UseLocalTimeZone", func(t *testing.T) {
		reader := &bsonrwtest.ValueReaderWriter{BSONType: bsontype.DateTime, Return: int64(now.UnixNano() / int64(time.Millisecond))}
		testCases := []struct {
			name string
			opts *bsonoptions.TimeCodecOptions
			utc  bool
		}{
			{"default", bsonoptions.TimeCodec(), true},
			{"false", bsonoptions.TimeCodec().SetUseLocalTimeZone(false), true},
			{"true", bsonoptions.TimeCodec().SetUseLocalTimeZone(true), false},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				timeCodec := NewTimeCodec(tc.opts)

				actual := reflect.New(reflect.TypeOf(now)).Elem()
				err := timeCodec.DecodeValue(DecodeContext{}, reader, actual)
				assert.Nil(t, err, "TimeCodec.DecodeValue error: %v", err)

				actualTime := actual.Interface().(time.Time)
				assert.Equal(t, actualTime.Location().String() == "UTC", tc.utc,
					"Expected UTC: %v, got %v", tc.utc, actualTime.Location())
				assert.Equal(t, now, actualTime, "expected time %v, got %v", now, actualTime)
			})
		}
	})

	t.Run("DecodeFromBsontype", func(t *testing.T) {
		testCases := []struct {
			name   string
			reader *bsonrwtest.ValueReaderWriter
		}{
			{"string", &bsonrwtest.ValueReaderWriter{BSONType: bsontype.String, Return: now.Format(timeFormatString)}},
			{"int64", &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Int64, Return: now.Unix()*1000 + int64(now.Nanosecond()/1e6)}},
			{"timestamp", &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Timestamp,
				Return: bsoncore.Value{
					Type: bsontype.Timestamp,
					Data: bsoncore.AppendTimestamp(nil, uint32(now.Unix()), 0),
				}},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				actual := reflect.New(reflect.TypeOf(now)).Elem()
				err := defaultTimeCodec.DecodeValue(DecodeContext{}, tc.reader, actual)
				assert.Nil(t, err, "DecodeValue error: %v", err)

				actualTime := actual.Interface().(time.Time)
				if tc.name == "timestamp" {
					now = time.Unix(now.Unix(), 0)
				}
				assert.Equal(t, now, actualTime, "expected time %v, got %v", now, actualTime)
			})
		}
	})
}
