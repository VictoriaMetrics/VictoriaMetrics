// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

var (
	connsCheckedOut int
)

func TestConvenientTransactions(t *testing.T) {
	client := setupConvenientTransactions(t)
	db := client.Database("TestConvenientTransactions")
	dbAdmin := client.Database("admin")

	defer func() {
		sessions := client.NumberSessionsInProgress()
		conns := connsCheckedOut

		err := dbAdmin.RunCommand(bgCtx, bson.D{
			{"killAllSessions", bson.A{}},
		}).Err()
		if err != nil {
			if ce, ok := err.(CommandError); !ok || ce.Code != errorInterrupted {
				t.Fatalf("killAllSessions error: %v", err)
			}
		}

		_ = db.Drop(bgCtx)
		_ = client.Disconnect(bgCtx)

		assert.Equal(t, 0, sessions, "%v sessions checked out", sessions)
		assert.Equal(t, 0, conns, "%v connections checked out", conns)
	}()

	t.Run("callback raises custom error", func(t *testing.T) {
		coll := db.Collection(t.Name())
		_, err := coll.InsertOne(bgCtx, bson.D{{"x", 1}})
		assert.Nil(t, err, "InsertOne error: %v", err)

		sess, err := client.StartSession()
		assert.Nil(t, err, "StartSession error: %v", err)
		defer sess.EndSession(context.Background())

		testErr := errors.New("test error")
		_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
			return nil, testErr
		})
		assert.Equal(t, testErr, err, "expected error %v, got %v", testErr, err)
	})
	t.Run("callback returns value", func(t *testing.T) {
		coll := db.Collection(t.Name())
		_, err := coll.InsertOne(bgCtx, bson.D{{"x", 1}})
		assert.Nil(t, err, "InsertOne error: %v", err)

		sess, err := client.StartSession()
		assert.Nil(t, err, "StartSession error: %v", err)
		defer sess.EndSession(context.Background())

		res, err := sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
			return false, nil
		})
		assert.Nil(t, err, "WithTransaction error: %v", err)
		resBool, ok := res.(bool)
		assert.True(t, ok, "expected result type %T, got %T", false, res)
		assert.False(t, resBool, "expected result false, got %v", resBool)
	})
	t.Run("retry timeout enforced", func(t *testing.T) {
		withTransactionTimeout = time.Second

		coll := db.Collection(t.Name())
		_, err := coll.InsertOne(bgCtx, bson.D{{"x", 1}})
		assert.Nil(t, err, "InsertOne error: %v", err)

		t.Run("transient transaction error", func(t *testing.T) {
			sess, err := client.StartSession()
			assert.Nil(t, err, "StartSession error: %v", err)
			defer sess.EndSession(context.Background())

			_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
				return nil, CommandError{Name: "test Error", Labels: []string{driver.TransientTransactionError}}
			})
			assert.NotNil(t, err, "expected WithTransaction error, got nil")
			cmdErr, ok := err.(CommandError)
			assert.True(t, ok, "expected error type %T, got %T", CommandError{}, err)
			assert.True(t, cmdErr.HasErrorLabel(driver.TransientTransactionError),
				"expected error with label %v, got %v", driver.TransientTransactionError, cmdErr)
		})
		t.Run("unknown transaction commit result", func(t *testing.T) {
			//set failpoint
			failpoint := bson.D{{"configureFailPoint", "failCommand"},
				{"mode", "alwaysOn"},
				{"data", bson.D{
					{"failCommands", bson.A{"commitTransaction"}},
					{"closeConnection", true},
				}},
			}
			err = dbAdmin.RunCommand(bgCtx, failpoint).Err()
			assert.Nil(t, err, "error setting failpoint: %v", err)
			defer func() {
				err = dbAdmin.RunCommand(bgCtx, bson.D{
					{"configureFailPoint", "failCommand"},
					{"mode", "off"},
				}).Err()
				assert.Nil(t, err, "error turning off failpoint: %v", err)
			}()

			sess, err := client.StartSession()
			assert.Nil(t, err, "StartSession error: %v", err)
			defer sess.EndSession(context.Background())

			_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
				_, err := coll.InsertOne(sessCtx, bson.D{{"x", 1}})
				return nil, err
			})
			assert.NotNil(t, err, "expected WithTransaction error, got nil")
			cmdErr, ok := err.(CommandError)
			assert.True(t, ok, "expected error type %T, got %T", CommandError{}, err)
			assert.True(t, cmdErr.HasErrorLabel(driver.UnknownTransactionCommitResult),
				"expected error with label %v, got %v", driver.UnknownTransactionCommitResult, cmdErr)
		})
		t.Run("commit transient transaction error", func(t *testing.T) {
			//set failpoint
			failpoint := bson.D{{"configureFailPoint", "failCommand"},
				{"mode", "alwaysOn"},
				{"data", bson.D{
					{"failCommands", bson.A{"commitTransaction"}},
					{"errorCode", 251},
				}},
			}
			err = dbAdmin.RunCommand(bgCtx, failpoint).Err()
			assert.Nil(t, err, "error setting failpoint: %v", err)
			defer func() {
				err = dbAdmin.RunCommand(bgCtx, bson.D{
					{"configureFailPoint", "failCommand"},
					{"mode", "off"},
				}).Err()
				assert.Nil(t, err, "error turning off failpoint: %v", err)
			}()

			sess, err := client.StartSession()
			assert.Nil(t, err, "StartSession error: %v", err)
			defer sess.EndSession(context.Background())

			_, err = sess.WithTransaction(context.Background(), func(sessCtx SessionContext) (interface{}, error) {
				_, err := coll.InsertOne(sessCtx, bson.D{{"x", 1}})
				return nil, err
			})
			assert.NotNil(t, err, "expected WithTransaction error, got nil")
			cmdErr, ok := err.(CommandError)
			assert.True(t, ok, "expected error type %T, got %T", CommandError{}, err)
			assert.True(t, cmdErr.HasErrorLabel(driver.TransientTransactionError),
				"expected error with label %v, got %v", driver.TransientTransactionError, cmdErr)
		})
	})
}

func setupConvenientTransactions(t *testing.T) *Client {
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
	client, err := Connect(bgCtx, clientOpts)
	assert.Nil(t, err, "Connect error: %v", err)

	version, err := getServerVersion(client.Database("admin"))
	assert.Nil(t, err, "getServerVersion error: %v", err)
	topoKind := client.deployment.(*topology.Topology).Kind()
	if compareVersions(t, version, "4.1") < 0 || topoKind == description.Single {
		t.Skip("skipping standalones and versions < 4.1")
	}

	// pin to a single mongos if necessary
	if topoKind != description.Sharded {
		return client
	}
	client, err = Connect(bgCtx, clientOpts.SetHosts([]string{cs.Hosts[0]}))
	assert.Nil(t, err, "Connect error: %v", err)
	return client
}

func getServerVersion(db *Database) (string, error) {
	serverStatus, err := db.RunCommand(
		context.Background(),
		bson.D{{"serverStatus", 1}},
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

// compareVersions compares two version number strings (i.e. positive integers separated by
// periods). Comparisons are done to the lesser precision of the two versions. For example, 3.2 is
// considered equal to 3.2.11, whereas 3.2.0 is considered less than 3.2.11.
//
// Returns a positive int if version1 is greater than version2, a negative int if version1 is less
// than version2, and 0 if version1 is equal to version2.
func compareVersions(t *testing.T, v1 string, v2 string) int {
	n1 := strings.Split(v1, ".")
	n2 := strings.Split(v2, ".")

	for i := 0; i < int(math.Min(float64(len(n1)), float64(len(n2)))); i++ {
		i1, err := strconv.Atoi(n1[i])
		if err != nil {
			return 1
		}

		i2, err := strconv.Atoi(n2[i])
		if err != nil {
			return -1
		}

		difference := i1 - i2
		if difference != 0 {
			return difference
		}
	}

	return 0
}
