// Copyright (C) MongoDB, Inc. 2019-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package description

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffTopology(t *testing.T) {
	s1 := Server{Addr: "1.0.0.0:27017"}
	s2 := Server{Addr: "2.0.0.0:27017"}
	s3 := Server{Addr: "3.0.0.0:27017"}
	s4 := Server{Addr: "4.0.0.0:27017"}
	s5 := Server{Addr: "5.0.0.0:27017"}
	s6 := Server{Addr: "6.0.0.0:27017"}

	t1 := Topology{
		Servers: []Server{s6, s1, s3, s2},
	}
	t2 := Topology{
		Servers: []Server{s2, s4, s3, s5},
	}

	diff := DiffTopology(t1, t2)

	assert.ElementsMatch(t, []Server{s4, s5}, diff.Added)
	assert.ElementsMatch(t, []Server{s1, s6}, diff.Removed)

	// Ensure that original topology servers were not reordered.
	assert.EqualValues(t, []Server{s6, s1, s3, s2}, t1.Servers)
	assert.EqualValues(t, []Server{s2, s4, s3, s5}, t2.Servers)
}

func TestTopology_DiffHostlist(t *testing.T) {
	h1 := "1.0.0.0:27017"
	h2 := "2.0.0.0:27017"
	h3 := "3.0.0.0:27017"
	h4 := "4.0.0.0:27017"
	h5 := "5.0.0.0:27017"
	h6 := "6.0.0.0:27017"
	s1 := Server{Addr: "1.0.0.0:27017"}
	s2 := Server{Addr: "2.0.0.0:27017"}
	s3 := Server{Addr: "3.0.0.0:27017"}
	s6 := Server{Addr: "6.0.0.0:27017"}

	topo := Topology{
		Servers: []Server{s6, s1, s3, s2},
	}
	hostlist := []string{h2, h4, h3, h5}

	diff := topo.DiffHostlist(hostlist)

	assert.ElementsMatch(t, []string{h4, h5}, diff.Added)
	assert.ElementsMatch(t, []string{h1, h6}, diff.Removed)

	// Ensure that original topology servers and hostlist were not reordered.
	assert.EqualValues(t, []Server{s6, s1, s3, s2}, topo.Servers)
	assert.EqualValues(t, []string{h2, h4, h3, h5}, hostlist)
}
