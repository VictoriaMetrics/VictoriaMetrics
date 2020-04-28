// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"errors"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

const (
	listCollCapped   = "listcoll_capped"
	listCollUncapped = "listcoll_uncapped"
)

func TestDatabase(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().CreateClient(false))
	defer mt.Close()

	mt.RunOpts("run command", noClientOpts, func(mt *mtest.T) {
		mt.Run("decode raw", func(mt *mtest.T) {
			res, err := mt.DB.RunCommand(mtest.Background, bson.D{{"ismaster", 1}}).DecodeBytes()
			assert.Nil(mt, err, "RunCommand error: %v", err)

			ok, err := res.LookupErr("ok")
			assert.Nil(mt, err, "ok field not found in result")
			assert.Equal(mt, bson.TypeDouble, ok.Type, "expected ok type %v, got %v", bson.TypeDouble, ok.Type)
			assert.Equal(mt, 1.0, ok.Double(), "expected ok value 1.0, got %v", ok.Double())

			isMaster, err := res.LookupErr("ismaster")
			assert.Nil(mt, err, "ismaster field not found in result")
			assert.Equal(mt, bson.TypeBoolean, isMaster.Type, "expected isMaster type %v, got %v", bson.TypeBoolean, isMaster.Type)
			assert.True(mt, isMaster.Boolean(), "expected isMaster value true, got false")
		})
		mt.Run("decode struct", func(mt *mtest.T) {
			result := struct {
				IsMaster bool    `bson:"ismaster"`
				Ok       float64 `bson:"ok"`
			}{}
			err := mt.DB.RunCommand(mtest.Background, bson.D{{"ismaster", 1}}).Decode(&result)
			assert.Nil(mt, err, "RunCommand error: %v", err)
			assert.Equal(mt, true, result.IsMaster, "expected isMaster value true, got false")
			assert.Equal(mt, 1.0, result.Ok, "expected ok value 1.0, got %v", result.Ok)
		})
	})

	dropOpts := mtest.NewOptions().DatabaseName("dropDb")
	mt.RunOpts("drop", dropOpts, func(mt *mtest.T) {
		err := mt.DB.Drop(mtest.Background)
		assert.Nil(mt, err, "Drop error: %v", err)

		list, err := mt.Client.ListDatabaseNames(mtest.Background, bson.D{})
		assert.Nil(mt, err, "ListDatabaseNames error: %v", err)
		for _, db := range list {
			if db == "dropDb" {
				mt.Fatal("dropped database 'dropDb' found in database names")
			}
		}
	})

	lcNamesOpts := mtest.NewOptions().MinServerVersion("4.0")
	mt.RunOpts("list collection names", lcNamesOpts, func(mt *mtest.T) {
		collName := "lcNamesCollection"
		mt.CreateCollection(mtest.Collection{Name: collName}, true)

		testCases := []struct {
			name   string
			filter bson.D
			found  bool
		}{
			{"no filter", bson.D{}, true},
			{"filter", bson.D{{"name", "lcNamesCollection"}}, true},
			{"filter not found", bson.D{{"name", "123"}}, false},
		}
		for _, tc := range testCases {
			mt.Run(tc.name, func(mt *mtest.T) {
				colls, err := mt.DB.ListCollectionNames(mtest.Background, tc.filter)
				assert.Nil(mt, err, "ListCollectionNames error: %v", err)

				var found bool
				for _, coll := range colls {
					if coll == collName {
						found = true
						break
					}
				}

				assert.Equal(mt, tc.found, found, "expected to find collection: %v, found collection: %v", tc.found, found)
			})
		}
	})

	mt.RunOpts("list collections", noClientOpts, func(mt *mtest.T) {
		testCases := []struct {
			name             string
			expectedTopology mtest.TopologyKind
			cappedOnly       bool
		}{
			{"standalone no filter", mtest.Single, false},
			{"standalone filter", mtest.Single, true},
			{"replica set no filter", mtest.ReplicaSet, false},
			{"replica set filter", mtest.ReplicaSet, true},
			{"sharded no filter", mtest.Sharded, false},
			{"sharded filter", mtest.Sharded, true},
		}
		for _, tc := range testCases {
			tcOpts := mtest.NewOptions().Topologies(tc.expectedTopology)
			mt.RunOpts(tc.name, tcOpts, func(mt *mtest.T) {
				mt.CreateCollection(mtest.Collection{Name: listCollUncapped}, true)
				mt.CreateCollection(mtest.Collection{
					Name:       listCollCapped,
					CreateOpts: bson.D{{"capped", true}, {"size", 64 * 1024}},
				}, true)

				filter := bson.D{}
				if tc.cappedOnly {
					filter = bson.D{{"options.capped", true}}
				}

				var err error
				for i := 0; i < 1; i++ {
					cursor, err := mt.DB.ListCollections(mtest.Background, filter)
					assert.Nil(mt, err, "ListCollections error (iteration %v): %v", i, err)

					err = verifyListCollections(cursor, tc.cappedOnly)
					if err == nil {
						return
					}
				}
				mt.Fatalf("error verifying list collections result: %v", err)
			})
		}
	})

	mt.RunOpts("run command cursor", noClientOpts, func(mt *mtest.T) {
		var data []interface{}
		for i := 0; i < 5; i++ {
			data = append(data, bson.D{{"x", i}})
		}
		findCollName := "runcommandcursor_find"
		findCmd := bson.D{{"find", findCollName}}
		aggCollName := "runcommandcursor_agg"
		aggCmd := bson.D{
			{"aggregate", aggCollName},
			{"pipeline", bson.A{}},
			{"cursor", bson.D{}},
		}
		pingCmd := bson.D{{"ping", 1}}
		pingErr := errors.New("cursor should be an embedded document but is of BSON type invalid")

		testCases := []struct {
			name        string
			collName    string
			cmd         interface{}
			toInsert    []interface{}
			expectedErr error
			numExpected int
			minVersion  string
		}{
			{"success find", findCollName, findCmd, data, nil, 5, "3.2"},
			{"success aggregate", aggCollName, aggCmd, data, nil, 5, ""},
			{"failures", "runcommandcursor_ping", pingCmd, nil, pingErr, 0, ""},
		}
		for _, tc := range testCases {
			tcOpts := mtest.NewOptions().CollectionName(tc.collName)
			if tc.minVersion != "" {
				tcOpts.MinServerVersion(tc.minVersion)
			}

			mt.RunOpts(tc.name, tcOpts, func(mt *mtest.T) {
				if len(tc.toInsert) > 0 {
					_, err := mt.Coll.InsertMany(mtest.Background, tc.toInsert)
					assert.Nil(mt, err, "InsertMany error: %v", err)
				}

				cursor, err := mt.DB.RunCommandCursor(mtest.Background, tc.cmd)
				assert.Equal(mt, tc.expectedErr, err, "expected error %v, got %v", tc.expectedErr, err)
				if tc.expectedErr != nil {
					return
				}

				var count int
				for cursor.Next(mtest.Background) {
					count++
				}
				assert.Equal(mt, tc.numExpected, count, "expected document count %v, got %v", tc.numExpected, count)
			})
		}
	})
}

func verifyListCollections(cursor *mongo.Cursor, cappedOnly bool) error {
	var cappedFound, uncappedFound bool

	for cursor.Next(mtest.Background) {
		nameElem, err := cursor.Current.LookupErr("name")
		if err != nil {
			return fmt.Errorf("name element not found in document %v", cursor.Current)
		}
		if nameElem.Type != bson.TypeString {
			return fmt.Errorf("expected name type %v, got %v", bson.TypeString, nameElem.Type)
		}

		name := nameElem.StringValue()
		// legacy servers can return an indexes collection that shouldn't be considered here
		if name != listCollUncapped && name != listCollCapped {
			continue
		}

		if name == listCollUncapped && !uncappedFound {
			if cappedOnly {
				return fmt.Errorf("found uncapped collection %v but expected only capped collections", listCollUncapped)
			}

			uncappedFound = true
			continue
		}
		if name == listCollCapped && !cappedFound {
			cappedFound = true
			continue
		}

		// duplicate found
		return fmt.Errorf("found duplicate collection %v", name)
	}

	if !cappedFound {
		return fmt.Errorf("capped collection %v not found", listCollCapped)
	}
	if !cappedOnly && !uncappedFound {
		return fmt.Errorf("uncapped collection %v not found", listCollUncapped)
	}
	return nil
}
