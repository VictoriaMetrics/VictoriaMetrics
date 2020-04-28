// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

// Package gridfs provides a MongoDB GridFS API. See https://docs.mongodb.com/manual/core/gridfs/ for more
// information about GridFS and its use cases.
//
// Buckets
//
// The main type defined in this package is Bucket. A Bucket wraps a mongo.Database instance and operates on two
// collections in the database. The first is the files collection, which contains one metadata document per file stored
// in the bucket. This collection is named "<bucket name>.files". The second is the chunks collection, which contains
// chunks of files. This collection is named "<bucket name>.chunks".
//
// Uploading a File
//
// Files can be uploaded in two ways:
// 	1. OpenUploadStream/OpenUploadStreamWithID - These methods return an UploadStream instance. UploadStream
// 	implements the io.Writer interface and the Write() method can be used to upload a file to the database.
//
//	2. UploadFromStream/UploadFromStreamWithID - These methods take an io.Reader, which represents the file to
// 	upload. They internally create a new UploadStream and close it once the operation is complete.
//
// Downloading a File
//
// Similar to uploads, files can be downloaded in two ways:
//	1. OpenDownloadStream/OpenDownloadStreamByName - These methods return a DownloadStream instance. DownloadStream
//	implements the io.Reader interface. A file can be read either using the Read() method or any standard library
//	methods that reads from an io.Reader such as io.Copy.
//
//	2. DownloadToStream/DownloadToStreamByName - These methods take an io.Writer, which represents the download
// 	destination. They internally create a new DownloadStream and close it once the operation is complete.
package gridfs
