// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"context"
	"flag"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	flag.Parse()
	uris := flag.Args()
	ctx := context.Background()

	for idx, uri := range uris {
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			panic(createErrorMessage(idx, "Connect error: %v", err))
		}

		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				panic(createErrorMessage(idx, "Disconnect error: %v", err))
			}
		}()

		db := client.Database("test")
		err = db.RunCommand(
			ctx,
			bson.D{{"isMaster", 1}},
		).Err()
		if err != nil {
			panic(createErrorMessage(idx, "isMaster error: %v", err))
		}

		coll := db.Collection("test")
		if err = coll.FindOne(ctx, bson.D{{"x", 1}}).Err(); err != nil && err != mongo.ErrNoDocuments {
			panic(createErrorMessage(idx, "FindOne error: %v", err))
		}
	}
}

func createErrorMessage(idx int, msg string, args ...interface{}) string {
	msg = fmt.Sprintf(msg, args...)
	return fmt.Sprintf("error for URI at index %d: %s", idx, msg)
}
