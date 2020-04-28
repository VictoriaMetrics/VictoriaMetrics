// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bson

import (
	"fmt"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestRawValue(t *testing.T) {
	t.Run("Unmarshal", func(t *testing.T) {
		t.Run("Uses registry attached to value", func(t *testing.T) {
			reg := bsoncodec.NewRegistryBuilder().Build()
			val := RawValue{Type: bsontype.String, Value: bsoncore.AppendString(nil, "foobar"), r: reg}
			var s string
			want := bsoncodec.ErrNoDecoder{Type: reflect.TypeOf(s)}
			got := val.Unmarshal(&s)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Uses default registry if no registry attached", func(t *testing.T) {
			want := "foobar"
			val := RawValue{Type: bsontype.String, Value: bsoncore.AppendString(nil, want)}
			var got string
			err := val.Unmarshal(&got)
			noerr(t, err)
			if got != want {
				t.Errorf("Expected strings to match. got %s; want %s", got, want)
			}
		})
	})
	t.Run("UnmarshalWithRegistry", func(t *testing.T) {
		t.Run("Returns error when registry is nil", func(t *testing.T) {
			want := ErrNilRegistry
			var val RawValue
			got := val.UnmarshalWithRegistry(nil, &D{})
			if got != want {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Returns lookup error", func(t *testing.T) {
			reg := bsoncodec.NewRegistryBuilder().Build()
			var val RawValue
			var s string
			want := bsoncodec.ErrNoDecoder{Type: reflect.TypeOf(s)}
			got := val.UnmarshalWithRegistry(reg, &s)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Returns DecodeValue error", func(t *testing.T) {
			reg := NewRegistryBuilder().Build()
			val := RawValue{Type: bsontype.Double, Value: bsoncore.AppendDouble(nil, 3.14159)}
			var s string
			want := fmt.Errorf("cannot decode %v into a string type", bsontype.Double)
			got := val.UnmarshalWithRegistry(reg, &s)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Success", func(t *testing.T) {
			reg := NewRegistryBuilder().Build()
			want := float64(3.14159)
			val := RawValue{Type: bsontype.Double, Value: bsoncore.AppendDouble(nil, want)}
			var got float64
			err := val.UnmarshalWithRegistry(reg, &got)
			noerr(t, err)
			if got != want {
				t.Errorf("Expected results to match. got %g; want %g", got, want)
			}
		})
	})
	t.Run("UnmarshalWithContext", func(t *testing.T) {
		t.Run("Returns error when DecodeContext is nil", func(t *testing.T) {
			want := ErrNilContext
			var val RawValue
			got := val.UnmarshalWithContext(nil, &D{})
			if got != want {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Returns lookup error", func(t *testing.T) {
			dc := bsoncodec.DecodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()}
			var val RawValue
			var s string
			want := bsoncodec.ErrNoDecoder{Type: reflect.TypeOf(s)}
			got := val.UnmarshalWithContext(&dc, &s)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Returns DecodeValue error", func(t *testing.T) {
			dc := bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()}
			val := RawValue{Type: bsontype.Double, Value: bsoncore.AppendDouble(nil, 3.14159)}
			var s string
			want := fmt.Errorf("cannot decode %v into a string type", bsontype.Double)
			got := val.UnmarshalWithContext(&dc, &s)
			if !compareErrors(got, want) {
				t.Errorf("Expected errors to match. got %v; want %v", got, want)
			}
		})
		t.Run("Success", func(t *testing.T) {
			dc := bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()}
			want := float64(3.14159)
			val := RawValue{Type: bsontype.Double, Value: bsoncore.AppendDouble(nil, want)}
			var got float64
			err := val.UnmarshalWithContext(&dc, &got)
			noerr(t, err)
			if got != want {
				t.Errorf("Expected results to match. got %g; want %g", got, want)
			}
		})
	})
}
