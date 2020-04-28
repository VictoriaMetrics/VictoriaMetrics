// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"bytes"
	"context"
	"math"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Helper functions to execute and verify results from CRUD methods.

var emptyDoc = []byte{5, 0, 0, 0, 0}

// create an update document or pipeline from a bson.RawValue
func createUpdate(mt *mtest.T, updateVal bson.RawValue) interface{} {
	switch updateVal.Type {
	case bson.TypeEmbeddedDocument:
		return updateVal.Document()
	case bson.TypeArray:
		var updateDocs []bson.Raw
		docs, _ := updateVal.Array().Values()
		for _, doc := range docs {
			updateDocs = append(updateDocs, doc.Document())
		}

		return updateDocs
	default:
		mt.Fatalf("unrecognized update type: %v", updateVal.Type)
	}

	return nil
}

// kill all open sessions on the server. This function uses mt.GlobalClient() because killAllSessions is not allowed
// for clients configured with specific options (e.g. client side encryption).
func killSessions(mt *mtest.T) {
	mt.Helper()

	err := mt.GlobalClient().Database("admin").RunCommand(mtest.Background, bson.D{
		{"killAllSessions", bson.A{}},
	}, options.RunCmd().SetReadPreference(mtest.PrimaryRp)).Err()
	if err == nil {
		return
	}
	if cmdErr, ok := err.(mongo.CommandError); !ok || cmdErr.Code != errorInterrupted {
		mt.Fatalf("unable to killAllSessions: %v", err)
	}
}

// aggregator is an interface used to run collection and database-level aggregations
type aggregator interface {
	Aggregate(context.Context, interface{}, ...*options.AggregateOptions) (*mongo.Cursor, error)
}

// watcher is an interface used to create client, db, and collection-level change streams
type watcher interface {
	Watch(context.Context, interface{}, ...*options.ChangeStreamOptions) (*mongo.ChangeStream, error)
}

func executeAggregate(mt *mtest.T, agg aggregator, sess mongo.Session, args bson.Raw) (*mongo.Cursor, error) {
	mt.Helper()

	var pipeline []interface{}
	opts := options.Aggregate()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "pipeline":
			pipeline = rawArrayToInterfaceSlice(val.Array())
		case "batchSize":
			opts.SetBatchSize(val.Int32())
		case "collation":
			opts.SetCollation(createCollation(mt, val.Document()))
		case "maxTimeMS":
			opts.SetMaxTime(time.Duration(val.Int32()) * time.Millisecond)
		case "allowDiskUse":
			opts.SetAllowDiskUse(val.Boolean())
		case "session":
		default:
			mt.Fatalf("unrecognized aggregate option: %v", key)
		}
	}

	if sess != nil {
		var cur *mongo.Cursor
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var aerr error
			cur, aerr = agg.Aggregate(sc, pipeline, opts)
			return aerr
		})
		return cur, err
	}
	return agg.Aggregate(mtest.Background, pipeline, opts)
}

func executeWatch(mt *mtest.T, w watcher, sess mongo.Session, args bson.Raw) (*mongo.ChangeStream, error) {
	mt.Helper()

	pipeline := []interface{}{}
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "pipeline":
			pipeline = rawArrayToInterfaceSlice(val.Array())
		default:
			mt.Fatalf("unrecognized watch option: %v", key)
		}
	}

	if sess != nil {
		var stream *mongo.ChangeStream
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var csErr error
			stream, csErr = w.Watch(sc, pipeline)
			return csErr
		})
		return stream, err
	}
	return w.Watch(mtest.Background, pipeline)
}

func executeCountDocuments(mt *mtest.T, sess mongo.Session, args bson.Raw) (int64, error) {
	mt.Helper()

	filter := emptyDoc
	opts := options.Count()

	elems, _ := args.Elements()
	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "filter":
			filter = opt.Document()
		case "skip":
			opts = opts.SetSkip(int64(opt.Int32()))
		case "limit":
			opts = opts.SetLimit(int64(opt.Int32()))
		case "collation":
			opts = opts.SetCollation(createCollation(mt, opt.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized count option: %v", name)
		}
	}

	if sess != nil {
		var count int64
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var countErr error
			count, countErr = mt.Coll.CountDocuments(sc, filter, opts)
			return countErr
		})
		return count, err
	}
	return mt.Coll.CountDocuments(mtest.Background, filter, opts)
}

func executeInsertOne(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.InsertOneResult, error) {
	mt.Helper()

	doc := emptyDoc
	opts := options.InsertOne()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "document":
			doc = val.Document()
		case "bypassDocumentValidation":
			opts.SetBypassDocumentValidation(val.Boolean())
		case "session":
		default:
			mt.Fatalf("unrecognized insertOne option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.InsertOneResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var insertErr error
			res, insertErr = mt.Coll.InsertOne(sc, doc, opts)
			return insertErr
		})
		return res, err
	}
	return mt.Coll.InsertOne(mtest.Background, doc, opts)
}

func executeInsertMany(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.InsertManyResult, error) {
	mt.Helper()

	var docs []interface{}
	opts := options.InsertMany()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "documents":
			docs = rawArrayToInterfaceSlice(val.Array())
		case "options":
			// Some of the older tests use this to set the "ordered" option
			optsDoc := val.Document()
			optsElems, _ := optsDoc.Elements()
			assert.Equal(mt, 1, len(optsElems), "expected 1 options element, got %v", len(optsElems))
			opts.SetOrdered(optsDoc.Lookup("ordered").Boolean())
		case "session":
		default:
			mt.Fatalf("unrecognized insertMany option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.InsertManyResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var insertErr error
			res, insertErr = mt.Coll.InsertMany(sc, docs, opts)
			return insertErr
		})
		return res, err
	}
	return mt.Coll.InsertMany(mtest.Background, docs)
}

func setFindModifiers(mt *mtest.T, modifiersDoc bson.Raw, opts *options.FindOptions) {
	elems, _ := modifiersDoc.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "$comment":
			opts.SetComment(val.StringValue())
		case "$hint":
			opts.SetHint(val.Document())
		case "$max":
			opts.SetMax(val.Document())
		case "$maxTimeMS":
			opts.SetMaxTime(time.Duration(val.Int32()) * time.Millisecond)
		case "$min":
			opts.SetMin(val.Document())
		case "$returnKey":
			opts.SetReturnKey(val.Boolean())
		case "$showDiskLoc":
			opts.SetShowRecordID(val.Boolean())
		}
	}
}

func executeFind(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.Cursor, error) {
	mt.Helper()

	filter := emptyDoc
	opts := options.Find()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "sort":
			opts = opts.SetSort(val.Document())
		case "skip":
			opts = opts.SetSkip(numberFromValue(mt, val))
		case "limit":
			opts = opts.SetLimit(numberFromValue(mt, val))
		case "batchSize":
			opts = opts.SetBatchSize(int32(numberFromValue(mt, val)))
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "modifiers":
			setFindModifiers(mt, val.Document(), opts)
		case "session":
		default:
			mt.Fatalf("unrecognized find option: %v", key)
		}
	}

	if sess != nil {
		var c *mongo.Cursor
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var findErr error
			c, findErr = mt.Coll.Find(sc, filter, opts)
			return findErr
		})
		return c, err
	}
	return mt.Coll.Find(mtest.Background, filter, opts)
}

func executeRunCommand(mt *mtest.T, sess mongo.Session, args bson.Raw) *mongo.SingleResult {
	mt.Helper()

	cmd := emptyDoc
	opts := options.RunCmd()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "command":
			cmd = val.Document()
		case "readPreference":
			opts.SetReadPreference(createReadPref(val))
		case "session":
		default:
			mt.Fatalf("unrecognized runCommand option: %v", key)
		}
	}

	if sess != nil {
		var sr *mongo.SingleResult
		_ = mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			sr = mt.DB.RunCommand(sc, cmd, opts)
			return nil
		})
		return sr
	}
	return mt.DB.RunCommand(mtest.Background, cmd, opts)
}

func executeListCollections(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.Cursor, error) {
	mt.Helper()

	filter := emptyDoc
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		default:
			mt.Fatalf("unrecognized listCollectionNames option: %v", key)
		}
	}

	if sess != nil {
		var c *mongo.Cursor
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var lcErr error
			c, lcErr = mt.DB.ListCollections(sc, filter)
			return lcErr
		})
		return c, err
	}
	return mt.DB.ListCollections(mtest.Background, filter)
}

func executeListCollectionNames(mt *mtest.T, sess mongo.Session, args bson.Raw) ([]string, error) {
	mt.Helper()

	filter := emptyDoc
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		default:
			mt.Fatalf("unrecognized listCollectionNames option: %v", key)
		}
	}

	if sess != nil {
		var res []string
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var lcErr error
			res, lcErr = mt.DB.ListCollectionNames(sc, filter)
			return lcErr
		})
		return res, err
	}
	return mt.DB.ListCollectionNames(mtest.Background, filter)
}

func executeListDatabaseNames(mt *mtest.T, sess mongo.Session, args bson.Raw) ([]string, error) {
	mt.Helper()

	filter := emptyDoc
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		default:
			mt.Fatalf("unrecognized listCollectionNames option: %v", key)
		}
	}

	if sess != nil {
		var res []string
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var ldErr error
			res, ldErr = mt.Client.ListDatabaseNames(sc, filter)
			return ldErr
		})
		return res, err
	}
	return mt.Client.ListDatabaseNames(mtest.Background, filter)
}

func executeListDatabases(mt *mtest.T, sess mongo.Session, args bson.Raw) (mongo.ListDatabasesResult, error) {
	mt.Helper()

	filter := emptyDoc
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		default:
			mt.Fatalf("unrecognized listCollectionNames option: %v", key)
		}
	}

	if sess != nil {
		var res mongo.ListDatabasesResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var ldErr error
			res, ldErr = mt.Client.ListDatabases(sc, filter)
			return ldErr
		})
		return res, err
	}
	return mt.Client.ListDatabases(mtest.Background, filter)
}

func executeFindOne(mt *mtest.T, sess mongo.Session, args bson.Raw) *mongo.SingleResult {
	mt.Helper()

	filter := emptyDoc
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		default:
			mt.Fatalf("unrecognized findOne option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.SingleResult
		_ = mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			res = mt.Coll.FindOne(sc, filter)
			return nil
		})
		return res
	}
	return mt.Coll.FindOne(mtest.Background, filter)
}

func executeListIndexes(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.Cursor, error) {
	mt.Helper()

	// no arguments expected. add a Fatal in case arguments are added in the future
	assert.Equal(mt, 0, len(args), "unexpected listIndexes arguments: %v", args)
	if sess != nil {
		var cursor *mongo.Cursor
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var listErr error
			cursor, listErr = mt.Coll.Indexes().List(sc)
			return listErr
		})
		return cursor, err
	}
	return mt.Coll.Indexes().List(mtest.Background)
}

func executeDistinct(mt *mtest.T, sess mongo.Session, args bson.Raw) ([]interface{}, error) {
	mt.Helper()

	var fieldName string
	filter := emptyDoc
	opts := options.Distinct()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "fieldName":
			fieldName = val.StringValue()
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized distinct option: %v", key)
		}
	}

	if sess != nil {
		var res []interface{}
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var derr error
			res, derr = mt.Coll.Distinct(sc, fieldName, filter, opts)
			return derr
		})
		return res, err
	}
	return mt.Coll.Distinct(mtest.Background, fieldName, filter, opts)
}

func executeFindOneAndDelete(mt *mtest.T, sess mongo.Session, args bson.Raw) *mongo.SingleResult {
	mt.Helper()

	filter := emptyDoc
	opts := options.FindOneAndDelete()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "sort":
			opts = opts.SetSort(val.Document())
		case "projection":
			opts = opts.SetProjection(val.Document())
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized findOneAndDelete option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.SingleResult
		_ = mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			res = mt.Coll.FindOneAndDelete(sc, filter, opts)
			return nil
		})
		return res
	}
	return mt.Coll.FindOneAndDelete(mtest.Background, filter, opts)
}

func executeFindOneAndUpdate(mt *mtest.T, sess mongo.Session, args bson.Raw) *mongo.SingleResult {
	mt.Helper()

	filter := emptyDoc
	var update interface{} = emptyDoc
	opts := options.FindOneAndUpdate()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "update":
			update = createUpdate(mt, val)
		case "arrayFilters":
			opts = opts.SetArrayFilters(options.ArrayFilters{
				Filters: rawArrayToInterfaceSlice(val.Array()),
			})
		case "sort":
			opts = opts.SetSort(val.Document())
		case "projection":
			opts = opts.SetProjection(val.Document())
		case "upsert":
			opts = opts.SetUpsert(val.Boolean())
		case "returnDocument":
			switch vstr := val.StringValue(); vstr {
			case "After":
				opts = opts.SetReturnDocument(options.After)
			case "Before":
				opts = opts.SetReturnDocument(options.Before)
			default:
				mt.Fatalf("unrecognized returnDocument value: %v", vstr)
			}
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized findOneAndUpdate option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.SingleResult
		_ = mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			res = mt.Coll.FindOneAndUpdate(sc, filter, update, opts)
			return nil
		})
		return res
	}
	return mt.Coll.FindOneAndUpdate(mtest.Background, filter, update, opts)
}

func executeFindOneAndReplace(mt *mtest.T, sess mongo.Session, args bson.Raw) *mongo.SingleResult {
	mt.Helper()

	filter := emptyDoc
	replacement := emptyDoc
	opts := options.FindOneAndReplace()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "replacement":
			replacement = val.Document()
		case "sort":
			opts = opts.SetSort(val.Document())
		case "projection":
			opts = opts.SetProjection(val.Document())
		case "upsert":
			opts = opts.SetUpsert(val.Boolean())
		case "returnDocument":
			switch vstr := val.StringValue(); vstr {
			case "After":
				opts = opts.SetReturnDocument(options.After)
			case "Before":
				opts = opts.SetReturnDocument(options.Before)
			default:
				mt.Fatalf("unrecognized returnDocument value: %v", vstr)
			}
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized findOneAndReplace option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.SingleResult
		_ = mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			res = mt.Coll.FindOneAndReplace(sc, filter, replacement, opts)
			return nil
		})
		return res
	}
	return mt.Coll.FindOneAndReplace(mtest.Background, filter, replacement, opts)
}

func executeDeleteOne(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.DeleteResult, error) {
	mt.Helper()

	filter := emptyDoc
	opts := options.Delete()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized deleteOne option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.DeleteResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var derr error
			res, derr = mt.Coll.DeleteOne(sc, filter, opts)
			return derr
		})
		return res, err
	}
	return mt.Coll.DeleteOne(mtest.Background, filter, opts)
}

func executeDeleteMany(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.DeleteResult, error) {
	mt.Helper()

	filter := emptyDoc
	opts := options.Delete()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized deleteMany option: %v", key)
		}
	}

	if sess != nil {
		var res *mongo.DeleteResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var derr error
			res, derr = mt.Coll.DeleteMany(sc, filter, opts)
			return derr
		})
		return res, err
	}
	return mt.Coll.DeleteMany(mtest.Background, filter, opts)
}

func executeUpdateOne(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.UpdateResult, error) {
	mt.Helper()

	filter := emptyDoc
	var update interface{} = emptyDoc
	opts := options.Update()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "update":
			update = createUpdate(mt, val)
		case "arrayFilters":
			opts = opts.SetArrayFilters(options.ArrayFilters{
				Filters: rawArrayToInterfaceSlice(val.Array()),
			})
		case "upsert":
			opts = opts.SetUpsert(val.Boolean())
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized updateOne option: %v", key)
		}
	}
	if opts.Upsert == nil {
		opts = opts.SetUpsert(false)
	}

	if sess != nil {
		var res *mongo.UpdateResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var uerr error
			res, uerr = mt.Coll.UpdateOne(sc, filter, update, opts)
			return uerr
		})
		return res, err
	}
	return mt.Coll.UpdateOne(mtest.Background, filter, update, opts)
}

func executeUpdateMany(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.UpdateResult, error) {
	mt.Helper()

	filter := emptyDoc
	var update interface{} = emptyDoc
	opts := options.Update()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "update":
			update = createUpdate(mt, val)
		case "arrayFilters":
			opts = opts.SetArrayFilters(options.ArrayFilters{
				Filters: rawArrayToInterfaceSlice(val.Array()),
			})
		case "upsert":
			opts = opts.SetUpsert(val.Boolean())
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized updateMany option: %v", key)
		}
	}
	if opts.Upsert == nil {
		opts = opts.SetUpsert(false)
	}

	if sess != nil {
		var res *mongo.UpdateResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var uerr error
			res, uerr = mt.Coll.UpdateMany(sc, filter, update, opts)
			return uerr
		})
		return res, err
	}
	return mt.Coll.UpdateMany(mtest.Background, filter, update, opts)
}

func executeReplaceOne(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.UpdateResult, error) {
	mt.Helper()

	filter := emptyDoc
	replacement := emptyDoc
	opts := options.Replace()

	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filter":
			filter = val.Document()
		case "replacement":
			replacement = val.Document()
		case "upsert":
			opts = opts.SetUpsert(val.Boolean())
		case "collation":
			opts = opts.SetCollation(createCollation(mt, val.Document()))
		case "session":
		default:
			mt.Fatalf("unrecognized replaceOne option: %v", key)
		}
	}
	if opts.Upsert == nil {
		opts = opts.SetUpsert(false)
	}

	if sess != nil {
		var res *mongo.UpdateResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var uerr error
			res, uerr = mt.Coll.ReplaceOne(sc, filter, replacement, opts)
			return uerr
		})
		return res, err
	}
	return mt.Coll.ReplaceOne(mtest.Background, filter, replacement, opts)
}

type withTransactionArgs struct {
	Callback *struct {
		Operations []*operation `bson:"operations"`
	} `bson:"callback"`
	Options bson.Raw `bson:"options"`
}

func runWithTransactionOperations(mt *mtest.T, operations []*operation, sess mongo.Session) error {
	mt.Helper()

	for _, op := range operations {
		if op.Name == "count" {
			mt.Skip("count has been deprecated")
		}

		// create collection with default read preference Primary (needed to prevent server selection fail)
		mt.CloneCollection(options.Collection().SetReadPreference(readpref.Primary()).SetReadConcern(readconcern.Local()))

		// execute the command on given object
		var err error
		switch op.Object {
		case "session0":
			err = executeSessionOperation(mt, op, sess)
		case "collection":
			err = executeCollectionOperation(mt, op, sess)
		default:
			mt.Fatalf("unrecognized withTransaction operation object: %v", op.Object)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func executeWithTransaction(mt *mtest.T, sess mongo.Session, args bson.Raw) error {
	mt.Helper()

	var testArgs withTransactionArgs
	err := bson.UnmarshalWithRegistry(specTestRegistry, args, &testArgs)
	assert.Nil(mt, err, "error creating withTransactionArgs: %v", err)
	opts := createTransactionOptions(mt, testArgs.Options)

	_, err = sess.WithTransaction(context.Background(), func(sc mongo.SessionContext) (interface{}, error) {
		err := runWithTransactionOperations(mt, testArgs.Callback.Operations, sess)
		return nil, err
	}, opts)
	return err
}

func executeBulkWrite(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.BulkWriteResult, error) {
	mt.Helper()

	models := createBulkWriteModels(mt, args.Lookup("requests").Array())
	opts := options.BulkWrite()

	rawOpts, err := args.LookupErr("options")
	if err == nil {
		elems, _ := rawOpts.Document().Elements()
		for _, elem := range elems {
			name := elem.Key()
			opt := elem.Value()

			switch name {
			case "ordered":
				opts.SetOrdered(opt.Boolean())
			default:
				mt.Fatalf("unrecognized bulk write option: %v", name)
			}
		}
	}

	if sess != nil {
		var res *mongo.BulkWriteResult
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var bwerr error
			res, bwerr = mt.Coll.BulkWrite(sc, models, opts)
			return bwerr
		})
		return res, err
	}
	return mt.Coll.BulkWrite(mtest.Background, models, opts)
}

func createBulkWriteModels(mt *mtest.T, rawModels bson.Raw) []mongo.WriteModel {
	vals, _ := rawModels.Values()
	models := make([]mongo.WriteModel, len(vals))

	for i, val := range vals {
		models[i] = createBulkWriteModel(mt, val.Document())
	}
	return models
}

func createBulkWriteModel(mt *mtest.T, rawModel bson.Raw) mongo.WriteModel {
	name := rawModel.Lookup("name").StringValue()
	args := rawModel.Lookup("arguments").Document()

	switch name {
	case "insertOne":
		return mongo.NewInsertOneModel().SetDocument(args.Lookup("document").Document())
	case "updateOne":
		uom := mongo.NewUpdateOneModel()
		uom.SetFilter(args.Lookup("filter").Document())
		uom.SetUpdate(createUpdate(mt, args.Lookup("update")))
		if upsert, err := args.LookupErr("upsert"); err == nil {
			uom.SetUpsert(upsert.Boolean())
		}
		if collation, err := args.LookupErr("collation"); err == nil {
			uom.SetCollation(createCollation(mt, collation.Document()))
		}
		if arrayFilters, err := args.LookupErr("arrayFilters"); err == nil {
			uom.SetArrayFilters(options.ArrayFilters{
				Filters: rawArrayToInterfaceSlice(arrayFilters.Array()),
			})
		}
		if uom.Upsert == nil {
			uom.SetUpsert(false)
		}

		return uom
	case "updateMany":
		umm := mongo.NewUpdateManyModel()
		umm.SetFilter(args.Lookup("filter").Document())
		umm.SetUpdate(createUpdate(mt, args.Lookup("update")))
		if upsert, err := args.LookupErr("upsert"); err == nil {
			umm.SetUpsert(upsert.Boolean())
		}
		if collation, err := args.LookupErr("collation"); err == nil {
			umm.SetCollation(createCollation(mt, collation.Document()))
		}
		if arrayFilters, err := args.LookupErr("arrayFilters"); err == nil {
			umm.SetArrayFilters(options.ArrayFilters{
				Filters: rawArrayToInterfaceSlice(arrayFilters.Array()),
			})
		}
		if umm.Upsert == nil {
			umm.SetUpsert(false)
		}

		return umm
	case "deleteOne":
		dom := mongo.NewDeleteOneModel()
		dom.SetFilter(args.Lookup("filter").Document())
		if collation, err := args.LookupErr("collation"); err == nil {
			dom.SetCollation(createCollation(mt, collation.Document()))
		}

		return dom
	case "deleteMany":
		dmm := mongo.NewDeleteManyModel()
		dmm.SetFilter(args.Lookup("filter").Document())
		if collation, err := args.LookupErr("collation"); err == nil {
			dmm.SetCollation(createCollation(mt, collation.Document()))
		}

		return dmm
	case "replaceOne":
		rom := mongo.NewReplaceOneModel()
		rom.SetFilter(args.Lookup("filter").Document())
		rom.SetReplacement(args.Lookup("replacement").Document())
		if upsert, err := args.LookupErr("upsert"); err == nil {
			rom.SetUpsert(upsert.Boolean())
		}
		if collation, err := args.LookupErr("collation"); err == nil {
			rom.SetCollation(createCollation(mt, collation.Document()))
		}
		if rom.Upsert == nil {
			rom.SetUpsert(false)
		}

		return rom
	default:
		mt.Fatalf("unrecognized model type: %v", name)
	}

	return nil
}

func executeEstimatedDocumentCount(mt *mtest.T, sess mongo.Session, args bson.Raw) (int64, error) {
	mt.Helper()

	// no arguments expected. add a Fatal in case arguments are added in the future
	assert.Equal(mt, 0, len(args), "unexpected estimatedDocumentCount arguments: %v", args)

	if sess != nil {
		var res int64
		err := mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			var countErr error
			res, countErr = mt.Coll.EstimatedDocumentCount(sc)
			return countErr
		})
		return res, err
	}
	return mt.Coll.EstimatedDocumentCount(mtest.Background)
}

func executeGridFSDownload(mt *mtest.T, bucket *gridfs.Bucket, args bson.Raw) (int64, error) {
	mt.Helper()

	var fileID primitive.ObjectID
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "id":
			fileID = val.ObjectID()
		default:
			mt.Fatalf("unrecognized download option: %v", key)
		}
	}

	return bucket.DownloadToStream(fileID, new(bytes.Buffer))
}

func executeGridFSDownloadByName(mt *mtest.T, bucket *gridfs.Bucket, args bson.Raw) (int64, error) {
	mt.Helper()

	var file string
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "filename":
			file = val.StringValue()
		default:
			mt.Fatalf("unrecognized download by name option: %v", key)
		}
	}

	return bucket.DownloadToStreamByName(file, new(bytes.Buffer))
}

// returns the result from the operation and the name of the target collection
func executeRenameCollection(mt *mtest.T, sess mongo.Session, args bson.Raw) (*mongo.SingleResult, string) {
	mt.Helper()

	var toName string
	elems, _ := args.Elements()
	for _, elem := range elems {
		key := elem.Key()
		opt := elem.Value()

		switch key {
		case "to":
			toName = opt.StringValue()
		default:
			mt.Fatalf("unrecognized renameCollection option %v", key)
		}
	}

	renameCmd := bson.D{
		{"renameCollection", mt.DB.Name() + "." + mt.Coll.Name()},
		{"to", mt.DB.Name() + "." + toName},
	}
	admin := mt.Client.Database("admin")

	if sess != nil {
		var res *mongo.SingleResult
		_ = mongo.WithSession(mtest.Background, sess, func(sc mongo.SessionContext) error {
			res = admin.RunCommand(sc, renameCmd)
			return nil
		})
		return res, toName
	}
	return admin.RunCommand(mtest.Background, renameCmd), toName
}

// verification function to use for all count operations
func verifyCountResult(mt *mtest.T, actualResult int64, expectedResult interface{}) {
	mt.Helper()
	if expectedResult == nil {
		return
	}

	expected := getIntFromInterface(expectedResult)
	assert.NotNil(mt, expected, "unexpected type for estimatedDocumentCount result: %T", expectedResult)
	assert.Equal(mt, *expected, actualResult, "count mismatch; expected %v, got %v", *expected, actualResult)
}

func verifyBulkWriteResult(mt *mtest.T, actualResult *mongo.BulkWriteResult, expectedResult interface{}) {
	mt.Helper()

	if expectedResult == nil {
		return
	}

	var expected struct {
		InsertedCount int64                  `bson:"insertedCount"`
		MatchedCount  int64                  `bson:"matchedCount"`
		ModifiedCount int64                  `bson:"modifiedCount"`
		DeletedCount  int64                  `bson:"deletedCount"`
		UpsertedCount int64                  `bson:"upsertedCount"`
		UpsertedIDs   map[string]interface{} `bson:"upsertedIds"`
	}
	err := bson.Unmarshal(expectedResult.(bson.Raw), &expected)
	assert.Nil(mt, err, "error creating BulkWriteResult: %v", err)

	assert.Equal(mt, expected.InsertedCount, actualResult.InsertedCount,
		"InsertedCount mismatch; expected %v, got %v", expected.InsertedCount, actualResult.InsertedCount)
	assert.Equal(mt, expected.MatchedCount, actualResult.MatchedCount,
		"MatchedCount mismatch; expected %v, got %v", expected.MatchedCount, actualResult.MatchedCount)
	assert.Equal(mt, expected.ModifiedCount, actualResult.ModifiedCount,
		"ModifiedCount mismatch; expected %v, got %v", expected.ModifiedCount, actualResult.ModifiedCount)
	assert.Equal(mt, expected.DeletedCount, actualResult.DeletedCount,
		"DeletedCount mismatch; expected %v, got %v", expected.DeletedCount, actualResult.DeletedCount)
	assert.Equal(mt, expected.UpsertedCount, actualResult.UpsertedCount,
		"UpsertedCount mismatch; expected %v, got %v", expected.UpsertedCount, actualResult.UpsertedCount)

	for idxStr, expectedID := range expected.UpsertedIDs {
		idx, err := strconv.Atoi(idxStr)
		assert.Nil(mt, err, "error converted index %v to int", idxStr)

		actualID, ok := actualResult.UpsertedIDs[int64(idx)]
		assert.True(mt, ok, "operation index %v not found in actual upserted IDs map", idx)
		assert.Equal(mt, expectedID, actualID,
			"upserted ID mismatch for key %v; expected %v, got %v", idx, expectedID, actualID)
	}
}

func verifyUpdateResult(mt *mtest.T, res *mongo.UpdateResult, result interface{}) {
	mt.Helper()

	if result == nil {
		return
	}

	var expected struct {
		MatchedCount  int64 `bson:"matchedCount"`
		ModifiedCount int64 `bson:"modifiedCount"`
		UpsertedCount int64 `bson:"upsertedCount"`
	}
	err := bson.Unmarshal(result.(bson.Raw), &expected)
	assert.Nil(mt, err, "error creating UpdateResult: %v", err)

	assert.Equal(mt, expected.MatchedCount, res.MatchedCount,
		"matched count mismatch; expected %v, got %v", expected.MatchedCount, res.MatchedCount)
	assert.Equal(mt, expected.ModifiedCount, res.ModifiedCount,
		"modified count mismatch; expected %v, got %v", expected.ModifiedCount, res.ModifiedCount)

	actualUpsertedCount := int64(0)
	if res.UpsertedID != nil {
		actualUpsertedCount = 1
	}
	assert.Equal(mt, expected.UpsertedCount, actualUpsertedCount,
		"upserted count mismatch; expected %v, got %v", expected.UpsertedCount, actualUpsertedCount)
}

func verifyDeleteResult(mt *mtest.T, res *mongo.DeleteResult, result interface{}) {
	mt.Helper()

	if result == nil {
		return
	}

	var expected struct {
		DeletedCount int64 `bson:"deletedCount"`
	}
	err := bson.Unmarshal(result.(bson.Raw), &expected)
	assert.Nil(mt, err, "error creating Delete result: %v", err)
	assert.Equal(mt, expected.DeletedCount, res.DeletedCount,
		"deleted count mismatch; expected %v, got %v", expected.DeletedCount, res.DeletedCount)
}

func verifyDistinctResult(mt *mtest.T, actualResult []interface{}, expectedResult interface{}) {
	mt.Helper()

	if expectedResult == nil {
		return
	}

	for i, expected := range expectedResult.(bson.A) {
		actual := actualResult[i]
		iExpected := getIntFromInterface(expected)
		iActual := getIntFromInterface(actual)

		if iExpected != nil {
			assert.NotNil(mt, iActual, "expected nil but got %v", iActual)
			assert.Equal(mt, *iExpected, *iActual, "expected value %v but got %v", *iExpected, *iActual)
			continue
		}

		assert.Equal(mt, expected, actual, "expected value %v but got %v", expected, actual)
	}
}

func verifyInsertOneResult(mt *mtest.T, actualResult *mongo.InsertOneResult, expectedResult interface{}) {
	mt.Helper()

	if expectedResult == nil {
		return
	}

	var expected mongo.InsertOneResult
	err := bson.Unmarshal(expectedResult.(bson.Raw), &expected)
	assert.Nil(mt, err, "error creating InsertOne result: %v", err)

	expectedID := expected.InsertedID
	if f, ok := expectedID.(float64); ok && f == math.Floor(f) {
		expectedID = int32(f)
	}

	if expectedID != nil {
		assert.NotNil(mt, actualResult, "expected result but got nil")
		assert.Equal(mt, expectedID, actualResult.InsertedID,
			"inserted ID mismatch; expected %v, got %v", expectedID, actualResult.InsertedID)
	}
}

func verifyInsertManyResult(mt *mtest.T, actualResult *mongo.InsertManyResult, expectedResult interface{}) {
	mt.Helper()

	if expectedResult == nil {
		return
	}

	assert.NotNil(mt, actualResult, "expected InsertMany result %v but got nil", expectedResult)
	var expected struct{ InsertedIds map[string]interface{} }
	err := bson.Unmarshal(expectedResult.(bson.Raw), &expected)
	assert.Nil(mt, err, "error creating expected InsertMany result: %v", err)

	for _, val := range expected.InsertedIds {
		var found bool
		for _, inserted := range actualResult.InsertedIDs {
			if val == inserted {
				found = true
				break
			}
		}

		assert.True(mt, found, "expected to find ID %v in %v", val, actualResult.InsertedIDs)
	}
}

func verifyListDatabasesResult(mt *mtest.T, actualResult mongo.ListDatabasesResult, expectedResult interface{}) {
	mt.Helper()

	if expectedResult == nil {
		return
	}

	var expected mongo.ListDatabasesResult
	err := bson.Unmarshal(expectedResult.(bson.Raw), &expected)
	assert.Nil(mt, err, "error creating ListDatabasesResult result: %v", err)

	assert.Equal(mt, expected, actualResult, "ListDatabasesResult mismatch; expected %v, got %v", expected, actualResult)
}

func verifyCursorResult(mt *mtest.T, cur *mongo.Cursor, result interface{}) {
	mt.Helper()

	if result == nil {
		return
	}

	assert.NotNil(mt, cur, "expected cursor to not be nil")
	for i, expected := range result.(bson.A) {
		assert.True(mt, cur.Next(mtest.Background), "expected Next to return true but got false")
		if err := compareDocs(mt, expected.(bson.Raw), cur.Current); err != nil {
			mt.Fatalf("cursor document mismatch at index %d: %s", i, err)
		}
	}

	assert.False(mt, cur.Next(mtest.Background), "expected Next to return false but got true")
	err := cur.Err()
	assert.Nil(mt, err, "cursor error: %v", err)
}

func verifySingleResult(mt *mtest.T, actualResult *mongo.SingleResult, expectedResult interface{}) {
	mt.Helper()

	if expectedResult == nil {
		return
	}

	expected := expectedResult.(bson.Raw)
	actual, _ := actualResult.DecodeBytes()
	if err := compareDocs(mt, expected, actual); err != nil {
		mt.Fatalf("SingleResult document mismatch: %s", err)
	}
}
