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
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

const (
	monitoringTestDir = "../../data/command-monitoring"
)

type monitoringTestFile struct {
	Data           []bson.Raw       `bson:"data"`
	CollectionName string           `bson:"collection_name"`
	DatabaseName   string           `bson:"database_name"`
	Namespace      string           `bson:"namespace"`
	Tests          []monitoringTest `bson:"tests"`
}

type monitoringTest struct {
	Description       string               `bson:"description"`
	Comment           string               `bson:"comment"`
	MinServerVersion  string               `bson:"ignore_if_server_version_less_than"`
	MaxServerVersion  string               `bson:"ignore_if_server_version_greater_than"`
	IgnoredTopologies []mtest.TopologyKind `bson:"ignore_if_topology_type"`
	Operation         monitoringOperation  `bson:"operation"`
	Expectations      []*expectation       `bson:"expectations"`
}

type monitoringOperation struct {
	Name              string   `bson:"name"`
	CollectionOptions bson.Raw `bson:"collectionOptions"`
	Arguments         bson.Raw `bson:"arguments"`
}

func TestCommandMonitoring(t *testing.T) {
	mt := mtest.New(t, noClientOpts)
	defer mt.Close()

	for _, file := range jsonFilesInDir(t, monitoringTestDir) {
		mt.Run(file, func(mt *mtest.T) {
			runMonitoringFile(mt, path.Join(monitoringTestDir, file))
		})
	}
}

func runMonitoringFile(mt *mtest.T, file string) {
	content, err := ioutil.ReadFile(file)
	assert.Nil(mt, err, "ReadFile error for %v: %v", file, err)

	var testFile monitoringTestFile
	err = bson.UnmarshalExtJSONWithRegistry(specTestRegistry, content, false, &testFile)
	assert.Nil(mt, err, "UnmarshalExtJSONWithRegistry error: %v", err)

	for _, test := range testFile.Tests {
		runMonitoringTest(mt, test, testFile)
	}
}

func runMonitoringTest(mt *mtest.T, test monitoringTest, testFile monitoringTestFile) {
	collOpts := createCollectionOptions(mt, test.Operation.CollectionOptions)
	mtOpts := mtest.NewOptions().CollectionName(testFile.CollectionName).DatabaseName(testFile.DatabaseName).
		MinServerVersion(test.MinServerVersion).MaxServerVersion(test.MaxServerVersion).CollectionOptions(collOpts)
	mt.RunOpts(test.Description, mtOpts, func(mt *mtest.T) {
		// ignored topologies have to be handled separately because mtest only accepts topologies to run on, not
		// topologies to ignore
		for _, top := range test.IgnoredTopologies {
			if top == mt.TopologyKind() {
				mt.Skipf("skipping topology %v", top)
			}
		}

		setupColl := mt.GlobalClient().Database(mt.DB.Name()).Collection(mt.Coll.Name())
		insertDocuments(mt, setupColl, testFile.Data)
		mt.ClearEvents()
		runMonitoringOperation(mt, test.Operation)
		checkExpectations(mt, test.Expectations, nil, nil)
	})
}

// runMonitoringOperation runs the given operation. This function iterates any cursors returned by an operation to
// completion and ignores all execution errors.
func runMonitoringOperation(mt *mtest.T, operation monitoringOperation) {
	switch operation.Name {
	case "count":
		mt.Skip("count has been deprecated")
	case "aggregate":
		cursor, err := executeAggregate(mt, mt.Coll, nil, operation.Arguments)
		if err != nil {
			return
		}
		for cursor.Next(mtest.Background) {
		}
	case "bulkWrite":
		_, _ = executeBulkWrite(mt, nil, operation.Arguments)
	case "countDocuments":
		_, _ = executeCountDocuments(mt, nil, operation.Arguments)
	case "distinct":
		_, _ = executeDistinct(mt, nil, operation.Arguments)
	case "find":
		cursor, err := executeFind(mt, nil, operation.Arguments)
		if err != nil {
			return
		}
		for cursor.Next(mtest.Background) {
		}
	case "deleteOne":
		_, _ = executeDeleteOne(mt, nil, operation.Arguments)
	case "deleteMany":
		_, _ = executeDeleteMany(mt, nil, operation.Arguments)
	case "findOneAndDelete":
		_ = executeFindOneAndDelete(mt, nil, operation.Arguments)
	case "findOneAndReplace":
		_ = executeFindOneAndReplace(mt, nil, operation.Arguments)
	case "findOneAndUpdate":
		_ = executeFindOneAndUpdate(mt, nil, operation.Arguments)
	case "insertOne":
		_, _ = executeInsertOne(mt, nil, operation.Arguments)
	case "insertMany":
		_, _ = executeInsertMany(mt, nil, operation.Arguments)
	case "replaceOne":
		_, _ = executeReplaceOne(mt, nil, operation.Arguments)
	case "updateOne":
		_, _ = executeUpdateOne(mt, nil, operation.Arguments)
	case "updateMany":
		_, _ = executeUpdateMany(mt, nil, operation.Arguments)
	default:
		mt.Fatalf("unrecognized operation: %v", operation.Name)
	}
}
