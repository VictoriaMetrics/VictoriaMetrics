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

const retryableWritesTestDir = "../../data/retryable-writes"

type retryableWritesTestFile struct {
	Data             []bson.Raw            `bson:"data"`
	MinServerVersion string                `bson:"minServerVersion"`
	MaxServerVersion string                `bson:"maxServerVersion"`
	Tests            []retryableWritesTest `bson:"tests"`
}

type retryableWritesTest struct {
	Description         string           `bson:"description"`
	ClientOptions       bson.Raw         `bson:"clientOptions"`
	UseMultipleMongoses bool             `bson:"useMultipleMongoses"`
	FailPoint           *mtest.FailPoint `bson:"failPoint"`
	Operation           crudOperation    `bson:"operation"`
	Outcome             crudOutcome      `bson:"outcome"`
}

func TestRetryableWritesSpec(t *testing.T) {
	for _, file := range jsonFilesInDir(t, retryableWritesTestDir) {
		t.Run(file, func(t *testing.T) {
			runRetryableWritesFile(t, path.Join(retryableWritesTestDir, file))
		})
	}
}

func runRetryableWritesFile(t *testing.T, filePath string) {
	content, err := ioutil.ReadFile(filePath)
	assert.Nil(t, err, "ReadFile error for %v: %v", filePath, err)

	var testFile retryableWritesTestFile
	err = bson.UnmarshalExtJSONWithRegistry(specTestRegistry, content, false, &testFile)
	assert.Nil(t, err, "UnmarshalExtJSONWithRegistry error: %v", err)

	mtOpts := mtest.NewOptions().MinServerVersion(testFile.MinServerVersion).MaxServerVersion(testFile.MaxServerVersion).
		Topologies(mtest.ReplicaSet).CreateClient(false)
	mt := mtest.New(t, mtOpts)
	defer mt.Close()

	for _, test := range testFile.Tests {
		runRetryableWritesTest(mt, test, testFile)
	}
}

func runRetryableWritesTest(mt *mtest.T, test retryableWritesTest, testFile retryableWritesTestFile) {
	testClientOpts := createClientOptions(mt, test.ClientOptions)
	opts := mtest.NewOptions().ClientOptions(testClientOpts)
	if mt.TopologyKind() == mtest.Sharded && !test.UseMultipleMongoses {
		// pin to a single mongos
		opts = opts.ClientType(mtest.Pinned)
	}

	mt.RunOpts(test.Description, opts, func(mt *mtest.T) {
		// setup - insert test data and set fail point
		insertDocuments(mt, mt.Coll, testFile.Data)
		if test.FailPoint != nil {
			mt.SetFailPoint(*test.FailPoint)
		}

		// run operation and verify outcome/error
		runCrudOperation(mt, test.Description, test.Operation, test.Outcome)
	})
}
