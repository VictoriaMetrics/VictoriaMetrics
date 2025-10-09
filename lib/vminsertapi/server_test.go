package vminsertapi

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestProtocolMigration(t *testing.T) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

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
	assertRows := func() {
		t.Helper()
		if diff := cmp.Diff(expectedRows, testStorage.parsedRows); len(diff) > 0 {
			t.Errorf("unexpected ingested rows (-want, +got):\n%s", diff)
		}
		testStorage.parsedMetadata = testStorage.parsedMetadata[:0]
		testStorage.parsedRows = testStorage.parsedRows[:0]
	}
	// test old storage and new client
	{
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-old", &testStorage)
		if err != nil {
			t.Fatalf("cannot create server: %s", err)
		}
		defer ts.MustStop()
		ts.handshakeFunc = handshake.VMInsertServerNonRPC

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
		assertRows()
	}

	// test old client and new storage
	{
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-new", &testStorage)
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
		bc.IsNotRPCCompatible = true
		// send data in prev protocol version
		if err := SendToConn(bc, rowsBuf); err != nil {
			t.Fatalf("unexpected write rows err :%s", err)
		}

		assertRows()
	}

	// new client and new storage
	{
		ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert-both-new", &testStorage)
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
		if err := StartRPCRequest(bc, MetricRowsRpcCall.VersionedName); err != nil {
			t.Fatalf("cannot start rpc request: %s", err)
		}
		if err := SendToConn(bc, rowsBuf); err != nil {
			t.Fatalf("unexpected write rows err :%s", err)
		}
		assertRows()
	}
}

type testStorage struct {
	isReadOnly atomic.Bool

	parsedRows     []storage.MetricRow
	parsedMetadata []storage.MetricMetadataRow
}

func (v *testStorage) WriteRows(rows []storage.MetricRow) error {
	v.parsedRows = append(v.parsedRows, rows...)
	return nil
}

func (v *testStorage) WriteMetadata(mrs []storage.MetricMetadataRow) error {
	v.parsedMetadata = append(v.parsedMetadata, mrs...)
	return nil
}

func (v *testStorage) IsReadOnly() bool {
	return v.isReadOnly.Load()
}
