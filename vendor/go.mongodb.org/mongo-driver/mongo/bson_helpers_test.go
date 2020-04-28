// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

// compare expected and actual BSON documents. comparison succeeds if actual contains each element in expected.
func compareDocuments(t *testing.T, expected, actual bson.Raw) {
	t.Helper()

	eElems, err := expected.Elements()
	assert.Nil(t, err, "error getting expected elements: %v", err)

	for _, e := range eElems {
		eKey := e.Key()
		aVal, err := actual.LookupErr(eKey)
		assert.Nil(t, err, "key %s not found in result", e.Key())
		compareBsonValues(t, eKey, e.Value(), aVal)
	}
}

func numberFromValue(t *testing.T, val bson.RawValue) int64 {
	switch val.Type {
	case bson.TypeInt32:
		return int64(val.Int32())
	case bson.TypeInt64:
		return val.Int64()
	case bson.TypeDouble:
		return int64(val.Double())
	default:
		t.Fatalf("unexpected type for number: %v", val.Type)
	}

	return 0
}

func compareNumberValues(t *testing.T, key string, expected, actual bson.RawValue) {
	eInt := numberFromValue(t, expected)
	aInt := numberFromValue(t, actual)
	assert.Equal(t, eInt, aInt, "value mismatch for key %s; expected %v, got %v", key, expected, actual)
}

// compare BSON values and fail if they are not equal. the key parameter is used for error strings.
// if the expected value is a numeric type (int32, int64, or double) and the value is 42, the function only asserts that
// the actual value is non-null.
func compareBsonValues(t *testing.T, key string, expected, actual bson.RawValue) {
	t.Helper()

	switch expected.Type {
	case bson.TypeInt32, bson.TypeInt64, bson.TypeDouble:
		compareNumberValues(t, key, expected, actual)
	case bson.TypeEmbeddedDocument:
		compareDocuments(t, expected.Document(), actual.Document())
	case bson.TypeArray:
		compareDocuments(t, expected.Array(), actual.Array())
	default:
		assert.Equal(t, expected.Value, actual.Value,
			"value mismatch for key %v; expected %v, got %v", key, expected.Value, actual.Value)
	}
}
