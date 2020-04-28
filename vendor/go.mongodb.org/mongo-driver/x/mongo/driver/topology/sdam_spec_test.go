// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/address"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
)

type response struct {
	Host     string
	IsMaster IsMaster
}

type IsMaster struct {
	Arbiters                     []string           `bson:"arbiters,omitempty"`
	ArbiterOnly                  bool               `bson:"arbiterOnly,omitempty"`
	ClusterTime                  bson.Raw           `bson:"$clusterTime,omitempty"`
	Compression                  []string           `bson:"compression,omitempty"`
	ElectionID                   primitive.ObjectID `bson:"electionId,omitempty"`
	Hidden                       bool               `bson:"hidden,omitempty"`
	Hosts                        []string           `bson:"hosts,omitempty"`
	IsMaster                     bool               `bson:"ismaster,omitempty"`
	IsReplicaSet                 bool               `bson:"isreplicaset,omitempty"`
	LastWrite                    *lastWriteDate     `bson:"lastWrite,omitempty"`
	LogicalSessionTimeoutMinutes uint32             `bson:"logicalSessionTimeoutMinutes,omitempty"`
	MaxBSONObjectSize            uint32             `bson:"maxBsonObjectSize,omitempty"`
	MaxMessageSizeBytes          uint32             `bson:"maxMessageSizeBytes,omitempty"`
	MaxWriteBatchSize            uint32             `bson:"maxWriteBatchSize,omitempty"`
	Me                           string             `bson:"me,omitempty"`
	MaxWireVersion               int32              `bson:"maxWireVersion,omitempty"`
	MinWireVersion               int32              `bson:"minWireVersion,omitempty"`
	Msg                          string             `bson:"msg,omitempty"`
	OK                           int32              `bson:"ok"`
	Passives                     []string           `bson:"passives,omitempty"`
	ReadOnly                     bool               `bson:"readOnly,omitempty"`
	SaslSupportedMechs           []string           `bson:"saslSupportedMechs,omitempty"`
	Secondary                    bool               `bson:"secondary,omitempty"`
	SetName                      string             `bson:"setName,omitempty"`
	SetVersion                   uint32             `bson:"setVersion,omitempty"`
	Tags                         map[string]string  `bson:"tags,omitempty"`
}

type lastWriteDate struct {
	LastWriteDate time.Time `bson:"lastWriteDate"`
}

type server struct {
	Type    string
	SetName string
}

type outcome struct {
	Servers      map[string]server
	TopologyType string
	SetName      string
	Compatible   *bool
}

type phase struct {
	Responses []response
	Outcome   outcome
}

type testCase struct {
	Description string
	URI         string
	Phases      []phase
}

const testsDir string = "../../../../data/server-discovery-and-monitoring/"

func (r *response) UnmarshalJSON(buf []byte) error {
	tmp := []interface{}{&r.Host, &r.IsMaster}
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}

	if len(tmp) != 2 {
		return fmt.Errorf("'response' JSON array must have exactly two elements")
	}

	return nil
}

func setUpFSM(t *testing.T, uri string) *fsm {
	fsm := newFSM()

	cs, err := connstring.Parse(uri)
	require.NoError(t, err)

	fsm.SetName = cs.ReplicaSet
	if fsm.SetName != "" {
		fsm.Kind = description.ReplicaSetNoPrimary
	} else if len(cs.Hosts) == 1 {
		fsm.Kind = description.Single
	}

	for _, host := range cs.Hosts {
		fsm.Servers = append(fsm.Servers, description.Server{Addr: address.Address(host).Canonicalize()})
	}

	return fsm
}

func applyResponses(f *fsm, responses []response) error {
	for _, response := range responses {
		doc, err := bson.Marshal(response.IsMaster)
		if err != nil {
			return err
		}
		server := description.NewServer(address.Address(response.Host), bsoncore.Document(doc))
		_, err = f.apply(server)

		if err != nil {
			return err
		}
	}

	return nil
}

func runTest(t *testing.T, directory string, filename string) {
	filepath := path.Join(testsDir, directory, filename)
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	// Remove ".json" from filename.
	filename = filename[:len(filename)-5]
	testName := directory + "/" + filename + ":"

	t.Run(testName, func(t *testing.T) {
		var test testCase
		require.NoError(t, json.Unmarshal(content, &test))
		f := setUpFSM(t, test.URI)

		for _, phase := range test.Phases {
			err = applyResponses(f, phase.Responses)
			if phase.Outcome.Compatible == nil || *phase.Outcome.Compatible {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				continue
			}

			require.Equal(t, phase.Outcome.TopologyType, f.Kind.String())
			require.Equal(t, phase.Outcome.SetName, f.SetName)
			require.Equal(t, len(phase.Outcome.Servers), len(f.Servers))

			for addr, server := range phase.Outcome.Servers {
				fsmServer, ok := f.Server(address.Address(addr))
				require.True(t, ok)

				require.Equal(t, address.Address(addr), fsmServer.Addr)
				require.Equal(t, server.SetName, fsmServer.SetName)

				// PossiblePrimary is only relevant to single-threaded drivers.
				if server.Type == "PossiblePrimary" {
					server.Type = "Unknown"
				}

				require.Equal(t, server.Type, fsmServer.Kind.String())
			}
		}
	})
}

// Test case for all SDAM spec tests.
func TestSDAMSpec(t *testing.T) {
	for _, subdir := range []string{"single", "rs", "sharded"} {
		for _, file := range testhelpers.FindJSONFilesInDir(t, path.Join(testsDir, subdir)) {
			runTest(t, subdir, file)
		}
	}
}
