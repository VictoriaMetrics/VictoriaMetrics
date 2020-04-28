// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
)

type testBatchCursor struct {
	batches []*bsoncore.DocumentSequence
	batch   *bsoncore.DocumentSequence
	closed  bool
}

func newTestBatchCursor(numBatches, batchSize int) *testBatchCursor {
	batches := make([]*bsoncore.DocumentSequence, 0, numBatches)

	counter := 0
	for batch := 0; batch < numBatches; batch++ {
		var docSequence []byte

		for doc := 0; doc < batchSize; doc++ {
			var elem []byte
			elem = bsoncore.AppendInt32Element(elem, "foo", int32(counter))
			counter++

			var doc []byte
			doc = bsoncore.BuildDocumentFromElements(doc, elem)
			docSequence = append(docSequence, doc...)
		}

		batches = append(batches, &bsoncore.DocumentSequence{
			Style: bsoncore.SequenceStyle,
			Data:  docSequence,
		})
	}

	return &testBatchCursor{
		batches: batches,
	}
}

func (tbc *testBatchCursor) ID() int64 {
	if len(tbc.batches) == 0 {
		return 0 // cursor exhausted
	}

	return 10
}

func (tbc *testBatchCursor) Next(context.Context) bool {
	if len(tbc.batches) == 0 {
		return false
	}

	tbc.batch = tbc.batches[0]
	tbc.batches = tbc.batches[1:]
	return true
}

func (tbc *testBatchCursor) Batch() *bsoncore.DocumentSequence {
	return tbc.batch
}

func (tbc *testBatchCursor) Server() driver.Server {
	return nil
}

func (tbc *testBatchCursor) Err() error {
	return nil
}

func (tbc *testBatchCursor) Close(context.Context) error {
	tbc.closed = true
	return nil
}

func TestCursor(t *testing.T) {
	t.Run("loops until docs available", func(t *testing.T) {})
	t.Run("returns false on context cancellation", func(t *testing.T) {})
	t.Run("returns false if error occurred", func(t *testing.T) {})
	t.Run("returns false if ID is zero and no more docs", func(t *testing.T) {})

	t.Run("TestAll", func(t *testing.T) {
		t.Run("errors if argument is not pointer to slice", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(1, 5), nil)
			assert.Nil(t, err, "newCursor error: %v", err)
			err = cursor.All(context.Background(), []bson.D{})
			assert.NotNil(t, err, "expected error, got nil")
		})

		t.Run("fills slice with all documents", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(1, 5), nil)
			assert.Nil(t, err, "newCursor error: %v", err)

			var docs []bson.D
			err = cursor.All(context.Background(), &docs)
			assert.Nil(t, err, "All error: %v", err)
			assert.Equal(t, 5, len(docs), "expected 5 docs, got %v", len(docs))

			for index, doc := range docs {
				expected := bson.D{{"foo", int32(index)}}
				assert.Equal(t, expected, doc, "expected doc %v, got %v", expected, doc)
			}
		})

		t.Run("decodes each document into slice type", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(1, 5), nil)
			assert.Nil(t, err, "newCursor error: %v", err)

			type Document struct {
				Foo int32 `bson:"foo"`
			}
			var docs []Document
			err = cursor.All(context.Background(), &docs)
			assert.Nil(t, err, "All error: %v", err)
			assert.Equal(t, 5, len(docs), "expected 5 documents, got %v", len(docs))

			for index, doc := range docs {
				expected := Document{Foo: int32(index)}
				assert.Equal(t, expected, doc, "expected doc %v, got %v", expected, doc)
			}
		})

		t.Run("multiple batches are included", func(t *testing.T) {
			cursor, err := newCursor(newTestBatchCursor(2, 5), nil)
			assert.Nil(t, err, "newCursor error: %v", err)
			var docs []bson.D
			err = cursor.All(context.Background(), &docs)
			assert.Nil(t, err, "All error: %v", err)
			assert.Equal(t, 10, len(docs), "expected 10 docs, got %v", len(docs))

			for index, doc := range docs {
				expected := bson.D{{"foo", int32(index)}}
				assert.Equal(t, expected, doc, "expected doc %v, got %v", expected, doc)
			}
		})

		t.Run("cursor is closed after All is called", func(t *testing.T) {
			var docs []bson.D

			tbc := newTestBatchCursor(1, 5)
			cursor, err := newCursor(tbc, nil)
			assert.Nil(t, err, "newCursor error: %v", err)

			err = cursor.All(context.Background(), &docs)
			assert.Nil(t, err, "All error: %v", err)
			assert.True(t, tbc.closed, "expected batch cursor to be closed but was not")
		})
	})
}
