// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	errorNotMaster             int32 = 10107
	errorShutdownInProgress    int32 = 91
	errorInterruptedAtShutdown int32 = 11600
)

var poolChan = make(chan *event.PoolEvent, 100)
var poolMonitor = &event.PoolMonitor{
	Event: func(event *event.PoolEvent) {
		poolChan <- event
	},
}

func isPoolCleared() bool {
	for len(poolChan) > 0 {
		curr := <-poolChan
		if curr.Type == event.PoolCleared {
			return true
		}
	}
	return false
}

func clearPoolChan() {
	for len(poolChan) < 0 {
		<-poolChan
	}
}

func TestConnectionsSurvivePrimaryStepDown(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().Topologies(mtest.ReplicaSet).CreateClient(false))
	defer mt.Close()

	clientOpts := options.Client().ApplyURI(mt.ConnString()).SetRetryWrites(false).SetPoolMonitor(poolMonitor)

	getMoreOpts := mtest.NewOptions().MinServerVersion("4.2").ClientOptions(clientOpts)
	mt.RunOpts("getMore iteration", getMoreOpts, func(mt *mtest.T) {
		clearPoolChan()

		initCollection(mt, mt.Coll)
		cur, err := mt.Coll.Find(mtest.Background, bson.D{}, options.Find().SetBatchSize(2))
		assert.Nil(mt, err, "Find error: %v", err)
		defer cur.Close(mtest.Background)
		assert.True(mt, cur.Next(mtest.Background), "expected Next true, got false")

		err = mt.Client.Database("admin").RunCommand(mtest.Background, bson.D{
			{"replSetStepDown", 5},
			{"force", true},
		}, options.RunCmd().SetReadPreference(readpref.Primary())).Err()
		assert.Nil(mt, err, "replSetStepDown error: %v", err)

		assert.True(mt, cur.Next(mtest.Background), "expected Next true, got false")
		assert.False(mt, isPoolCleared(), "expected pool to not be cleared but was")
	})
	mt.RunOpts("server errors", noClientOpts, func(mt *mtest.T) {
		testCases := []struct {
			name                   string
			minVersion, maxVersion string
			errCode                int32
			poolCleared            bool
		}{
			{"notMaster keep pool", "4.2", "", errorNotMaster, false},
			{"notMaster reset pool", "4.0", "4.0", errorNotMaster, true},
			{"shutdown in progress reset pool", "4.0", "", errorShutdownInProgress, true},
			{"interrupted at shutdown reset pool", "4.0", "", errorInterruptedAtShutdown, true},
		}
		for _, tc := range testCases {
			tcOpts := mtest.NewOptions().ClientOptions(clientOpts)
			if tc.minVersion != "" {
				tcOpts.MinServerVersion(tc.minVersion)
			}
			if tc.maxVersion != "" {
				tcOpts.MaxServerVersion(tc.maxVersion)
			}
			mt.RunOpts(tc.name, tcOpts, func(mt *mtest.T) {
				clearPoolChan()

				mt.SetFailPoint(mtest.FailPoint{
					ConfigureFailPoint: "failCommand",
					Mode: mtest.FailPointMode{
						Times: 1,
					},
					Data: mtest.FailPointData{
						FailCommands: []string{"insert"},
						ErrorCode:    tc.errCode,
					},
				})

				_, err := mt.Coll.InsertOne(mtest.Background, bson.D{{"test", 1}})
				assert.NotNil(mt, err, "expected InsertOne error, got nil")
				cerr, ok := err.(mongo.CommandError)
				assert.True(mt, ok, "expected error type %v, got %v", mongo.CommandError{}, err)
				assert.Equal(mt, tc.errCode, cerr.Code, "expected error code %v, got %v", tc.errCode, cerr.Code)

				if tc.poolCleared {
					assert.True(mt, isPoolCleared(), "expected pool to be cleared but was not")
					return
				}

				// if pool shouldn't be cleared, another operation should succeed
				_, err = mt.Coll.InsertOne(mtest.Background, bson.D{{"test", 1}})
				assert.Nil(mt, err, "InsertOne error: %v", err)
				assert.False(mt, isPoolCleared(), "expected pool to not be cleared but was")
			})
		}
	})
	mt.RunOpts("network errors", mtest.NewOptions().ClientOptions(clientOpts).MinServerVersion("4.0"), func(mt *mtest.T) {
		// expect that a server's connection pool will be cleared if a non-timeout network error occurs during an
		// operation

		clearPoolChan()
		mt.SetFailPoint(mtest.FailPoint{
			ConfigureFailPoint: "failCommand",
			Mode: mtest.FailPointMode{
				Times: 1,
			},
			Data: mtest.FailPointData{
				FailCommands:    []string{"insert"},
				CloseConnection: true,
			},
		})

		_, err := mt.Coll.InsertOne(mtest.Background, bson.D{{"test", 1}})
		assert.NotNil(mt, err, "expected InsertOne error, got nil")
		assert.True(mt, isPoolCleared(), "expected pool to be cleared but was not")
	})
}
