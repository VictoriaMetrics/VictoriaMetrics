// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package gridfs

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

var (
	connsCheckedOut int
)

func TestGridFS(t *testing.T) {
	cs := testutil.ConnString(t)
	poolMonitor := &event.PoolMonitor{
		Event: func(evt *event.PoolEvent) {
			switch evt.Type {
			case event.GetSucceeded:
				connsCheckedOut++
			case event.ConnectionReturned:
				connsCheckedOut--
			}
		},
	}
	clientOpts := options.Client().ApplyURI(cs.Original).SetReadPreference(readpref.Primary()).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority())).SetPoolMonitor(poolMonitor)
	client, err := mongo.Connect(context.Background(), clientOpts)
	assert.Nil(t, err, "Connect error: %v", err)
	db := client.Database("gridfs")
	defer func() {
		sessions := client.NumberSessionsInProgress()
		conns := connsCheckedOut

		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
		assert.Equal(t, 0, sessions, "%v sessions checked out", sessions)
		assert.Equal(t, 0, conns, "%v connections checked out", conns)
	}()

	// Unit tests showing the chunk size is set correctly on the bucket and upload stream objects.
	t.Run("ChunkSize", func(t *testing.T) {
		chunkSizeTests := []struct {
			testName   string
			bucketOpts *options.BucketOptions
			uploadOpts *options.UploadOptions
		}{
			{"Default values", nil, nil},
			{"Options provided without chunk size", options.GridFSBucket(), options.GridFSUpload()},
			{"Bucket chunk size set", options.GridFSBucket().SetChunkSizeBytes(27), nil},
			{"Upload stream chunk size set", nil, options.GridFSUpload().SetChunkSizeBytes(27)},
			{"Bucket and upload set to different values", options.GridFSBucket().SetChunkSizeBytes(27), options.GridFSUpload().SetChunkSizeBytes(31)},
		}

		for _, tt := range chunkSizeTests {
			t.Run(tt.testName, func(t *testing.T) {
				bucket, err := NewBucket(db, tt.bucketOpts)
				assert.Nil(t, err, "NewBucket error: %v", err)

				us, err := bucket.OpenUploadStream("filename", tt.uploadOpts)
				assert.Nil(t, err, "OpenUploadStream error: %v", err)

				expectedBucketChunkSize := DefaultChunkSize
				if tt.bucketOpts != nil && tt.bucketOpts.ChunkSizeBytes != nil {
					expectedBucketChunkSize = *tt.bucketOpts.ChunkSizeBytes
				}
				assert.Equal(t, expectedBucketChunkSize, bucket.chunkSize,
					"expected chunk size %v, got %v", expectedBucketChunkSize, bucket.chunkSize)

				expectedUploadChunkSize := expectedBucketChunkSize
				if tt.uploadOpts != nil && tt.uploadOpts.ChunkSizeBytes != nil {
					expectedUploadChunkSize = *tt.uploadOpts.ChunkSizeBytes
				}
				assert.Equal(t, expectedUploadChunkSize, us.chunkSize,
					"expected chunk size %v, got %v", expectedUploadChunkSize, us.chunkSize)
			})
		}
	})
}
