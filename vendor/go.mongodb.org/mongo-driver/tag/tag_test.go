// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package tag

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTagSets_NewTagSet(t *testing.T) {
	t.Parallel()

	ts := Set{Tag{Name: "a", Value: "1"}}

	require.True(t, ts.Contains("a", "1"))
	require.False(t, ts.Contains("1", "a"))
	require.False(t, ts.Contains("A", "1"))
	require.False(t, ts.Contains("a", "10"))
}

func TestTagSets_NewTagSetFromMap(t *testing.T) {
	t.Parallel()

	ts := NewTagSetFromMap(map[string]string{"a": "1"})

	require.True(t, ts.Contains("a", "1"))
	require.False(t, ts.Contains("1", "a"))
	require.False(t, ts.Contains("A", "1"))
	require.False(t, ts.Contains("a", "10"))
}

func TestTagSets_NewTagSetsFromMaps(t *testing.T) {
	t.Parallel()

	tss := NewTagSetsFromMaps([]map[string]string{{"a": "1"}, {"b": "1"}})

	require.Len(t, tss, 2)

	ts := tss[0]
	require.True(t, ts.Contains("a", "1"))
	require.False(t, ts.Contains("1", "a"))
	require.False(t, ts.Contains("A", "1"))
	require.False(t, ts.Contains("a", "10"))

	ts = tss[1]
	require.True(t, ts.Contains("b", "1"))
	require.False(t, ts.Contains("1", "b"))
	require.False(t, ts.Contains("B", "1"))
	require.False(t, ts.Contains("b", "10"))
}

func TestTagSets_ContainsAll(t *testing.T) {
	t.Parallel()

	ts := Set{
		Tag{Name: "a", Value: "1"},
		Tag{Name: "b", Value: "2"},
	}

	test := Set{Tag{Name: "a", Value: "1"}}
	require.True(t, ts.ContainsAll(test))
	test = Set{Tag{Name: "a", Value: "1"}, Tag{Name: "b", Value: "2"}}
	require.True(t, ts.ContainsAll(test))
	test = Set{Tag{Name: "a", Value: "1"}, Tag{Name: "b", Value: "2"}}
	require.True(t, ts.ContainsAll(test))

	test = Set{Tag{Name: "a", Value: "2"}, Tag{Name: "b", Value: "1"}}
	require.False(t, ts.ContainsAll(test))
	test = Set{Tag{Name: "a", Value: "1"}, Tag{Name: "b", Value: "1"}}
	require.False(t, ts.ContainsAll(test))
	test = Set{Tag{Name: "a", Value: "2"}, Tag{Name: "b", Value: "2"}}
	require.False(t, ts.ContainsAll(test))
}
