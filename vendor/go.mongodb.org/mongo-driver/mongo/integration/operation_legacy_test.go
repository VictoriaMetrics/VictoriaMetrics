// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"bytes"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

func TestOperationLegacy(t *testing.T) {
	mt := mtest.New(t, noClientOpts)
	defer mt.Close()

	mt.RunOpts("verify wiremessage", noClientOpts, func(mt *mtest.T) {
		res := bson.D{{"ok", 1}}
		resBytes, err := bson.Marshal(res)
		assert.Nil(mt, err, "Marshal error: %v", err)
		fakeOpReply := drivertest.MakeReply(resBytes)
		// mock connection
		testConn := &drivertest.ChannelConn{
			Written:  make(chan []byte, 5),
			ReadResp: make(chan []byte, 10),
			Desc: description.Server{
				WireVersion: &description.VersionRange{
					Max: 2,
				},
			},
		}
		defer func() {
			close(testConn.Written)
			close(testConn.ReadResp)
		}()
		for i := 0; i < 10; i++ {
			testConn.ReadResp <- fakeOpReply
		}
		testClientOpts := &options.ClientOptions{Deployment: driver.SingleConnectionDeployment{C: testConn}}

		// test cases for commands that will generate an OP_QUERY
		cases := []struct {
			name  string
			cmdFn func(*mtest.T) opQuery // runs a command and returns the expected wire message
		}{
			{"find", runFindWithOptions},
			{"list collections", runListCollectionsWithOptions},
			{"list indexes", runListIndexesWithOptions},
		}
		for _, tc := range cases {
			mt.RunOpts(tc.name, mtest.NewOptions().ClientOptions(testClientOpts), func(mt *mtest.T) {
				// clear any messages written during test setup
				for len(testConn.Written) > 0 {
					<-testConn.Written
				}
				testConn.ReadResp <- fakeOpReply
				expectedQuery := tc.cmdFn(mt)

				assert.NotEqual(mt, 0, len(testConn.Written), "no message written to connection")
				validateQueryWiremessage(mt, <-testConn.Written, expectedQuery)
			})
		}
	})
	mt.RunOpts("verify results", noClientOpts, func(mt *mtest.T) {
		mt.RunOpts("find", mtest.NewOptions().MaxServerVersion("3.0"), func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetSort(bson.D{{"x", 1}}))
			assert.Nil(mt, err, "Find error: %v", err)

			for i := 1; i <= 5; i++ {
				assert.True(mt, cursor.Next(mtest.Background), "Next returned false on iteration %v", i)
				got := cursor.Current.Lookup("x").Int32()
				assert.Equal(mt, int32(i), got, "expected x value %v, got %v", i, got)
			}
			assert.False(mt, cursor.Next(mtest.Background), "found extra document %v", cursor.Current)
			err = cursor.Err()
			assert.Nil(mt, err, "cursor error: %v", err)
		})
		mt.RunOpts("list collections", mtest.NewOptions().MaxServerVersion("2.7.6").DatabaseName("test_legacy"), func(mt *mtest.T) {
			// run on a separate database to avoid finding other collections if we run these tests in parallel
			cursor, err := mt.DB.ListCollections(mtest.Background, bson.D{})
			assert.Nil(mt, err, "ListCollections error: %v", err)

			for i := 0; i < 2; i++ {
				assert.True(mt, cursor.Next(mtest.Background), "Next returned false on iteration %v", i)
				collName := cursor.Current.Lookup("name").StringValue()
				assert.True(mt, collName == mt.Coll.Name() || collName == "system.indexes",
					"unexpected collection %v", collName)
			}
			assert.False(mt, cursor.Next(mtest.Background), "found extra document %v", cursor.Current)
			err = cursor.Err()
			assert.Nil(mt, err, "cursor error: %v", err)
		})
		mt.RunOpts("list indexes", mtest.NewOptions().MaxServerVersion("2.7.6"), func(mt *mtest.T) {
			// create index so an index besides _id is found
			iv := mt.Coll.Indexes()
			indexName, err := iv.CreateOne(mtest.Background, mongo.IndexModel{
				Keys: bson.D{{"x", 1}},
			})
			assert.Nil(mt, err, "CreateOne error: %v", err)

			cursor, err := iv.List(mtest.Background)
			expectedNs := fullCollName(mt, mt.Coll.Name())
			assert.Nil(mt, err, "List error: %v", err)
			for i := 0; i < 2; i++ {
				assert.True(mt, cursor.Next(mtest.Background), "Next returned false on iteration %v", i)
				ns := cursor.Current.Lookup("ns").StringValue()
				assert.Equal(mt, expectedNs, ns, "expected ns %v, got %v", expectedNs, ns)

				name := cursor.Current.Lookup("name").StringValue()
				assert.True(mt, name == "_id_" || name == indexName, "unexpected index %v", name)
			}
			assert.False(mt, cursor.Next(mtest.Background), "found extra document %v", cursor.Current)
			err = cursor.Err()
			assert.Nil(mt, err, "cursor error: %v", err)
		})
		mt.RunOpts("find and killCursors", mtest.NewOptions().MaxServerVersion("3.0"), func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			// set batch size to force multiple batches
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetBatchSize(2))
			assert.Nil(mt, err, "Find error: %v", err)
			// close cursor to force a killCursors to be sent
			mt.ClearEvents()
			err = cursor.Close(mtest.Background)
			assert.Nil(mt, err, "Close error: %v", err)
			evt := mt.GetStartedEvent()
			assert.NotNil(mt, evt, "expected CommandStartedEvent, got nil")
			assert.Equal(mt, "killCursors", evt.CommandName, "expected command 'killCursors', got '%v'", evt.CommandName)
		})
	})
}

type opQuery struct {
	flags                       wiremessage.QueryFlag
	fullCollectionName          string
	numToSkip, numToReturn      int32
	query, returnFieldsSelector bson.D
}

func fullCollName(mt *mtest.T, coll string) string {
	return mt.DB.Name() + "." + coll
}

func runFindWithOptions(mt *mtest.T) opQuery {
	maxDoc := bson.D{{"indexBounds", bson.D{{"x", 50}}}}
	minDoc := bson.D{{"indexBounds", bson.D{{"x", 50}}}}
	projection := bson.D{{"y", 0}}
	sort := bson.D{{"x", 1}}
	filter := bson.D{{"x", 1}}

	opts := options.Find().
		SetAllowPartialResults(true).
		SetBatchSize(2).
		SetComment("hello").
		SetCursorType(options.Tailable).
		SetHint("hintFoo").
		SetLimit(5).
		SetMax(maxDoc).
		SetMaxTime(10000 * time.Millisecond).
		SetMin(minDoc).
		SetNoCursorTimeout(true).
		SetOplogReplay(true).
		SetProjection(projection).
		SetReturnKey(false).
		SetShowRecordID(false).
		SetSkip(1).
		SetSnapshot(false).
		SetSort(sort)
	_, _ = mt.Coll.Find(mtest.Background, filter, opts)

	// find expectations
	findQueryDoc := bson.D{
		{"$query", filter},
		{"$comment", "hello"},
		{"$hint", "hintFoo"},
		{"$max", maxDoc},
		{"$maxTimeMS", int64(10000)},
		{"$min", minDoc},
		{"$returnKey", false},
		{"$showDiskLoc", false},
		{"$snapshot", false},
		{"$orderby", sort},
	}
	return opQuery{
		flags:                wiremessage.QueryFlag(wiremessage.Partial | wiremessage.TailableCursor | wiremessage.NoCursorTimeout | wiremessage.OplogReplay | wiremessage.SlaveOK),
		fullCollectionName:   fullCollName(mt, mt.Coll.Name()),
		numToSkip:            1,
		numToReturn:          2,
		query:                findQueryDoc,
		returnFieldsSelector: projection,
	}
}

func runListCollectionsWithOptions(mt *mtest.T) opQuery {
	_, _ = mt.DB.ListCollections(mtest.Background, bson.D{{"name", "foo"}})

	regexDoc := bson.D{{"name", primitive.Regex{Pattern: "^[^$]*$"}}}
	modifiedFilterDoc := bson.D{{"name", fullCollName(mt, "foo")}}
	listCollDoc := bson.D{
		{"$and", bson.A{regexDoc, modifiedFilterDoc}},
	}
	return opQuery{
		flags:              wiremessage.SlaveOK,
		fullCollectionName: fullCollName(mt, "system.namespaces"),
		query:              listCollDoc,
	}
}

func runListIndexesWithOptions(mt *mtest.T) opQuery {
	_, _ = mt.Coll.Indexes().List(mtest.Background, options.ListIndexes().SetBatchSize(2).SetMaxTime(10000*time.Millisecond))

	listIndexesDoc := bson.D{
		{"$query", bson.D{{"ns", fullCollName(mt, mt.Coll.Name())}}},
		{"$maxTimeMS", int64(10000)},
	}
	return opQuery{
		flags:              wiremessage.SlaveOK,
		fullCollectionName: fullCollName(mt, "system.indexes"),
		numToReturn:        2,
		query:              listIndexesDoc,
	}
}

func validateHeader(mt *mtest.T, wm []byte, expectedOpcode wiremessage.OpCode) []byte {
	mt.Helper()

	actualLen := len(wm)
	var readLen int32
	var opcode wiremessage.OpCode
	var ok bool

	readLen, _, _, opcode, wm, ok = wiremessage.ReadHeader(wm)
	assert.True(mt, ok, "could not read header")
	assert.Equal(mt, int32(actualLen), readLen, "expected header length %v, got %v", actualLen, readLen)
	assert.Equal(mt, expectedOpcode, opcode, "expected opcode %v, got %v", expectedOpcode, opcode)
	return wm
}

func validateQueryWiremessage(mt *mtest.T, wm []byte, expected opQuery) {
	mt.Helper()

	var numToSkip, numToReturn int32
	var flags wiremessage.QueryFlag
	var fullCollName string
	var query, returnFieldsSelector bsoncore.Document
	var ok bool

	wm = validateHeader(mt, wm, wiremessage.OpQuery)

	flags, wm, ok = wiremessage.ReadQueryFlags(wm)
	assert.True(mt, ok, "could not read flags")
	assert.Equal(mt, expected.flags, flags, "expected query flags %v, got %v", expected.flags, flags)

	fullCollName, wm, ok = wiremessage.ReadQueryFullCollectionName(wm)
	assert.True(mt, ok, "could not read fullCollectionName")
	assert.Equal(mt, expected.fullCollectionName, fullCollName, "expected namespace %v, got %v", expected.fullCollectionName, fullCollName)

	numToSkip, wm, ok = wiremessage.ReadQueryNumberToSkip(wm)
	assert.True(mt, ok, "could not read numToSkip")
	assert.Equal(mt, expected.numToSkip, numToSkip, "expected skip %v, got %v", expected.numToSkip, numToSkip)

	numToReturn, wm, ok = wiremessage.ReadQueryNumberToReturn(wm)
	assert.True(mt, ok, "could not read numToReturn")
	assert.Equal(mt, expected.numToReturn, numToReturn, "expected num to return %v, got %v", expected.numToReturn, numToReturn)

	query, wm, ok = wiremessage.ReadQueryQuery(wm)
	assert.True(mt, ok, "could not read query document")
	expectedQueryBytes, err := bson.Marshal(expected.query)
	assert.True(mt, bytes.Equal(query, expectedQueryBytes), "expected query %v, got %v", bsoncore.Document(expectedQueryBytes), query)

	if len(expected.returnFieldsSelector) == 0 {
		assert.Equal(mt, 0, len(wm), "wire message had extraneous bytes")
		return
	}

	returnFieldsSelector, wm, ok = wiremessage.ReadQueryReturnFieldsSelector(wm)
	assert.True(mt, ok, "could not read returnFieldsSelector")
	assert.Equal(mt, 0, len(wm), "wire message had extraneous bytes after return fields selector")

	expectedRfsBytes, err := bson.Marshal(expected.returnFieldsSelector)
	assert.Nil(mt, err, "Marshal error for return fields selector: %v", err)
	assert.True(mt, bytes.Equal(returnFieldsSelector, expectedRfsBytes),
		"expected return fields selector %v, got %v", bsoncore.Document(expectedRfsBytes), returnFieldsSelector)
}
