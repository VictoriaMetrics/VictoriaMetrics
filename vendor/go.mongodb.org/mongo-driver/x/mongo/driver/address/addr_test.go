// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package address

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddress_String(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{"a", "a:27017"},
		{"A", "a:27017"},
		{"A:27017", "a:27017"},
		{"a:27017", "a:27017"},
		{"a.sock", "a.sock"},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			require.Equal(t, Address(test.in).String(), test.expected)
		})
	}
}

func TestAddress_Canonicalize(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{"a", "a:27017"},
		{"A", "a:27017"},
		{"A:27017", "a:27017"},
		{"a:27017", "a:27017"},
		{"a.sock", "a.sock"},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			require.Equal(t, Address(test.in).Canonicalize(), Address(test.expected))
		})
	}
}
