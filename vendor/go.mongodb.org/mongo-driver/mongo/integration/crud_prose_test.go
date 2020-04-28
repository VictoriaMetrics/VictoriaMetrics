// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"bytes"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func TestAggregateSecondaryPreferredReadPreference(t *testing.T) {
	// Use secondaryPreferred instead of secondary because sharded clusters started up by mongo-orchestration have
	// one-node shards, so a secondary read preference is not satisfiable.
	secondaryPrefClientOpts := options.Client().
		SetWriteConcern(mtest.MajorityWc).
		SetReadPreference(readpref.SecondaryPreferred()).
		SetReadConcern(mtest.MajorityRc)
	mtOpts := mtest.NewOptions().
		ClientOptions(secondaryPrefClientOpts).
		MinServerVersion("4.1.0") // Consistent with tests in aggregate-out-readConcern.json

	mt := mtest.New(t, mtOpts)
	mt.Run("aggregate $out with read preference secondary", func(mt *mtest.T) {
		doc, err := bson.Marshal(bson.D{
			{"_id", 1},
			{"x", 11},
		})
		assert.Nil(mt, err, "Marshal error: %v", err)
		_, err = mt.Coll.InsertOne(mtest.Background, doc)
		assert.Nil(mt, err, "InsertOne error: %v", err)

		mt.ClearEvents()
		outputCollName := "aggregate-read-pref-secondary-output"
		outStage := bson.D{
			{"$out", outputCollName},
		}
		cursor, err := mt.Coll.Aggregate(mtest.Background, mongo.Pipeline{outStage})
		assert.Nil(mt, err, "Aggregate error: %v", err)
		_ = cursor.Close(mtest.Background)

		// Assert that the output collection contains the document we expect.
		outputColl := mt.CreateCollection(mtest.Collection{Name: outputCollName}, false)
		cursor, err = outputColl.Find(mtest.Background, bson.D{})
		assert.Nil(mt, err, "Find error: %v", err)
		defer cursor.Close(mtest.Background)

		assert.True(mt, cursor.Next(mtest.Background), "expected Next to return true, got false")
		assert.True(mt, bytes.Equal(doc, cursor.Current), "expected document %s, got %s", bson.Raw(doc), cursor.Current)
		assert.False(mt, cursor.Next(mtest.Background), "unexpected document returned by Find: %s", cursor.Current)

		// Assert that no read preference was sent to the server.
		evt := mt.GetStartedEvent()
		assert.Equal(mt, "aggregate", evt.CommandName, "expected command 'aggregate', got '%s'", evt.CommandName)
		_, err = evt.Command.LookupErr("$readPreference")
		assert.NotNil(mt, err, "expected command %s to not contain $readPreference", evt.Command)
	})
}
