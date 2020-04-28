// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bson

import (
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

func TestExtJSON(t *testing.T) {
	timestampNegativeInt32Err := fmt.Errorf("$timestamp i number should be uint32: -1")
	timestampNegativeInt64Err := fmt.Errorf("$timestamp i number should be uint32: -2147483649")
	timestampLargeValueErr := fmt.Errorf("$timestamp i number should be uint32: 4294967296")

	testCases := []struct {
		name      string
		input     string
		canonical bool
		err       error
	}{
		{"timestamp - negative int32 value", `{"":{"$timestamp":{"t":0,"i":-1}}}`, false, timestampNegativeInt32Err},
		{"timestamp - negative int64 value", `{"":{"$timestamp":{"t":0,"i":-2147483649}}}`, false, timestampNegativeInt64Err},
		{"timestamp - value overflows uint32", `{"":{"$timestamp":{"t":0,"i":4294967296}}}`, false, timestampLargeValueErr},
		{"top level key is not treated as special", `{"$code": "foo"}`, false, nil},
		{"escaped signle quote errors", `{"f\'oo": "bar"}`, false, bsonrw.ErrInvalidJSON},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var res Raw
			err := UnmarshalExtJSON([]byte(tc.input), tc.canonical, &res)
			if tc.err == nil {
				assert.Nil(t, err, "UnmarshalExtJSON error: %v", err)
				return
			}

			assert.NotNil(t, err, "expected error %v, got nil", tc.err)
			assert.Equal(t, tc.err.Error(), err.Error(), "expected error %v, got %v", tc.err, err)
		})
	}
}
