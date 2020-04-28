// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/address"
	"go.mongodb.org/mongo-driver/x/mongo/driver/auth"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

func makeIsMasterReply() []byte {
	didx, doc := bsoncore.AppendDocumentStart(nil)
	doc = bsoncore.AppendInt32Element(doc, "ok", 1)
	doc, _ = bsoncore.AppendDocumentEnd(doc, didx)
	return drivertest.MakeReply(doc)
}

type channelNetConnDialer struct{}

func (cncd *channelNetConnDialer) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	cnc := &drivertest.ChannelNetConn{
		Written:  make(chan []byte, 1),
		ReadResp: make(chan []byte, 2),
	}
	if err := cnc.AddResponse(makeIsMasterReply()); err != nil {
		return nil, err
	}

	return cnc, nil
}

type testHandshaker struct {
	getDescription  func(context.Context, address.Address, driver.Connection) (description.Server, error)
	finishHandshake func(context.Context, driver.Connection) error
}

// GetDescription implements the Handshaker interface.
func (th *testHandshaker) GetDescription(ctx context.Context, addr address.Address, conn driver.Connection) (description.Server, error) {
	if th.getDescription != nil {
		return th.getDescription(ctx, addr, conn)
	}
	return description.Server{}, nil
}

// FinishHandshake implements the Handshaker interface.
func (th *testHandshaker) FinishHandshake(ctx context.Context, conn driver.Connection) error {
	if th.finishHandshake != nil {
		return th.finishHandshake(ctx, conn)
	}
	return nil
}

var _ driver.Handshaker = &testHandshaker{}

func TestServer(t *testing.T) {
	var serverTestTable = []struct {
		name            string
		connectionError bool
		networkError    bool
		hasDesc         bool
	}{
		{"auth_error", true, false, false},
		{"no_error", false, false, false},
		{"network_error_no_desc", false, true, false},
		{"network_error_desc", false, true, true},
	}

	authErr := ConnectionError{Wrapped: &auth.Error{}}
	netErr := ConnectionError{Wrapped: &net.AddrError{}}
	for _, tt := range serverTestTable {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewServer(
				address.Address("localhost"),
				WithConnectionOptions(func(connOpts ...ConnectionOption) []ConnectionOption {
					return append(connOpts,
						WithHandshaker(func(Handshaker) Handshaker {
							return &testHandshaker{
								finishHandshake: func(context.Context, driver.Connection) error {
									var err error
									if tt.connectionError {
										err = authErr.Wrapped
									}
									return err
								},
							}
						}),
						WithDialer(func(Dialer) Dialer {
							return DialerFunc(func(context.Context, string, string) (net.Conn, error) {
								var err error
								if tt.networkError {
									err = netErr.Wrapped
								}
								return &net.TCPConn{}, err
							})
						}),
					)
				}),
			)
			require.NoError(t, err)

			var desc *description.Server
			descript := s.Description()
			if tt.hasDesc {
				desc = &descript
				require.Nil(t, desc.LastError)
			}
			//err = s.Connect(nil)
			err = s.pool.connect()
			require.NoError(t, err, "unable to connect to pool")
			s.connectionstate = connected

			_, err = s.Connection(context.Background())

			switch {
			case tt.connectionError && !cmp.Equal(err, authErr, cmp.Comparer(compareErrors)):
				t.Errorf("Expected connection error. got %v; want %v", err, authErr)
			case tt.networkError && !cmp.Equal(err, netErr, cmp.Comparer(compareErrors)):
				t.Errorf("Expected network error. got %v; want %v", err, netErr)
			case !tt.connectionError && !tt.networkError && err != nil:
				t.Errorf("Expected error to be nil. got %v; want %v", err, "<nil>")
			}

			if tt.hasDesc {
				require.Equal(t, s.Description().Kind, (description.ServerKind)(description.Unknown))
				require.NotNil(t, s.Description().LastError)
			}

			if (tt.connectionError || tt.networkError) && atomic.LoadUint64(&s.pool.generation) != 1 {
				t.Errorf("Expected pool to be drained once on connection or network error. got %d; want %d", s.pool.generation, 1)
			}
		})
	}

	t.Run("Cannot starve connection request", func(t *testing.T) {
		cleanup := make(chan struct{})
		addr := bootstrapConnections(t, 3, func(nc net.Conn) {
			<-cleanup
			_ = nc.Close()
		})
		d := newdialer(&net.Dialer{})
		s, err := NewServer(address.Address(addr.String()),
			WithConnectionOptions(func(option ...ConnectionOption) []ConnectionOption {
				return []ConnectionOption{WithDialer(func(_ Dialer) Dialer { return d })}
			}),
			WithMaxConnections(func(u uint64) uint64 {
				return 1
			}))
		noerr(t, err)
		s.connectionstate = connected
		err = s.pool.connect()
		noerr(t, err)

		conn, err := s.Connection(context.Background())
		if d.lenopened() != 1 {
			t.Errorf("Should have opened 1 connections, but didn't. got %d; want %d", d.lenopened(), 1)
		}

		var wg sync.WaitGroup

		wg.Add(1)
		ch := make(chan struct{})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			ch <- struct{}{}
			_, err := s.Connection(ctx)
			if err != nil {
				t.Errorf("Should not be able to starve connection request, but got error: %v", err)
			}
			wg.Done()
		}()
		<-ch
		runtime.Gosched()
		err = conn.Close()
		noerr(t, err)
		wg.Wait()
		close(cleanup)
	})
	t.Run("WriteConcernError", func(t *testing.T) {
		s, err := NewServer(address.Address("localhost"))
		require.NoError(t, err)

		var desc *description.Server
		descript := s.Description()
		desc = &descript
		require.Nil(t, desc.LastError)
		s.connectionstate = connected
		s.pool.connected = connected

		wce := driver.WriteConcernError{"", 10107, "not master", []byte{}}
		s.ProcessError(wce)

		// should set ServerDescription to Unknown
		resultDesc := s.Description()
		require.Equal(t, resultDesc.Kind, (description.ServerKind)(description.Unknown))
		require.Equal(t, resultDesc.LastError, wce)

		// pool should be drained
		if s.pool.generation < 1 {
			t.Errorf("Expected pool to be drained once from a write concern error. got %d; want %d", s.pool.generation, 1)
		}
	})
	t.Run("no WriteConcernError", func(t *testing.T) {
		s, err := NewServer(address.Address("localhost"))
		require.NoError(t, err)

		var desc *description.Server
		descript := s.Description()
		desc = &descript
		require.Nil(t, desc.LastError)
		s.connectionstate = connected
		s.pool.connected = connected

		wce := driver.WriteConcernError{}
		s.ProcessError(&wce)

		// should not be a LastError
		require.Nil(t, s.Description().LastError)

		// pool should not be drained
		if s.pool.generation != 0 {
			t.Errorf("Expected pool to not be drained. got %d; want %d", s.pool.generation, 0)
		}
	})
	t.Run("update topology", func(t *testing.T) {
		var updated atomic.Value // bool
		updated.Store(false)
		s, err := ConnectServer(address.Address("localhost"), func(description.Server) { updated.Store(true) })
		require.NoError(t, err)
		s.updateDescription(description.Server{Addr: s.address}, false)
		require.True(t, updated.Load().(bool))
	})
	t.Run("heartbeat", func(t *testing.T) {
		// test that client metadata is sent on handshakes but not heartbeats
		dialer := &channelNetConnDialer{}
		dialerOpt := WithDialer(func(Dialer) Dialer {
			return dialer
		})
		serverOpt := WithConnectionOptions(func(connOpts ...ConnectionOption) []ConnectionOption {
			return append(connOpts, dialerOpt)
		})

		s, err := NewServer(address.Address("localhost:27017"), serverOpt)
		if err != nil {
			t.Fatalf("error from NewServer: %v", err)
		}

		// do a heartbeat with a nil connection so a new one will be dialed
		_, conn := s.heartbeat(nil)
		if conn == nil {
			t.Fatal("no connection dialed")
		}
		channelConn := conn.nc.(*drivertest.ChannelNetConn)
		wm := channelConn.GetWrittenMessage()
		if wm == nil {
			t.Fatal("no wire message written for handshake")
		}
		if !includesMetadata(t, wm) {
			t.Fatal("client metadata expected in handshake but not found")
		}

		// do a heartbeat with a non-nil connection
		if err = channelConn.AddResponse(makeIsMasterReply()); err != nil {
			t.Fatalf("error adding response: %v", err)
		}
		_, _ = s.heartbeat(conn)
		wm = channelConn.GetWrittenMessage()
		if wm == nil {
			t.Fatal("no wire message written for heartbeat")
		}
		if includesMetadata(t, wm) {
			t.Fatal("client metadata not expected in heartbeat but found")
		}
	})
	t.Run("WithServerAppName", func(t *testing.T) {
		name := "test"

		s, err := NewServer(address.Address("localhost"),
			WithServerAppName(func(string) string { return name }))
		require.Nil(t, err, "error from NewServer: %v", err)
		require.Equal(t, name, s.cfg.appname, "expected appname to be: %v, got: %v", name, s.cfg.appname)
	})
}

func includesMetadata(t *testing.T, wm []byte) bool {
	var ok bool
	_, _, _, _, wm, ok = wiremessage.ReadHeader(wm)
	if !ok {
		t.Fatal("could not read header")
	}
	_, wm, ok = wiremessage.ReadQueryFlags(wm)
	if !ok {
		t.Fatal("could not read flags")
	}
	_, wm, ok = wiremessage.ReadQueryFullCollectionName(wm)
	if !ok {
		t.Fatal("could not read fullCollectionName")
	}
	_, wm, ok = wiremessage.ReadQueryNumberToSkip(wm)
	if !ok {
		t.Fatal("could not read numberToSkip")
	}
	_, wm, ok = wiremessage.ReadQueryNumberToReturn(wm)
	if !ok {
		t.Fatal("could not read numberToReturn")
	}
	var query bsoncore.Document
	query, wm, ok = wiremessage.ReadQueryQuery(wm)
	if !ok {
		t.Fatal("could not read query")
	}

	if _, err := query.LookupErr("client"); err == nil {
		return true
	}
	if _, err := query.LookupErr("$query", "client"); err == nil {
		return true
	}
	return false
}
