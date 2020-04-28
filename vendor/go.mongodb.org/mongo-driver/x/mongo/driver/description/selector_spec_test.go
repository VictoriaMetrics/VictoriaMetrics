// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package description

import (
	"path"
	"testing"

	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
)

const selectorTestsDir = "../../../../data/server-selection/server_selection"

// Test case for all SDAM spec tests.
func TestServerSelectionSpec(t *testing.T) {
	for _, topology := range [...]string{
		"ReplicaSetNoPrimary",
		"ReplicaSetWithPrimary",
		"Sharded",
		"Single",
		"Unknown",
	} {
		for _, subdir := range [...]string{"read", "write"} {
			subdirPath := path.Join(topology, subdir)

			for _, file := range testhelpers.FindJSONFilesInDir(t,
				path.Join(selectorTestsDir, subdirPath)) {

				runTest(t, selectorTestsDir, subdirPath, file)
			}
		}
	}
}
