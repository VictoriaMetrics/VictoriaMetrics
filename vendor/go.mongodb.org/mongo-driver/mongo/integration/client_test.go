// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"fmt"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
)

const certificatesDir = "../../data/certificates"

var noClientOpts = mtest.NewOptions().CreateClient(false)

type negateCodec struct {
	ID int64 `bson:"_id"`
}

func (e *negateCodec) EncodeValue(ectx bsoncodec.EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
	return vw.WriteInt64(val.Int())
}

// DecodeValue negates the value of ID when reading
func (e *negateCodec) DecodeValue(ectx bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	i, err := vr.ReadInt64()
	if err != nil {
		return err
	}

	val.SetInt(i * -1)
	return nil
}

func TestClient(t *testing.T) {
	mt := mtest.New(t, noClientOpts)
	defer mt.Close()

	registryOpts := options.Client().
		SetRegistry(bson.NewRegistryBuilder().RegisterCodec(reflect.TypeOf(int64(0)), &negateCodec{}).Build())
	mt.RunOpts("registry passed to cursors", mtest.NewOptions().ClientOptions(registryOpts), func(mt *mtest.T) {
		_, err := mt.Coll.InsertOne(mtest.Background, negateCodec{ID: 10})
		assert.Nil(mt, err, "InsertOne error: %v", err)
		var got negateCodec
		err = mt.Coll.FindOne(mtest.Background, bson.D{}).Decode(&got)
		assert.Nil(mt, err, "Find error: %v", err)

		assert.Equal(mt, int64(-10), got.ID, "expected ID -10, got %v", got.ID)
	})
	mt.RunOpts("tls connection", mtest.NewOptions().MinServerVersion("3.0").Auth(true), func(mt *mtest.T) {
		var result bson.Raw
		err := mt.Coll.Database().RunCommand(mtest.Background, bson.D{
			{"serverStatus", 1},
		}).Decode(&result)
		assert.Nil(mt, err, "serverStatus error: %v", err)

		security := result.Lookup("security")
		assert.Equal(mt, bson.TypeEmbeddedDocument, security.Type,
			"expected security field to be type %v, got %v", bson.TypeMaxKey, security.Type)
		_, found := security.Document().LookupErr("SSLServerSubjectName")
		assert.Nil(mt, found, "SSLServerSubjectName not found in result")
		_, found = security.Document().LookupErr("SSLServerHasCertificateAuthority")
		assert.Nil(mt, found, "SSLServerHasCertificateAuthority not found in result")
	})
	mt.RunOpts("x509", mtest.NewOptions().Auth(true), func(mt *mtest.T) {
		const user = "C=US,ST=New York,L=New York City,O=MongoDB,OU=other,CN=external"
		db := mt.Client.Database("$external")

		// We don't care if the user doesn't already exist.
		_ = db.RunCommand(
			mtest.Background,
			bson.D{{"dropUser", user}},
		)
		err := db.RunCommand(
			mtest.Background,
			bson.D{
				{"createUser", user},
				{"roles", bson.A{
					bson.D{{"role", "readWrite"}, {"db", "test"}},
				}},
			},
		).Err()
		assert.Nil(mt, err, "createUser error: %v", err)

		baseConnString := mt.ConnString()
		// remove username/password from base conn string
		revisedConnString := "mongodb://"
		split := strings.Split(baseConnString, "@")
		assert.Equal(t, 2, len(split), "expected 2 parts after split, got %v (connstring %v)", split, baseConnString)
		revisedConnString += split[1]

		cs := fmt.Sprintf(
			"%s&sslClientCertificateKeyFile=%s&authMechanism=MONGODB-X509&authSource=$external",
			revisedConnString,
			path.Join(certificatesDir, "client.pem"),
		)
		authClient, err := mongo.Connect(mtest.Background, options.Client().ApplyURI(cs))
		assert.Nil(mt, err, "authClient Connect error: %v", err)
		defer func() { _ = authClient.Disconnect(mtest.Background) }()

		rdr, err := authClient.Database("test").RunCommand(mtest.Background, bson.D{
			{"connectionStatus", 1},
		}).DecodeBytes()
		assert.Nil(mt, err, "connectionStatus error: %v", err)
		users, err := rdr.LookupErr("authInfo", "authenticatedUsers")
		assert.Nil(mt, err, "authenticatedUsers not found in response")
		elems, err := users.Array().Elements()
		assert.Nil(mt, err, "error getting users elements: %v", err)

		for _, userElem := range elems {
			rdr := userElem.Value().Document()
			var u struct {
				User string
				DB   string
			}

			if err := bson.Unmarshal(rdr, &u); err != nil {
				continue
			}
			if u.User == user && u.DB == "$external" {
				return
			}
		}
		mt.Fatal("unable to find authenticated user")
	})
	mt.RunOpts("list databases", noClientOpts, func(mt *mtest.T) {
		testCases := []struct {
			name             string
			filter           bson.D
			hasTestDb        bool
			minServerVersion string
		}{
			{"no filter", bson.D{}, true, ""},
			{"filter", bson.D{{"name", "foobar"}}, false, "3.6"},
		}

		for _, tc := range testCases {
			opts := mtest.NewOptions()
			if tc.minServerVersion != "" {
				opts.MinServerVersion(tc.minServerVersion)
			}

			mt.RunOpts(tc.name, opts, func(mt *mtest.T) {
				res, err := mt.Client.ListDatabases(mtest.Background, tc.filter)
				assert.Nil(mt, err, "ListDatabases error: %v", err)

				var found bool
				for _, db := range res.Databases {
					if db.Name == mtest.TestDb {
						found = true
						break
					}
				}
				assert.Equal(mt, tc.hasTestDb, found, "expected to find test db: %v, found: %v", tc.hasTestDb, found)
			})
		}
	})
	mt.RunOpts("list database names", noClientOpts, func(mt *mtest.T) {
		testCases := []struct {
			name             string
			filter           bson.D
			hasTestDb        bool
			minServerVersion string
		}{
			{"no filter", bson.D{}, true, ""},
			{"filter", bson.D{{"name", "foobar"}}, false, "3.6"},
		}

		for _, tc := range testCases {
			opts := mtest.NewOptions()
			if tc.minServerVersion != "" {
				opts.MinServerVersion(tc.minServerVersion)
			}

			mt.RunOpts(tc.name, opts, func(mt *mtest.T) {
				dbs, err := mt.Client.ListDatabaseNames(mtest.Background, tc.filter)
				assert.Nil(mt, err, "ListDatabaseNames error: %v", err)

				var found bool
				for _, db := range dbs {
					if db == mtest.TestDb {
						found = true
						break
					}
				}
				assert.Equal(mt, tc.hasTestDb, found, "expected to find test db: %v, found: %v", tc.hasTestDb, found)
			})
		}
	})
	mt.RunOpts("ping", noClientOpts, func(mt *mtest.T) {
		mt.Run("default read preference", func(mt *mtest.T) {
			err := mt.Client.Ping(mtest.Background, nil)
			assert.Nil(mt, err, "Ping error: %v", err)
		})
		mt.Run("invalid host", func(mt *mtest.T) {
			// manually create client rather than using RunOpts with ClientOptions because the testing lib will
			// apply the correct URI.
			invalidClientOpts := options.Client().
				SetServerSelectionTimeout(100 * time.Millisecond).SetHosts([]string{"invalid:123"}).
				SetConnectTimeout(500 * time.Millisecond).SetSocketTimeout(500 * time.Millisecond)
			client, err := mongo.Connect(mtest.Background, invalidClientOpts)
			assert.Nil(mt, err, "Connect error: %v", err)
			err = client.Ping(mtest.Background, readpref.Primary())
			assert.NotNil(mt, err, "expected error for pinging invalid host, got nil")
			_ = client.Disconnect(mtest.Background)
		})
	})
	mt.RunOpts("disconnect", noClientOpts, func(mt *mtest.T) {
		mt.Run("nil context", func(mt *mtest.T) {
			err := mt.Client.Disconnect(nil)
			assert.Nil(mt, err, "Disconnect error: %v", err)
		})
	})
	mt.RunOpts("watch", noClientOpts, func(mt *mtest.T) {
		mt.Run("disconnected", func(mt *mtest.T) {
			c, err := mongo.NewClient(options.Client().ApplyURI(mt.ConnString()))
			assert.Nil(mt, err, "NewClient error: %v", err)
			_, err = c.Watch(mtest.Background, mongo.Pipeline{})
			assert.Equal(mt, mongo.ErrClientDisconnected, err, "expected error %v, got %v", mongo.ErrClientDisconnected, err)
		})
	})
	mt.RunOpts("end sessions", mtest.NewOptions().MinServerVersion("3.6"), func(mt *mtest.T) {
		_, err := mt.Client.ListDatabases(mtest.Background, bson.D{})
		assert.Nil(mt, err, "ListDatabases error: %v", err)

		mt.ClearEvents()
		err = mt.Client.Disconnect(mtest.Background)
		assert.Nil(mt, err, "Disconnect error: %v", err)

		started := mt.GetStartedEvent()
		assert.Equal(mt, "endSessions", started.CommandName, "expected cmd name endSessions, got %v", started.CommandName)
	})
	mt.RunOpts("isMaster lastWriteDate", mtest.NewOptions().Topologies(mtest.ReplicaSet), func(mt *mtest.T) {
		_, err := mt.Coll.InsertOne(mtest.Background, bson.D{{"x", 1}})
		assert.Nil(mt, err, "InsertOne error: %v", err)
	})
	sessionOpts := mtest.NewOptions().MinServerVersion("3.6.0").CreateClient(false)
	mt.RunOpts("causal consistency", sessionOpts, func(mt *mtest.T) {
		testCases := []struct {
			name       string
			opts       *options.SessionOptions
			consistent bool
		}{
			{"default", options.Session(), true},
			{"true", options.Session().SetCausalConsistency(true), true},
			{"false", options.Session().SetCausalConsistency(false), false},
		}

		for _, tc := range testCases {
			mt.Run(tc.name, func(mt *mtest.T) {
				sess, err := mt.Client.StartSession(tc.opts)
				assert.Nil(mt, err, "StartSession error: %v", err)
				defer sess.EndSession(mtest.Background)
				xs := sess.(mongo.XSession)
				consistent := xs.ClientSession().Consistent
				assert.Equal(mt, tc.consistent, consistent, "expected consistent to be %v, got %v", tc.consistent, consistent)
			})
		}
	})
	retryOpts := mtest.NewOptions().MinServerVersion("3.6.0").ClientType(mtest.Mock)
	mt.RunOpts("retry writes error 20 wrapped", retryOpts, func(mt *mtest.T) {
		writeErrorCode20 := mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Message: "Transaction numbers",
			Code:    20,
		})
		writeErrorCode19 := mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Message: "Transaction numbers",
			Code:    19,
		})
		writeErrorCode20WrongMsg := mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Message: "Not transaction numbers",
			Code:    20,
		})
		cmdErrCode20 := mtest.CreateCommandErrorResponse(mtest.CommandError{
			Message: "Transaction numbers",
			Code:    20,
		})
		cmdErrCode19 := mtest.CreateCommandErrorResponse(mtest.CommandError{
			Message: "Transaction numbers",
			Code:    19,
		})
		cmdErrCode20WrongMsg := mtest.CreateCommandErrorResponse(mtest.CommandError{
			Message: "Not transaction numbers",
			Code:    20,
		})

		testCases := []struct {
			name                 string
			errResponse          bson.D
			expectUnsupportedMsg bool
		}{
			{"write error code 20", writeErrorCode20, true},
			{"write error code 20 wrong msg", writeErrorCode20WrongMsg, false},
			{"write error code 19 right msg", writeErrorCode19, false},
			{"command error code 20", cmdErrCode20, true},
			{"command error code 20 wrong msg", cmdErrCode20WrongMsg, false},
			{"command error code 19 right msg", cmdErrCode19, false},
		}
		for _, tc := range testCases {
			mt.Run(tc.name, func(mt *mtest.T) {
				mt.ClearMockResponses()
				mt.AddMockResponses(tc.errResponse)

				sess, err := mt.Client.StartSession()
				assert.Nil(mt, err, "StartSession error: %v", err)
				defer sess.EndSession(mtest.Background)

				_, err = mt.Coll.InsertOne(mtest.Background, bson.D{{"x", 1}})
				assert.NotNil(mt, err, "expected err but got nil")
				if tc.expectUnsupportedMsg {
					assert.Equal(mt, driver.ErrUnsupportedStorageEngine.Error(), err.Error(),
						"expected error %v, got %v", driver.ErrUnsupportedStorageEngine, err)
					return
				}
				assert.NotEqual(mt, driver.ErrUnsupportedStorageEngine.Error(), err.Error(),
					"got ErrUnsupportedStorageEngine but wanted different error")
			})
		}
	})
}
