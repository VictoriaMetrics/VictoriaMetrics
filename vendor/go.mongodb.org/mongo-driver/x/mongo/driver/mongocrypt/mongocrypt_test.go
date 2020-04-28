// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

// +build cse

package mongocrypt

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/mongocrypt/options"
)

const resourcesDir = "../../../../data/mongocrypt"

func noerr(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Errorf("Unexpected error: (%T)%v", err, err)
		t.FailNow()
	}
}

func compareStates(t *testing.T, expected, actual State) {
	t.Helper()
	if expected != actual {
		t.Fatalf("state mismatch; expected %s, got %s", expected, actual)
	}
}

func compareResources(t *testing.T, expected, actual bsoncore.Document) {
	t.Helper()
	if !bytes.Equal(expected, actual) {
		t.Fatalf("resource mismatch; expected %v, got %v", expected, actual)
	}
}

func createMongoCrypt(t *testing.T) *MongoCrypt {
	t.Helper()
	localMasterKey := make([]byte, 96)
	awsOpts := options.AwsKmsProvider().SetAccessKeyID("example").SetSecretAccessKey("example")
	localOpts := options.LocalKmsProvider().SetMasterKey(localMasterKey)
	cryptOpts := options.MongoCrypt().SetAwsProviderOptions(awsOpts).SetLocalProviderOptions(localOpts)

	crypt, err := NewMongoCrypt(cryptOpts)
	noerr(t, err)
	if crypt == nil {
		t.Fatalf("expected MongoCrypt instance but got nil")
	}
	return crypt
}

func resourceToDocument(t *testing.T, filename string) bsoncore.Document {
	t.Helper()
	filepath := path.Join(resourcesDir, filename)
	content, err := ioutil.ReadFile(filepath)
	noerr(t, err)

	var doc bsoncore.Document
	noerr(t, bson.UnmarshalExtJSON(content, false, &doc))
	return doc
}

func httpResponseToBytes(t *testing.T, filename string) []byte {
	t.Helper()
	file, err := os.Open(path.Join(resourcesDir, filename))
	noerr(t, err)
	defer func() {
		_ = file.Close()
	}()

	firstLine := true
	var fileStr string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if !firstLine {
			fileStr += "\r\n"
		}
		firstLine = false
		fileStr += scanner.Text()
	}
	noerr(t, scanner.Err())

	return []byte(fileStr)
}

// iterate a context in the NeedKms state
func testKmsCtx(t *testing.T, ctx *Context, keyAltName bool) {
	// get op to send to key vault
	keyFilter, err := ctx.NextOperation()
	noerr(t, err)
	filterFile := "key-filter.json"
	if keyAltName {
		filterFile = "key-filter-keyAltName.json"
	}
	compareResources(t, resourceToDocument(t, filterFile), keyFilter)

	// feed result and finish op
	noerr(t, ctx.AddOperationResult(resourceToDocument(t, "key-document.json")))
	noerr(t, ctx.CompleteOperation())
	compareStates(t, NeedKms, ctx.State())

	// verify KMS hostname
	kmsCtx := ctx.NextKmsContext()
	hostname, err := kmsCtx.HostName()
	noerr(t, err)
	expectedHost := "kms.us-east-1.amazonaws.com"
	if hostname != expectedHost {
		t.Fatalf("hostname mismatch; expected %s, got %s", expectedHost, hostname)
	}

	// get message to send to KMS
	kmsMsg, err := kmsCtx.Message()
	noerr(t, err)
	if len(kmsMsg) != 781 {
		t.Fatalf("message length mismatch; expected 781, got %d", len(kmsMsg))
	}

	// feed mock KMS response
	bytesNeeded := kmsCtx.BytesNeeded()
	if bytesNeeded != 1024 {
		t.Fatalf("number of bytes mismatch; expected 1024, got %d", bytesNeeded)
	}
	noerr(t, kmsCtx.FeedResponse(httpResponseToBytes(t, "kms-reply.txt")))
	bytesNeeded = kmsCtx.BytesNeeded()
	if bytesNeeded != 0 {
		t.Fatalf("number of bytes mismatch; expected 0, got %d", bytesNeeded)
	}

	// verify that there are no more KMS contexts left
	kmsCtx = ctx.NextKmsContext()
	if kmsCtx != nil {
		t.Fatalf("expected nil but got a KmsContext")
	}
	noerr(t, ctx.FinishKmsContexts())
}

func TestMongoCrypt(t *testing.T) {
	t.Run("encrypt", func(t *testing.T) {
		t.Run("remote schema", func(t *testing.T) {
			crypt := createMongoCrypt(t)
			defer crypt.Close()

			// create encryption context and check initial state
			cmdDoc := resourceToDocument(t, "command.json")
			encryptCtx, err := crypt.CreateEncryptionContext("test", cmdDoc)
			noerr(t, err)
			defer encryptCtx.Close()
			compareStates(t, NeedMongoCollInfo, encryptCtx.State())

			// get listCollections op
			listCollFilter, err := encryptCtx.NextOperation()
			noerr(t, err)
			compareResources(t, resourceToDocument(t, "list-collections-filter.json"), listCollFilter)

			// feed result and finish op
			noerr(t, encryptCtx.AddOperationResult(resourceToDocument(t, "collection-info.json")))
			noerr(t, encryptCtx.CompleteOperation())
			compareStates(t, NeedMongoMarkings, encryptCtx.State())

			// get mongocryptd op
			mongocryptdCmd, err := encryptCtx.NextOperation()
			noerr(t, err)
			compareResources(t, resourceToDocument(t, "mongocryptd-command-remote.json"), mongocryptdCmd)

			// feed result and finish op
			noerr(t, encryptCtx.AddOperationResult(resourceToDocument(t, "mongocryptd-reply.json")))
			noerr(t, encryptCtx.CompleteOperation())
			compareStates(t, NeedMongoKeys, encryptCtx.State())

			// mock KMS communication and iterate encryptCtx
			testKmsCtx(t, encryptCtx, false)
			compareStates(t, Ready, encryptCtx.State())

			// perform final encryption
			encryptedDoc, err := encryptCtx.Finish()
			noerr(t, err)
			compareResources(t, resourceToDocument(t, "encrypted-command.json"), encryptedDoc)
		})
		t.Run("local schema", func(t *testing.T) {
			// take schema from collection info and create MongoCrypt instance
			collInfo := resourceToDocument(t, "collection-info.json")
			schema := collInfo.Lookup("options", "validator", "$jsonSchema").Document()
			schemaMap := map[string]bsoncore.Document{
				"test.test": schema,
			}
			awsOpts := options.AwsKmsProvider().SetSecretAccessKey("example").SetAccessKeyID("example")
			cryptOpts := options.MongoCrypt().SetAwsProviderOptions(awsOpts).SetLocalSchemaMap(schemaMap)
			crypt, err := NewMongoCrypt(cryptOpts)
			noerr(t, err)
			defer crypt.Close()

			// create encryption context and check initial state
			encryptCtx, err := crypt.CreateEncryptionContext("test", resourceToDocument(t, "command.json"))
			noerr(t, err)
			defer encryptCtx.Close()
			compareStates(t, NeedMongoMarkings, encryptCtx.State())

			// get mongocryptd op
			mongocryptdCmd, err := encryptCtx.NextOperation()
			noerr(t, err)
			compareResources(t, resourceToDocument(t, "mongocryptd-command-local.json"), mongocryptdCmd)

			// feed result and finish op
			noerr(t, encryptCtx.AddOperationResult(resourceToDocument(t, "mongocryptd-reply.json")))
			noerr(t, encryptCtx.CompleteOperation())
			compareStates(t, NeedMongoKeys, encryptCtx.State())

			// mock KMS communication and iterate encryptCtx
			testKmsCtx(t, encryptCtx, false)
			compareStates(t, Ready, encryptCtx.State())

			// perform final encryption
			encryptedDoc, err := encryptCtx.Finish()
			noerr(t, err)
			compareResources(t, resourceToDocument(t, "encrypted-command.json"), encryptedDoc)
		})
		t.Run("invalid bson", func(t *testing.T) {
			crypt := createMongoCrypt(t)
			defer crypt.Close()

			_, err := crypt.CreateEncryptionContext("test", []byte{0x1, 0x2, 0x3})
			if err == nil {
				t.Fatalf("expected error creating encryption context for invalid BSON but got nil")
			}
			if _, ok := err.(Error); !ok {
				t.Fatalf("error type mismatch; expected Error, got %v", err)
			}
		})
	})
	t.Run("decrypt", func(t *testing.T) {
		crypt := createMongoCrypt(t)
		defer crypt.Close()

		// create decryption context and check initial state
		decryptCtx, err := crypt.CreateDecryptionContext(resourceToDocument(t, "encrypted-command-reply.json"))
		noerr(t, err)
		defer decryptCtx.Close()
		compareStates(t, NeedMongoKeys, decryptCtx.State())

		// mock KMS communication and iterate decryptCtx
		testKmsCtx(t, decryptCtx, false)
		compareStates(t, Ready, decryptCtx.State())

		// perform final decryption
		decryptedDoc, err := decryptCtx.Finish()
		noerr(t, err)
		compareResources(t, resourceToDocument(t, "command-reply.json"), decryptedDoc)
	})
	t.Run("data key creation", func(t *testing.T) {
		crypt := createMongoCrypt(t)
		defer crypt.Close()

		// create master key document
		var midx int32
		var masterKey bsoncore.Document
		midx, masterKey = bsoncore.AppendDocumentStart(nil)
		masterKey, _ = bsoncore.AppendDocumentEnd(masterKey, midx)

		// create data key context and check initial state
		dataKeyOpts := options.DataKey().SetMasterKey(masterKey)
		dataKeyCtx, err := crypt.CreateDataKeyContext(LocalProvider, dataKeyOpts)
		noerr(t, err)
		defer dataKeyCtx.Close()
		compareStates(t, Ready, dataKeyCtx.State())

		// create data key
		dataKeyDoc, err := dataKeyCtx.Finish()
		noerr(t, err)
		if len(dataKeyDoc) == 0 {
			t.Fatalf("expected data key document but got empty doc")
		}
		compareStates(t, Done, dataKeyCtx.State())
	})
	t.Run("explicit roundtrip", func(t *testing.T) {
		// algorithm to use for encryption/decryption
		algorithm := "AEAD_AES_256_CBC_HMAC_SHA_512-Deterministic"
		// doc to encrypt
		idx, originalDoc := bsoncore.AppendDocumentStart(nil)
		originalDoc = bsoncore.AppendStringElement(originalDoc, "v", "hello")
		originalDoc, _ = bsoncore.AppendDocumentEnd(originalDoc, idx)

		t.Run("no keyAltName", func(t *testing.T) {
			crypt := createMongoCrypt(t)
			defer crypt.Close()

			// create explicit encryption context and check initial state
			keyID := primitive.Binary{
				Subtype: 0x04, // 0x04 is UUID subtype
				Data:    []byte("aaaaaaaaaaaaaaaa"),
			}
			opts := options.ExplicitEncryption().SetKeyID(keyID).SetAlgorithm(algorithm)
			encryptCtx, err := crypt.CreateExplicitEncryptionContext(originalDoc, opts)
			noerr(t, err)
			defer encryptCtx.Close()
			compareStates(t, NeedMongoKeys, encryptCtx.State())

			// mock KMS communication and iterate encryptCtx
			testKmsCtx(t, encryptCtx, false)
			compareStates(t, Ready, encryptCtx.State())

			// perform final encryption
			encryptedDoc, err := encryptCtx.Finish()
			noerr(t, err)
			compareStates(t, Done, encryptCtx.State())
			compareResources(t, resourceToDocument(t, "encrypted-value.json"), encryptedDoc)

			// create explicit decryption context and check initial state
			decryptCtx, err := crypt.CreateDecryptionContext(encryptedDoc)
			noerr(t, err)
			defer decryptCtx.Close()
			compareStates(t, Ready, decryptCtx.State())

			// perform final decryption
			decryptedDoc, err := decryptCtx.Finish()
			noerr(t, err)
			compareStates(t, Done, decryptCtx.State())
			compareResources(t, originalDoc, decryptedDoc)
		})
		t.Run("keyAltName", func(t *testing.T) {
			crypt := createMongoCrypt(t)
			defer crypt.Close()

			// create explicit encryption context and check initial state
			opts := options.ExplicitEncryption().SetKeyAltName("altKeyName").SetAlgorithm(algorithm)
			encryptCtx, err := crypt.CreateExplicitEncryptionContext(originalDoc, opts)
			noerr(t, err)
			defer encryptCtx.Close()
			compareStates(t, NeedMongoKeys, encryptCtx.State())

			// mock KMS communication and iterate encryptCtx
			testKmsCtx(t, encryptCtx, true)
			compareStates(t, Ready, encryptCtx.State())

			// perform final encryption
			encryptedDoc, err := encryptCtx.Finish()
			noerr(t, err)
			compareStates(t, Done, encryptCtx.State())
			compareResources(t, resourceToDocument(t, "encrypted-value.json"), encryptedDoc)

			// create explicit decryption context and check initial state
			// the cryptCtx should automatically be in the Ready state because the key should be cached from the
			// encryption process.
			decryptCtx, err := crypt.CreateExplicitDecryptionContext(encryptedDoc)
			noerr(t, err)
			defer decryptCtx.Close()
			compareStates(t, Ready, decryptCtx.State())

			// perform final decryption
			decryptedDoc, err := decryptCtx.Finish()
			noerr(t, err)
			compareStates(t, Done, decryptCtx.State())
			compareResources(t, originalDoc, decryptedDoc)
		})
	})
}
