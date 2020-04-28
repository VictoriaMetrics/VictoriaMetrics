// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

type gridfsTestFile struct {
	Data  gridfsData   `bson:"data"`
	Tests []gridfsTest `bson:"tests"`
}

type gridfsData struct {
	Files  []bson.Raw  `bson:"files"`
	Chunks []bsonx.Doc `bson:"chunks"`
}

type gridfsTest struct {
	Description string        `bson:"description"`
	Arrange     gridfsArrange `bson:"arrange"`
	Act         gridfsAct     `bson:"act"`
	Assert      gridfsAssert  `bson:"assert"`
}

type gridfsArrange struct {
	Data []bsonx.Doc `bson:"data"`
}

type gridfsAct struct {
	Operation string   `bson:"operation"`
	Arguments bson.Raw `bson:"arguments"`
}

type gridfsAssert struct {
	Result interface{} `bson:"result"`
	Error  string      `bson:"error"`
	Data   []bsonx.Doc `bson:"data"`
}

const (
	gridfsTestsDir       = "../../data/gridfs"
	gridfsFiles          = "fs.files"
	gridfsChunks         = "fs.chunks"
	gridfsExpectedFiles  = "expected.files"
	gridfsExpectedChunks = "expected.chunks"
)

var (
	gridfsDeadline = time.Now().Add(time.Hour)
	downloadBuffer = make([]byte, 100)
)

func TestGridFSSpec(t *testing.T) {
	mt := mtest.New(t, noClientOpts)
	defer mt.Close()

	for _, file := range jsonFilesInDir(mt, gridfsTestsDir) {
		mt.Run(file, func(mt *mtest.T) {
			runGridfsTestFile(mt, path.Join(gridfsTestsDir, file))
		})
	}
}

func runGridfsTestFile(mt *mtest.T, filePath string) {
	content, err := ioutil.ReadFile(filePath)
	assert.Nil(mt, err, "ReadFile error for %v: %v", filePath, err)

	var testFile gridfsTestFile
	err = bson.UnmarshalExtJSONWithRegistry(specTestRegistry, content, false, &testFile)
	assert.Nil(mt, err, "UnmarshalExtJSONWithRegistry error: %v", err)

	for _, test := range testFile.Tests {
		mt.Run(test.Description, func(mt *mtest.T) {
			runGridfsTest(mt, test, testFile)
		})
	}
}

func runGridfsTest(mt *mtest.T, test gridfsTest, testFile gridfsTestFile) {
	chunkSize := setupGridfsTest(mt, testFile.Data)
	if chunkSize == 0 {
		chunkSize = gridfs.DefaultChunkSize
	}
	bucket, err := gridfs.NewBucket(mt.DB, options.GridFSBucket().SetChunkSizeBytes(chunkSize))
	assert.Nil(mt, err, "NewBucket error: %v", err)
	err = bucket.SetWriteDeadline(gridfsDeadline)
	assert.Nil(mt, err, "SetWriteDeadline error: %v", err)
	err = bucket.SetReadDeadline(gridfsDeadline)
	assert.Nil(mt, err, "SetReadDeadline error: %v", err)

	arrangeGridfsCollections(mt, test.Arrange)
	switch test.Act.Operation {
	case "upload":
		executeGridfsUpload(mt, test, bucket)
		checkGridfsResults(mt, test)
		clearGridfsCollections(mt)

		arrangeGridfsCollections(mt, test.Arrange)
		executeGridfsUploadFromStream(mt, test, bucket)
		checkGridfsResults(mt, test)
	case "download":
		executeGridfsDownload(mt, test, bucket)
		checkGridfsResults(mt, test)

		executeGridfsDownloadToStream(mt, test, bucket)
		checkGridfsResults(mt, test)
	case "download_by_name":
		executeGridfsDownloadByName(mt, test, bucket)
		checkGridfsResults(mt, test)

		executeGridfsDownloadByNameToStream(mt, test, bucket)
		checkGridfsResults(mt, test)
	case "delete":
		executeGridfsDelete(mt, test, bucket)
		checkGridfsResults(mt, test)
	}
}

func checkGridfsResults(mt *mtest.T, test gridfsTest) {
	if test.Assert.Error != "" {
		// don't compare collections in error cases
		return
	}
	compareGridfsCollections(mt, gridfsExpectedChunks, gridfsChunks)
	compareGridfsCollections(mt, gridfsExpectedFiles, gridfsFiles)
}

func compareGridfsCollections(mt *mtest.T, expected, actual string) {
	expectedCursor, err := mt.DB.Collection(expected).Find(mtest.Background, bson.D{})
	assert.Nil(mt, err, "Find error for collection %v: %v", expected, err)
	actualCursor, err := mt.DB.Collection(actual).Find(mtest.Background, bson.D{})
	assert.Nil(mt, err, "Find error for collection %v: %v", actual, err)

	var idx int
	for expectedCursor.Next(mtest.Background) {
		assert.True(mt, actualCursor.Next(mtest.Background), "Next returned false at index %v", idx)
		idx++

		compareGridfsDocs(mt, expectedCursor.Current, actualCursor.Current)
	}
	assert.False(mt, actualCursor.Next(mtest.Background),
		"found unexpected document in collection %v: %s", expected, actualCursor.Current)
}

func compareGridfsDocs(mt *mtest.T, expected, actual bson.Raw) {
	mt.Helper()

	eElems, err := expected.Elements()
	assert.Nil(mt, err, "error getting expected elements: %v", err)

	for i, e := range eElems {
		eKey := e.Key()
		// skip deprecated fields
		if eKey == "md5" || eKey == "contentType" {
			continue
		}

		aVal, err := actual.LookupErr(eKey)
		assert.Nil(mt, err, "key %s not found in result", e.Key())

		// skip fields with unknown values
		if eKey == "_id" || eKey == "uploadDate" {
			continue
		}

		eVal := e.Value()
		if err := compareValues(mt, eKey, eVal, aVal); err != nil {
			mt.Fatalf("document mismatch at index %d: %s", i, err)
		}
	}
}

func arrangeGridfsCollections(mt *mtest.T, arrange gridfsArrange) {
	if len(arrange.Data) == 0 {
		return
	}

	var arrangeCmds []interface{}
	for _, cmd := range arrange.Data {
		if cmd[0].Key != "update" {
			arrangeCmds = append(arrangeCmds, cmd)
			continue
		}

		updatesIdx := cmd.IndexOf("updates")
		if updatesIdx == -1 {
			arrangeCmds = append(arrangeCmds, cmd)
			continue
		}
		updates := cmd[updatesIdx].Value.Array()
		for idx, update := range updates {
			updateDoc := update.Document()
			hexBytes := hexStringToBytes(mt, updateDoc.Lookup("u", "$set", "data", "$hex").StringValue())
			query := updateDoc.Lookup("q").Document()
			newUpdate := bsonx.Doc{
				{"q", bsonx.Document(query)},
				{"u", bsonx.Document(bsonx.Doc{
					{"$set", bsonx.Document(bsonx.Doc{
						{"data", bsonx.Binary(0x00, hexBytes)},
					})},
				})},
			}
			updates[idx] = bsonx.Document(newUpdate)
		}
		cmd[updatesIdx] = bsonx.Elem{"updates", bsonx.Array(updates)}
		arrangeCmds = append(arrangeCmds, cmd)
	}
	runCommands(mt, arrangeCmds)
}

func executeUploadAssert(mt *mtest.T, fileID primitive.ObjectID, assert gridfsAssert) {
	fileIDVal := bsonx.ObjectID(fileID)

	var assertCommands []interface{}
	for _, data := range assert.Data {
		documentsIdx := data.IndexOf("documents")
		if documentsIdx == -1 {
			continue
		}

		documents := data[documentsIdx].Value.Array()
		for idx, arrayDoc := range documents {
			doc := arrayDoc.Document()

			// set or remove _id field
			if idIdx := doc.IndexOf("_id"); idIdx != -1 {
				idVal := doc[idIdx].Value
				switch idVal.Type() {
				case bsontype.String:
					if idVal.StringValue() == "*actual" {
						// _id will be generated by server
						doc = append(doc[:idIdx], doc[idIdx+1:]...)
					}
				default:
					doc[idIdx] = bsonx.Elem{"_id", fileIDVal}
				}
			}
			// set files_id field
			if filesIdx := doc.IndexOf("files_id"); filesIdx != -1 {
				doc[filesIdx] = bsonx.Elem{"files_id", fileIDVal}
			}

			dataIdx := doc.IndexOf("data")
			if dataIdx == -1 {
				continue
			}
			data := doc[dataIdx].Value
			if data.Type() != bsontype.EmbeddedDocument {
				continue
			}

			hexBytes := hexStringToBytes(mt, data.Document().Lookup("$hex").StringValue())
			doc[dataIdx] = bsonx.Elem{"data", bsonx.Binary(0x00, hexBytes)}
			documents[idx] = bsonx.Document(doc)
		}
		data[documentsIdx] = bsonx.Elem{"documents", bsonx.Array(documents)}
		assertCommands = append(assertCommands, data)
	}

	runCommands(mt, assertCommands)
}

func createUploadOptions(mt *mtest.T, args bson.Raw) *options.UploadOptions {
	opts := options.GridFSUpload()
	optionsVal, err := args.LookupErr("options")
	if err != nil {
		return opts
	}

	elems, _ := optionsVal.Document().Elements()
	for _, elem := range elems {
		key := elem.Key()
		opt := elem.Value()

		switch key {
		case "chunkSizeBytes":
			opts.SetChunkSizeBytes(opt.Int32())
		case "metadata":
			opts.SetMetadata(opt.Document())
		case "contentType", "disableMD5":
			// deprecated options
		default:
			mt.Fatalf("unrecognized upload option %v", key)
		}
	}
	return opts
}

func executeGridfsUpload(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	args := test.Act.Arguments
	uploadOpts := createUploadOptions(mt, args)
	hexBytes := hexStringToBytes(mt, args.Lookup("source", "$hex").StringValue())
	fileID := primitive.NewObjectID()
	stream, err := bucket.OpenUploadStreamWithID(fileID, args.Lookup("filename").StringValue(), uploadOpts)
	err = stream.SetWriteDeadline(gridfsDeadline)
	assert.Nil(mt, err, "SetWriteDeadline error: %v", err)

	n, err := stream.Write(hexBytes)
	assert.Nil(mt, err, "Write error: %v", err)
	assert.Equal(mt, len(hexBytes), n, "expected %v bytes written, got %v", len(hexBytes), n)
	err = stream.Close()
	assert.Nil(mt, err, "Close error: %v", err)
	executeUploadAssert(mt, fileID, test.Assert)
}

func executeGridfsUploadFromStream(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	args := test.Act.Arguments
	uploadOpts := createUploadOptions(mt, args)
	hexBytes := hexStringToBytes(mt, args.Lookup("source", "$hex").StringValue())
	fileID := primitive.NewObjectID()
	filename := args.Lookup("filename").StringValue()
	buffer := bytes.NewBuffer(hexBytes)
	err := bucket.UploadFromStreamWithID(fileID, filename, buffer, uploadOpts)
	assert.Nil(mt, err, "UploadFromStreamWithID error: %v", err)
	executeUploadAssert(mt, fileID, test.Assert)
}

func executeGridfsDownload(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	stream, err := bucket.OpenDownloadStream(test.Act.Arguments.Lookup("id").ObjectID())
	var copied int
	if err == nil {
		copied, err = stream.Read(downloadBuffer)
	}
	compareGridfsDownloadAssert(mt, copied, err, test)
}

func executeGridfsDownloadToStream(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	buffer := bytes.NewBuffer(downloadBuffer)
	copied, err := bucket.DownloadToStream(test.Act.Arguments.Lookup("id").ObjectID(), buffer)
	compareGridfsDownloadAssert(mt, int(copied), err, test)
}

func compareGridfsDownloadAssert(mt *mtest.T, copied int, err error, test gridfsTest) {
	if test.Assert.Error != "" {
		assert.NotNil(mt, err, "expected Read error, got nil")
		compareGridfsAssertError(mt, test.Assert.Error, err)
	}
	if test.Assert.Result != nil {
		result := test.Assert.Result.(bson.Raw)
		hexBytes := hexStringToBytes(mt, result.Lookup("$hex").StringValue())
		assert.Equal(mt, len(hexBytes), copied, "expected to read %v bytes, read %v", len(hexBytes), copied)
		assert.Equal(mt, hexBytes, downloadBuffer[:copied], "expected bytes %v, got %v", hexBytes, downloadBuffer[:copied])
		return
	}
}

func createDownloadByNameOptions(mt *mtest.T, args bson.Raw) *options.NameOptions {
	opts := options.GridFSName()
	optionsVal, err := args.LookupErr("options")
	if err != nil {
		return opts
	}

	elems, _ := optionsVal.Document().Elements()
	for _, elem := range elems {
		key := elem.Key()
		opt := elem.Value()

		switch key {
		case "revision":
			opts.SetRevision(opt.Int32())
		default:
			mt.Fatalf("unrecognized download by name option: %v", key)
		}
	}
	return opts
}

func executeGridfsDownloadByName(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	args := test.Act.Arguments
	opts := createDownloadByNameOptions(mt, args)
	stream, err := bucket.OpenDownloadStreamByName(args.Lookup("filename").StringValue(), opts)
	var copied int
	if err == nil {
		copied, err = stream.Read(downloadBuffer)
	}
	compareGridfsDownloadAssert(mt, copied, err, test)
}

func executeGridfsDownloadByNameToStream(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	args := test.Act.Arguments
	opts := createDownloadByNameOptions(mt, args)
	buffer := bytes.NewBuffer(downloadBuffer)
	copied, err := bucket.DownloadToStreamByName(args.Lookup("filename").StringValue(), buffer, opts)
	compareGridfsDownloadAssert(mt, int(copied), err, test)
}

func executeGridfsDelete(mt *mtest.T, test gridfsTest, bucket *gridfs.Bucket) {
	err := bucket.Delete(test.Act.Arguments.Lookup("id").ObjectID())
	if test.Assert.Error != "" {
		assert.NotNil(mt, err, "expected Delete error, got nil")
		compareGridfsAssertError(mt, test.Assert.Error, err)
		return
	}
	var cmds []interface{}
	for _, cmd := range test.Assert.Data {
		cmds = append(cmds, cmd)
	}
	runCommands(mt, cmds)
}

func setupGridfsTest(mt *mtest.T, data gridfsData) int32 {
	filesColl := mt.CreateCollection(mtest.Collection{Name: gridfsFiles}, true)
	chunksColl := mt.CreateCollection(mtest.Collection{Name: gridfsChunks}, true)
	expectedFilesColl := mt.CreateCollection(mtest.Collection{Name: gridfsExpectedFiles}, true)
	expectedChunksColl := mt.CreateCollection(mtest.Collection{Name: gridfsExpectedChunks}, true)

	var chunkSize int32
	for _, file := range data.Files {
		if cs, err := file.LookupErr("chunkSize"); err == nil {
			chunkSize = cs.Int32()
			break
		}
	}
	var chunksDocs []interface{}
	for _, chunk := range data.Chunks {
		if hexStr, err := chunk.LookupErr("data", "$hex"); err == nil {
			hexBytes := hexStringToBytes(mt, hexStr.StringValue())
			chunk = chunk.Set("data", bsonx.Binary(0x00, hexBytes))
		}
		chunksDocs = append(chunksDocs, chunk)
	}

	insertDocuments(mt, filesColl, data.Files)
	insertDocuments(mt, expectedFilesColl, data.Files)
	if len(chunksDocs) == 0 {
		return chunkSize
	}

	_, err := chunksColl.InsertMany(mtest.Background, chunksDocs)
	assert.Nil(mt, err, "InsertMany error for collection %v: %v", chunksColl.Name(), err)
	_, err = expectedChunksColl.InsertMany(mtest.Background, chunksDocs)
	assert.Nil(mt, err, "InsertMany error for collection %v: %v", expectedChunksColl.Name(), err)
	return chunkSize
}

func hexStringToBytes(mt *mtest.T, hexStr string) []byte {
	hexBytes, err := hex.DecodeString(hexStr)
	assert.Nil(mt, err, "DecodeString error for %v: %v", hexStr, err)
	return hexBytes
}

func replaceBsonValue(original bson.Raw, key string, newValue bson.RawValue) bson.Raw {
	idx, newDoc := bsoncore.AppendDocumentStart(nil)
	elems, _ := original.Elements()
	for _, elem := range elems {
		rawValue := elem.Value()
		if elem.Key() == key {
			rawValue = newValue
		}

		newDoc = bsoncore.AppendValueElement(newDoc, elem.Key(), bsoncore.Value{Type: rawValue.Type, Data: rawValue.Value})
	}
	newDoc, _ = bsoncore.AppendDocumentEnd(newDoc, idx)
	return bson.Raw(newDoc)
}

func runCommands(mt *mtest.T, commands []interface{}) {
	for _, cmd := range commands {
		err := mt.DB.RunCommand(mtest.Background, cmd).Err()
		assert.Nil(mt, err, "RunCommand error for command %v: %v", cmd, err)
	}
}

func clearGridfsCollections(mt *mtest.T) {
	mt.Helper()
	for _, coll := range []string{gridfsFiles, gridfsChunks, gridfsExpectedFiles, gridfsExpectedChunks} {
		_, err := mt.DB.Collection(coll).DeleteMany(mtest.Background, bson.D{})
		assert.Nil(mt, err, "DeleteMany error for %v: %v", coll, err)
	}
}

func compareGridfsAssertError(mt *mtest.T, assertErrString string, err error) {
	mt.Helper()

	var wantErr error
	switch assertErrString {
	case "FileNotFound", "RevisionNotFound":
		wantErr = gridfs.ErrFileNotFound
	case "ChunkIsMissing":
		wantErr = gridfs.ErrWrongIndex
	case "ChunkIsWrongSize":
		wantErr = gridfs.ErrWrongSize
	default:
		mt.Fatalf("unrecognized assert error string %v", assertErrString)
	}

	assert.Equal(mt, wantErr, err, "expected error %s, got %s", wantErr, err)
}
