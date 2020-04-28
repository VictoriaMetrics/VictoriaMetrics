// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func setupDb(name string, opts ...*options.DatabaseOptions) *Database {
	client := setupClient()
	return client.Database(name, opts...)
}

func compareDbs(t *testing.T, expected, got *Database) {
	t.Helper()
	assert.Equal(t, expected.readPreference, got.readPreference,
		"expected read preference %v, got %v", expected.readPreference, got.readPreference)
	assert.Equal(t, expected.readConcern, got.readConcern,
		"expected read concern %v, got %v", expected.readConcern, got.readConcern)
	assert.Equal(t, expected.writeConcern, got.writeConcern,
		"expected write concern %v, got %v", expected.writeConcern, got.writeConcern)
	assert.Equal(t, expected.registry, got.registry,
		"expected write concern %v, got %v", expected.registry, got.registry)
}

func TestDatabase(t *testing.T) {
	t.Run("initialize", func(t *testing.T) {
		name := "foo"
		db := setupDb(name)
		assert.Equal(t, name, db.Name(), "expected db name %v, got %v", name, db.Name())
		assert.NotNil(t, db.Client(), "expected valid client, got nil")
	})
	t.Run("options", func(t *testing.T) {
		t.Run("custom", func(t *testing.T) {
			rpPrimary := readpref.Primary()
			rpSecondary := readpref.Secondary()
			wc1 := writeconcern.New(writeconcern.W(5))
			wc2 := writeconcern.New(writeconcern.W(10))
			rcLocal := readconcern.Local()
			rcMajority := readconcern.Majority()
			reg := bsoncodec.NewRegistryBuilder().Build()

			opts := options.Database().SetReadPreference(rpPrimary).SetReadConcern(rcLocal).SetWriteConcern(wc1).
				SetReadPreference(rpSecondary).SetReadConcern(rcMajority).SetWriteConcern(wc2).SetRegistry(reg)
			expected := &Database{
				readPreference: rpSecondary,
				readConcern:    rcMajority,
				writeConcern:   wc2,
				registry:       reg,
			}
			got := setupDb("foo", opts)
			compareDbs(t, expected, got)
		})
		t.Run("inherit", func(t *testing.T) {
			rpPrimary := readpref.Primary()
			rcLocal := readconcern.Local()
			wc1 := writeconcern.New(writeconcern.W(10))
			reg := bsoncodec.NewRegistryBuilder().Build()

			client := setupClient(options.Client().SetReadPreference(rpPrimary).SetReadConcern(rcLocal).SetRegistry(reg))
			got := client.Database("foo", options.Database().SetWriteConcern(wc1))
			expected := &Database{
				readPreference: rpPrimary,
				readConcern:    rcLocal,
				writeConcern:   wc1,
				registry:       reg,
			}
			compareDbs(t, expected, got)
		})
	})
	t.Run("replace topology error", func(t *testing.T) {
		db := setupDb("foo")

		err := db.RunCommand(bgCtx, bson.D{{"x", 1}}).Err()
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)

		err = db.Drop(bgCtx)
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)

		_, err = db.ListCollections(bgCtx, bson.D{})
		assert.Equal(t, ErrClientDisconnected, err, "expected error %v, got %v", ErrClientDisconnected, err)
	})
	t.Run("nil document error", func(t *testing.T) {
		db := setupDb("foo")

		err := db.RunCommand(bgCtx, nil).Err()
		assert.Equal(t, ErrNilDocument, err, "expected error %v, got %v", ErrNilDocument, err)

		_, err = db.Watch(context.Background(), nil)
		watchErr := errors.New("can only transform slices and arrays into aggregation pipelines, but got invalid")
		assert.Equal(t, watchErr, err, "expected error %v, got %v", watchErr, err)

		_, err = db.ListCollections(context.Background(), nil)
		assert.Equal(t, ErrNilDocument, err, "expected error %v, got %v", ErrNilDocument, err)

		_, err = db.ListCollectionNames(context.Background(), nil)
		assert.Equal(t, ErrNilDocument, err, "expected error %v, got %v", ErrNilDocument, err)
	})
}
