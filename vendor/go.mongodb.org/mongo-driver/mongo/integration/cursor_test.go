// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	errorCursorNotFound = 43
)

func TestCursor(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().CreateClient(false))
	defer mt.Close()

	// server versions 2.6 and 3.0 use OP_GET_MORE so this works on >= 3.2
	mt.RunOpts("cursor is killed on server", mtest.NewOptions().MinServerVersion("3.2"), func(mt *mtest.T) {
		initCollection(mt, mt.Coll)
		c, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetBatchSize(2))
		assert.Nil(mt, err, "Find error: %v", err)

		id := c.ID()
		assert.True(mt, c.Next(mtest.Background), "expected Next true, got false")
		err = c.Close(mtest.Background)
		assert.Nil(mt, err, "Close error: %v", err)

		err = mt.DB.RunCommand(mtest.Background, bson.D{
			{"getMore", id},
			{"collection", mt.Coll.Name()},
		}).Err()
		ce := err.(mongo.CommandError)
		assert.Equal(mt, int32(errorCursorNotFound), ce.Code, "expected error code %v, got %v", errorCursorNotFound, ce.Code)
	})
	mt.RunOpts("try next", noClientOpts, func(mt *mtest.T) {
		mt.Run("existing non-empty batch", func(mt *mtest.T) {
			// If there's already documents in the current batch, TryNext should return true without doing a getMore

			initCollection(mt, mt.Coll)
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{})
			assert.Nil(mt, err, "Find error: %v", err)
			defer cursor.Close(mtest.Background)
			tryNextExistingBatchTest(mt, cursor)
		})
		cappedOpts := bson.D{{"capped", true}, {"size", 64 * 1024}}
		mt.RunOpts("one getMore sent", mtest.NewOptions().CollectionCreateOptions(cappedOpts), func(mt *mtest.T) {
			// If the current batch is empty, TryNext should send one getMore and return.

			// insert a document because a tailable cursor will only have a non-zero ID if the initial Find matches
			// at least one document
			_, err := mt.Coll.InsertOne(mtest.Background, bson.D{{"x", 1}})
			assert.Nil(mt, err, "InsertOne error: %v", err)

			cursor, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetCursorType(options.Tailable))
			assert.Nil(mt, err, "Find error: %v", err)
			defer cursor.Close(mtest.Background)

			// first call to TryNext should return 1 document
			assert.True(mt, cursor.TryNext(mtest.Background), "expected Next to return true, got false")
			// TryNext should attempt one getMore
			mt.ClearEvents()
			assert.False(mt, cursor.TryNext(mtest.Background), "unexpected document %v", cursor.Current)
			verifyOneGetmoreSent(mt, cursor)
		})
		mt.RunOpts("getMore error", mtest.NewOptions().ClientType(mtest.Mock), func(mt *mtest.T) {
			findRes := mtest.CreateCursorResponse(50, "foo.bar", mtest.FirstBatch)
			mt.AddMockResponses(findRes)
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{})
			assert.Nil(mt, err, "Find error: %v", err)
			defer cursor.Close(mtest.Background)
			tryNextGetmoreError(mt, cursor)
		})
	})
}

type tryNextCursor interface {
	TryNext(context.Context) bool
	Err() error
}

func tryNextExistingBatchTest(mt *mtest.T, cursor tryNextCursor) {
	mt.Helper()

	mt.ClearEvents()
	assert.True(mt, cursor.TryNext(mtest.Background), "expected TryNext to return true, got false")
	evt := mt.GetStartedEvent()
	if evt != nil {
		mt.Fatalf("unexpected event sent during TryNext: %v", evt.CommandName)
	}
}

// use command monitoring to verify that a single getMore was sent
func verifyOneGetmoreSent(mt *mtest.T, cursor tryNextCursor) {
	mt.Helper()

	evt := mt.GetStartedEvent()
	assert.NotNil(mt, evt, "expected getMore event, got nil")
	assert.Equal(mt, "getMore", evt.CommandName, "expected 'getMore' event, got '%v'", evt.CommandName)
	evt = mt.GetStartedEvent()
	if evt != nil {
		mt.Fatalf("unexpected event sent during TryNext: %v", evt.CommandName)
	}
}

// should be called in a test run with a mock deployment
func tryNextGetmoreError(mt *mtest.T, cursor tryNextCursor) {
	getMoreRes := mtest.CreateCommandErrorResponse(mtest.CommandError{
		Code:    100,
		Message: "getMore error",
		Name:    "CursorError",
		Labels:  []string{"NonResumableChangeStreamError"},
	})
	mt.AddMockResponses(getMoreRes)

	// first call to TryNext should return false because first batch was empty so batch cursor returns false
	// without doing a getMore
	// next call to TryNext should attempt a getMore
	for i := 0; i < 2; i++ {
		assert.False(mt, cursor.TryNext(mtest.Background), "TryNext returned true on iteration %v", i)
	}

	err := cursor.Err()
	assert.NotNil(mt, err, "expected change stream error, got nil")
}
