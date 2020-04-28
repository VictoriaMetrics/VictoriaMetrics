// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"io/ioutil"
	"math"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

const (
	awsAccessKeyID     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
)

// Helper functions to do read JSON spec test files and convert JSON objects into the appropriate driver types.
// Functions in this file should take testing.TB rather than testing.T/mtest.T for generality because they
// do not do any database communication.

// generate a slice of all JSON file names in a directory
func jsonFilesInDir(t testing.TB, dir string) []string {
	t.Helper()

	files := make([]string, 0)

	entries, err := ioutil.ReadDir(dir)
	assert.Nil(t, err, "unable to read json file: %v", err)

	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}

		files = append(files, entry.Name())
	}

	return files
}

// create client options from a map
func createClientOptions(t testing.TB, opts bson.Raw) *options.ClientOptions {
	t.Helper()

	clientOpts := options.Client()
	elems, _ := opts.Elements()
	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "retryWrites":
			clientOpts.SetRetryWrites(opt.Boolean())
		case "w":
			switch opt.Type {
			case bson.TypeInt32:
				w := int(opt.Int32())
				clientOpts.SetWriteConcern(writeconcern.New(writeconcern.W(w)))
			case bson.TypeDouble:
				w := int(opt.Double())
				clientOpts.SetWriteConcern(writeconcern.New(writeconcern.W(w)))
			case bson.TypeString:
				clientOpts.SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
			default:
				t.Fatalf("unrecognized type for w client option: %v", opt.Type)
			}
		case "readConcernLevel":
			clientOpts.SetReadConcern(readconcern.New(readconcern.Level(opt.StringValue())))
		case "readPreference":
			clientOpts.SetReadPreference(readPrefFromString(opt.StringValue()))
		case "heartbeatFrequencyMS":
			hfms := time.Duration(opt.Int32()) * time.Millisecond
			clientOpts.SetHeartbeatInterval(hfms)
		case "retryReads":
			clientOpts.SetRetryReads(opt.Boolean())
		case "autoEncryptOpts":
			clientOpts.SetAutoEncryptionOptions(createAutoEncryptionOptions(t, opt.Document()))
		default:
			t.Fatalf("unrecognized client option: %v", name)
		}
	}

	return clientOpts
}

func createAutoEncryptionOptions(t testing.TB, opts bson.Raw) *options.AutoEncryptionOptions {
	t.Helper()

	aeo := options.AutoEncryption()
	var kvnsFound bool
	elems, _ := opts.Elements()

	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "kmsProviders":
			aeo.SetKmsProviders(createKmsProvidersMap(t, opt.Document()))
		case "schemaMap":
			var schemaMap map[string]interface{}
			err := bson.Unmarshal(opt.Document(), &schemaMap)
			if err != nil {
				t.Fatalf("error creating schema map: %v", err)
			}

			aeo.SetSchemaMap(schemaMap)
		case "keyVaultNamespace":
			kvnsFound = true
			aeo.SetKeyVaultNamespace(opt.StringValue())
		case "bypassAutoEncryption":
			aeo.SetBypassAutoEncryption(opt.Boolean())
		default:
			t.Fatalf("unrecognized auto encryption option: %v", name)
		}
	}
	if !kvnsFound {
		aeo.SetKeyVaultNamespace("admin.datakeys")
	}

	return aeo
}

func createKmsProvidersMap(t testing.TB, opts bson.Raw) map[string]map[string]interface{} {
	t.Helper()

	// aws: value is always empty object. create new map value from access key ID and secret access key
	// local: value is {"key": primitive.Binary}. transform to {"key": []byte}

	kmsMap := make(map[string]map[string]interface{})
	elems, _ := opts.Elements()

	for _, elem := range elems {
		provider := elem.Key()
		providerOpt := elem.Value()

		switch provider {
		case "aws":
			keyID := os.Getenv(awsAccessKeyID)
			if keyID == "" {
				t.Fatalf("%s env var not set", awsAccessKeyID)
			}
			secretAccessKey := os.Getenv(awsSecretAccessKey)
			if secretAccessKey == "" {
				t.Fatalf("%s env var not set", awsSecretAccessKey)
			}

			awsMap := map[string]interface{}{
				"accessKeyId":     keyID,
				"secretAccessKey": secretAccessKey,
			}
			kmsMap["aws"] = awsMap
		case "local":
			_, key := providerOpt.Document().Lookup("key").Binary()
			localMap := map[string]interface{}{
				"key": key,
			}
			kmsMap["local"] = localMap
		default:
			t.Fatalf("unrecognized KMS provider: %v", provider)
		}
	}

	return kmsMap
}

// create session options from a map
func createSessionOptions(t testing.TB, opts bson.Raw) *options.SessionOptions {
	t.Helper()

	sessOpts := options.Session()
	elems, _ := opts.Elements()
	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "causalConsistency":
			sessOpts = sessOpts.SetCausalConsistency(opt.Boolean())
		case "defaultTransactionOptions":
			txnOpts := createTransactionOptions(t, opt.Document())
			if txnOpts.ReadConcern != nil {
				sessOpts.SetDefaultReadConcern(txnOpts.ReadConcern)
			}
			if txnOpts.ReadPreference != nil {
				sessOpts.SetDefaultReadPreference(txnOpts.ReadPreference)
			}
			if txnOpts.WriteConcern != nil {
				sessOpts.SetDefaultWriteConcern(txnOpts.WriteConcern)
			}
			if txnOpts.MaxCommitTime != nil {
				sessOpts.SetDefaultMaxCommitTime(txnOpts.MaxCommitTime)
			}
		default:
			t.Fatalf("unrecognized session option: %v", name)
		}
	}

	return sessOpts
}

// create database options from a BSON document.
func createDatabaseOptions(t testing.TB, opts bson.Raw) *options.DatabaseOptions {
	t.Helper()

	do := options.Database()
	elems, _ := opts.Elements()
	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "readConcern":
			do.SetReadConcern(createReadConcern(opt))
		default:
			t.Fatalf("unrecognized database option: %v", name)
		}
	}

	return do
}

// create collection options from a map
func createCollectionOptions(t testing.TB, opts bson.Raw) *options.CollectionOptions {
	t.Helper()

	co := options.Collection()
	elems, _ := opts.Elements()
	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "readConcern":
			co.SetReadConcern(createReadConcern(opt))
		case "writeConcern":
			co.SetWriteConcern(createWriteConcern(t, opt))
		case "readPreference":
			co.SetReadPreference(createReadPref(opt))
		default:
			t.Fatalf("unrecognized collection option: %v", name)
		}
	}

	return co
}

// create transaction options from a map
func createTransactionOptions(t testing.TB, opts bson.Raw) *options.TransactionOptions {
	t.Helper()

	txnOpts := options.Transaction()
	elems, _ := opts.Elements()
	for _, elem := range elems {
		name := elem.Key()
		opt := elem.Value()

		switch name {
		case "writeConcern":
			txnOpts.SetWriteConcern(createWriteConcern(t, opt))
		case "readPreference":
			txnOpts.SetReadPreference(createReadPref(opt))
		case "readConcern":
			txnOpts.SetReadConcern(createReadConcern(opt))
		case "maxCommitTimeMS":
			t := time.Duration(opt.Int32()) * time.Millisecond
			txnOpts.SetMaxCommitTime(&t)
		default:
			t.Fatalf("unrecognized transaction option: %v", opt)
		}
	}
	return txnOpts
}

// create a read concern from a map
func createReadConcern(opt bson.RawValue) *readconcern.ReadConcern {
	return readconcern.New(readconcern.Level(opt.Document().Lookup("level").StringValue()))
}

// create a read concern from a map
func createWriteConcern(t testing.TB, opt bson.RawValue) *writeconcern.WriteConcern {
	wcDoc, ok := opt.DocumentOK()
	if !ok {
		return nil
	}

	var opts []writeconcern.Option
	elems, _ := wcDoc.Elements()
	for _, elem := range elems {
		key := elem.Key()
		val := elem.Value()

		switch key {
		case "wtimeout":
			wtimeout := time.Duration(val.Int32()) * time.Millisecond
			opts = append(opts, writeconcern.WTimeout(wtimeout))
		case "j":
			opts = append(opts, writeconcern.J(val.Boolean()))
		case "w":
			switch val.Type {
			case bson.TypeString:
				if val.StringValue() != "majority" {
					break
				}
				opts = append(opts, writeconcern.WMajority())
			case bson.TypeInt32:
				w := int(val.Int32())
				opts = append(opts, writeconcern.W(w))
			default:
				t.Fatalf("unrecognized type for w: %v", val.Type)
			}
		default:
			t.Fatalf("unrecognized write concern option: %v", key)
		}
	}
	return writeconcern.New(opts...)
}

// create a read preference from a string.
// returns readpref.Primary() if the string doesn't match any known read preference modes.
func readPrefFromString(s string) *readpref.ReadPref {
	switch strings.ToLower(s) {
	case "primary":
		return readpref.Primary()
	case "primarypreferred":
		return readpref.PrimaryPreferred()
	case "secondary":
		return readpref.Secondary()
	case "secondarypreferred":
		return readpref.SecondaryPreferred()
	case "nearest":
		return readpref.Nearest()
	}
	return readpref.Primary()
}

// create a read preference from a map.
func createReadPref(opt bson.RawValue) *readpref.ReadPref {
	mode := opt.Document().Lookup("mode").StringValue()
	return readPrefFromString(mode)
}

// transform a slice of BSON documents to a slice of interface{}.
func rawSliceToInterfaceSlice(docs []bson.Raw) []interface{} {
	out := make([]interface{}, len(docs))

	for i, doc := range docs {
		out[i] = doc
	}

	return out
}

// transform a BSON raw array to a slice of interface{}.
func rawArrayToInterfaceSlice(docs bson.Raw) []interface{} {
	vals, _ := docs.Values()

	out := make([]interface{}, len(vals))
	for i, val := range vals {
		out[i] = val.Document()
	}

	return out
}

// retrieve the error associated with a result.
func errorFromResult(t testing.TB, result interface{}) *operationError {
	t.Helper()

	// embedded doc will be unmarshalled as Raw
	raw, ok := result.(bson.Raw)
	if !ok {
		return nil
	}

	var expected operationError
	err := bson.Unmarshal(raw, &expected)
	if err != nil {
		return nil
	}
	if expected.ErrorCodeName == nil && expected.ErrorContains == nil && len(expected.ErrorLabelsOmit) == 0 &&
		len(expected.ErrorLabelsContain) == 0 {
		return nil
	}

	return &expected
}

// verify that an error returned by an operation matches the expected error.
func verifyError(t testing.TB, expected *operationError, actual error) {
	t.Helper()

	expectErr := expected != nil
	gotErr := actual != nil

	// spec test format doesn't treat ErrNoDocuments as an error. checking that no documents were returned is handled
	// in the result checking section.
	if !expectErr && actual == mongo.ErrNoDocuments {
		return
	}

	assert.Equal(t, expectErr, gotErr, "expected error: %v, got err: %v", expectErr, gotErr)
	if !expectErr {
		return
	}

	// check ErrorContains for all error types
	if expected.ErrorContains != nil {
		emsg := strings.ToLower(*expected.ErrorContains)
		amsg := strings.ToLower(actual.Error())
		assert.True(t, strings.Contains(amsg, emsg), "expected error message '%v' to contain '%v'", amsg, emsg)
	}

	cerr, ok := actual.(mongo.CommandError)
	if !ok {
		return
	}

	if expected.ErrorCodeName != nil {
		assert.Equal(t, *expected.ErrorCodeName, cerr.Name,
			"error name mismatch; expected %v, got %v", *expected.ErrorCodeName, cerr.Name)
	}
	for _, label := range expected.ErrorLabelsContain {
		assert.True(t, cerr.HasErrorLabel(label), "expected error %v to contain label %v", actual, label)
	}
	for _, label := range expected.ErrorLabelsOmit {
		assert.False(t, cerr.HasErrorLabel(label), "expected error %v to not contain label %v", actual, label)
	}
}

// get the underlying value of i as an int64. returns nil if i is not an int, int32, or int64 type.
func getIntFromInterface(i interface{}) *int64 {
	var out int64

	switch v := i.(type) {
	case int:
		out = int64(v)
	case int32:
		out = int64(v)
	case int64:
		out = v
	case float32:
		f := float64(v)
		if math.Floor(f) != f || f > float64(math.MaxInt64) {
			break
		}

		out = int64(f)
	case float64:
		if math.Floor(v) != v || v > float64(math.MaxInt64) {
			break
		}

		out = int64(v)
	default:
		return nil
	}

	return &out
}

func createCollation(t testing.TB, m bson.Raw) *options.Collation {
	var collation options.Collation
	elems, _ := m.Elements()

	for _, elem := range elems {
		switch elem.Key() {
		case "locale":
			collation.Locale = elem.Value().StringValue()
		case "caseLevel":
			collation.CaseLevel = elem.Value().Boolean()
		case "caseFirst":
			collation.CaseFirst = elem.Value().StringValue()
		case "strength":
			collation.Strength = int(elem.Value().Int32())
		case "numericOrdering":
			collation.NumericOrdering = elem.Value().Boolean()
		case "alternate":
			collation.Alternate = elem.Value().StringValue()
		case "maxVariable":
			collation.MaxVariable = elem.Value().StringValue()
		case "normalization":
			collation.Normalization = elem.Value().Boolean()
		case "backwards":
			collation.Backwards = elem.Value().Boolean()
		default:
			t.Fatalf("unrecognized collation option: %v", elem.Key())
		}
	}
	return &collation
}

func createChangeStreamOptions(t testing.TB, opts bson.Raw) *options.ChangeStreamOptions {
	t.Helper()

	csOpts := options.ChangeStream()
	elems, _ := opts.Elements()
	for _, elem := range elems {
		key := elem.Key()
		opt := elem.Value()

		switch key {
		case "batchSize":
			csOpts.SetBatchSize(opt.Int32())
		default:
			t.Fatalf("unrecognized change stream option: %v", key)
		}
	}
	return csOpts
}
