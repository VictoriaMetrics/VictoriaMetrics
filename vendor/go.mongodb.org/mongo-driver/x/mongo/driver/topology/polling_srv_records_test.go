// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/x/mongo/driver/address"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/dns"
)

type mockResolver struct {
	recordsToAdd    []*net.SRV
	recordsToRemove []*net.SRV
	lookupFail      bool
	lookupTimeout   bool
	ranLookup       int32
	fail            int32
}

func newMockResolver(recordsToAdd []*net.SRV, recordsToRemove []*net.SRV, lookupFail bool, lookupTimeout bool) mockResolver {
	res := mockResolver{
		recordsToAdd:    recordsToAdd,
		recordsToRemove: recordsToRemove,
		lookupFail:      lookupFail,
		lookupTimeout:   lookupTimeout,
	}
	return res
}

func (r *mockResolver) LookupSRV(service, proto, name string) (string, []*net.SRV, error) {
	atomic.AddInt32(&r.ranLookup, 1)
	if r.lookupFail {
		return "", nil, &net.DNSError{}
	}
	if r.lookupTimeout {
		return "", nil, &net.DNSError{IsTimeout: true}
	}
	str, addresses, err := net.LookupSRV("mongodb", "tcp", name)
	if err != nil {
		return str, addresses, err
	}
	if r.fail > 0 {
		r.fail--
		return str, nil, nil
	}

	// Add/remove records to mimic changing the DNS records.
	if r.recordsToAdd != nil {
		addresses = append(addresses, r.recordsToAdd...)
	}
	if r.recordsToRemove != nil {
		for _, removeAddr := range r.recordsToRemove {
			for j, addr := range addresses {
				if removeAddr.Target == addr.Target && removeAddr.Port == addr.Port {
					addresses = append(addresses[:j], addresses[j+1:]...)
				}
			}
		}
	}
	return str, addresses, err
}

func (r *mockResolver) LookupTXT(name string) ([]string, error) { return nil, nil }

var srvPollingTests = []struct {
	name            string
	recordsToAdd    []*net.SRV
	recordsToRemove []*net.SRV
	lookupFail      bool
	lookupTimeout   bool
	expectedHosts   []string
	heartbeatTime   bool
}{
	{"Add new record", []*net.SRV{{"localhost.test.build.10gen.cc.", 27019, 0, 0}}, nil, false, false, []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018", "localhost.test.build.10gen.cc:27019"}, false},
	{"Remove existing record", nil, []*net.SRV{{"localhost.test.build.10gen.cc.", 27018, 0, 0}}, false, false, []string{"localhost.test.build.10gen.cc:27017"}, false},
	{"Replace existing record", []*net.SRV{{"localhost.test.build.10gen.cc.", 27019, 0, 0}}, []*net.SRV{{"localhost.test.build.10gen.cc.", 27018, 0, 0}}, false, false, []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27019"}, false},
	{"Replace both with one new", []*net.SRV{{"localhost.test.build.10gen.cc.", 27019, 0, 0}}, []*net.SRV{{"localhost.test.build.10gen.cc.", 27017, 0, 0}, {"localhost.test.build.10gen.cc.", 27018, 0, 0}}, false, false, []string{"localhost.test.build.10gen.cc:27019"}, false},
	{"Replace both with two new", []*net.SRV{{"localhost.test.build.10gen.cc.", 27019, 0, 0}, {"localhost.test.build.10gen.cc.", 27020, 0, 0}}, []*net.SRV{{"localhost.test.build.10gen.cc.", 27017, 0, 0}, {"localhost.test.build.10gen.cc.", 27018, 0, 0}}, false, false, []string{"localhost.test.build.10gen.cc:27019", "localhost.test.build.10gen.cc:27020"}, false},
	{"DNS lookup timeout", nil, nil, false, true, []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018"}, true},
	{"DNS lookup failure", nil, nil, true, false, []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018"}, true},
	{"Remove all", nil, []*net.SRV{{"localhost.test.build.10gen.cc.", 27017, 0, 0}, {"localhost.test.build.10gen.cc.", 27018, 0, 0}}, false, false, []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018"}, true},
}

type serverSorter []description.Server

func (ss serverSorter) Len() int      { return len(ss) }
func (ss serverSorter) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }
func (ss serverSorter) Less(i, j int) bool {
	return strings.Compare(ss[i].Addr.String(), ss[j].Addr.String()) < 0
}

func compareHosts(t *testing.T, received []description.Server, expected []string) {
	if len(received) != len(expected) {
		t.Fatalf("Number of hosts in topology does not match expected value. Got %v; want %v.", len(received), len(expected))
	}

	// Take a copy of servers so we don't risk a data race similar to GODRIVER-1301.
	servers := make([]description.Server, len(received))
	copy(servers, received)
	actual := serverSorter(servers)
	sort.Sort(actual)
	sort.Strings(expected)

	for i := range servers {
		if servers[i].Addr.String() != expected[i] {
			t.Errorf("Hosts in topology differ from expected values. Got %v; want %v.",
				servers[i].Addr.String(), expected[i])
		}
	}
}

func TestPollingSRVRecordsSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	for _, tt := range srvPollingTests {
		t.Run(tt.name, func(t *testing.T) {
			cs, err := connstring.Parse("mongodb+srv://test1.test.build.10gen.cc/?heartbeatFrequencyMS=100")
			require.NoError(t, err, "Problem parsing the uri: %v", err)
			topo, err := New(
				WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }),
				WithURI(func(string) string { return cs.Original }),
			)
			require.NoError(t, err, "Could not create the topology: %v", err)
			mockRes := newMockResolver(tt.recordsToAdd, tt.recordsToRemove, tt.lookupFail, tt.lookupTimeout)
			topo.dnsResolver = &dns.Resolver{mockRes.LookupSRV, mockRes.LookupTXT}
			topo.rescanSRVInterval = time.Millisecond * 5
			err = topo.Connect()
			require.NoError(t, err, "Could not Connect to the topology: %v", err)

			// wait for description to update
			sub, err := topo.Subscribe()
			require.NoError(t, err, "Couldn't subscribe: %v", err)
			var desc description.Topology
			for atomic.LoadInt32(&mockRes.ranLookup) < 2 {
				desc = <-sub.Updates
			}

			require.True(t, tt.heartbeatTime == topo.pollHeartbeatTime.Load().(bool), "Not polling on correct intervals")
			compareHosts(t, desc.Servers, tt.expectedHosts)
			for _, e := range tt.expectedHosts {
				addr := address.Address(e).Canonicalize()
				if _, ok := topo.servers[addr]; !ok {
					t.Errorf("Topology server list did not contain expected value %v", e)
				}
			}
			_ = topo.Disconnect(context.Background())
		})
	}
}

func TestPollSRVRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Run("Not unknown or sharded topology", func(t *testing.T) {
		cs, err := connstring.Parse("mongodb+srv://test1.test.build.10gen.cc/?heartbeatFrequencyMS=100")
		require.NoError(t, err, "Problem parsing the uri: %v", err)
		topo, err := New(
			WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }),
			WithURI(func(string) string { return cs.Original }),
		)
		require.NoError(t, err, "Could not create the topology: %v", err)
		mockRes := newMockResolver(nil, nil, false, false)
		topo.dnsResolver = &dns.Resolver{mockRes.LookupSRV, mockRes.LookupTXT}
		topo.rescanSRVInterval = time.Millisecond * 5
		err = topo.Connect()
		require.NoError(t, err, "Could not Connect to the topology: %v", err)
		topo.serversLock.Lock()
		topo.fsm.Kind = description.Single
		topo.desc.Store(description.Topology{
			Kind:                  topo.fsm.Kind,
			Servers:               topo.fsm.Servers,
			SessionTimeoutMinutes: topo.fsm.SessionTimeoutMinutes,
		})
		topo.serversLock.Unlock()

		// wait for description to update
		sub, err := topo.Subscribe()
		if err != nil {
			t.Fatalf("Couldn't subscribe: %v", err)
		}

		for i := 0; i < 4; i++ {
			<-sub.Updates
		}
		require.False(t, atomic.LoadInt32(&mockRes.ranLookup) > 0)

		actualHosts := topo.Description().Servers
		expectedHosts := []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018"}
		compareHosts(t, actualHosts, expectedHosts)
		for _, e := range expectedHosts {
			addr := address.Address(e).Canonicalize()
			if _, ok := topo.servers[addr]; !ok {
				t.Errorf("Topology server list did not contain expected value %v", e)
			}
		}
		_ = topo.Disconnect(context.Background())

	})
	t.Run("Failed Hostname Verification", func(t *testing.T) {
		cs, err := connstring.Parse("mongodb+srv://test1.test.build.10gen.cc/?heartbeatFrequencyMS=100")
		require.NoError(t, err, "Problem parsing the uri: %v", err)
		topo, err := New(
			WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }),
			WithURI(func(string) string { return cs.Original }),
		)
		require.NoError(t, err, "Could not create the topology: %v", err)
		mockRes := newMockResolver([]*net.SRV{{"blah.bleh", 27019, 0, 0}, {"localhost.test.build.10gen.cc.", 27020, 0, 0}}, nil, false, false)
		topo.dnsResolver = &dns.Resolver{mockRes.LookupSRV, mockRes.LookupTXT}
		topo.rescanSRVInterval = time.Millisecond * 5
		err = topo.Connect()
		require.NoError(t, err, "Could not Connect to the topology: %v", err)

		// wait for description to update
		sub, err := topo.Subscribe()
		require.NoError(t, err, "Couldn't subscribe: %v", err)
		var desc description.Topology
		for atomic.LoadInt32(&mockRes.ranLookup) < 2 {
			desc = <-sub.Updates
		}

		require.False(t, topo.pollHeartbeatTime.Load().(bool))
		expectedHosts := []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018", "localhost.test.build.10gen.cc:27020"}
		compareHosts(t, desc.Servers, expectedHosts)
		for _, e := range expectedHosts {
			addr := address.Address(e).Canonicalize()
			if _, ok := topo.servers[addr]; !ok {
				t.Errorf("Topology server list did not contain expected value %v", e)
			}
		}
		_ = topo.Disconnect(context.Background())

	})
	t.Run("Return to polling time", func(t *testing.T) {
		cs, err := connstring.Parse("mongodb+srv://test1.test.build.10gen.cc/?heartbeatFrequencyMS=100")
		require.NoError(t, err, "Problem parsing the uri: %v", err)
		topo, err := New(
			WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }),
			WithURI(func(string) string { return cs.Original }),
		)
		require.NoError(t, err, "Could not create the topology: %v", err)
		mockRes := newMockResolver(nil, nil, false, false)
		mockRes.fail = 1
		topo.dnsResolver = &dns.Resolver{mockRes.LookupSRV, mockRes.LookupTXT}
		topo.rescanSRVInterval = time.Millisecond * 5
		err = topo.Connect()
		require.NoError(t, err, "Could not Connect to the topology: %v", err)

		// wait for description to update
		sub, err := topo.Subscribe()
		require.NoError(t, err, "Couldn't subscribe: %v", err)
		var desc description.Topology
		for atomic.LoadInt32(&mockRes.ranLookup) < 3 {
			desc = <-sub.Updates
		}

		require.False(t, topo.pollHeartbeatTime.Load().(bool))
		expectedHosts := []string{"localhost.test.build.10gen.cc:27017", "localhost.test.build.10gen.cc:27018"}
		compareHosts(t, desc.Servers, expectedHosts)
		for _, e := range expectedHosts {
			addr := address.Address(e).Canonicalize()
			if _, ok := topo.servers[addr]; !ok {
				t.Errorf("Topology server list did not contain expected value %v", e)
			}
		}
		_ = topo.Disconnect(context.Background())
	})
}
