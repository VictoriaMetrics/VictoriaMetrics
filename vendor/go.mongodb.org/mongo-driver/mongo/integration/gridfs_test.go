// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"bytes"
	"context"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/internal/testutil/israce"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

func TestGridFS(x *testing.T) {
	mt := mtest.New(x, noClientOpts)
	defer mt.Close()

	mt.Run("index creation", func(mt *mtest.T) {
		// Unit tests showing that UploadFromStream creates indexes on the chunks and files collections.
		bucket, err := gridfs.NewBucket(mt.DB)
		assert.Nil(mt, err, "NewBucket error: %v", err)
		err = bucket.SetWriteDeadline(time.Now().Add(5 * time.Second))
		assert.Nil(mt, err, "SetWriteDeadline error: %v", err)

		byteData := []byte("Hello, world!")
		r := bytes.NewReader(byteData)

		_, err = bucket.UploadFromStream("filename", r)
		assert.Nil(mt, err, "UploadFromStream error: %v", err)

		findCtx, cancel := context.WithTimeout(mtest.Background, 5*time.Second)
		defer cancel()
		findIndex(findCtx, mt, mt.DB.Collection("fs.files"), false, "key", "filename")
		findIndex(findCtx, mt, mt.DB.Collection("fs.chunks"), true, "key", "files_id")
	})
	mt.RunOpts("round trip", mtest.NewOptions().MaxServerVersion("3.6"), func(mt *mtest.T) {
		skipRoundTripTest(mt)
		oneK := 1024
		smallBuffSize := 100

		tests := []struct {
			name      string
			chunkSize int // make -1 for no capacity for no chunkSize
			fileSize  int
			bufSize   int // make -1 for no capacity for no bufSize
		}{
			{"RoundTrip: original", -1, oneK, -1},
			{"RoundTrip: chunk size multiple of file", oneK, oneK * 16, -1},
			{"RoundTrip: chunk size is file size", oneK, oneK, -1},
			{"RoundTrip: chunk size multiple of file size and with strict buffer size", oneK, oneK * 16, smallBuffSize},
			{"RoundTrip: chunk size multiple of file size and buffer size", oneK, oneK * 16, oneK * 16},
			{"RoundTrip: chunk size, file size, buffer size all the same", oneK, oneK, oneK},
		}

		for _, test := range tests {
			mt.Run(test.name, func(mt *mtest.T) {
				var chunkSize *int32
				var temp int32
				if test.chunkSize != -1 {
					temp = int32(test.chunkSize)
					chunkSize = &temp
				}

				bucket, err := gridfs.NewBucket(mt.DB, &options.BucketOptions{
					ChunkSizeBytes: chunkSize,
				})
				assert.Nil(mt, err, "NewBucket error: %v", err)

				timeout := 5 * time.Second
				if israce.Enabled {
					timeout = 20 * time.Second // race detector causes 2-20x slowdown
				}

				err = bucket.SetWriteDeadline(time.Now().Add(timeout))
				assert.Nil(mt, err, "SetWriteDeadline error: %v", err)

				// Test that Upload works when the buffer to write is longer than the upload stream's internal buffer.
				// This requires multiple calls to uploadChunks.
				size := test.fileSize
				p := make([]byte, size)
				for i := 0; i < size; i++ {
					p[i] = byte(rand.Intn(100))
				}

				_, err = bucket.UploadFromStream("filename", bytes.NewReader(p))
				assert.Nil(mt, err, "UploadFromStream error: %v", err)

				var w *bytes.Buffer
				if test.bufSize == -1 {
					w = bytes.NewBuffer(make([]byte, 0))
				} else {
					w = bytes.NewBuffer(make([]byte, 0, test.bufSize))
				}

				_, err = bucket.DownloadToStreamByName("filename", w)
				assert.Nil(mt, err, "DownloadToStreamByName error: %v", err)
				assert.Equal(mt, p, w.Bytes(), "downloaded file did not match p")
			})
		}
	})
}

func findIndex(ctx context.Context, mt *mtest.T, coll *mongo.Collection, unique bool, keys ...string) {
	mt.Helper()
	cur, err := coll.Indexes().List(ctx)
	assert.Nil(mt, err, "Indexes List error: %v", err)

	foundIndex := false
	for cur.Next(ctx) {
		if _, err := cur.Current.LookupErr(keys...); err == nil {
			if uVal, err := cur.Current.LookupErr("unique"); (unique && err == nil && uVal.Boolean() == true) ||
				(!unique && (err != nil || uVal.Boolean() == false)) {

				foundIndex = true
			}
		}
	}
	assert.True(mt, foundIndex, "index %v not found", keys)
}

func skipRoundTripTest(mt *mtest.T) {
	if runtime.GOOS != "darwin" {
		return
	}

	var serverStatus bsonx.Doc
	err := mt.DB.RunCommand(
		context.Background(),
		bsonx.Doc{{"serverStatus", bsonx.Int32(1)}},
	).Decode(&serverStatus)
	assert.Nil(mt, err, "serverStatus error %v", err)

	// can run on non-sharded clusters or on sharded cluster with auth/ssl disabled
	_, err = serverStatus.LookupErr("sharding")
	if err != nil {
		return
	}
	_, err = serverStatus.LookupErr("security")
	if err != nil {
		return
	}
	mt.Skip("skipping round trip test")
}
