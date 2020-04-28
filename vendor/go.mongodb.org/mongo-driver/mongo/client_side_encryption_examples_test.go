// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Example_clientSideEncryption() {
	// This would have to be the same master key that was used to create the encryption key
	localKey := make([]byte, 96)
	if _, err := rand.Read(localKey); err != nil {
		log.Fatal(err)
	}
	kmsProviders := map[string]map[string]interface{}{
		"local": {
			"key": localKey,
		},
	}
	keyVaultNamespace := "admin.datakeys"

	uri := "mongodb://localhost:27017"
	autoEncryptionOpts := options.AutoEncryption().
		SetKeyVaultNamespace(keyVaultNamespace).
		SetKmsProviders(kmsProviders)
	clientOpts := options.Client().ApplyURI(uri).SetAutoEncryptionOptions(autoEncryptionOpts)
	client, err := Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatalf("Disconnect error: %v", err)
		}
	}()

	collection := client.Database("test").Collection("coll")
	if err := collection.Drop(context.TODO()); err != nil {
		log.Fatalf("Collection.Drop error: %v", err)
	}

	if _, err = collection.InsertOne(context.TODO(), bson.D{{"encryptedField", "123456789"}}); err != nil {
		log.Fatalf("InsertOne error: %v", err)
	}
	res, err := collection.FindOne(context.TODO(), bson.D{}).DecodeBytes()
	if err != nil {
		log.Fatalf("FindOne error: %v", err)
	}
	fmt.Println(res)
}

func Example_clientSideEncryptionCreateKey() {
	keyVaultNamespace := "admin.datakeys"
	uri := "mongodb://localhost:27017"
	// kmsProviders would have to be populated with the correct KMS provider information before it's used
	var kmsProviders map[string]map[string]interface{}

	// Create Client and ClientEncryption
	clientEncryptionOpts := options.ClientEncryption().
		SetKeyVaultNamespace(keyVaultNamespace).
		SetKmsProviders(kmsProviders)
	keyVaultClient, err := Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Connect error for keyVaultClient: %v", err)
	}
	clientEnc, err := NewClientEncryption(keyVaultClient, clientEncryptionOpts)
	if err != nil {
		log.Fatalf("NewClientEncryption error: %v", err)
	}
	defer func() {
		// this will disconnect the keyVaultClient as well
		if err = clientEnc.Close(context.TODO()); err != nil {
			log.Fatalf("Close error: %v", err)
		}
	}()

	// Create a new data key and encode it as base64
	dataKeyID, err := clientEnc.CreateDataKey(context.TODO(), "local")
	if err != nil {
		log.Fatalf("CreateDataKey error: %v", err)
	}
	dataKeyBase64 := base64.StdEncoding.EncodeToString(dataKeyID.Data)

	// Create a JSON schema using the new data key. This schema could also be written in a separate file and read in
	// using I/O functions.
	schema := `{
		"properties": {
			"encryptedField": {
				"encrypt": {
					"keyId": [{
						"$binary": {
							"base64": "%s",
							"subType": "04"
						}
					}],
					"bsonType": "string",
					"algorithm": "AEAD_AES_256_CBC_HMAC_SHA_512-Deterministic"
				}
			}
		},
		"bsonType": "object"
	}`
	schema = fmt.Sprintf(schema, dataKeyBase64)
	var schemaDoc bson.Raw
	if err = bson.UnmarshalExtJSON([]byte(schema), true, &schemaDoc); err != nil {
		log.Fatalf("UnmarshalExtJSON error: %v", err)
	}

	// Configure a Client with auto encryption using the new schema
	dbName := "test"
	collName := "coll"
	schemaMap := map[string]interface{}{
		dbName + "." + collName: schemaDoc,
	}
	autoEncryptionOpts := options.AutoEncryption().
		SetKmsProviders(kmsProviders).
		SetKeyVaultNamespace(keyVaultNamespace).
		SetSchemaMap(schemaMap)
	client, err := Connect(context.TODO(), options.Client().ApplyURI(uri).SetAutoEncryptionOptions(autoEncryptionOpts))
	if err != nil {
		log.Fatalf("Connect error for encrypted client: %v", err)
	}
	defer func() {
		_ = client.Disconnect(context.TODO())
	}()

	// Use client for operations.
}

func Example_explictEncryption() {
	var localMasterKey []byte // This must be the same master key that was used to create the encryption key.
	kmsProviders := map[string]map[string]interface{}{
		"local": {
			"key": localMasterKey,
		},
	}

	// The MongoDB namespace (db.collection) used to store the encryption data keys.
	keyVaultDBName, keyVaultCollName := "encryption", "testKeyVault"
	keyVaultNamespace := keyVaultDBName + "." + keyVaultCollName

	// The Client used to read/write application data.
	client, err := Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Disconnect(context.TODO()) }()

	// Get a handle to the application collection and clear existing data.
	coll := client.Database("test").Collection("coll")
	_ = coll.Drop(context.TODO())

	// Set up the key vault for this example.
	keyVaultColl := client.Database(keyVaultDBName).Collection(keyVaultCollName)
	_ = keyVaultColl.Drop(context.TODO())
	// Ensure that two data keys cannot share the same keyAltName.
	keyVaultIndex := IndexModel{
		Keys: bson.D{{"keyAltNames", 1}},
		Options: options.Index().
			SetUnique(true).
			SetPartialFilterExpression(bson.D{
				{"keyAltNames", bson.D{
					{"$exists", true},
				}},
			}),
	}
	if _, err = keyVaultColl.Indexes().CreateOne(context.TODO(), keyVaultIndex); err != nil {
		panic(err)
	}

	// Create the ClientEncryption object to use for explicit encryption/decryption. The Client passed to
	// NewClientEncryption is used to read/write to the key vault. This can be the same Client used by the main
	// application.
	clientEncryptionOpts := options.ClientEncryption().
		SetKmsProviders(kmsProviders).
		SetKeyVaultNamespace(keyVaultNamespace)
	clientEncryption, err := NewClientEncryption(client, clientEncryptionOpts)
	if err != nil {
		panic(err)
	}
	defer func() { _ = clientEncryption.Close(context.TODO()) }()

	// Create a new data key for the encrypted field.
	dataKeyOpts := options.DataKey().SetKeyAltNames([]string{"go_encryption_example"})
	dataKeyID, err := clientEncryption.CreateDataKey(context.TODO(), "local", dataKeyOpts)
	if err != nil {
		panic(err)
	}

	// Create a bson.RawValue to encrypt and encrypt it using the key that was just created.
	rawValueType, rawValueData, err := bson.MarshalValue("123456789")
	if err != nil {
		panic(err)
	}
	rawValue := bson.RawValue{Type: rawValueType, Value: rawValueData}
	encryptionOpts := options.Encrypt().
		SetAlgorithm("AEAD_AES_256_CBC_HMAC_SHA_512-Deterministic").
		SetKeyID(dataKeyID)
	encryptedField, err := clientEncryption.Encrypt(context.TODO(), rawValue, encryptionOpts)
	if err != nil {
		panic(err)
	}

	// Insert a document with the encrypted field and then find it.
	if _, err = coll.InsertOne(context.TODO(), bson.D{{"encryptedField", encryptedField}}); err != nil {
		panic(err)
	}
	var foundDoc bson.M
	if err = coll.FindOne(context.TODO(), bson.D{}).Decode(&foundDoc); err != nil {
		panic(err)
	}

	// Decrypt the encrypted field in the found document.
	decrypted, err := clientEncryption.Decrypt(context.TODO(), foundDoc["encryptedField"].(primitive.Binary))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Decrypted value: %s\n", decrypted)
}

func Example_explictEncryptionWithAutomaticDecryption() {
	// Automatic encryption requires MongoDB 4.2 enterprise, but automatic decryption is supported for all users.

	var localMasterKey []byte // This must be the same master key that was used to create the encryption key.
	kmsProviders := map[string]map[string]interface{}{
		"local": {
			"key": localMasterKey,
		},
	}

	// The MongoDB namespace (db.collection) used to store the encryption data keys.
	keyVaultDBName, keyVaultCollName := "encryption", "testKeyVault"
	keyVaultNamespace := keyVaultDBName + "." + keyVaultCollName

	// Create the Client for reading/writing application data. Configure it with BypassAutoEncryption=true to disable
	// automatic encryption but keep automatic decryption. Setting BypassAutoEncryption will also bypass spawning
	// mongocryptd in the driver.
	autoEncryptionOpts := options.AutoEncryption().
		SetKmsProviders(kmsProviders).
		SetKeyVaultNamespace(keyVaultNamespace).
		SetBypassAutoEncryption(true)
	clientOpts := options.Client().
		ApplyURI("mongodb://localhost:27017").
		SetAutoEncryptionOptions(autoEncryptionOpts)
	client, err := Connect(context.TODO(), clientOpts)
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Disconnect(context.TODO()) }()

	// Get a handle to the application collection and clear existing data.
	coll := client.Database("test").Collection("coll")
	_ = coll.Drop(context.TODO())

	// Set up the key vault for this example.
	keyVaultColl := client.Database(keyVaultDBName).Collection(keyVaultCollName)
	_ = keyVaultColl.Drop(context.TODO())
	// Ensure that two data keys cannot share the same keyAltName.
	keyVaultIndex := IndexModel{
		Keys: bson.D{{"keyAltNames", 1}},
		Options: options.Index().
			SetUnique(true).
			SetPartialFilterExpression(bson.D{
				{"keyAltNames", bson.D{
					{"$exists", true},
				}},
			}),
	}
	if _, err = keyVaultColl.Indexes().CreateOne(context.TODO(), keyVaultIndex); err != nil {
		panic(err)
	}

	// Create the ClientEncryption object to use for explicit encryption/decryption. The Client passed to
	// NewClientEncryption is used to read/write to the key vault. This can be the same Client used by the main
	// application.
	clientEncryptionOpts := options.ClientEncryption().
		SetKmsProviders(kmsProviders).
		SetKeyVaultNamespace(keyVaultNamespace)
	clientEncryption, err := NewClientEncryption(client, clientEncryptionOpts)
	if err != nil {
		panic(err)
	}
	defer func() { _ = clientEncryption.Close(context.TODO()) }()

	// Create a new data key for the encrypted field.
	dataKeyOpts := options.DataKey().SetKeyAltNames([]string{"go_encryption_example"})
	dataKeyID, err := clientEncryption.CreateDataKey(context.TODO(), "local", dataKeyOpts)
	if err != nil {
		panic(err)
	}

	// Create a bson.RawValue to encrypt and encrypt it using the key that was just created.
	rawValueType, rawValueData, err := bson.MarshalValue("123456789")
	if err != nil {
		panic(err)
	}
	rawValue := bson.RawValue{Type: rawValueType, Value: rawValueData}
	encryptionOpts := options.Encrypt().
		SetAlgorithm("AEAD_AES_256_CBC_HMAC_SHA_512-Deterministic").
		SetKeyID(dataKeyID)
	encryptedField, err := clientEncryption.Encrypt(context.TODO(), rawValue, encryptionOpts)
	if err != nil {
		panic(err)
	}

	// Insert a document with the encrypted field and then find it. The FindOne call will automatically decrypt the
	// field in the document.
	if _, err = coll.InsertOne(context.TODO(), bson.D{{"encryptedField", encryptedField}}); err != nil {
		panic(err)
	}
	var foundDoc bson.M
	if err = coll.FindOne(context.TODO(), bson.D{}).Decode(&foundDoc); err != nil {
		panic(err)
	}
	fmt.Printf("Decrypted document: %v\n", foundDoc)
}
