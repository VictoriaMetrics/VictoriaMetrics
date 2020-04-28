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

const maxStalenessTestsDir = "../../../../data/max-staleness"

// Test case for all max staleness spec tests.
func TestMaxStalenessSpec(t *testing.T) {
	for _, topology := range [...]string{
		"ReplicaSetNoPrimary",
		"ReplicaSetWithPrimary",
		"Sharded",
		"Single",
		"Unknown",
	} {
		for _, file := range testhelpers.FindJSONFilesInDir(t,
			path.Join(maxStalenessTestsDir, topology)) {

			runTest(t, maxStalenessTestsDir, topology, file)
		}
	}
}
