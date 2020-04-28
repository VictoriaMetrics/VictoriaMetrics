// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

func TestSingleResult(t *testing.T) {
	t.Run("Decode", func(t *testing.T) {
		t.Run("decode twice", func(t *testing.T) {
			// Test that Decode and DecodeBytes can be called more than once
			c, err := newCursor(newTestBatchCursor(1, 1), bson.DefaultRegistry)
			assert.Nil(t, err, "newCursor error: %v", err)

			sr := &SingleResult{cur: c, reg: bson.DefaultRegistry}
			var firstDecode, secondDecode bson.Raw
			err = sr.Decode(&firstDecode)
			assert.Nil(t, err, "Decode error: %v", err)
			err = sr.Decode(&secondDecode)
			assert.Nil(t, err, "Decode error: %v", err)

			decodeBytes, err := sr.DecodeBytes()
			assert.Nil(t, err, "DecodeBytes error: %v", err)

			assert.Equal(t, firstDecode, secondDecode, "expected contents %v, got %v", firstDecode, secondDecode)
			assert.Equal(t, firstDecode, decodeBytes, "expected contents %v, got %v", firstDecode, decodeBytes)
		})
		t.Run("decode with error", func(t *testing.T) {
			r := []byte("foo")
			sr := &SingleResult{rdr: r, err: errors.New("DecodeBytes error")}
			res, err := sr.DecodeBytes()
			resBytes := []byte(res)
			assert.Equal(t, r, resBytes, "expected contents %v, got %v", r, resBytes)
			assert.Equal(t, sr.err, err, "expected error %v, got %v", sr.err, err)
		})
	})

	t.Run("Err", func(t *testing.T) {
		sr := &SingleResult{}
		assert.Equal(t, ErrNoDocuments, sr.Err(), "expected error %v, got %v", ErrNoDocuments, sr.Err())
	})
}
