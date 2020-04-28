// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package description

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaxStaleness(t *testing.T) {
	wireVersionSupported := NewVersionRange(0, 5)
	wireVersionUnsupported := NewVersionRange(0, 4)

	tests := []struct {
		version  Version
		wire     *VersionRange
		expected bool
	}{
		{Version{Parts: []uint8{2, 4, 0}}, nil, false},
		{Version{Parts: []uint8{3, 3, 99}}, nil, false},
		{Version{Parts: []uint8{3, 4, 0}}, nil, true},
		{Version{Parts: []uint8{3, 4, 1}}, nil, true},
		{Version{Parts: []uint8{2, 4, 0}}, &wireVersionSupported, false},
		{Version{Parts: []uint8{2, 4, 0}}, &wireVersionUnsupported, false},
		{Version{Parts: []uint8{3, 4, 1}}, &wireVersionSupported, true},
		{Version{Parts: []uint8{3, 4, 1}}, &wireVersionUnsupported, false},
	}

	for _, test := range tests {
		t.Run(test.version.String(), func(t *testing.T) {
			t.Parallel()

			err := MaxStalenessSupported(test.wire)
			if test.expected {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
