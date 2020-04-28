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
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/mongo/driver/session"
)

const (
	gridFSFiles       = "fs.files"
	gridFSChunks      = "fs.chunks"
	cseMaxVersionTest = "operation fails with maxWireVersion < 8"
)

type testFile struct {
	RunOn          []mtest.RunOnBlock `bson:"runOn"`
	DatabaseName   string             `bson:"database_name"`
	CollectionName string             `bson:"collection_name"`
	BucketName     string             `bson:"bucket_name"`
	Data           testData           `bson:"data"`
	JSONSchema     bson.Raw           `bson:"json_schema"`
	KeyVaultData   []bson.Raw         `bson:"key_vault_data"`
	Tests          []*testCase        `bson:"tests"`
}

type testData struct {
	Documents  []bson.Raw
	GridFSData struct {
		Files  []bson.Raw `bson:"fs.files"`
		Chunks []bson.Raw `bson:"fs.chunks"`
	}
}

// custom decoder for testData type
func decodeTestData(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	switch vr.Type() {
	case bsontype.Array:
		docsVal := val.FieldByName("Documents")
		decoder, err := dc.Registry.LookupDecoder(docsVal.Type())
		if err != nil {
			return err
		}

		return decoder.DecodeValue(dc, vr, docsVal)
	case bsontype.EmbeddedDocument:
		gridfsDataVal := val.FieldByName("GridFSData")
		decoder, err := dc.Registry.LookupDecoder(gridfsDataVal.Type())
		if err != nil {
			return err
		}

		return decoder.DecodeValue(dc, vr, gridfsDataVal)
	}
	return nil
}

type testCase struct {
	Description         string           `bson:"description"`
	SkipReason          string           `bson:"skipReason"`
	FailPoint           *mtest.FailPoint `bson:"failPoint"`
	ClientOptions       bson.Raw         `bson:"clientOptions"`
	SessionOptions      bson.Raw         `bson:"sessionOptions"`
	Operations          []*operation     `bson:"operations"`
	Expectations        []*expectation   `bson:"expectations"`
	UseMultipleMongoses bool             `bson:"useMultipleMongoses"`
	Outcome             *outcome         `bson:"outcome"`

	// set in code if the test is a GridFS test
	chunkSize int32
	bucket    *gridfs.Bucket
}

type operation struct {
	Name              string      `bson:"name"`
	Object            string      `bson:"object"`
	CollectionOptions bson.Raw    `bson:"collectionOptions"`
	DatabaseOptions   bson.Raw    `bson:"databaseOptions"`
	Result            interface{} `bson:"result"`
	Arguments         bson.Raw    `bson:"arguments"`
	Error             bool        `bson:"error"`

	// set in code after determining whether or not result represents an error
	opError *operationError
}

type expectation struct {
	CommandStartedEvent *struct {
		CommandName  string   `bson:"command_name"`
		DatabaseName string   `bson:"database_name"`
		Command      bson.Raw `bson:"command"`
	} `bson:"command_started_event"`
	CommandSucceededEvent *struct {
		CommandName string   `bson:"command_name"`
		Reply       bson.Raw `bson:"reply"`
	} `bson:"command_succeeded_event"`
	CommandFailedEvent *struct {
		CommandName string `bson:"command_name"`
	} `bson:"command_failed_event"`
}

type outcome struct {
	Collection *outcomeCollection `bson:"collection"`
}

type outcomeCollection struct {
	Name string      `bson:"name"`
	Data interface{} `bson:"data"`
}

type operationError struct {
	ErrorContains      *string  `bson:"errorContains"`
	ErrorCodeName      *string  `bson:"errorCodeName"`
	ErrorLabelsContain []string `bson:"errorLabelsContain"`
	ErrorLabelsOmit    []string `bson:"errorLabelsOmit"`
}

const dataPath string = "../../data/"

var directories = []string{
	"transactions",
	"convenient-transactions",
	"crud/v2",
	"retryable-reads",
	"sessions",
}

var checkOutcomeOpts = options.Collection().SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Local())
var specTestRegistry = bson.NewRegistryBuilder().
	RegisterTypeMapEntry(bson.TypeEmbeddedDocument, reflect.TypeOf(bson.Raw{})).
	RegisterTypeDecoder(reflect.TypeOf(testData{}), bsoncodec.ValueDecoderFunc(decodeTestData)).Build()

func TestUnifiedSpecs(t *testing.T) {
	for _, specDir := range directories {
		t.Run(specDir, func(t *testing.T) {
			for _, fileName := range jsonFilesInDir(t, path.Join(dataPath, specDir)) {
				t.Run(fileName, func(t *testing.T) {
					runSpecTestFile(t, specDir, fileName)
				})
			}
		})
	}
}

// specDir: name of directory for a spec in the data/ folder
// fileName: name of test file in specDir
func runSpecTestFile(t *testing.T, specDir, fileName string) {
	filePath := path.Join(dataPath, specDir, fileName)
	content, err := ioutil.ReadFile(filePath)
	assert.Nil(t, err, "unable to read spec test file %v: %v", filePath, err)

	var testFile testFile
	err = bson.UnmarshalExtJSONWithRegistry(specTestRegistry, content, false, &testFile)
	assert.Nil(t, err, "unable to unmarshal spec test file at %v: %v", filePath, err)

	// create mtest wrapper and skip if needed
	mt := mtest.New(t, mtest.NewOptions().RunOn(testFile.RunOn...).CreateClient(false))
	defer mt.Close()

	for _, test := range testFile.Tests {
		runSpecTestCase(mt, test, testFile)
	}
}

func getSessionID(mt *mtest.T, sess mongo.Session) bsonx.Doc {
	mt.Helper()
	xsess, ok := sess.(mongo.XSession)
	assert.True(mt, ok, "expected %T to implement mongo.XSession", sess)
	return xsess.ID()
}

func runSpecTestCase(mt *mtest.T, test *testCase, testFile testFile) {
	testClientOpts := createClientOptions(mt, test.ClientOptions)
	testClientOpts.SetHeartbeatInterval(50 * time.Millisecond)

	opts := mtest.NewOptions().DatabaseName(testFile.DatabaseName).CollectionName(testFile.CollectionName)
	if mt.TopologyKind() == mtest.Sharded && !test.UseMultipleMongoses {
		// pin to a single mongos
		opts = opts.ClientType(mtest.Pinned)
	}
	if len(testFile.JSONSchema) > 0 {
		validator := bson.D{
			{"$jsonSchema", testFile.JSONSchema},
		}
		opts.CollectionCreateOptions(bson.D{
			{"validator", validator},
		})
	}
	if test.Description != cseMaxVersionTest {
		// don't specify client options for the maxWireVersion CSE test because the client cannot
		// be created successfully. Should be fixed by SPEC-1403.
		opts.ClientOptions(testClientOpts)
	}

	mt.RunOpts(test.Description, opts, func(mt *mtest.T) {
		if len(test.SkipReason) > 0 {
			mt.Skip(test.SkipReason)
		}
		if test.Description == cseMaxVersionTest {
			// This test checks to see if the correct error is thrown when auto encrypting with a server < 4.2.
			// Currently, the test will fail because a server < 4.2 wouldn't have mongocryptd, so Client construction
			// would fail with a mongocryptd spawn error. SPEC-1403 tracks the work to fix this.
			mt.Skip("skipping maxWireVersion test")
		}

		// work around for SERVER-39704: run a non-transactional distinct against each shard in a sharded cluster
		if mt.TopologyKind() == mtest.Sharded && test.Description == "distinct" {
			opts := options.Client().ApplyURI(mt.ConnString())
			for _, host := range opts.Hosts {
				shardClient, err := mongo.Connect(mtest.Background, opts.SetHosts([]string{host}))
				assert.Nil(mt, err, "Connect error for shard %v: %v", host, err)
				coll := shardClient.Database(mt.DB.Name()).Collection(mt.Coll.Name())
				_, err = coll.Distinct(mtest.Background, "x", bson.D{})
				assert.Nil(mt, err, "Distinct error for shard %v: %v", host, err)
				_ = shardClient.Disconnect(mtest.Background)
			}
		}

		// defer killSessions to ensure it runs regardless of the state of the test because the client has already
		// been created and the collection drop in mongotest will hang for transactions to be aborted (60 seconds)
		// in error cases.
		defer killSessions(mt)
		setupTest(mt, &testFile, test)

		// create the GridFS bucket after resetting the client so it will be created with a connected client
		createBucket(mt, testFile, test)

		// create sessions, fail points, and collection
		sess0, sess1 := setupSessions(mt, test)
		if sess0 != nil {
			defer func() {
				sess0.EndSession(mtest.Background)
				sess1.EndSession(mtest.Background)
			}()
		}
		if test.FailPoint != nil {
			mt.SetFailPoint(*test.FailPoint)
		}

		// run operations
		mt.ClearEvents()
		for _, op := range test.Operations {
			runOperation(mt, test, op, sess0, sess1)
		}

		// Needs to be done here (in spite of defer) because some tests
		// require end session to be called before we check expectation
		sess0.EndSession(mtest.Background)
		sess1.EndSession(mtest.Background)
		mt.ClearFailPoints()

		checkExpectations(mt, test.Expectations, getSessionID(mt, sess0), getSessionID(mt, sess1))

		if test.Outcome != nil {
			verifyTestOutcome(mt, test.Outcome.Collection)
		}
	})
}

func createBucket(mt *mtest.T, testFile testFile, testCase *testCase) {
	if testFile.BucketName == "" {
		return
	}

	bucketOpts := options.GridFSBucket()
	if testFile.BucketName != "" {
		bucketOpts.SetName(testFile.BucketName)
	}
	chunkSize := testCase.chunkSize
	if chunkSize == 0 {
		chunkSize = gridfs.DefaultChunkSize
	}
	bucketOpts.SetChunkSizeBytes(chunkSize)

	var err error
	testCase.bucket, err = gridfs.NewBucket(mt.DB, bucketOpts)
	assert.Nil(mt, err, "NewBucket error: %v", err)
}

func runOperation(mt *mtest.T, testCase *testCase, op *operation, sess0, sess1 mongo.Session) {
	if op.Name == "count" {
		mt.Skip("count has been deprecated")
	}

	var sess mongo.Session
	if sessVal, err := op.Arguments.LookupErr("session"); err == nil {
		sessStr := sessVal.StringValue()
		switch sessStr {
		case "session0":
			sess = sess0
		case "session1":
			sess = sess1
		default:
			mt.Fatalf("unrecognized session identifier: %v", sessStr)
		}
	}

	if op.Object == "testRunner" {
		executeTestRunnerOperation(mt, op, sess)
		return
	}

	mt.CloneDatabase(createDatabaseOptions(mt, op.DatabaseOptions))
	mt.CloneCollection(createCollectionOptions(mt, op.CollectionOptions))

	// execute the command on the given object
	var err error
	switch op.Object {
	case "session0":
		err = executeSessionOperation(mt, op, sess0)
	case "session1":
		err = executeSessionOperation(mt, op, sess1)
	case "", "collection":
		// object defaults to "collection" if not specified
		err = executeCollectionOperation(mt, op, sess)
	case "database":
		err = executeDatabaseOperation(mt, op, sess)
	case "gridfsbucket":
		err = executeGridFSOperation(mt, testCase.bucket, op)
	case "client":
		err = executeClientOperation(mt, op, sess)
	default:
		mt.Fatalf("unrecognized operation object: %v", op.Object)
	}

	// ensure error occurred and it's the error we expect
	if op.Error {
		assert.NotNil(mt, err, "expected error but got nil")
	}

	// some tests (e.g. crud/v2) only specify that an error should occur via the op.Error field but do not specify
	// which error via the op.Result field.
	if op.Error && op.Result == nil {
		return
	}
	// compute expected error from op.Result and compare that to the actual error
	op.opError = errorFromResult(mt, op.Result)
	verifyError(mt, op.opError, err)
}

func executeGridFSOperation(mt *mtest.T, bucket *gridfs.Bucket, op *operation) error {
	// no results for GridFS operations
	assert.Nil(mt, op.Result, "unexpected result for GridFS operation")

	switch op.Name {
	case "download":
		_, err := executeGridFSDownload(mt, bucket, op.Arguments)
		return err
	case "download_by_name":
		_, err := executeGridFSDownloadByName(mt, bucket, op.Arguments)
		return err
	default:
		mt.Fatalf("unrecognized gridfs operation: %v", op.Name)
	}
	return nil
}

func executeTestRunnerOperation(mt *mtest.T, op *operation, sess mongo.Session) {
	var clientSession *session.Client
	if sess != nil {
		xsess, ok := sess.(mongo.XSession)
		assert.True(mt, ok, "expected %T to implement mongo.XSession", sess)
		clientSession = xsess.ClientSession()
	}

	switch op.Name {
	case "targetedFailPoint":
		fpDoc, err := op.Arguments.LookupErr("failPoint")
		assert.Nil(mt, err, "failPoint not found in arguments")

		var fp mtest.FailPoint
		err = bson.Unmarshal(fpDoc.Document(), &fp)
		assert.Nil(mt, err, "error creating fail point: %v", err)

		targetHost := clientSession.PinnedServer.Addr.String()
		opts := options.Client().ApplyURI(mt.ConnString()).SetHosts([]string{targetHost})
		client, err := mongo.Connect(mtest.Background, opts)
		assert.Nil(mt, err, "error creating targeted client: %v", err)
		defer func() { _ = client.Disconnect(mtest.Background) }()

		err = client.Database("admin").RunCommand(mtest.Background, fp).Err()
		assert.Nil(mt, err, "error setting targeted fail point: %v", err)
		mt.TrackFailPoint(fp.ConfigureFailPoint)
	case "assertSessionPinned":
		assert.NotNil(mt, clientSession.PinnedServer, "expected pinned server but got nil")
	case "assertSessionUnpinned":
		assert.Nil(mt, clientSession.PinnedServer,
			"expected pinned server to be nil but got %v", clientSession.PinnedServer)
	case "assertSessionDirty":
		assert.NotNil(mt, clientSession.Server, "expected server session but got nil")
		assert.True(mt, clientSession.Server.Dirty, "expected server session to be marked dirty but was not")
	case "assertSessionNotDirty":
		assert.NotNil(mt, clientSession.Server, "expected server session but got nil")
		assert.False(mt, clientSession.Server.Dirty, "expected server session not to be marked dirty but was")
	case "assertSameLsidOnLastTwoCommands":
		first, second := lastTwoIDs(mt)
		assert.Equal(mt, first, second, "expected last two lsids to be equal but got %v and %v", first, second)
	case "assertDifferentLsidOnLastTwoCommands":
		first, second := lastTwoIDs(mt)
		assert.NotEqual(mt, first, second, "expected last two lsids to be not equal but got %v and %v", first, second)
	default:
		mt.Fatalf("unrecognized testRunner operation %v", op.Name)
	}
}

func lastTwoIDs(mt *mtest.T) (bson.RawValue, bson.RawValue) {
	events := mt.GetAllStartedEvents()
	lastTwoEvents := events[len(events)-2:]

	first := lastTwoEvents[0].Command.Lookup("lsid")
	second := lastTwoEvents[1].Command.Lookup("lsid")
	return first, second
}

func executeSessionOperation(mt *mtest.T, op *operation, sess mongo.Session) error {
	switch op.Name {
	case "startTransaction":
		var txnOpts *options.TransactionOptions
		if opts, err := op.Arguments.LookupErr("options"); err == nil {
			txnOpts = createTransactionOptions(mt, opts.Document())
		}
		return sess.StartTransaction(txnOpts)
	case "commitTransaction":
		return sess.CommitTransaction(mtest.Background)
	case "abortTransaction":
		return sess.AbortTransaction(mtest.Background)
	case "withTransaction":
		return executeWithTransaction(mt, sess, op.Arguments)
	case "endSession":
		sess.EndSession(mtest.Background)
		return nil
	default:
		mt.Fatalf("unrecognized session operation: %v", op.Name)
	}
	return nil
}

func executeCollectionOperation(mt *mtest.T, op *operation, sess mongo.Session) error {
	switch op.Name {
	case "countDocuments":
		// no results to verify with count
		res, err := executeCountDocuments(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyCountResult(mt, res, op.Result)
		}
		return err
	case "distinct":
		res, err := executeDistinct(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyDistinctResult(mt, res, op.Result)
		}
		return err
	case "insertOne":
		res, err := executeInsertOne(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyInsertOneResult(mt, res, op.Result)
		}
		return err
	case "insertMany":
		res, err := executeInsertMany(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyInsertManyResult(mt, res, op.Result)
		}
		return err
	case "find":
		cursor, err := executeFind(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyCursorResult(mt, cursor, op.Result)
			_ = cursor.Close(mtest.Background)
		}
		return err
	case "findOneAndDelete":
		res := executeFindOneAndDelete(mt, sess, op.Arguments)
		if op.opError == nil && res.Err() == nil {
			verifySingleResult(mt, res, op.Result)
		}
		return res.Err()
	case "findOneAndUpdate":
		res := executeFindOneAndUpdate(mt, sess, op.Arguments)
		if op.opError == nil && res.Err() == nil {
			verifySingleResult(mt, res, op.Result)
		}
		return res.Err()
	case "findOneAndReplace":
		res := executeFindOneAndReplace(mt, sess, op.Arguments)
		if op.opError == nil && res.Err() == nil {
			verifySingleResult(mt, res, op.Result)
		}
		return res.Err()
	case "deleteOne":
		res, err := executeDeleteOne(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyDeleteResult(mt, res, op.Result)
		}
		return err
	case "deleteMany":
		res, err := executeDeleteMany(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyDeleteResult(mt, res, op.Result)
		}
		return err
	case "updateOne":
		res, err := executeUpdateOne(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyUpdateResult(mt, res, op.Result)
		}
		return err
	case "updateMany":
		res, err := executeUpdateMany(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyUpdateResult(mt, res, op.Result)
		}
		return err
	case "replaceOne":
		res, err := executeReplaceOne(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyUpdateResult(mt, res, op.Result)
		}
		return err
	case "aggregate":
		cursor, err := executeAggregate(mt, mt.Coll, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyCursorResult(mt, cursor, op.Result)
			_ = cursor.Close(mtest.Background)
		}
		return err
	case "bulkWrite":
		res, err := executeBulkWrite(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyBulkWriteResult(mt, res, op.Result)
		}
		return err
	case "estimatedDocumentCount":
		res, err := executeEstimatedDocumentCount(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyCountResult(mt, res, op.Result)
		}
		return err
	case "findOne":
		res := executeFindOne(mt, sess, op.Arguments)
		if op.opError == nil && res.Err() == nil {
			verifySingleResult(mt, res, op.Result)
		}
		return res.Err()
	case "listIndexes":
		cursor, err := executeListIndexes(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyCursorResult(mt, cursor, op.Result)
			_ = cursor.Close(mtest.Background)
		}
		return err
	case "watch":
		stream, err := executeWatch(mt, mt.Coll, sess, op.Arguments)
		if op.opError == nil && err == nil {
			assert.Nil(mt, op.Result, "unexpected result for watch: %v", op.Result)
			_ = stream.Close(mtest.Background)
		}
		return err
	case "listIndexNames", "mapReduce":
		mt.Skipf("operation %v not implemented", op.Name)
	default:
		mt.Fatalf("unrecognized collection operation: %v", op.Name)
	}
	return nil
}

func executeDatabaseOperation(mt *mtest.T, op *operation, sess mongo.Session) error {
	switch op.Name {
	case "runCommand":
		res := executeRunCommand(mt, sess, op.Arguments)
		if op.opError == nil && res.Err() == nil {
			verifySingleResult(mt, res, op.Result)
		}
		return res.Err()
	case "aggregate":
		cursor, err := executeAggregate(mt, mt.DB, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyCursorResult(mt, cursor, op.Result)
			_ = cursor.Close(mtest.Background)
		}
		return err
	case "listCollections":
		cursor, err := executeListCollections(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			assert.Nil(mt, op.Result, "unexpected result for listCollections: %v", op.Result)
			_ = cursor.Close(mtest.Background)
		}
		return err
	case "listCollectionNames":
		_, err := executeListCollectionNames(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			assert.Nil(mt, op.Result, "unexpected result for listCollectionNames: %v", op.Result)
		}
		return err
	case "watch":
		stream, err := executeWatch(mt, mt.DB, sess, op.Arguments)
		if op.opError == nil && err == nil {
			assert.Nil(mt, op.Result, "unexpected result for watch: %v", op.Result)
			_ = stream.Close(mtest.Background)
		}
		return err
	case "listCollectionObjects":
		mt.Skipf("operation %v not implemented", op.Name)
	default:
		mt.Fatalf("unrecognized database operation: %v", op.Name)
	}
	return nil
}

func executeClientOperation(mt *mtest.T, op *operation, sess mongo.Session) error {
	switch op.Name {
	case "listDatabaseNames":
		_, err := executeListDatabaseNames(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			assert.Nil(mt, op.Result, "unexpected result for countDocuments: %v", op.Result)
		}
		return err
	case "listDatabases":
		res, err := executeListDatabases(mt, sess, op.Arguments)
		if op.opError == nil && err == nil {
			verifyListDatabasesResult(mt, res, op.Result)
		}
		return err
	case "watch":
		stream, err := executeWatch(mt, mt.Client, sess, op.Arguments)
		if op.opError == nil && err == nil {
			assert.Nil(mt, op.Result, "unexpected result for watch: %v", op.Result)
			_ = stream.Close(mtest.Background)
		}
		return err
	case "listDatabaseObjects":
		mt.Skipf("operation %v not implemented", op.Name)
	default:
		mt.Fatalf("unrecognized client operation: %v", op.Name)
	}
	return nil
}

func setupSessions(mt *mtest.T, test *testCase) (mongo.Session, mongo.Session) {
	mt.Helper()

	var sess0Opts, sess1Opts *options.SessionOptions
	if opts, err := test.SessionOptions.LookupErr("session0"); err == nil {
		sess0Opts = createSessionOptions(mt, opts.Document())
	}
	if opts, err := test.SessionOptions.LookupErr("session1"); err == nil {
		sess1Opts = createSessionOptions(mt, opts.Document())
	}

	sess0, err := mt.Client.StartSession(sess0Opts)
	assert.Nil(mt, err, "error creating session0: %v", err)
	sess1, err := mt.Client.StartSession(sess1Opts)
	assert.Nil(mt, err, "error creating session1: %v", err)

	return sess0, sess1
}

func insertDocuments(mt *mtest.T, coll *mongo.Collection, rawDocs []bson.Raw) {
	mt.Helper()

	docsToInsert := rawSliceToInterfaceSlice(rawDocs)
	if len(docsToInsert) == 0 {
		return
	}

	_, err := coll.InsertMany(mtest.Background, docsToInsert)
	assert.Nil(mt, err, "InsertMany error for collection %v: %v", coll.Name(), err)
}

// load initial data into appropriate collections and set chunkSize for the test case if necessary
func setupTest(mt *mtest.T, testFile *testFile, testCase *testCase) {
	mt.Helper()

	// all setup should be done with the global client instead of the test client to prevent any errors created by
	// client configurations.
	setupClient := mt.GlobalClient()
	// key vault data
	if len(testFile.KeyVaultData) > 0 {
		keyVaultColl := mt.CreateCollection(mtest.Collection{
			Name:   "datakeys",
			DB:     "admin",
			Client: setupClient,
		}, false)

		insertDocuments(mt, keyVaultColl, testFile.KeyVaultData)
	}

	// regular documents
	if testFile.Data.Documents != nil {
		insertColl := setupClient.Database(mt.DB.Name()).Collection(mt.Coll.Name())
		insertDocuments(mt, insertColl, testFile.Data.Documents)
		return
	}

	// GridFS data
	gfsData := testFile.Data.GridFSData

	if gfsData.Chunks != nil {
		chunks := mt.CreateCollection(mtest.Collection{
			Name:   gridFSChunks,
			Client: setupClient,
		}, false)
		insertDocuments(mt, chunks, gfsData.Chunks)
	}
	if gfsData.Files != nil {
		files := mt.CreateCollection(mtest.Collection{
			Name:   gridFSFiles,
			Client: setupClient,
		}, false)
		insertDocuments(mt, files, gfsData.Files)

		csVal, err := gfsData.Files[0].LookupErr("chunkSize")
		if err == nil {
			testCase.chunkSize = csVal.Int32()
		}
	}
}

func verifyTestOutcome(mt *mtest.T, outcomeColl *outcomeCollection) {
	// Outcome needs to be verified using the global client instead of the test client because certain client
	// configurations will cause outcome checking to fail. For example, a client configured with auto encryption
	// will decrypt results, causing comparisons to fail.

	collName := mt.Coll.Name()
	if outcomeColl.Name != "" {
		collName = outcomeColl.Name
	}
	coll := mt.GlobalClient().Database(mt.DB.Name()).Collection(collName)

	var err error
	coll, err = coll.Clone(checkOutcomeOpts)
	assert.Nil(mt, err, "Clone error: %v", err)

	cursor, err := coll.Find(mtest.Background, bson.D{})
	assert.Nil(mt, err, "Find error: %v", err)
	verifyCursorResult(mt, cursor, outcomeColl.Data)
}
