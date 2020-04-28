// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/address"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
)

const testTimeout = 2 * time.Second

func noerr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		t.FailNow()
	}
}

func compareErrors(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	if err1.Error() != err2.Error() {
		return false
	}

	return true
}

func TestServerSelection(t *testing.T) {
	var selectFirst description.ServerSelectorFunc = func(_ description.Topology, candidates []description.Server) ([]description.Server, error) {
		if len(candidates) == 0 {
			return []description.Server{}, nil
		}
		return candidates[0:1], nil
	}
	var selectNone description.ServerSelectorFunc = func(description.Topology, []description.Server) ([]description.Server, error) {
		return []description.Server{}, nil
	}
	var errSelectionError = errors.New("encountered an error in the selector")
	var selectError description.ServerSelectorFunc = func(description.Topology, []description.Server) ([]description.Server, error) {
		return nil, errSelectionError
	}

	t.Run("Success", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		desc := description.Topology{
			Servers: []description.Server{
				{Addr: address.Address("one"), Kind: description.Standalone},
				{Addr: address.Address("two"), Kind: description.Standalone},
				{Addr: address.Address("three"), Kind: description.Standalone},
			},
		}
		subCh := make(chan description.Topology, 1)
		subCh <- desc

		state := newServerSelectionState(selectFirst, nil)
		srvs, err := topo.selectServerFromSubscription(context.Background(), subCh, state)
		noerr(t, err)
		if len(srvs) != 1 {
			t.Errorf("Incorrect number of descriptions returned. got %d; want %d", len(srvs), 1)
		}
		if srvs[0].Addr != desc.Servers[0].Addr {
			t.Errorf("Incorrect sever selected. got %s; want %s", srvs[0].Addr, desc.Servers[0].Addr)
		}
	})
	t.Run("Updated", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		desc := description.Topology{Servers: []description.Server{}}
		subCh := make(chan description.Topology, 1)
		subCh <- desc

		resp := make(chan []description.Server)
		go func() {
			state := newServerSelectionState(selectFirst, nil)
			srvs, err := topo.selectServerFromSubscription(context.Background(), subCh, state)
			noerr(t, err)
			resp <- srvs
		}()

		desc = description.Topology{
			Servers: []description.Server{
				{Addr: address.Address("one"), Kind: description.Standalone},
				{Addr: address.Address("two"), Kind: description.Standalone},
				{Addr: address.Address("three"), Kind: description.Standalone},
			},
		}
		select {
		case subCh <- desc:
		case <-time.After(100 * time.Millisecond):
			t.Error("Timed out while trying to send topology description")
		}

		var srvs []description.Server
		select {
		case srvs = <-resp:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timed out while trying to retrieve selected servers")
		}

		if len(srvs) != 1 {
			t.Errorf("Incorrect number of descriptions returned. got %d; want %d", len(srvs), 1)
		}
		if srvs[0].Addr != desc.Servers[0].Addr {
			t.Errorf("Incorrect sever selected. got %s; want %s", srvs[0].Addr, desc.Servers[0].Addr)
		}
	})
	t.Run("Cancel", func(t *testing.T) {
		desc := description.Topology{
			Servers: []description.Server{
				{Addr: address.Address("one"), Kind: description.Standalone},
				{Addr: address.Address("two"), Kind: description.Standalone},
				{Addr: address.Address("three"), Kind: description.Standalone},
			},
		}
		topo, err := New()
		noerr(t, err)
		subCh := make(chan description.Topology, 1)
		subCh <- desc
		resp := make(chan error)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			state := newServerSelectionState(selectNone, nil)
			_, err := topo.selectServerFromSubscription(ctx, subCh, state)
			resp <- err
		}()

		select {
		case err := <-resp:
			t.Errorf("Received error from server selection too soon: %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		cancel()

		select {
		case err = <-resp:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timed out while trying to retrieve selected servers")
		}

		if err != context.Canceled {
			t.Errorf("Incorrect error received. got %v; want %v", err, context.Canceled)
		}
	})
	t.Run("Timeout", func(t *testing.T) {
		desc := description.Topology{
			Servers: []description.Server{
				{Addr: address.Address("one"), Kind: description.Standalone},
				{Addr: address.Address("two"), Kind: description.Standalone},
				{Addr: address.Address("three"), Kind: description.Standalone},
			},
		}
		topo, err := New()
		noerr(t, err)
		subCh := make(chan description.Topology, 1)
		subCh <- desc
		resp := make(chan error)
		timeout := make(chan time.Time)
		go func() {
			state := newServerSelectionState(selectNone, timeout)
			_, err := topo.selectServerFromSubscription(context.Background(), subCh, state)
			resp <- err
		}()

		select {
		case err := <-resp:
			t.Errorf("Received error from server selection too soon: %v", err)
		case timeout <- time.Now():
		}

		select {
		case err = <-resp:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timed out while trying to retrieve selected servers")
		}

		if err == nil {
			t.Fatalf("did not receive error from server selection")
		}
	})
	t.Run("Error", func(t *testing.T) {
		desc := description.Topology{
			Servers: []description.Server{
				{Addr: address.Address("one"), Kind: description.Standalone},
				{Addr: address.Address("two"), Kind: description.Standalone},
				{Addr: address.Address("three"), Kind: description.Standalone},
			},
		}
		topo, err := New()
		noerr(t, err)
		subCh := make(chan description.Topology, 1)
		subCh <- desc
		resp := make(chan error)
		timeout := make(chan time.Time)
		go func() {
			state := newServerSelectionState(selectError, timeout)
			_, err := topo.selectServerFromSubscription(context.Background(), subCh, state)
			resp <- err
		}()

		select {
		case err = <-resp:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timed out while trying to retrieve selected servers")
		}

		if err == nil {
			t.Fatalf("did not receive error from server selection")
		}
	})
	t.Run("findServer returns topology kind", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		atomic.StoreInt32(&topo.connectionstate, connected)
		srvr, err := ConnectServer(address.Address("one"), func(desc description.Server) { topo.apply(context.Background(), desc) })
		noerr(t, err)
		topo.servers[address.Address("one")] = srvr
		desc := topo.desc.Load().(description.Topology)
		desc.Kind = description.Single
		topo.desc.Store(desc)

		selected := description.Server{Addr: address.Address("one")}

		ss, err := topo.FindServer(selected)
		noerr(t, err)
		if ss.Kind != description.Single {
			t.Errorf("findServer does not properly set the topology description kind. got %v; want %v", ss.Kind, description.Single)
		}
	})
	t.Run("Update on not master error", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		topo.cfg.cs.HeartbeatInterval = time.Minute
		atomic.StoreInt32(&topo.connectionstate, connected)

		addr1 := address.Address("one")
		addr2 := address.Address("two")
		addr3 := address.Address("three")
		desc := description.Topology{
			Servers: []description.Server{
				{Addr: addr1, Kind: description.RSPrimary},
				{Addr: addr2, Kind: description.RSSecondary},
				{Addr: addr3, Kind: description.RSSecondary},
			},
		}

		// manually add the servers to the topology
		for _, srv := range desc.Servers {
			s, err := ConnectServer(srv.Addr, func(desc description.Server) { topo.apply(context.Background(), desc) })
			noerr(t, err)
			topo.servers[srv.Addr] = s
		}

		// Send updated description
		desc = description.Topology{
			Servers: []description.Server{
				{Addr: addr1, Kind: description.RSSecondary},
				{Addr: addr2, Kind: description.RSPrimary},
				{Addr: addr3, Kind: description.RSSecondary},
			},
		}

		subCh := make(chan description.Topology, 1)
		subCh <- desc

		// send a not master error to the server forcing an update
		serv, err := topo.FindServer(desc.Servers[0])
		noerr(t, err)
		atomic.StoreInt32(&serv.connectionstate, connected)
		serv.ProcessError(driver.Error{Message: "not master"})

		resp := make(chan []description.Server)

		go func() {
			// server selection should discover the new topology
			state := newServerSelectionState(description.WriteSelector(), nil)
			srvs, err := topo.selectServerFromSubscription(context.Background(), subCh, state)
			noerr(t, err)
			resp <- srvs
		}()

		var srvs []description.Server
		select {
		case srvs = <-resp:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timed out while trying to retrieve selected servers")
		}

		if len(srvs) != 1 {
			t.Errorf("Incorrect number of descriptions returned. got %d; want %d", len(srvs), 1)
		}
		if srvs[0].Addr != desc.Servers[1].Addr {
			t.Errorf("Incorrect sever selected. got %s; want %s", srvs[0].Addr, desc.Servers[1].Addr)
		}
	})
	t.Run("no subscription in fast path", func(t *testing.T) {
		// assert that a subscription is not created if there is a server available
		topo, err := New()
		noerr(t, err)
		topo.cfg.cs.HeartbeatInterval = time.Minute
		atomic.StoreInt32(&topo.connectionstate, connected)

		primaryAddr := address.Address("one")
		desc := description.Topology{
			Servers: []description.Server{
				{Addr: primaryAddr, Kind: description.RSPrimary},
			},
		}
		topo.desc.Store(desc)
		for _, srv := range desc.Servers {
			s, err := ConnectServer(srv.Addr, func(desc description.Server) { topo.apply(context.Background(), desc) })
			noerr(t, err)
			topo.servers[srv.Addr] = s
		}

		// manually close subscriptions so calls to Subscribe will error
		topo.subscriptionsClosed = true
		selectedServer, err := topo.SelectServer(context.Background(), description.WriteSelector())
		noerr(t, err)
		selectedAddr := selectedServer.(*SelectedServer).address
		assert.Equal(t, primaryAddr, selectedAddr, "expected address %v, got %v", primaryAddr, selectedAddr)
	})
}

func TestSessionTimeout(t *testing.T) {
	t.Run("UpdateSessionTimeout", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		topo.servers["foo"] = nil

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		desc := description.Server{
			Addr:                  "foo",
			Kind:                  description.RSPrimary,
			SessionTimeoutMinutes: 30,
		}
		topo.apply(ctx, desc)

		currDesc := topo.desc.Load().(description.Topology)
		if currDesc.SessionTimeoutMinutes != 30 {
			t.Errorf("session timeout minutes mismatch. got: %d. expected: 30", currDesc.SessionTimeoutMinutes)
		}
	})
	t.Run("MultipleUpdates", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		topo.servers["foo"] = nil
		topo.servers["bar"] = nil

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		desc1 := description.Server{
			Addr:                  "foo",
			Kind:                  description.RSPrimary,
			SessionTimeoutMinutes: 30,
		}
		// should update because new timeout is lower
		desc2 := description.Server{
			Addr:                  "bar",
			Kind:                  description.RSPrimary,
			SessionTimeoutMinutes: 20,
		}
		topo.apply(ctx, desc1)
		topo.apply(ctx, desc2)

		currDesc := topo.Description()
		if currDesc.SessionTimeoutMinutes != 20 {
			t.Errorf("session timeout minutes mismatch. got: %d. expected: 20", currDesc.SessionTimeoutMinutes)
		}
	})
	t.Run("NoUpdate", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		topo.servers["foo"] = nil
		topo.servers["bar"] = nil

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		desc1 := description.Server{
			Addr:                  "foo",
			Kind:                  description.RSPrimary,
			SessionTimeoutMinutes: 20,
		}
		// should not update because new timeout is higher
		desc2 := description.Server{
			Addr:                  "bar",
			Kind:                  description.RSPrimary,
			SessionTimeoutMinutes: 30,
		}
		topo.apply(ctx, desc1)
		topo.apply(ctx, desc2)

		currDesc := topo.desc.Load().(description.Topology)
		if currDesc.SessionTimeoutMinutes != 20 {
			t.Errorf("session timeout minutes mismatch. got: %d. expected: 20", currDesc.SessionTimeoutMinutes)
		}
	})
	t.Run("TimeoutDataBearing", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		topo.servers["foo"] = nil
		topo.servers["bar"] = nil

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		desc1 := description.Server{
			Addr:                  "foo",
			Kind:                  description.RSPrimary,
			SessionTimeoutMinutes: 20,
		}
		// should not update because not a data bearing server
		desc2 := description.Server{
			Addr:                  "bar",
			Kind:                  description.Unknown,
			SessionTimeoutMinutes: 10,
		}
		topo.apply(ctx, desc1)
		topo.apply(ctx, desc2)

		currDesc := topo.desc.Load().(description.Topology)
		if currDesc.SessionTimeoutMinutes != 20 {
			t.Errorf("session timeout minutes mismatch. got: %d. expected: 20", currDesc.SessionTimeoutMinutes)
		}
	})
	t.Run("MixedSessionSupport", func(t *testing.T) {
		topo, err := New()
		noerr(t, err)
		topo.fsm.Kind = description.ReplicaSetWithPrimary
		topo.servers["one"] = nil
		topo.servers["two"] = nil
		topo.servers["three"] = nil
		topo.fsm.Servers = []description.Server{
			{Addr: address.Address("one"), Kind: description.RSPrimary, SessionTimeoutMinutes: 20},
			{Addr: address.Address("two"), Kind: description.RSSecondary}, // does not support sessions
		}

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		desc := description.Server{
			Addr: address.Address("three"), Kind: description.RSSecondary, SessionTimeoutMinutes: 30}
		topo.apply(ctx, desc)

		currDesc := topo.desc.Load().(description.Topology)
		if currDesc.SessionTimeoutMinutes != 0 {
			t.Errorf("session timeout minutes mismatch. got: %d. expected: 0", currDesc.SessionTimeoutMinutes)
		}
	})
}

func TestMinPoolSize(t *testing.T) {
	connStr := connstring.ConnString{
		Hosts:          []string{"localhost:27017"},
		MinPoolSize:    10,
		MinPoolSizeSet: true,
	}
	topo, err := New(WithConnString(func(connstring.ConnString) connstring.ConnString { return connStr }))
	if err != nil {
		t.Errorf("topology.New shouldn't error. got: %v", err)
	}
	err = topo.Connect()
	if err != nil {
		t.Errorf("topology.Connect shouldn't error. got: %v", err)
	}
}

func TestTopology_String_Race(t *testing.T) {
	ch := make(chan bool)
	topo := &Topology{
		servers: make(map[address.Address]*Server),
	}

	go func() {
		topo.serversLock.Lock()
		srv := &Server{}
		srv.desc.Store(description.Server{})
		topo.servers[address.Address("127.0.0.1:27017")] = srv
		topo.serversLock.Unlock()
		ch <- true
	}()

	go func() {
		topo.String()
		ch <- true
	}()

	<-ch
	<-ch
}

func TestTopologyConstruction(t *testing.T) {
	t.Run("construct with URI", func(t *testing.T) {
		testCases := []struct {
			name            string
			uri             string
			pollingRequired bool
		}{
			{"normal", "mongodb://localhost:27017", false},
			{"srv", "mongodb+srv://localhost:27017", true},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				topo, err := New(
					WithURI(func(string) string { return tc.uri }),
				)
				assert.Nil(t, err, "topology.New error: %v", err)

				assert.Equal(t, tc.uri, topo.cfg.uri, "expected topology URI to be %v, got %v", tc.uri, topo.cfg.uri)
				assert.Equal(t, tc.pollingRequired, topo.pollingRequired,
					"expected topo.pollingRequired to be %v, got %v", tc.pollingRequired, topo.pollingRequired)
			})
		}
	})
}
