// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

const (
	seedlistDiscoveryTestsDir = "../../data/initial-dns-seedlist-discovery"
)

type seedlistTest struct {
	URI     string   `bson:"uri"`
	Seeds   []string `bson:"seeds"`
	Hosts   []string `bson:"hosts"`
	Error   bool     `bson:"error"`
	Options bson.Raw `bson:"options"`
}

func TestInitialDNSSeedlistDiscoverySpec(t *testing.T) {
	mtOpts := mtest.NewOptions().Topologies(mtest.ReplicaSet).CreateClient(false)
	mt := mtest.New(t, mtOpts)
	defer mt.Close()

	for _, file := range jsonFilesInDir(mt, seedlistDiscoveryTestsDir) {
		mt.RunOpts(file, noClientOpts, func(mt *mtest.T) {
			runSeedlistDiscoveryTest(mt, path.Join(seedlistDiscoveryTestsDir, file))
		})
	}
}

func runSeedlistDiscoveryTest(mt *mtest.T, file string) {
	content, err := ioutil.ReadFile(file)
	assert.Nil(mt, err, "ReadFile error for %v: %v", file, err)

	var test seedlistTest
	err = bson.UnmarshalExtJSONWithRegistry(specTestRegistry, content, false, &test)
	assert.Nil(mt, err, "UnmarshalExtJSONWithRegistry error: %v", err)

	if runtime.GOOS == "windows" && strings.HasSuffix(file, "/two-txt-records.json") {
		mt.Skip("skipping to avoid windows multiple TXT record lookup bug")
	}
	if strings.HasPrefix(runtime.Version(), "go1.11") && strings.HasSuffix(file, "/one-txt-record-multiple-strings.json") {
		mt.Skip("skipping to avoid go1.11 problem with multiple strings in one TXT record")
	}

	cs, err := connstring.Parse(test.URI)
	if test.Error {
		assert.NotNil(mt, err, "expected URI parsing error, got nil")
		return
	}
	// the resolved connstring may not have valid credentials
	if err != nil && err.Error() == "error parsing uri: authsource without username is invalid" {
		err = nil
	}
	assert.Nil(mt, err, "Connect error: %v", err)
	assert.Equal(mt, connstring.SchemeMongoDBSRV, cs.Scheme,
		"expected scheme %v, got %v", connstring.SchemeMongoDBSRV, cs.Scheme)

	// DNS records may be out of order from the test file's ordering
	expectedSeedlist := buildSet(test.Seeds)
	actualSeedlist := buildSet(cs.Hosts)
	assert.Equal(mt, expectedSeedlist, actualSeedlist, "expected seedlist %v, got %v", expectedSeedlist, actualSeedlist)
	verifyConnstringOptions(mt, test.Options, cs)
	setSSLSettings(mt, &cs, test)

	// make a topology from the options
	topo, err := topology.New(topology.WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }))
	assert.Nil(mt, err, "topology.New error: %v", err)
	err = topo.Connect()
	assert.Nil(mt, err, "topology.Connect error: %v", err)
	defer func() { _ = topo.Disconnect(mtest.Background) }()

	for _, host := range test.Hosts {
		_, err := getServerByAddress(host, topo)
		assert.Nil(mt, err, "did not find host %v", host)
	}
}

func buildSet(list []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, s := range list {
		set[s] = struct{}{}
	}
	return set
}

func verifyConnstringOptions(mt *mtest.T, expected bson.Raw, cs connstring.ConnString) {
	mt.Helper()

	elems, _ := expected.Elements()
	for _, elem := range elems {
		key := elem.Key()
		opt := elem.Value()

		switch key {
		case "replicaSet":
			rs := opt.StringValue()
			assert.Equal(mt, rs, cs.ReplicaSet, "expected replicaSet value %v, got %v", rs, cs.ReplicaSet)
		case "ssl":
			ssl := opt.Boolean()
			assert.Equal(mt, ssl, cs.SSL, "expected ssl value %v, got %v", ssl, cs.SSL)
		case "authSource":
			source := opt.StringValue()
			assert.Equal(mt, source, cs.AuthSource, "expected auth source value %v, got %v", source, cs.AuthSource)
		default:
			mt.Fatalf("unrecognized connstring option %v", key)
		}
	}
}

// Because the Go driver tests can be run either against a server with SSL enabled or without, a
// number of configurations have to be checked to ensure that the SRV tests are run properly.
//
// First, the "ssl" option in the JSON test description has to be checked. If this option is not
// present, we assume that the test will assert an error, so we proceed with the test as normal.
// If the option is false, then we skip the test if the server is running with SSL enabled.
// If the option is true, then we skip the test if the server is running without SSL enabled; if
// the server is running with SSL enabled, then we manually set the necessary SSL options in the
// connection string.
func setSSLSettings(mt *mtest.T, cs *connstring.ConnString, test seedlistTest) {
	ssl, err := test.Options.LookupErr("ssl")
	if err != nil {
		// No "ssl" option is specified
		return
	}
	testCaseExpectsSSL := ssl.Boolean()
	envSSL := os.Getenv("SSL") == "ssl"

	// Skip non-SSL tests if the server is running with SSL.
	if !testCaseExpectsSSL && envSSL {
		mt.Skip("skipping test that does not expect ssl in an ssl environment")
	}

	// Skip SSL tests if the server is running without SSL.
	if testCaseExpectsSSL && !envSSL {
		mt.Skip("skipping test that expectes ssl in a non-ssl environment")
	}

	// If SSL tests are running, set the CA file.
	if testCaseExpectsSSL && envSSL {
		cs.SSLInsecure = true
	}
}

func getServerByAddress(address string, topo *topology.Topology) (description.Server, error) {
	selectByName := description.ServerSelectorFunc(func(_ description.Topology, servers []description.Server) ([]description.Server, error) {
		for _, s := range servers {
			if s.Addr.String() == address {
				return []description.Server{s}, nil
			}
		}
		return []description.Server{}, nil
	})

	selectedServer, err := topo.SelectServerLegacy(context.Background(), selectByName)
	if err != nil {
		return description.Server{}, err
	}
	return selectedServer.Server.Description(), nil
}
