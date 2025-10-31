package vminsertapi

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestProtocolMigration(t *testing.T) {

	testStorage := testStorage{}
	var rowsBuf []byte

	expectedRows := []storage.MetricRow{
		{MetricNameRaw: []uint8("m1"), Timestamp: 0, Value: 0},
		{MetricNameRaw: []uint8("m2"), Timestamp: 1, Value: 1},
		{MetricNameRaw: []uint8("m3"), Timestamp: 2, Value: 2},
	}
	for _, row := range expectedRows {
		rowsBuf = storage.MarshalMetricRow(rowsBuf, row.MetricNameRaw, row.Timestamp, row.Value)
	}
	assertRows := func(expected []storage.MetricRow) {
		t.Helper()
		protoparserutil.StopUnmarshalWorkers()
		// wait for unmarshal workers finish parsing and data ingestion
		got := testStorage.getRows()
		if diff := cmp.Diff(expected, got); len(diff) > 0 {
			t.Errorf("unexpected ingested rows (-want, +got):\n%s", diff)
		}
		testStorage.reset()
	}

	// test old storage and new client
	{
		protoparserutil.StartUnmarshalWorkers()
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-old", &testStorage, nil)
		if err != nil {
			t.Fatalf("cannot create server: %s", err)
		}
		defer ts.MustStop()
		ts.handshakeFunc = handshake.VMInsertServerWithLegacyHello

		// client must fallback to the prev hello message
		bc, err := handshake.VMInsertClientWithDialer(func() (net.Conn, error) {
			return net.Dial("tcp", ts.ln.Addr().String())
		}, 0)
		if err != nil {
			t.Fatalf("cannot perform handshake with server: %s", err)
		}

		// write metrics via prev API - non rpc compatible
		if err := SendToConn(bc, rowsBuf); err != nil {
			t.Fatalf("unexpected previous protocol write rows err :%s", err)
		}
		assertRows(expectedRows)
	}

	// test old client and new storage
	{
		protoparserutil.StartUnmarshalWorkers()
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-new", &testStorage, nil)
		if err != nil {
			t.Fatalf("cannot create server: %s", err)
		}
		defer ts.MustStop()

		c, err := net.Dial("tcp", ts.ln.Addr().String())
		if err != nil {
			t.Fatalf("cannot dial to the server: %s", err)
		}
		bc, err := handshake.VMInsertClientWithHello(c, "vminsert.02", 0)
		if err != nil {
			t.Fatalf("cannot perform handshake with vmstorage: %s", err)
		}
		bc.IsLegacy = true
		// send data in prev protocol version
		if err := SendToConn(bc, rowsBuf); err != nil {
			t.Fatalf("unexpected write rows err :%s", err)
		}

		assertRows(expectedRows)
	}

	// new client and new storage
	{
		protoparserutil.StartUnmarshalWorkers()
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-both-new", &testStorage, nil)
		if err != nil {
			t.Fatalf("cannot create server: %s", err)
		}
		defer ts.MustStop()

		// client must fallback to the prev hello message
		bc, err := handshake.VMInsertClientWithDialer(func() (net.Conn, error) {
			return net.Dial("tcp", ts.ln.Addr().String())
		}, 0)
		if err != nil {
			t.Fatalf("cannot perform handshake with server: %s", err)
		}
		if err := SendRPCRequestToConn(bc, MetricRowsRpcCall.VersionedName, rowsBuf); err != nil {
			t.Fatalf("unexpected write rows err :%s", err)
		}
		assertRows(expectedRows)
	}

	// new server in ready-only mode
	{
		testStorage.isReadOnly.Store(true)
		defer testStorage.isReadOnly.Store(false)
		protoparserutil.StartUnmarshalWorkers()
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-read-only", &testStorage, nil)
		if err != nil {
			t.Fatalf("cannot create server: %s", err)
		}
		defer ts.MustStop()

		// client must fallback to the prev hello message
		bc, err := handshake.VMInsertClientWithDialer(func() (net.Conn, error) {
			return net.Dial("tcp", ts.ln.Addr().String())
		}, 0)
		if err != nil {
			t.Fatalf("cannot perform handshake with server: %s", err)
		}
		if err := SendRPCRequestToConn(bc, MetricRowsRpcCall.VersionedName, rowsBuf); err != nil {
			if !errors.Is(err, storage.ErrReadOnly) {
				t.Fatalf("unexpected write rows err :%s", err)
			}
		}
		assertRows([]storage.MetricRow{})

	}
}

type testStorage struct {
	isReadOnly atomic.Bool

	mu         sync.Mutex
	parsedRows []storage.MetricRow
}

func (ts *testStorage) WriteRows(rows []storage.MetricRow) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.parsedRows = append(ts.parsedRows, rows...)
	return nil
}

func (ts *testStorage) IsReadOnly() bool {
	return ts.isReadOnly.Load()
}

func (ts *testStorage) getRows() []storage.MetricRow {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return append([]storage.MetricRow{}, ts.parsedRows...)
}

func (ts *testStorage) reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.parsedRows = ts.parsedRows[:0]
}
