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

func TestResults(t *testing.T) {
	t.Run("delete result", func(t *testing.T) {
		t.Run("unmarshal into", func(t *testing.T) {
			doc := bson.D{
				{"n", int64(2)},
				{"ok", int64(1)},
			}

			b, err := bson.Marshal(doc)
			assert.Nil(t, err, "Marshal error: %v", err)

			var result DeleteResult
			err = bson.Unmarshal(b, &result)
			assert.Nil(t, err, "Unmarshal error: %v", err)
			assert.Equal(t, int64(2), result.DeletedCount, "expected DeletedCount 2, got %v", result.DeletedCount)
		})
		t.Run("marshal from", func(t *testing.T) {
			result := DeleteResult{DeletedCount: 1}
			buf, err := bson.Marshal(result)
			assert.Nil(t, err, "Marshal error: %v", err)

			var doc bson.D
			err = bson.Unmarshal(buf, &doc)
			assert.Nil(t, err, "Unmarshal error: %v", err)

			assert.Equal(t, 1, len(doc), "expected document length 1, got %v", len(doc))
			for _, elem := range doc {
				if elem.Key != "n" {
					continue
				}

				n, ok := elem.Value.(int64)
				assert.True(t, ok, "expected n type %T, got %T", int64(0), elem.Value)
				assert.Equal(t, int64(1), n, "expected n 1, got %v", n)
				return
			}
			t.Fatal("key n not found in document")
		})
	})
	t.Run("update result", func(t *testing.T) {
		t.Run("unmarshal into", func(t *testing.T) {
			doc := bson.D{
				{"n", 1},
				{"nModified", 2},
				{"upserted", bson.A{
					bson.D{
						{"index", 0},
						{"_id", 3},
					},
				}},
			}
			b, err := bson.Marshal(doc)
			assert.Nil(t, err, "Marshal error: %v", err)

			var result UpdateResult
			err = bson.Unmarshal(b, &result)
			assert.Nil(t, err, "Unmarshal error: %v", err)
			assert.Equal(t, int64(1), result.MatchedCount, "expected MatchedCount 1, got %v", result.MatchedCount)
			assert.Equal(t, int64(2), result.ModifiedCount, "expected ModifiedCount 2, got %v", result.ModifiedCount)
			upsertedID := result.UpsertedID.(int32)
			assert.Equal(t, int32(3), upsertedID, "expected upsertedID 3, got %v", upsertedID)
		})
	})
}
