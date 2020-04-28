// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongosPinning(t *testing.T) {
	clientOpts := options.Client().SetLocalThreshold(1 * time.Second).SetWriteConcern(mtest.MajorityWc)
	mtOpts := mtest.NewOptions().Topologies(mtest.Sharded).MinServerVersion("4.1").CreateClient(false).
		ClientOptions(clientOpts)
	mt := mtest.New(t, mtOpts)
	defer mt.Close()

	if len(options.Client().ApplyURI(mt.ConnString()).Hosts) < 2 {
		mt.Skip("skipping because at least 2 mongoses are required")
	}

	mt.Run("unpin for next transaction", func(mt *mtest.T) {
		addresses := map[string]struct{}{}
		_ = mt.Client.UseSession(mtest.Background, func(sc mongo.SessionContext) error {
			// Insert a document in a transaction to pin session to a mongos
			err := sc.StartTransaction()
			assert.Nil(mt, err, "StartTransaction error: %v", err)
			_, err = mt.Coll.InsertOne(sc, bson.D{{"x", 1}})
			assert.Nil(mt, err, "InsertOne error: %v", err)
			err = sc.CommitTransaction(sc)
			assert.Nil(mt, err, "CommitTransaction error: %v", err)

			for i := 0; i < 50; i++ {
				// Call Find in a new transaction to unpin from the old mongos and select a new one
				err = sc.StartTransaction()
				assert.Nil(mt, err, iterationErrmsg("StartTransaction", i, err))

				cursor, err := mt.Coll.Find(sc, bson.D{})
				assert.Nil(mt, err, iterationErrmsg("Find", i, err))
				assert.True(mt, cursor.Next(mtest.Background), "Next returned false on iteration %v", i)

				descConn, err := mongo.BatchCursorFromCursor(cursor).Server().Connection(mtest.Background)
				assert.Nil(mt, err, iterationErrmsg("Connection", i, err))
				addresses[descConn.Description().Addr.String()] = struct{}{}
				err = descConn.Close()
				assert.Nil(mt, err, iterationErrmsg("connection Close", i, err))

				err = sc.CommitTransaction(sc)
				assert.Nil(mt, err, iterationErrmsg("CommitTransaction", i, err))
			}
			return nil
		})
		assert.True(mt, len(addresses) > 1, "expected more than 1 address, got %v", addresses)
	})
	mt.Run("unpin for non transaction operation", func(mt *mtest.T) {
		addresses := map[string]struct{}{}
		_ = mt.Client.UseSession(mtest.Background, func(sc mongo.SessionContext) error {
			// Insert a document in a transaction to pin session to a mongos
			err := sc.StartTransaction()
			assert.Nil(mt, err, "StartTransaction error: %v", err)
			_, err = mt.Coll.InsertOne(sc, bson.D{{"x", 1}})
			assert.Nil(mt, err, "InsertOne error: %v", err)
			err = sc.CommitTransaction(sc)
			assert.Nil(mt, err, "CommitTransaction error: %v", err)

			for i := 0; i < 50; i++ {
				// Call Find with the session but outside of a transaction
				cursor, err := mt.Coll.Find(sc, bson.D{})
				assert.Nil(mt, err, iterationErrmsg("Find", i, err))
				assert.True(mt, cursor.Next(mtest.Background), "Next returned false on iteration %v", i)

				descConn, err := mongo.BatchCursorFromCursor(cursor).Server().Connection(mtest.Background)
				assert.Nil(mt, err, iterationErrmsg("Connection", i, err))
				addresses[descConn.Description().Addr.String()] = struct{}{}
				err = descConn.Close()
				assert.Nil(mt, err, iterationErrmsg("connection Close", i, err))
			}
			return nil
		})
		assert.True(mt, len(addresses) > 1, "expected more than 1 address, got %v", addresses)
	})
}

func iterationErrmsg(op string, i int, wrapped error) string {
	return fmt.Sprintf("%v error on iteration %v: %v", op, i, wrapped)
}
