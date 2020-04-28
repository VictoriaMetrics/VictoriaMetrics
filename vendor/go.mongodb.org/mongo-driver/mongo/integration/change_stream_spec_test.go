// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"io/ioutil"
	"path"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

const (
	changeStreamsTestsDir = "../../data/change-streams"
)

type changeStreamTestFile struct {
	DatabaseName    string             `bson:"database_name"`
	CollectionName  string             `bson:"collection_name"`
	DatabaseName2   string             `bson:"database2_name"`
	CollectionName2 string             `bson:"collection2_name"`
	Tests           []changeStreamTest `bson:"tests"`
}

type changeStreamTest struct {
	Description      string                  `bson:"description"`
	MinServerVersion string                  `bson:"minServerVersion"`
	MaxServerVersion string                  `bson:"maxServerVersion"`
	FailPoint        *mtest.FailPoint        `bson:"failPoint"`
	Target           string                  `bson:"target"`
	Topology         []mtest.TopologyKind    `bson:"topology"`
	Pipeline         []bson.Raw              `bson:"changeStreamPipeline"`
	Options          bson.Raw                `bson:"changeStreamOptions"`
	Operations       []changeStreamOperation `bson:"operations"`
	Expectations     []*expectation          `bson:"expectations"`
	Result           changeStreamResult      `bson:"result"`

	// set of namespaces created in a test
	nsMap map[string]struct{}
}

type changeStreamOperation struct {
	Database   string   `bson:"database"`
	Collection string   `bson:"collection"`
	Name       string   `bson:"name"`
	Arguments  bson.Raw `bson:"arguments"`
}

type changeStreamResult struct {
	Error   bson.Raw   `bson:"error"`
	Success []bson.Raw `bson:"success"`

	// set in code
	actualEvents []bson.Raw
}

type cmdErr struct {
	Code    int32    `bson:"code"`
	Message string   `bson:"message"`
	Labels  []string `bson:"errorLabels"`
	Name    string   `bson:"name"`
}

func TestChangeStreamSpec(t *testing.T) {
	mt := mtest.New(t, noClientOpts)
	defer mt.Close()

	for _, file := range jsonFilesInDir(t, changeStreamsTestsDir) {
		mt.Run(file, func(mt *mtest.T) {
			runChangeStreamTestFile(mt, path.Join(changeStreamsTestsDir, file))
		})
	}
}

func runChangeStreamTestFile(mt *mtest.T, file string) {
	content, err := ioutil.ReadFile(file)
	assert.Nil(mt, err, "ReadFile error for %v: %v", file, err)

	var testFile changeStreamTestFile
	err = bson.UnmarshalExtJSONWithRegistry(specTestRegistry, content, false, &testFile)
	assert.Nil(mt, err, "UnmarshalExtJSONWithRegistry error: %v", err)

	for _, test := range testFile.Tests {
		runChangeStreamTest(mt, test, testFile)
	}
}

func runChangeStreamTest(mt *mtest.T, test changeStreamTest, testFile changeStreamTestFile) {
	mtOpts := mtest.NewOptions().MinServerVersion(test.MinServerVersion).MaxServerVersion(test.MaxServerVersion).
		Topologies(test.Topology...).DatabaseName(testFile.DatabaseName).CollectionName(testFile.CollectionName)
	mt.RunOpts(test.Description, mtOpts, func(mt *mtest.T) {
		test.nsMap = make(map[string]struct{})

		mt.ClearEvents()
		var watcher watcher
		switch test.Target {
		case "client":
			watcher = mt.Client
		case "database":
			watcher = mt.DB
		case "collection":
			watcher = mt.Coll
		default:
			mt.Fatalf("unrecognized change stream target: %v", test.Target)
		}
		csOpts := createChangeStreamOptions(mt, test.Options)
		changeStream, err := watcher.Watch(mtest.Background, test.Pipeline, csOpts)
		if err == nil {
			err = runChangeStreamOperations(mt, test)
		}
		if err == nil && test.Result.Error != nil {
			// if there was no error and an error is expected, capture the result from iterating the stream once
			changeStream.Next(mtest.Background)
			err = changeStream.Err()
		}
		if err == nil && len(test.Result.Success) != 0 {
			// if there was no error and success array is non-empty, iterate stream until it returns as many changes
			// as there are elements in the success array or an error is thrown
			for i := 0; i < len(test.Result.Success); i++ {
				if !changeStream.Next(mtest.Background) {
					break
				}

				var event bson.Raw
				decodeErr := changeStream.Decode(&event)
				assert.Nil(mt, decodeErr, "Decode error for document %v: %v", changeStream.Current, decodeErr)
				test.Result.actualEvents = append(test.Result.actualEvents, event)
			}
			err = changeStream.Err()
		}
		if changeStream != nil {
			closeErr := changeStream.Close(mtest.Background)
			assert.Nil(mt, closeErr, "Close error: %v", err)
		}

		verifyChangeStreamResults(mt, test.Result, err)
		checkExpectations(mt, test.Expectations, nil, nil)
	})
}

// run operations until all are executed or an error occurs.
func runChangeStreamOperations(mt *mtest.T, test changeStreamTest) error {
	for _, op := range test.Operations {
		ns := op.Database + "." + op.Collection
		if _, ok := test.nsMap[ns]; !ok {
			// create target collection on the server if it's not already being tracked
			test.nsMap[ns] = struct{}{}
			mt.CreateCollection(mtest.Collection{
				Name: op.Collection,
				DB:   op.Database,
			}, true)
		}
		mt.DB = mt.Client.Database(op.Database)
		mt.Coll = mt.DB.Collection(op.Collection)

		var err error
		switch op.Name {
		case "insertOne":
			_, err = executeInsertOne(mt, nil, op.Arguments)
		case "updateOne":
			_, err = executeUpdateOne(mt, nil, op.Arguments)
		case "replaceOne":
			_, err = executeReplaceOne(mt, nil, op.Arguments)
		case "deleteOne":
			_, err = executeDeleteOne(mt, nil, op.Arguments)
		case "rename":
			res, targetColl := executeRenameCollection(mt, nil, op.Arguments)
			mt.CreateCollection(mtest.Collection{
				Name: targetColl,
				DB:   mt.DB.Name(),
			}, false)
			err = res.Err()
		case "drop":
			err = mt.Coll.Drop(mtest.Background)
		default:
			mt.Fatalf("unrecognized operation: %v", op.Name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyChangeStreamResults(mt *mtest.T, result changeStreamResult, err error) {
	mt.Helper()

	if err != nil {
		assert.NotNil(mt, result.Error, "unexpected change stream error: %v", err)
		ce, ok := err.(mongo.CommandError)
		assert.True(mt, ok, "expected error of type %T, got %v of type %T", mongo.CommandError{}, err, err)
		actualErrDoc, marshalErr := bson.Marshal(cmdErr{
			Code:    ce.Code,
			Message: ce.Message,
			Labels:  ce.Labels,
			Name:    ce.Name,
		})
		assert.Nil(mt, marshalErr, "Marshal error: %v", marshalErr)

		if comparisonErr := compareDocs(mt, result.Error, bson.Raw(actualErrDoc)); comparisonErr != nil {
			mt.Fatalf("comparing change stream errors mismatch: %v", comparisonErr)
		}
		return
	}

	assert.Nil(mt, result.Error, "expected change stream error %v, got nil", result.Error)
	for i, expectedEvent := range result.Success {
		if comparisonErr := compareDocs(mt, expectedEvent, result.actualEvents[i]); comparisonErr != nil {
			mt.Fatalf("success event mismatch at index %d: %s", i, comparisonErr)
		}
	}
}
