// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo_test

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func ExampleClient() {
	// Create a Client and execute a ListDatabases operation.

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()

	collection := client.Database("db").Collection("coll")
	result, err := collection.InsertOne(context.TODO(), bson.D{{"x", 1}})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("inserted ID: %v\n", result.InsertedID)
}

func ExampleConnect_ping() {
	// Create a Client to a MongoDB server and use Ping to verify that the server is running.

	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()

	// Call Ping to verify that the deployment is up and the Client was configured successfully.
	// As mentioned in the Ping documentation, this reduces application resiliency as the server may be
	// temporarily unavailable when Ping is called.
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
}

func ExampleConnect_replicaSet() {
	// Create and connect a Client to a replica set deployment.
	// Given this URI, the Go driver will first communicate with localhost:27017 and use the response to discover
	// any other members in the replica set.
	// The URI in this example specifies multiple members of the replica set to increase resiliency as one of the
	// members may be down when the application is started.

	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017,localhost:27018/?replicaSet=replset")
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_sharded() {
	// Create and connect a Client to a sharded deployment.
	// The URI for a sharded deployment should specify the mongos servers that the application wants to send
	// messages to.

	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017,localhost:27018")
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_sRV() {
	// Create and connect a Client using an SRV record.
	// SRV records allow administrators to configure a single domain to return a list of host names.
	// The driver will resolve SRV records prefixed with "_mongodb_tcp" and use the returned host names to
	// build its view of the deployment.
	// See https://docs.mongodb.com/manual/reference/connection-string/ for more information about SRV.
	// Full support for SRV records with sharded clusters requires driver version 1.1.0 or higher.

	clientOpts := options.Client().ApplyURI("mongodb+srv://mongodb.example.com")
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_direct() {
	// Create a direct connection to a host. The driver will send all requests to that host and will not
	// automatically discover other hosts in the deployment.

	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017/?connect=direct")
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_sCRAM() {
	// Configure a Client with SCRAM authentication (https://docs.mongodb.com/manual/core/security-scram/).
	// The default authentication database for SCRAM is "admin". This can be configured via the
	// authSource query parameter in the URI or the AuthSource field in the options.Credential struct.
	// SCRAM is the default auth mechanism so specifying a mechanism is not required.

	// To configure auth via URI instead of a Credential, use
	// "mongodb://user:password@localhost:27017".
	credential := options.Credential{
		Username: "user",
		Password: "password",
	}
	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017").SetAuth(credential)
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_x509() {
	// Configure a Client with X509 authentication (https://docs.mongodb.com/manual/core/security-x.509/).

	// X509 can be configured with different sets of options in the connection string:
	// 1. tlsCAFile (or SslCertificateAuthorityFile): Path to the file with either a single or bundle of certificate
	// authorities to be considered trusted when making a TLS connection.
	// 2. tlsCertificateKeyFile (or SslClientCertificateKeyFile): Path to the client certificate file or the client
	// private key file. In the case that both are needed, the files should be concatenated.

	// The SetAuth client option should also be used. The username field is optional. If it is not specified, it will
	// be extracted from the certificate key file. The AuthSource is required to be $external.

	caFilePath := "path/to/cafile"
	certificateKeyFilePath := "path/to/client-certificate"

	// To configure auth via a URI instead of a Credential, append "&authMechanism=MONGODB-X509" to the URI.
	uri := "mongodb://host:port/?tlsCAFile=%s&tlsCertificateKeyFile=%s"
	uri = fmt.Sprintf(uri, caFilePath, certificateKeyFilePath)
	credential := options.Credential{
		AuthMechanism: "MONGODB-X509",
	}
	clientOpts := options.Client().ApplyURI(uri).SetAuth(credential)

	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_pLAIN() {
	// Configure a Client with LDAP authentication
	// (https://docs.mongodb.com/manual/core/authentication-mechanisms-enterprise/#security-auth-ldap).
	// MongoDB Enterprise supports proxy authentication through an LDAP service that can be used through the PLAIN
	// authentication mechanism.
	// This auth mechanism sends the password in plaintext and therefore should only be used with TLS connections.

	// To configure auth via a URI instead of a Credential, use
	// "mongodb://ldap-user:ldap-pwd@localhost:27017/?authMechanism=PLAIN".
	credential := options.Credential{
		AuthMechanism: "PLAIN",
		Username:      "ldap-user",
		Password:      "ldap-pwd",
	}
	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017").SetAuth(credential)

	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleConnect_kerberos() {
	// Configure a Client with GSSAPI/SSPI authentication (https://docs.mongodb.com/manual/core/kerberos/).
	// MongoDB Enterprise supports proxy authentication through a Kerberos service.
	// Using Kerberos authentication requires the "gssapi" build tag and cgo support during compilation.
	// The default service name for Kerberos is "mongodb". This can be configured via the AuthMechanismProperties
	// field in the options.Credential struct or the authMechanismProperties URI parameter.

	// For Linux, the libkrb5 library is required.
	// Users can authenticate in one of two ways:
	// 1. Use an explicit password. In this case, a password must be specified in the URI or the options.Credential
	// struct and no further setup is required.
	// 2. Store authentication keys in keytab files. To do this, the kinit binary should be used to initialize a
	// credential cache for authenticating the user principal. In this example, the invocation would be
	// "kinit drivers@KERBEROS.EXAMPLE.COM".

	// To configure auth via a URI instead of a Credential, use
	// "mongodb://drivers%40KERBEROS.EXAMPLE.COM@mongo-server.example.com:27017/?authMechanism=GSSAPI".
	credential := options.Credential{
		AuthMechanism: "GSSAPI",
		Username:      "drivers@KERBEROS.EXAMPLE.COM",
	}
	uri := "mongo-server.example.com:27017"
	clientOpts := options.Client().ApplyURI(uri).SetAuth(credential)

	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}
