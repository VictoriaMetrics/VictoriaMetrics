// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"io/ioutil"
	"path"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

const (
	crudTestsDir = "../../data/crud"
	crudReadDir  = "v1/read"
	crudWriteDir = "v1/write"
)

type crudTestFile struct {
	Data             []bson.Raw `bson:"data"`
	MinServerVersion string     `bson:"minServerVersion"`
	MaxServerVersion string     `bson:"maxServerVersion"`
	Tests            []crudTest `bson:"tests"`
}

type crudTest struct {
	Description string        `bson:"description"`
	SkipReason  string        `bson:"skipReason"`
	Operation   crudOperation `bson:"operation"`
	Outcome     crudOutcome   `bson:"outcome"`
}

type crudOperation struct {
	Name      string   `bson:"name"`
	Arguments bson.Raw `bson:"arguments"`
}

type crudOutcome struct {
	Error      bool               `bson:"error"` // only used by retryable writes tests
	Result     interface{}        `bson:"result"`
	Collection *outcomeCollection `bson:"collection"`
}

var crudRegistry = bson.NewRegistryBuilder().
	RegisterTypeMapEntry(bson.TypeEmbeddedDocument, reflect.TypeOf(bson.Raw{})).Build()

func TestCrudSpec(t *testing.T) {
	for _, dir := range []string{crudReadDir, crudWriteDir} {
		for _, file := range jsonFilesInDir(t, path.Join(crudTestsDir, dir)) {
			t.Run(file, func(t *testing.T) {
				runCrudFile(t, path.Join(crudTestsDir, dir, file))
			})
		}
	}
}

func runCrudFile(t *testing.T, file string) {
	content, err := ioutil.ReadFile(file)
	assert.Nil(t, err, "ReadFile error for %v: %v", file, err)

	var testFile crudTestFile
	err = bson.UnmarshalExtJSONWithRegistry(crudRegistry, content, false, &testFile)
	assert.Nil(t, err, "UnmarshalExtJSONWithRegistry error: %v", err)

	mt := mtest.New(t, mtest.NewOptions().MinServerVersion(testFile.MinServerVersion).MaxServerVersion(testFile.MaxServerVersion))
	defer mt.Close()

	for _, test := range testFile.Tests {
		mt.Run(test.Description, func(mt *mtest.T) {
			runCrudTest(mt, test, testFile)
		})
	}
}

func runCrudTest(mt *mtest.T, test crudTest, testFile crudTestFile) {
	if len(testFile.Data) > 0 {
		docs := rawSliceToInterfaceSlice(testFile.Data)
		_, err := mt.Coll.InsertMany(mtest.Background, docs)
		assert.Nil(mt, err, "InsertMany error: %v", err)
	}

	runCrudOperation(mt, test.Description, test.Operation, test.Outcome)
}

// run a CRUD operation and verify errors and outcomes.
// the test description is needed to see determine if the test is an aggregate with $out
func runCrudOperation(mt *mtest.T, testDescription string, operation crudOperation, outcome crudOutcome) {
	switch operation.Name {
	case "aggregate":
		cursor, err := executeAggregate(mt, mt.Coll, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected Aggregate error, got nil")
			return
		}
		assert.Nil(mt, err, "Aggregate error: %v", err)
		// only verify cursor contents for pipelines without $out
		if !strings.Contains(testDescription, "$out") {
			verifyCursorResult(mt, cursor, outcome.Result)
		}
	case "bulkWrite":
		res, err := executeBulkWrite(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected BulkWrite error, got nil")
			return
		}
		assert.Nil(mt, err, "BulkWrite error: %v", err)
		verifyBulkWriteResult(mt, res, outcome.Result)
	case "count":
		res, err := executeCountDocuments(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected CountDocuments error, got nil")
			return
		}
		assert.Nil(mt, err, "CountDocuments error: %v", err)
		verifyCountResult(mt, res, outcome.Result)
	case "distinct":
		res, err := executeDistinct(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected Distinct error, got nil")
			return
		}
		assert.Nil(mt, err, "Distinct error: %v", err)
		verifyDistinctResult(mt, res, outcome.Result)
	case "find":
		cursor, err := executeFind(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected Find error, got nil")
			return
		}
		assert.Nil(mt, err, "Find error: %v", err)
		verifyCursorResult(mt, cursor, outcome.Result)
	case "deleteOne":
		res, err := executeDeleteOne(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected DeleteOne error, got nil")
			return
		}
		assert.Nil(mt, err, "DeleteOne error: %v", err)
		verifyDeleteResult(mt, res, outcome.Result)
	case "deleteMany":
		res, err := executeDeleteMany(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected DeleteMany error, got nil")
			return
		}
		assert.Nil(mt, err, "DeleteMany error: %v", err)
		verifyDeleteResult(mt, res, outcome.Result)
	case "findOneAndDelete":
		res := executeFindOneAndDelete(mt, nil, operation.Arguments)
		err := res.Err()
		if outcome.Error {
			assert.NotNil(mt, err, "expected FindOneAndDelete error, got nil")
			return
		}
		if outcome.Result == nil {
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
			break
		}
		assert.Nil(mt, err, "FindOneAndDelete error: %v", err)
		verifySingleResult(mt, res, outcome.Result)
	case "findOneAndReplace":
		res := executeFindOneAndReplace(mt, nil, operation.Arguments)
		err := res.Err()
		if outcome.Error {
			assert.NotNil(mt, err, "expected FindOneAndReplace error, got nil")
			return
		}
		if outcome.Result == nil {
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
			break
		}
		assert.Nil(mt, err, "FindOneAndReplace error: %v", err)
		verifySingleResult(mt, res, outcome.Result)
	case "findOneAndUpdate":
		res := executeFindOneAndUpdate(mt, nil, operation.Arguments)
		err := res.Err()
		if outcome.Error {
			assert.NotNil(mt, err, "expected FindOneAndUpdate error, got nil")
			return
		}
		if outcome.Result == nil {
			assert.Equal(mt, mongo.ErrNoDocuments, err, "expected error %v, got %v", mongo.ErrNoDocuments, err)
			break
		}
		assert.Nil(mt, err, "FindOneAndUpdate error: %v", err)
		verifySingleResult(mt, res, outcome.Result)
	case "insertOne":
		res, err := executeInsertOne(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected InsertOne error, got nil")
			return
		}
		assert.Nil(mt, err, "InsertOne error: %v", err)
		verifyInsertOneResult(mt, res, outcome.Result)
	case "insertMany":
		res, err := executeInsertMany(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected InsertMany error, got nil")
			return
		}
		assert.Nil(mt, err, "InsertMany error: %v", err)
		verifyInsertManyResult(mt, res, outcome.Result)
	case "replaceOne":
		res, err := executeReplaceOne(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected ReplaceOne error, got nil")
			return
		}
		assert.Nil(mt, err, "ReplaceOne error: %v", err)
		verifyUpdateResult(mt, res, outcome.Result)
	case "updateOne":
		res, err := executeUpdateOne(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected UpdateOne error, got nil")
			return
		}
		assert.Nil(mt, err, "UpdateOne error: %v", err)
		verifyUpdateResult(mt, res, outcome.Result)
	case "updateMany":
		res, err := executeUpdateMany(mt, nil, operation.Arguments)
		if outcome.Error {
			assert.NotNil(mt, err, "expected UpdateMany error, got nil")
			return
		}
		assert.Nil(mt, err, "UpdateMany error: %v", err)
		verifyUpdateResult(mt, res, outcome.Result)
	default:
		mt.Fatalf("unrecognized operation: %v", operation.Name)
	}

	if outcome.Collection != nil {
		verifyTestOutcome(mt, outcome.Collection)
	}
}
