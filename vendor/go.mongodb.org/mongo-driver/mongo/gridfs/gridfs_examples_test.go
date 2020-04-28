// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package gridfs_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ExampleBucket_OpenUploadStream() {
	var fileContent []byte
	var bucket *gridfs.Bucket

	// Specify the Metadata option to include a "metadata" field in the files collection document.
	uploadOpts := options.GridFSUpload().SetMetadata(bson.D{{"metadata tag", "tag"}})
	uploadStream, err := bucket.OpenUploadStream("filename", uploadOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = uploadStream.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Use SetWriteDeadline to force a timeout if the upload does not succeed in 2 seconds.
	if err = uploadStream.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Fatal(err)
	}

	if _, err = uploadStream.Write(fileContent); err != nil {
		log.Fatal(err)
	}
}

func ExampleBucket_UploadFromStream() {
	var fileContent []byte
	var bucket *gridfs.Bucket

	// Specify the Metadata option to include a "metadata" field in the files collection document.
	uploadOpts := options.GridFSUpload().SetMetadata(bson.D{{"metadata tag", "tag"}})
	fileID, err := bucket.UploadFromStream("filename", bytes.NewBuffer(fileContent), uploadOpts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("new file created with ID %s", fileID)
}

func ExampleBucket_OpenDownloadStream() {
	var bucket *gridfs.Bucket
	var fileID primitive.ObjectID

	downloadStream, err := bucket.OpenDownloadStream(fileID)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := downloadStream.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Use SetReadDeadline to force a timeout if the download does not succeed in 2 seconds.
	if err = downloadStream.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Fatal(err)
	}

	fileBuffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(fileBuffer, downloadStream); err != nil {
		log.Fatal(err)
	}
}

func ExampleBucket_DownloadToStream() {
	var bucket *gridfs.Bucket
	var fileID primitive.ObjectID

	fileBuffer := bytes.NewBuffer(nil)
	if _, err := bucket.DownloadToStream(fileID, fileBuffer); err != nil {
		log.Fatal(err)
	}
}

func ExampleBucket_Delete() {
	var bucket *gridfs.Bucket
	var fileID primitive.ObjectID

	if err := bucket.Delete(fileID); err != nil {
		log.Fatal(err)
	}
}

func ExampleBucket_Find() {
	var bucket *gridfs.Bucket

	// Specify a filter to find all files with a length greater than 1000 bytes.
	filter := bson.D{
		{"length", bson.D{{"$gt", 1000}}},
	}
	cursor, err := bucket.Find(filter)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := cursor.Close(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()

	type gridfsFile struct {
		Name   string `bson:"filename"`
		Length int64  `bson:"length"`
	}
	var foundFiles []gridfsFile
	if err = cursor.All(context.TODO(), &foundFiles); err != nil {
		log.Fatal(err)
	}

	for _, file := range foundFiles {
		fmt.Printf("filename: %s, length: %d\n", file.Name, file.Length)
	}
}

func ExampleBucket_Rename() {
	var bucket *gridfs.Bucket
	var fileID primitive.ObjectID

	if err := bucket.Rename(fileID, "new file name"); err != nil {
		log.Fatal(err)
	}
}

func ExampleBucket_Drop() {
	var bucket *gridfs.Bucket

	if err := bucket.Drop(); err != nil {
		log.Fatal(err)
	}
}
