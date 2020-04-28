// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
)

// Test case for all server selection rtt spec tests.
func TestServerSelectionRTTSpec(t *testing.T) {

	type testCase struct {
		AvgRttMs  json.Number `json:"avg_rtt_ms"`
		NewRttMs  float64     `json:"new_rtt_ms"`
		NewAvgRtt float64     `json:"new_avg_rtt"`
	}

	const testsDir string = "../../../../data/server-selection/rtt"

	for _, file := range testhelpers.FindJSONFilesInDir(t, testsDir) {
		func(t *testing.T, filename string) {
			filepath := path.Join(testsDir, filename)
			content, err := ioutil.ReadFile(filepath)
			require.NoError(t, err)

			// Remove ".json" from filename.
			testName := filename[:len(filename)-5]

			t.Run(testName, func(t *testing.T) {
				var test testCase
				require.NoError(t, json.Unmarshal(content, &test))

				var server Server

				if test.AvgRttMs != "NULL" {
					avg, err := test.AvgRttMs.Float64()
					require.NoError(t, err)

					server.averageRTT = time.Duration(avg * float64(time.Millisecond))
					server.averageRTTSet = true
				}

				server.updateAverageRTT(time.Duration(test.NewRttMs * float64(time.Millisecond)))
				require.Equal(t, server.averageRTT, time.Duration(test.NewAvgRtt*float64(time.Millisecond)))
			})
		}(t, file)
	}
}
