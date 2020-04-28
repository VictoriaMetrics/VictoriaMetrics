// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

const (
	errorDuplicateKey           = 11000
	errorCappedCollDeleteLegacy = 10101
	errorCappedCollDelete       = 20
	errorModifiedIDLegacy       = 16837
	errorModifiedID             = 66
)

func TestCollection(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().CreateClient(false))
	defer mt.Close()

	// impossibleWc is a write concern that can't be satisfied and is used to test write concern errors
	// for various operations. It includes a timeout because legacy servers will wait for all W nodes to respond,
	// causing tests to hang.
	impossibleWc := writeconcern.New(writeconcern.W(30), writeconcern.WTimeout(time.Second))

	mt.RunOpts("insert one", noClientOpts, func(mt *mtest.T) {
		mt.Run("success", func(mt *mtest.T) {
			id := primitive.NewObjectID()
			doc := bson.D{{"_id", id}, {"x", 1}}
			res, err := mt.Coll.InsertOne(mtest.Background, doc)
			assert.Nil(mt, err, "InsertOne error: %v", err)
			assert.Equal(mt, id, res.InsertedID, "expected inserted ID %v, got %v", id, res.InsertedID)
		})
		mt.Run("write error", func(mt *mtest.T) {
			doc := bson.D{{"_id", 1}}
			_, err := mt.Coll.InsertOne(mtest.Background, doc)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			_, err = mt.Coll.InsertOne(mtest.Background, doc)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %T, got %T", mongo.WriteException{}, err)
			assert.Equal(mt, 1, len(we.WriteErrors), "expected 1 write error, got %v", len(we.WriteErrors))
			writeErr := we.WriteErrors[0]
			assert.Equal(mt, errorDuplicateKey, writeErr.Code, "expected code %v, got %v", errorDuplicateKey, writeErr.Code)
		})

		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().CollectionOptions(wcCollOpts).Topologies(mtest.ReplicaSet)
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			doc := bson.D{{"_id", 1}}
			_, err := mt.Coll.InsertOne(mtest.Background, doc)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %+v", we)
		})
	})
	mt.RunOpts("insert many", noClientOpts, func(mt *mtest.T) {
		mt.Run("success", func(mt *mtest.T) {
			want1 := int32(11)
			want2 := int32(12)
			docs := []interface{}{
				bson.D{{"_id", want1}},
				bson.D{{"x", 6}},
				bson.D{{"_id", want2}},
			}

			res, err := mt.Coll.InsertMany(mtest.Background, docs)
			assert.Nil(mt, err, "InsertMany error: %v", err)
			assert.Equal(mt, 3, len(res.InsertedIDs), "expected 3 inserted IDs, got %v", len(res.InsertedIDs))
			assert.Equal(mt, want1, res.InsertedIDs[0], "expected inserted ID %v, got %v", want1, res.InsertedIDs[0])
			assert.NotNil(mt, res.InsertedIDs[1], "expected ID but got nil")
			assert.Equal(mt, want2, res.InsertedIDs[2], "expected inserted ID %v, got %v", want2, res.InsertedIDs[2])
		})
		mt.Run("batches", func(mt *mtest.T) {
			// TODO(GODRIVER-425): remove this as part a larger project to
			// refactor integration and other longrunning tasks.
			if os.Getenv("EVR_TASK_ID") == "" {
				mt.Skip("skipping long running integration test outside of evergreen")
			}

			const (
				megabyte = 10 * 10 * 10 * 10 * 10 * 10
				numDocs  = 700000
			)
			var docs []interface{}
			total := uint32(0)
			expectedDocSize := uint32(26)
			for i := 0; i < numDocs; i++ {
				d := bson.D{
					{"a", int32(i)},
					{"b", int32(i * 2)},
					{"c", int32(i * 3)},
				}
				b, _ := bson.Marshal(d)
				assert.Equal(mt, int(expectedDocSize), len(b), "expected doc len %v, got %v", expectedDocSize, len(b))
				docs = append(docs, d)
				total += uint32(len(b))
			}
			assert.True(mt, total > 16*megabyte, "expected total greater than 16mb but got %v", total)
			res, err := mt.Coll.InsertMany(context.Background(), docs)
			assert.Nil(mt, err, "InsertMany error: %v", err)
			assert.Equal(mt, numDocs, len(res.InsertedIDs), "expected %v inserted IDs, got %v", numDocs, len(res.InsertedIDs))
		})
		mt.Run("large document batches", func(mt *mtest.T) {
			// TODO(GODRIVER-425): remove this as part a larger project to
			// refactor integration and other longrunning tasks.
			if os.Getenv("EVR_TASK_ID") == "" {
				mt.Skip("skipping long running integration test outside of evergreen")
			}

			docs := []interface{}{create16MBDocument(mt), create16MBDocument(mt)}
			_, err := mt.Coll.InsertMany(mtest.Background, docs)
			assert.Nil(mt, err, "InsertMany error: %v", err)
			evt := mt.GetStartedEvent()
			assert.Equal(mt, "insert", evt.CommandName, "expected 'insert' event, got '%v'", evt.CommandName)
			evt = mt.GetStartedEvent()
			assert.Equal(mt, "insert", evt.CommandName, "expected 'insert' event, got '%v'", evt.CommandName)
		})
		mt.RunOpts("write error", noClientOpts, func(mt *mtest.T) {
			docs := []interface{}{
				bson.D{{"_id", primitive.NewObjectID()}},
				bson.D{{"_id", primitive.NewObjectID()}},
				bson.D{{"_id", primitive.NewObjectID()}},
			}

			testCases := []struct {
				name      string
				ordered   bool
				numErrors int
			}{
				{"unordered", false, 3},
				{"ordered", true, 1},
			}
			for _, tc := range testCases {
				mt.Run(tc.name, func(mt *mtest.T) {
					_, err := mt.Coll.InsertMany(mtest.Background, docs)
					assert.Nil(mt, err, "InsertMany error: %v", err)
					_, err = mt.Coll.InsertMany(mtest.Background, docs, options.InsertMany().SetOrdered(tc.ordered))

					we, ok := err.(mongo.BulkWriteException)
					assert.True(mt, ok, "expected error type %T, got %T", mongo.BulkWriteException{}, err)
					numErrors := len(we.WriteErrors)
					assert.Equal(mt, tc.numErrors, numErrors, "expected %v write errors, got %v", tc.numErrors, numErrors)
					gotCode := we.WriteErrors[0].Code
					assert.Equal(mt, errorDuplicateKey, gotCode, "expected error code %v, got %v", errorDuplicateKey, gotCode)
				})
			}
		})
		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().CollectionOptions(wcCollOpts).Topologies(mtest.ReplicaSet)
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			_, err := mt.Coll.InsertMany(mtest.Background, []interface{}{bson.D{{"_id", 1}}})
			we, ok := err.(mongo.BulkWriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.BulkWriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %+v", err)
		})
	})
	mt.RunOpts("delete one", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			res, err := mt.Coll.DeleteOne(mtest.Background, bson.D{{"x", 1}})
			assert.Nil(mt, err, "DeleteOne error: %v", err)
			assert.Equal(mt, int64(1), res.DeletedCount, "expected DeletedCount 1, got %v", res.DeletedCount)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			res, err := mt.Coll.DeleteOne(mtest.Background, bson.D{{"x", 0}})
			assert.Nil(mt, err, "DeleteOne error: %v", err)
			assert.Equal(mt, int64(0), res.DeletedCount, "expected DeletedCount 0, got %v", res.DeletedCount)
		})
		mt.RunOpts("not found with options", mtest.NewOptions().MinServerVersion("3.4"), func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			opts := options.Delete().SetCollation(&options.Collation{Locale: "en_US"})
			res, err := mt.Coll.DeleteOne(mtest.Background, bson.D{{"x", 0}}, opts)
			assert.Nil(mt, err, "DeleteOne error: %v", err)
			assert.Equal(mt, int64(0), res.DeletedCount, "expected DeletedCount 0, got %v", res.DeletedCount)
		})
		mt.Run("write error", func(mt *mtest.T) {
			cappedOpts := bson.D{{"capped", true}, {"size", 64 * 1024}}
			capped := mt.CreateCollection(mtest.Collection{
				Name:       "deleteOne_capped",
				CreateOpts: cappedOpts,
			}, true)
			_, err := capped.DeleteOne(mtest.Background, bson.D{{"x", 1}})

			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %T, got %T", mongo.WriteException{}, err)
			numWriteErrors := len(we.WriteErrors)
			assert.Equal(mt, 1, numWriteErrors, "expected 1 write error, got %v", numWriteErrors)
			gotCode := we.WriteErrors[0].Code
			assert.True(mt, gotCode == errorCappedCollDeleteLegacy || gotCode == errorCappedCollDelete,
				"expected error code %v or %v, got %v", errorCappedCollDeleteLegacy, errorCappedCollDelete, gotCode)
		})
		mt.RunOpts("write concern error", mtest.NewOptions().Topologies(mtest.ReplicaSet), func(mt *mtest.T) {
			// 2.6 returns right away if the document doesn't exist
			filter := bson.D{{"x", 1}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			mt.CloneCollection(options.Collection().SetWriteConcern(impossibleWc))
			_, err = mt.Coll.DeleteOne(mtest.Background, filter)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %T, got %T", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got nil")
		})
	})
	mt.RunOpts("delete many", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			res, err := mt.Coll.DeleteMany(mtest.Background, bson.D{{"x", bson.D{{"$gte", 3}}}})
			assert.Nil(mt, err, "DeleteMany error: %v", err)
			assert.Equal(mt, int64(3), res.DeletedCount, "expected DeletedCount 3, got %v", res.DeletedCount)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			res, err := mt.Coll.DeleteMany(mtest.Background, bson.D{{"x", bson.D{{"$lt", 1}}}})
			assert.Nil(mt, err, "DeleteMany error: %v", err)
			assert.Equal(mt, int64(0), res.DeletedCount, "expected DeletedCount 0, got %v", res.DeletedCount)
		})
		mt.RunOpts("not found with options", mtest.NewOptions().MinServerVersion("3.4"), func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			opts := options.Delete().SetCollation(&options.Collation{Locale: "en_US"})
			res, err := mt.Coll.DeleteMany(mtest.Background, bson.D{{"x", bson.D{{"$lt", 1}}}}, opts)
			assert.Nil(mt, err, "DeleteMany error: %v", err)
			assert.Equal(mt, int64(0), res.DeletedCount, "expected DeletedCount 0, got %v", res.DeletedCount)
		})
		mt.Run("write error", func(mt *mtest.T) {
			cappedOpts := bson.D{{"capped", true}, {"size", 64 * 1024}}
			capped := mt.CreateCollection(mtest.Collection{
				Name:       "deleteMany_capped",
				CreateOpts: cappedOpts,
			}, true)
			_, err := capped.DeleteMany(mtest.Background, bson.D{{"x", 1}})

			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			numWriteErrors := len(we.WriteErrors)
			assert.Equal(mt, 1, len(we.WriteErrors), "expected 1 write error, got %v", numWriteErrors)
			gotCode := we.WriteErrors[0].Code
			assert.True(mt, gotCode == errorCappedCollDeleteLegacy || gotCode == errorCappedCollDelete,
				"expected error code %v or %v, got %v", errorCappedCollDeleteLegacy, errorCappedCollDelete, gotCode)
		})
		mt.RunOpts("write concern error", mtest.NewOptions().Topologies(mtest.ReplicaSet), func(mt *mtest.T) {
			// 2.6 server returns right away if the document doesn't exist
			filter := bson.D{{"x", 1}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			mt.CloneCollection(options.Collection().SetWriteConcern(impossibleWc))
			_, err = mt.Coll.DeleteMany(mtest.Background, filter)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %+v", err)
		})
	})
	mt.RunOpts("update one", noClientOpts, func(mt *mtest.T) {
		mt.Run("empty update", func(mt *mtest.T) {
			_, err := mt.Coll.UpdateOne(mtest.Background, bson.D{}, bson.D{})
			assert.NotNil(mt, err, "expected error, got nil")
		})
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 1}}
			update := bson.D{{"$inc", bson.D{{"x", 1}}}}

			res, err := mt.Coll.UpdateOne(mtest.Background, filter, update)
			assert.Nil(mt, err, "UpdateOne error: %v", err)
			assert.Equal(mt, int64(1), res.MatchedCount, "expected matched count 1, got %v", res.MatchedCount)
			assert.Equal(mt, int64(1), res.ModifiedCount, "expected matched count 1, got %v", res.ModifiedCount)
			assert.Nil(mt, res.UpsertedID, "expected upserted ID nil, got %v", res.UpsertedID)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 0}}
			update := bson.D{{"$inc", bson.D{{"x", 1}}}}

			res, err := mt.Coll.UpdateOne(mtest.Background, filter, update)
			assert.Nil(mt, err, "UpdateOne error: %v", err)
			assert.Equal(mt, int64(0), res.MatchedCount, "expected matched count 0, got %v", res.MatchedCount)
			assert.Equal(mt, int64(0), res.ModifiedCount, "expected matched count 0, got %v", res.ModifiedCount)
			assert.Nil(mt, res.UpsertedID, "expected upserted ID nil, got %v", res.UpsertedID)
		})
		mt.Run("upsert", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 0}}
			update := bson.D{{"$inc", bson.D{{"x", 1}}}}

			res, err := mt.Coll.UpdateOne(mtest.Background, filter, update, options.Update().SetUpsert(true))
			assert.Nil(mt, err, "UpdateOne error: %v", err)
			assert.Equal(mt, int64(0), res.MatchedCount, "expected matched count 0, got %v", res.MatchedCount)
			assert.Equal(mt, int64(0), res.ModifiedCount, "expected matched count 0, got %v", res.ModifiedCount)
			assert.NotNil(mt, res.UpsertedID, "expected upserted ID, got nil")
		})
		mt.Run("write error", func(mt *mtest.T) {
			filter := bson.D{{"_id", "foo"}}
			update := bson.D{{"$set", bson.D{{"_id", 3.14159}}}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			_, err = mt.Coll.UpdateOne(mtest.Background, filter, update)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			numWriteErrors := len(we.WriteErrors)
			assert.Equal(mt, 1, numWriteErrors, "expected 1 write error, got %v", numWriteErrors)
			gotCode := we.WriteErrors[0].Code
			assert.Equal(mt, errorModifiedID, gotCode, "expected error code %v, got %v", errorModifiedID, gotCode)
		})
		mt.RunOpts("write concern error", mtest.NewOptions().Topologies(mtest.ReplicaSet), func(mt *mtest.T) {
			// 2.6 returns right away if the document doesn't exist
			filter := bson.D{{"_id", "foo"}}
			update := bson.D{{"$set", bson.D{{"pi", 3.14159}}}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			mt.CloneCollection(options.Collection().SetWriteConcern(impossibleWc))
			_, err = mt.Coll.UpdateOne(mtest.Background, filter, update)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %+v", err)
		})
		mt.RunOpts("special slice types", noClientOpts, func(mt *mtest.T) {
			// test special types that should be converted to a document for updates even though the underlying type is
			// a slice/array
			doc := bson.D{{"$set", bson.D{{"x", 2}}}}
			docBytes, err := bson.Marshal(doc)
			assert.Nil(mt, err, "Marshal error: %v", err)
			xUpdate := bsonx.Doc{{"x", bsonx.Int32(2)}}
			xDoc := bsonx.Doc{{"$set", bsonx.Document(xUpdate)}}

			testCases := []struct {
				name   string
				update interface{}
			}{
				{"bsoncore Document", bsoncore.Document(docBytes)},
				{"bson Raw", bson.Raw(docBytes)},
				{"bson D", doc},
				{"bsonx Document", xDoc},
				{"byte slice", docBytes},
			}
			for _, tc := range testCases {
				mt.Run(tc.name, func(mt *mtest.T) {
					filter := bson.D{{"x", 1}}
					_, err := mt.Coll.InsertOne(mtest.Background, filter)
					assert.Nil(mt, err, "InsertOne error: %v", err)

					res, err := mt.Coll.UpdateOne(mtest.Background, filter, tc.update)
					assert.Nil(mt, err, "UpdateOne error: %v", err)
					assert.Equal(mt, int64(1), res.MatchedCount, "expected matched count 1, got %v", res.MatchedCount)
					assert.Equal(mt, int64(1), res.ModifiedCount, "expected modified count 1, got %v", res.ModifiedCount)
				})
			}
		})
	})
	mt.RunOpts("update many", noClientOpts, func(mt *mtest.T) {
		mt.Run("empty update", func(mt *mtest.T) {
			_, err := mt.Coll.UpdateMany(mtest.Background, bson.D{}, bson.D{})
			assert.NotNil(mt, err, "expected error, got nil")
		})
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", bson.D{{"$gte", 3}}}}
			update := bson.D{{"$inc", bson.D{{"x", 1}}}}

			res, err := mt.Coll.UpdateMany(mtest.Background, filter, update)
			assert.Nil(mt, err, "UpdateMany error: %v", err)
			assert.Equal(mt, int64(3), res.MatchedCount, "expected matched count 3, got %v", res.MatchedCount)
			assert.Equal(mt, int64(3), res.ModifiedCount, "expected modified count 3, got %v", res.ModifiedCount)
			assert.Nil(mt, res.UpsertedID, "expected upserted ID nil, got %v", res.UpsertedID)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", bson.D{{"$lt", 1}}}}
			update := bson.D{{"$inc", bson.D{{"x", 1}}}}

			res, err := mt.Coll.UpdateMany(mtest.Background, filter, update)
			assert.Nil(mt, err, "UpdateMany error: %v", err)
			assert.Equal(mt, int64(0), res.MatchedCount, "expected matched count 0, got %v", res.MatchedCount)
			assert.Equal(mt, int64(0), res.ModifiedCount, "expected modified count 0, got %v", res.ModifiedCount)
			assert.Nil(mt, res.UpsertedID, "expected upserted ID nil, got %v", res.UpsertedID)
		})
		mt.Run("upsert", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", bson.D{{"$lt", 1}}}}
			update := bson.D{{"$inc", bson.D{{"x", 1}}}}

			res, err := mt.Coll.UpdateMany(mtest.Background, filter, update, options.Update().SetUpsert(true))
			assert.Nil(mt, err, "UpdateMany error: %v", err)
			assert.Equal(mt, int64(0), res.MatchedCount, "expected matched count 0, got %v", res.MatchedCount)
			assert.Equal(mt, int64(0), res.ModifiedCount, "expected modified count 0, got %v", res.ModifiedCount)
			assert.NotNil(mt, res.UpsertedID, "expected upserted ID, got nil")
		})
		mt.Run("write error", func(mt *mtest.T) {
			filter := bson.D{{"_id", "foo"}}
			update := bson.D{{"$set", bson.D{{"_id", 3.14159}}}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			_, err = mt.Coll.UpdateMany(mtest.Background, filter, update)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			numWriteErrors := len(we.WriteErrors)
			assert.Equal(mt, 1, numWriteErrors, "expected 1 write error, got %v", numWriteErrors)
			gotCode := we.WriteErrors[0].Code
			assert.Equal(mt, errorModifiedID, gotCode, "expected error code %v, got %v", errorModifiedID, gotCode)
		})
		mt.RunOpts("write concern error", mtest.NewOptions().Topologies(mtest.ReplicaSet), func(mt *mtest.T) {
			// 2.6 returns right away if the document doesn't exist
			filter := bson.D{{"_id", "foo"}}
			update := bson.D{{"$set", bson.D{{"pi", 3.14159}}}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			mt.CloneCollection(options.Collection().SetWriteConcern(impossibleWc))
			_, err = mt.Coll.UpdateMany(mtest.Background, filter, update)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %+v", we)
		})
	})
	mt.RunOpts("replace one", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 1}}
			replacement := bson.D{{"y", 1}}

			res, err := mt.Coll.ReplaceOne(mtest.Background, filter, replacement)
			assert.Nil(mt, err, "ReplaceOne error: %v", err)
			assert.Equal(mt, int64(1), res.MatchedCount, "expected matched count 1, got %v", res.MatchedCount)
			assert.Equal(mt, int64(1), res.ModifiedCount, "expected modified count 1, got %v", res.ModifiedCount)
			assert.Nil(mt, res.UpsertedID, "expected upserted ID nil, got %v", res.UpsertedID)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 0}}
			replacement := bson.D{{"y", 1}}

			res, err := mt.Coll.ReplaceOne(mtest.Background, filter, replacement)
			assert.Nil(mt, err, "ReplaceOne error: %v", err)
			assert.Equal(mt, int64(0), res.MatchedCount, "expected matched count 0, got %v", res.MatchedCount)
			assert.Equal(mt, int64(0), res.ModifiedCount, "expected modified count 0, got %v", res.ModifiedCount)
			assert.Nil(mt, res.UpsertedID, "expected upserted ID nil, got %v", res.UpsertedID)
		})
		mt.Run("upsert", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 0}}
			replacement := bson.D{{"y", 1}}

			res, err := mt.Coll.ReplaceOne(mtest.Background, filter, replacement, options.Replace().SetUpsert(true))
			assert.Nil(mt, err, "ReplaceOne error: %v", err)
			assert.Equal(mt, int64(0), res.MatchedCount, "expected matched count 0, got %v", res.MatchedCount)
			assert.Equal(mt, int64(0), res.ModifiedCount, "expected modified count 0, got %v", res.ModifiedCount)
			assert.NotNil(mt, res.UpsertedID, "expected upserted ID, got nil")
		})
		mt.Run("write error", func(mt *mtest.T) {
			filter := bsonx.Doc{{"_id", bsonx.String("foo")}}
			replacement := bsonx.Doc{{"_id", bsonx.Double(3.14159)}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			_, err = mt.Coll.ReplaceOne(mtest.Background, filter, replacement)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			numWriteErrors := len(we.WriteErrors)
			assert.Equal(mt, 1, numWriteErrors, "expected 1 write error, got %v", numWriteErrors)
			gotCode := we.WriteErrors[0].Code
			assert.True(mt, gotCode == errorModifiedID || gotCode == errorModifiedIDLegacy,
				"expected error code %v or %v, got %v", errorModifiedID, errorModifiedIDLegacy, gotCode)
		})
		mt.RunOpts("write concern error", mtest.NewOptions().Topologies(mtest.ReplicaSet), func(mt *mtest.T) {
			// 2.6 returns right away if document doesn't exist
			filter := bson.D{{"_id", "foo"}}
			replacement := bson.D{{"pi", 3.14159}}
			_, err := mt.Coll.InsertOne(mtest.Background, filter)
			assert.Nil(mt, err, "InsertOne error: %v", err)

			mt.CloneCollection(options.Collection().SetWriteConcern(impossibleWc))
			_, err = mt.Coll.ReplaceOne(mtest.Background, filter, replacement)
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got nil")
		})
	})
	mt.RunOpts("aggregate", noClientOpts, func(mt *mtest.T) {
		mt.Run("success", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			pipeline := bson.A{
				bson.D{{"$match", bson.D{{"x", bson.D{{"$gte", 2}}}}}},
				bson.D{{"$project", bson.D{
					{"_id", 0},
					{"x", 1},
				}}},
				bson.D{{"$sort", bson.D{{"x", 1}}}},
			}
			cursor, err := mt.Coll.Aggregate(mtest.Background, pipeline)
			assert.Nil(mt, err, "Aggregate error: %v", err)

			for i := 2; i < 5; i++ {
				assert.True(mt, cursor.Next(mtest.Background), "expected Next true, got false (i=%v)", i)
				elems, _ := cursor.Current.Elements()
				assert.Equal(mt, 1, len(elems), "expected doc with 1 element, got %v", cursor.Current)

				num, err := cursor.Current.LookupErr("x")
				assert.Nil(mt, err, "x not found in document %v", cursor.Current)
				assert.Equal(mt, bson.TypeInt32, num.Type, "expected 'x' type %v, got %v", bson.TypeInt32, num.Type)
				assert.Equal(mt, int32(i), num.Int32(), "expected x value %v, got %v", i, num.Int32())
			}
		})
		mt.RunOpts("index hint", mtest.NewOptions().MinServerVersion("3.6"), func(mt *mtest.T) {
			hint := bson.D{{"x", 1}}
			testAggregateWithOptions(mt, true, options.Aggregate().SetHint(hint))
		})
		mt.Run("options", func(mt *mtest.T) {
			testAggregateWithOptions(mt, false, options.Aggregate().SetAllowDiskUse(true))
		})
		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().Topologies(mtest.ReplicaSet).MinServerVersion("3.6").CollectionOptions(wcCollOpts)
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			pipeline := mongo.Pipeline{{{"$out", mt.Coll.Name()}}}
			cursor, err := mt.Coll.Aggregate(mtest.Background, pipeline)
			assert.Nil(mt, cursor, "expected cursor nil, got %v", cursor)
			_, ok := err.(mongo.WriteConcernError)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteConcernError{}, err)
		})
	})
	mt.RunOpts("count documents", noClientOpts, func(mt *mtest.T) {
		testCases := []struct {
			name   string
			filter bson.D
			opts   *options.CountOptions
			count  int64
		}{
			{"no filter", bson.D{}, nil, 5},
			{"filter", bson.D{{"x", bson.D{{"$gt", 2}}}}, nil, 3},
			{"limit", bson.D{}, options.Count().SetLimit(3), 3},
			{"skip", bson.D{}, options.Count().SetSkip(3), 2},
		}
		for _, tc := range testCases {
			mt.Run(tc.name, func(mt *mtest.T) {
				initCollection(mt, mt.Coll)
				count, err := mt.Coll.CountDocuments(mtest.Background, tc.filter, tc.opts)
				assert.Nil(mt, err, "CountDocuments error: %v", err)
				assert.Equal(mt, tc.count, count, "expected count %v, got %v", tc.count, count)
			})
		}
	})
	mt.RunOpts("estimated document count", noClientOpts, func(mt *mtest.T) {
		testCases := []struct {
			name  string
			opts  *options.EstimatedDocumentCountOptions
			count int64
		}{
			{"no options", nil, 5},
			{"options", options.EstimatedDocumentCount().SetMaxTime(1 * time.Second), 5},
		}
		for _, tc := range testCases {
			mt.Run(tc.name, func(mt *mtest.T) {
				initCollection(mt, mt.Coll)
				count, err := mt.Coll.EstimatedDocumentCount(mtest.Background, tc.opts)
				assert.Nil(mt, err, "EstimatedDocumentCount error: %v", err)
				assert.Equal(mt, tc.count, count, "expected count %v, got %v", tc.count, count)
			})
		}
	})
	mt.RunOpts("distinct", noClientOpts, func(mt *mtest.T) {
		all := []interface{}{int32(1), int32(2), int32(3), int32(4), int32(5)}
		last3 := []interface{}{int32(3), int32(4), int32(5)}
		testCases := []struct {
			name     string
			filter   bson.D
			opts     *options.DistinctOptions
			expected []interface{}
		}{
			{"no options", bson.D{}, nil, all},
			{"filter", bson.D{{"x", bson.D{{"$gt", 2}}}}, nil, last3},
			{"options", bson.D{}, options.Distinct().SetMaxTime(5000000000), all},
		}
		for _, tc := range testCases {
			mt.Run(tc.name, func(mt *mtest.T) {
				initCollection(mt, mt.Coll)
				res, err := mt.Coll.Distinct(mtest.Background, "x", tc.filter, tc.opts)
				assert.Nil(mt, err, "Distinct error: %v", err)
				assert.Equal(mt, tc.expected, res, "expected result %v, got %v", tc.expected, res)
			})
		}
	})
	mt.RunOpts("find", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetSort(bson.D{{"x", 1}}))
			assert.Nil(mt, err, "Find error: %v", err)

			results := make([]int, 0, 5)
			for cursor.Next(mtest.Background) {
				x, err := cursor.Current.LookupErr("x")
				assert.Nil(mt, err, "x not found in document %v", cursor.Current)
				assert.Equal(mt, bson.TypeInt32, x.Type, "expected x type %v, got %v", bson.TypeInt32, x.Type)
				results = append(results, int(x.Int32()))
			}
			assert.Equal(mt, 5, len(results), "expected 5 results, got %v", len(results))
			expected := []int{1, 2, 3, 4, 5}
			assert.Equal(mt, expected, results, "expected results %v, got %v", expected, results)
		})
		mt.Run("limit and batch size", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			for _, batchSize := range []int32{2, 3, 4} {
				cursor, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetLimit(3).SetBatchSize(batchSize))
				assert.Nil(mt, err, "Find error: %v", err)

				numReceived := 0
				for cursor.Next(mtest.Background) {
					numReceived++
				}
				err = cursor.Err()
				assert.Nil(mt, err, "cursor error: %v", err)
				assert.Equal(mt, 3, numReceived, "expected 3 results, got %v", numReceived)
			}
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{{"x", 6}})
			assert.Nil(mt, err, "Find error: %v", err)
			assert.False(mt, cursor.Next(mtest.Background), "expected no documents, found %v", cursor.Current)
		})
		mt.Run("invalid identifier error", func(mt *mtest.T) {
			cursor, err := mt.Coll.Find(mtest.Background, bson.D{{"$foo", 1}})
			assert.NotNil(mt, err, "expected error for invalid identifier, got nil")
			assert.Nil(mt, cursor, "expected nil cursor, got %v", cursor)
		})
		mt.Run("negative limit", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			c, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetLimit(-2))
			assert.Nil(mt, err, "Find error: %v", err)
			// single batch returned so cursor should have ID 0
			assert.Equal(mt, int64(0), c.ID(), "expected cursor ID 0, got %v", c.ID())

			var numDocs int
			for c.Next(mtest.Background) {
				numDocs++
			}
			assert.Equal(mt, 2, numDocs, "expected 2 documents, got %v", numDocs)
		})
		mt.Run("exhaust cursor", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			c, err := mt.Coll.Find(mtest.Background, bson.D{})
			assert.Nil(mt, err, "Find error: %v", err)

			var numDocs int
			for c.Next(mtest.Background) {
				numDocs++
			}
			assert.Equal(mt, 5, numDocs, "expected 5 documents, got %v", numDocs)
			err = c.Close(mtest.Background)
			assert.Nil(mt, err, "Close error: %v", err)
		})
		mt.Run("hint", func(mt *mtest.T) {
			_, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetHint("_id_"))
			assert.Nil(mt, err, "Find error with string hint: %v", err)

			_, err = mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetHint(bson.D{{"_id", 1}}))
			assert.Nil(mt, err, "Find error with document hint: %v", err)

			_, err = mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetHint("foobar"))
			_, ok := err.(mongo.CommandError)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.CommandError{}, err)
		})
	})
	mt.RunOpts("find one", noClientOpts, func(mt *mtest.T) {
		mt.Run("limit", func(mt *mtest.T) {
			err := mt.Coll.FindOne(mtest.Background, bson.D{}).Err()
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)

			started := mt.GetStartedEvent()
			assert.NotNil(mt, started, "expected CommandStartedEvent, got nil")
			limitVal, err := started.Command.LookupErr("limit")
			assert.Nil(mt, err, "limit not found in command")
			limit := limitVal.Int64()
			assert.Equal(mt, int64(1), limit, "expected limit 1, got %v", limit)
		})
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			res, err := mt.Coll.FindOne(mtest.Background, bson.D{{"x", 1}}).DecodeBytes()
			assert.Nil(mt, err, "FindOne error: %v", err)

			x, err := res.LookupErr("x")
			assert.Nil(mt, err, "x not found in document %v", res)
			assert.Equal(mt, bson.TypeInt32, x.Type, "expected x type %v, got %v", bson.TypeInt32, x.Type)
			got := x.Int32()
			assert.Equal(mt, int32(1), got, "expected x value 1, got %v", got)
		})
		mt.Run("options", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			opts := options.FindOne().SetComment("here's a query for ya")
			res, err := mt.Coll.FindOne(mtest.Background, bson.D{{"x", 1}}, opts).DecodeBytes()
			assert.Nil(mt, err, "FindOne error: %v", err)

			x, err := res.LookupErr("x")
			assert.Nil(mt, err, "x not found in document %v", res)
			assert.Equal(mt, bson.TypeInt32, x.Type, "expected x type %v, got %v", bson.TypeInt32, x.Type)
			got := x.Int32()
			assert.Equal(mt, int32(1), got, "expected x value 1, got %v", got)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			err := mt.Coll.FindOne(mtest.Background, bson.D{{"x", 6}}).Err()
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
		})
	})
	mt.RunOpts("find one and delete", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			res, err := mt.Coll.FindOneAndDelete(mtest.Background, bson.D{{"x", 3}}).DecodeBytes()
			assert.Nil(mt, err, "FindOneAndDelete error: %v", err)

			elem, err := res.LookupErr("x")
			assert.Nil(mt, err, "x not found in result %v", res)
			assert.Equal(mt, bson.TypeInt32, elem.Type, "expected x type %v, got %v", bson.TypeInt32, elem.Type)
			x := elem.Int32()
			assert.Equal(mt, int32(3), x, "expected x value 3, got %v", x)
		})
		mt.Run("found ignore result", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			err := mt.Coll.FindOneAndDelete(mtest.Background, bson.D{{"x", 3}}).Err()
			assert.Nil(mt, err, "FindOneAndDelete error: %v", err)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			err := mt.Coll.FindOneAndDelete(mtest.Background, bson.D{{"x", 6}}).Err()
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
		})
		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().CollectionOptions(wcCollOpts).Topologies(mtest.ReplicaSet).MinServerVersion("3.2")
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			err := mt.Coll.FindOneAndDelete(mtest.Background, bson.D{{"x", 3}}).Err()
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %v", err)
		})
	})
	mt.RunOpts("find one and replace", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 3}}
			replacement := bson.D{{"y", 3}}

			res, err := mt.Coll.FindOneAndReplace(mtest.Background, filter, replacement).DecodeBytes()
			assert.Nil(mt, err, "FindOneAndReplace error: %v", err)
			elem, err := res.LookupErr("x")
			assert.Nil(mt, err, "x not found in result %v", res)
			assert.Equal(mt, bson.TypeInt32, elem.Type, "expected x type %v, got %v", bson.TypeInt32, elem.Type)
			x := elem.Int32()
			assert.Equal(mt, int32(3), x, "expected x value 3, got %v", x)
		})
		mt.Run("found ignore result", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 3}}
			replacement := bson.D{{"y", 3}}

			err := mt.Coll.FindOneAndReplace(mtest.Background, filter, replacement).Err()
			assert.Nil(mt, err, "FindOneAndReplace error: %v", err)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 6}}
			replacement := bson.D{{"y", 6}}

			err := mt.Coll.FindOneAndReplace(mtest.Background, filter, replacement).Err()
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
		})
		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().CollectionOptions(wcCollOpts).Topologies(mtest.ReplicaSet).MinServerVersion("3.2")
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			filter := bson.D{{"x", 3}}
			replacement := bson.D{{"y", 3}}
			err := mt.Coll.FindOneAndReplace(mtest.Background, filter, replacement).Err()
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %v", err)
		})
	})
	mt.RunOpts("find one and update", noClientOpts, func(mt *mtest.T) {
		mt.Run("found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 3}}
			update := bson.D{{"$set", bson.D{{"x", 6}}}}

			res, err := mt.Coll.FindOneAndUpdate(mtest.Background, filter, update).DecodeBytes()
			assert.Nil(mt, err, "FindOneAndUpdate error: %v", err)
			elem, err := res.LookupErr("x")
			assert.Nil(mt, err, "x not found in result %v", res)
			assert.Equal(mt, bson.TypeInt32, elem.Type, "expected x type %v, got %v", bson.TypeInt32, elem.Type)
			x := elem.Int32()
			assert.Equal(mt, int32(3), x, "expected x value 3, got %v", x)
		})
		mt.Run("empty update", func(mt *mtest.T) {
			err := mt.Coll.FindOneAndUpdate(mtest.Background, bson.D{}, bson.D{})
			assert.NotNil(mt, err, "expected error, got nil")
		})
		mt.Run("found ignore result", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 3}}
			update := bson.D{{"$set", bson.D{{"x", 6}}}}

			err := mt.Coll.FindOneAndUpdate(mtest.Background, filter, update).Err()
			assert.Nil(mt, err, "FindOneAndUpdate error: %v", err)
		})
		mt.Run("not found", func(mt *mtest.T) {
			initCollection(mt, mt.Coll)
			filter := bson.D{{"x", 6}}
			update := bson.D{{"$set", bson.D{{"y", 6}}}}

			err := mt.Coll.FindOneAndUpdate(mtest.Background, filter, update).Err()
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
		})
		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().CollectionOptions(wcCollOpts).Topologies(mtest.ReplicaSet).MinServerVersion("3.2")
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			filter := bson.D{{"x", 3}}
			update := bson.D{{"$set", bson.D{{"x", 6}}}}
			err := mt.Coll.FindOneAndUpdate(mtest.Background, filter, update).Err()
			we, ok := err.(mongo.WriteException)
			assert.True(mt, ok, "expected error type %v, got %v", mongo.WriteException{}, err)
			assert.NotNil(mt, we.WriteConcernError, "expected write concern error, got %v", err)
		})
	})
	mt.RunOpts("bulk write", noClientOpts, func(mt *mtest.T) {
		wcCollOpts := options.Collection().SetWriteConcern(impossibleWc)
		wcTestOpts := mtest.NewOptions().CollectionOptions(wcCollOpts).Topologies(mtest.ReplicaSet).CreateClient(false)
		mt.RunOpts("write concern error", wcTestOpts, func(mt *mtest.T) {
			filter := bson.D{{"foo", "bar"}}
			update := bson.D{{"$set", bson.D{{"foo", 10}}}}
			insertModel := mongo.NewInsertOneModel().SetDocument(bson.D{{"foo", 1}})
			updateModel := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
			deleteModel := mongo.NewDeleteOneModel().SetFilter(filter)

			testCases := []struct {
				name   string
				models []mongo.WriteModel
			}{
				{"insert", []mongo.WriteModel{insertModel}},
				{"update", []mongo.WriteModel{updateModel}},
				{"delete", []mongo.WriteModel{deleteModel}},
			}
			for _, tc := range testCases {
				mt.Run(tc.name, func(mt *mtest.T) {
					_, err := mt.Coll.BulkWrite(mtest.Background, tc.models)
					bwe, ok := err.(mongo.BulkWriteException)
					assert.True(mt, ok, "expected error type %v, got %v", mongo.BulkWriteException{}, err)
					numWriteErrors := len(bwe.WriteErrors)
					assert.Equal(mt, 0, numWriteErrors, "expected 0 write errors, got %v", numWriteErrors)
					assert.NotNil(mt, bwe.WriteConcernError, "expected write concern error, got %v", err)
				})
			}
		})

		mt.RunOpts("insert write errors", noClientOpts, func(mt *mtest.T) {
			doc1 := mongo.NewInsertOneModel().SetDocument(bson.D{{"_id", "x"}})
			doc2 := mongo.NewInsertOneModel().SetDocument(bson.D{{"_id", "y"}})
			models := []mongo.WriteModel{doc1, doc1, doc2, doc2}

			testCases := []struct {
				name           string
				ordered        bool
				insertedCount  int64
				numWriteErrors int
			}{
				{"ordered", true, 1, 1},
				{"unordered", false, 2, 2},
			}
			for _, tc := range testCases {
				mt.Run(tc.name, func(mt *mtest.T) {
					res, err := mt.Coll.BulkWrite(mtest.Background, models, options.BulkWrite().SetOrdered(tc.ordered))
					assert.Equal(mt, tc.insertedCount, res.InsertedCount,
						"expected inserted count %v, got %v", tc.insertedCount, res.InsertedCount)

					bwe, ok := err.(mongo.BulkWriteException)
					assert.True(mt, ok, "expected error type %v, got %v", mongo.BulkWriteException{}, err)
					numWriteErrors := len(bwe.WriteErrors)
					assert.Equal(mt, tc.numWriteErrors, numWriteErrors,
						"expected %v write errors, got %v", tc.numWriteErrors, numWriteErrors)
					gotCode := bwe.WriteErrors[0].Code
					assert.Equal(mt, errorDuplicateKey, gotCode, "expected error code %v, got %v", errorDuplicateKey, gotCode)
				})
			}
		})
		mt.Run("delete write errors", func(mt *mtest.T) {
			doc := mongo.NewDeleteOneModel().SetFilter(bson.D{{"x", 1}})
			models := []mongo.WriteModel{doc, doc}
			cappedOpts := bson.D{{"capped", true}, {"size", 64 * 1024}}
			capped := mt.CreateCollection(mtest.Collection{
				Name:       "delete_write_errors",
				CreateOpts: cappedOpts,
			}, true)

			testCases := []struct {
				name           string
				ordered        bool
				numWriteErrors int
			}{
				{"ordered", true, 1},
				{"unordered", false, 2},
			}
			for _, tc := range testCases {
				mt.Run(tc.name, func(mt *mtest.T) {
					_, err := capped.BulkWrite(mtest.Background, models, options.BulkWrite().SetOrdered(tc.ordered))
					bwe, ok := err.(mongo.BulkWriteException)
					assert.True(mt, ok, "expected error type %v, got %v", mongo.BulkWriteException{}, err)
					numWriteErrors := len(bwe.WriteErrors)
					assert.Equal(mt, tc.numWriteErrors, numWriteErrors,
						"expected %v write errors, got %v", tc.numWriteErrors, numWriteErrors)
					gotCode := bwe.WriteErrors[0].Code
					assert.True(mt, gotCode == errorCappedCollDeleteLegacy || gotCode == errorCappedCollDelete,
						"expected error code %v or %v, got %v", errorCappedCollDeleteLegacy, errorCappedCollDelete, gotCode)
				})
			}
		})
		mt.RunOpts("update write errors", noClientOpts, func(mt *mtest.T) {
			filter := bson.D{{"_id", "foo"}}
			doc1 := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.D{{"$set", bson.D{{"_id", 3.14159}}}})
			doc2 := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.D{{"$set", bson.D{{"x", "fa"}}}})
			models := []mongo.WriteModel{doc1, doc1, doc2}

			testCases := []struct {
				name           string
				ordered        bool
				modifiedCount  int64
				numWriteErrors int
			}{
				{"ordered", true, 0, 1},
				{"unordered", false, 1, 2},
			}
			for _, tc := range testCases {
				mt.Run(tc.name, func(mt *mtest.T) {
					_, err := mt.Coll.InsertOne(mtest.Background, filter)
					assert.Nil(mt, err, "InsertOne error: %v", err)
					res, err := mt.Coll.BulkWrite(mtest.Background, models, options.BulkWrite().SetOrdered(tc.ordered))
					assert.Equal(mt, tc.modifiedCount, res.ModifiedCount,
						"expected modified count %v, got %v", tc.modifiedCount, res.ModifiedCount)

					bwe, ok := err.(mongo.BulkWriteException)
					assert.True(mt, ok, "expected error type %v, got %v", mongo.BulkWriteException{}, err)
					numWriteErrors := len(bwe.WriteErrors)
					assert.Equal(mt, tc.numWriteErrors, numWriteErrors,
						"expected %v write errors, got %v", tc.numWriteErrors, numWriteErrors)
					gotCode := bwe.WriteErrors[0].Code
					assert.Equal(mt, errorModifiedID, gotCode, "expected error code %v, got %v", errorModifiedID, gotCode)
				})
			}
		})
		mt.Run("correct model in errors", func(mt *mtest.T) {
			models := []mongo.WriteModel{
				mongo.NewUpdateOneModel().SetFilter(bson.M{}).SetUpdate(bson.M{}),
				mongo.NewInsertOneModel().SetDocument(bson.M{
					"_id": "notduplicate",
				}),
				mongo.NewInsertOneModel().SetDocument(bson.M{
					"_id": "duplicate1",
				}),
				mongo.NewInsertOneModel().SetDocument(bson.M{
					"_id": "duplicate1",
				}),
				mongo.NewInsertOneModel().SetDocument(bson.M{
					"_id": "duplicate2",
				}),
				mongo.NewInsertOneModel().SetDocument(bson.M{
					"_id": "duplicate2",
				}),
			}

			_, err := mt.Coll.BulkWrite(mtest.Background, models)
			bwException, ok := err.(mongo.BulkWriteException)
			assert.True(mt, ok, "expected error of type %T, got %T", mongo.BulkWriteException{}, err)

			expectedModel := models[3]
			actualModel := bwException.WriteErrors[0].Request
			assert.Equal(mt, expectedModel, actualModel, "expected model %v in BulkWriteException, got %v",
				expectedModel, actualModel)
		})
	})
}

func initCollection(mt *mtest.T, coll *mongo.Collection) {
	mt.Helper()

	var docs []interface{}
	for i := 1; i <= 5; i++ {
		docs = append(docs, bson.D{{"x", int32(i)}})
	}

	_, err := coll.InsertMany(mtest.Background, docs)
	assert.Nil(mt, err, "InsertMany error for initial data: %v", err)
}

func testAggregateWithOptions(mt *mtest.T, createIndex bool, opts *options.AggregateOptions) {
	mt.Helper()
	initCollection(mt, mt.Coll)

	if createIndex {
		indexView := mt.Coll.Indexes()
		_, err := indexView.CreateOne(context.Background(), mongo.IndexModel{
			Keys: bsonx.Doc{{"x", bsonx.Int32(1)}},
		})
		assert.Nil(mt, err, "CreateOne error: %v", err)
	}

	pipeline := mongo.Pipeline{
		{{"$match", bson.D{{"x", bson.D{{"$gte", 2}}}}}},
		{{"$project", bson.D{{"_id", 0}, {"x", 1}}}},
		{{"$sort", bson.D{{"x", 1}}}},
	}

	cursor, err := mt.Coll.Aggregate(context.Background(), pipeline, opts)
	assert.Nil(mt, err, "Aggregate error: %v", err)

	for i := 2; i < 5; i++ {
		assert.True(mt, cursor.Next(mtest.Background), "expected Next true, got false")
		elems, _ := cursor.Current.Elements()
		assert.Equal(mt, 1, len(elems), "expected doc with 1 element, got %v", cursor.Current)

		num, err := cursor.Current.LookupErr("x")
		assert.Nil(mt, err, "x not found in document %v", cursor.Current)
		assert.Equal(mt, bson.TypeInt32, num.Type, "expected 'x' type %v, got %v", bson.TypeInt32, num.Type)
		assert.Equal(mt, int32(i), num.Int32(), "expected x value %v, got %v", i, num.Int32())
	}
}

func create16MBDocument(mt *mtest.T) bsoncore.Document {
	// 4 bytes = document length
	// 1 byte = element type (ObjectID = \x07)
	// 4 bytes = key name ("_id" + \x00)
	// 12 bytes = ObjectID value
	// 1 byte = element type (string = \x02)
	// 4 bytes = key name ("key" + \x00)
	// 4 bytes = string length
	// X bytes = string of length X bytes
	// 1 byte = \x00
	// 1 byte = \x00
	//
	// Therefore the string length should be: 1024*1024*16 - 32

	targetDocSize := 1024 * 1024 * 16
	strSize := targetDocSize - 32
	var b strings.Builder
	b.Grow(strSize)
	for i := 0; i < strSize; i++ {
		b.WriteByte('A')
	}

	idx, doc := bsoncore.AppendDocumentStart(nil)
	doc = bsoncore.AppendObjectIDElement(doc, "_id", primitive.NewObjectID())
	doc = bsoncore.AppendStringElement(doc, "key", b.String())
	doc, _ = bsoncore.AppendDocumentEnd(doc, idx)
	assert.Equal(mt, targetDocSize, len(doc), "expected document length %v, got %v", targetDocSize, len(doc))
	return doc
}
