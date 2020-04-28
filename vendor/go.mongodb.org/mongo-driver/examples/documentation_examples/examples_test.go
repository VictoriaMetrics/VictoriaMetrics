// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

// NOTE: Any time this file is modified, a WEBSITE ticket should be opened to sync the changes with
// the "What is MongoDB" webpage, which the example was originally added to as part of WEBSITE-5148.

package documentation_examples_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/examples/documentation_examples"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

func TestDocumentationExamples(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cs := testutil.ConnString(t)
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cs.String()))
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	db := client.Database("documentation_examples")

	documentation_examples.InsertExamples(t, db)
	documentation_examples.QueryToplevelFieldsExamples(t, db)
	documentation_examples.QueryEmbeddedDocumentsExamples(t, db)
	documentation_examples.QueryArraysExamples(t, db)
	documentation_examples.QueryArrayEmbeddedDocumentsExamples(t, db)
	documentation_examples.QueryNullMissingFieldsExamples(t, db)
	documentation_examples.ProjectionExamples(t, db)
	documentation_examples.UpdateExamples(t, db)
	documentation_examples.DeleteExamples(t, db)
}

func TestTransactionExamples(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	topo := createTopology(t)
	client, err := mongo.Connect(context.Background(), &options.ClientOptions{Deployment: topo})
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	ver, err := getServerVersion(ctx, client)
	if err != nil || testutil.CompareVersions(t, ver, "4.0") < 0 || topo.Kind() != description.ReplicaSet {
		t.Skip("server does not support transactions")
	}
	err = documentation_examples.TransactionsExamples(ctx, client)
	require.NoError(t, err)
}

func TestChangeStreamExamples(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	topo := createTopology(t)
	client, err := mongo.Connect(context.Background(), &options.ClientOptions{Deployment: topo})
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	db := client.Database("changestream_examples")
	ver, err := getServerVersion(ctx, client)
	if err != nil || testutil.CompareVersions(t, ver, "3.6") < 0 || topo.Kind() != description.ReplicaSet {
		t.Skip("server does not support changestreams")
	}
	documentation_examples.ChangeStreamExamples(t, db)
}

func getServerVersion(ctx context.Context, client *mongo.Client) (string, error) {
	serverStatus, err := client.Database("admin").RunCommand(
		ctx,
		bsonx.Doc{{"serverStatus", bsonx.Int32(1)}},
	).DecodeBytes()
	if err != nil {
		return "", err
	}

	version, err := serverStatus.LookupErr("version")
	if err != nil {
		return "", err
	}

	return version.StringValue(), nil
}

func createTopology(t *testing.T) *topology.Topology {
	topo, err := topology.New(topology.WithConnString(func(connstring.ConnString) connstring.ConnString {
		return testutil.ConnString(t)
	}))
	if err != nil {
		t.Fatalf("topology.New error: %v", err)
	}
	return topo
}
