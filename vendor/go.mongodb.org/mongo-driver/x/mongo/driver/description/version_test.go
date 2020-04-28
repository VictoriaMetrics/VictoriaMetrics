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

func TestVersion_AtLeast(t *testing.T) {
	t.Parallel()

	subject := Version{
		Parts: []uint8{3, 4, 0},
	}

	tests := []struct {
		version  Version
		expected bool
	}{
		{Version{Parts: []uint8{1, 0, 0}}, true},
		{Version{Parts: []uint8{3, 0, 0}}, true},
		{Version{Parts: []uint8{3, 4}}, true},
		{Version{Parts: []uint8{3, 4, 0}}, true},
		{Version{Parts: []uint8{3, 4, 1}}, false},
		{Version{Parts: []uint8{3, 4, 1, 0}}, false},
		{Version{Parts: []uint8{3, 4, 1, 1}}, false},
		{Version{Parts: []uint8{3, 4, 2}}, false},
		{Version{Parts: []uint8{3, 5}}, false},
		{Version{Parts: []uint8{10, 0, 0}}, false},
	}

	for _, test := range tests {
		t.Run(test.version.String(), func(t *testing.T) {
			actual := subject.AtLeast(test.version.Parts...)
			require.Equal(t, test.expected, actual)
		})

	}
}
