// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package primitive

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The same interface as bsoncodec.Zeroer implemented for tests.
type zeroer interface {
	IsZero() bool
}

func TestTimestampCompare(t *testing.T) {
	testcases := []struct {
		name     string
		tp       Timestamp
		tp2      Timestamp
		expected int
	}{
		{"equal", Timestamp{T: 12345, I: 67890}, Timestamp{T: 12345, I: 67890}, 0},
		{"T greater than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 2345, I: 67890}, 1},
		{"I greater than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 12345, I: 7890}, 1},
		{"T less than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 112345, I: 67890}, -1},
		{"I less than", Timestamp{T: 12345, I: 67890}, Timestamp{T: 12345, I: 167890}, -1},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareTimestamp(tc.tp, tc.tp2)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestPrimitiveIsZero(t *testing.T) {
	testcases := []struct {
		name    string
		zero    zeroer
		nonzero zeroer
	}{
		{"binary", Binary{}, Binary{Data: []byte{0x01, 0x02, 0x03}, Subtype: 0xFF}},
		{"regex", Regex{}, Regex{Pattern: "foo", Options: "bar"}},
		{"dbPointer", DBPointer{}, DBPointer{DB: "foobar", Pointer: ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}}},
		{"timestamp", Timestamp{}, Timestamp{T: 12345, I: 67890}},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.zero.IsZero())
			require.False(t, tc.nonzero.IsZero())
		})
	}
}

func TestRegexCompare(t *testing.T) {
	testcases := []struct {
		name string
		r1   Regex
		r2   Regex
		eq   bool
	}{
		{"equal", Regex{Pattern: "foo1", Options: "bar1"}, Regex{Pattern: "foo1", Options: "bar1"}, true},
		{"not equal", Regex{Pattern: "foo1", Options: "bar1"}, Regex{Pattern: "foo2", Options: "bar2"}, false},
		{"not equal", Regex{Pattern: "foo1", Options: "bar1"}, Regex{Pattern: "foo1", Options: "bar2"}, false},
		{"not equal", Regex{Pattern: "foo1", Options: "bar1"}, Regex{Pattern: "foo2", Options: "bar1"}, false},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.r1.Equal(tc.r2) == tc.eq)
		})
	}
}
