package vminsertapi

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/google/go-cmp/cmp"
)

func TestProtocolMigration(t *testing.T) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	testStorage := testStorage{}
	ts, err := NewVMInsertServer("localhost:0", time.Second, "vminsert", &testStorage)
	if err != nil {
		t.Fatalf("cannot create vminsert server: %s", err)
	}
	defer ts.MustStop()

	dialer, err := net.Dial("tcp", ts.ln.Addr().String())
	if err != nil {
		t.Fatalf("cannot dial: %s", err)
	}
	bc, err := handshake.VMInsertClient(dialer, 0)
	if err != nil {
		t.Fatalf("cannot handshake with vminsert server: %s", err)
	}
	defer bc.Close()

	if err := sendToConn(bc, "healthcheck_v1", nil); err != nil {
		t.Fatalf("unexpected healthcheck err :%s", err)
	}

	// send previous API health check with empty buffer
	if err := sendToConn(bc, "", nil); err != nil {
		t.Fatalf("unexpected healthcheck err :%s", err)
	}

	var rowsBuf []byte

	expectedRows := []storage.MetricRow{
		{MetricNameRaw: []uint8("m1"), Timestamp: 0, Value: 0},
		{MetricNameRaw: []uint8("m2"), Timestamp: 1, Value: 1},
		{MetricNameRaw: []uint8("m3"), Timestamp: 2, Value: 2},
	}
	for _, row := range expectedRows {
		rowsBuf = storage.MarshalMetricRow(rowsBuf, row.MetricNameRaw, row.Timestamp, row.Value)
	}

	// send data with new protocol version
	if err := sendToConn(bc, "writeRows_v1", rowsBuf); err != nil {
		t.Fatalf("unexpected new protocol write rows err :%s", err)
	}

	if diff := cmp.Diff(expectedRows, testStorage.parsedRows); len(diff) > 0 {
		t.Errorf("unexpected rows for new protocol version (-want, +got):\n%s", diff)
	}

	// send data with previos protocol version - without rpc
	testStorage.parsedRows = testStorage.parsedRows[:0]

	if err := sendToConn(bc, "", rowsBuf); err != nil {
		t.Fatalf("unexpected previous protocol write rows err :%s", err)
	}
	if diff := cmp.Diff(expectedRows, testStorage.parsedRows); len(diff) > 0 {
		t.Errorf("unexpected rows for previous protocol version (-want, +got):\n%s", diff)
	}

	expectedMetadataRows := []metricsmetadata.Row{
		{MetricFamilyName: []byte("mn1"), Help: []byte("uselesshelp"), Unit: []byte("second")},
		{MetricFamilyName: []byte("mn2"), Help: []byte("uselesshelp"), Unit: []byte("second")},
	}
	var metadataBuf []byte
	for _, mdr := range expectedMetadataRows {
		metadataBuf = mdr.MarshalTo(metadataBuf)
	}
	if err := sendToConn(bc, "writeMetadata_v1", metadataBuf); err != nil {
		t.Fatalf("unexpected writeMetadata_v1 error: %s", err)
	}

	if diff := cmp.Diff(expectedMetadataRows, testStorage.parsedMetadata, cmp.AllowUnexported(metricsmetadata.Row{})); len(diff) > 0 {
		t.Errorf("unexpected metadata rows for new protocol version (-want, +got):\n%s", diff)
	}

	testStorage.parsedMetadata = testStorage.parsedMetadata[:0]
	testStorage.parsedRows = testStorage.parsedRows[:0]

	// try send metadata buf without rpc for previous protocol
	if err := sendToConn(bc, "", metadataBuf); err != nil {
		t.Fatalf("cannot write metadata :%s", err)
	}
	if len(testStorage.parsedMetadata) > 0 {
		t.Fatalf("expected empty metadata after write err,got: %d", len(testStorage.parsedMetadata))
	}
	if len(testStorage.parsedRows) > 0 {
		t.Fatalf("expected empty rows after write err,got: %d", len(testStorage.parsedRows))
	}

	// send rows for previous api version after parsing err
	if err := sendToConn(bc, "", rowsBuf); err != nil {
		t.Fatalf("unexpected previous protocol write rows err :%s", err)
	}
	if diff := cmp.Diff(expectedRows, testStorage.parsedRows); len(diff) > 0 {
		t.Errorf("unexpected rows for previous protocol version (-want, +got):\n%s", diff)
	}
}

func sendToConn(bc *handshake.BufferedConn, rpcName string, buf []byte) error {
	// if len(tsBuf) == 0, it must be sent to the vmstorage too in order to check for vmstorage health
	// See checkReadOnlyMode() and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4870

	timeoutSeconds := len(buf) / 3e5
	if timeoutSeconds < 60 {
		timeoutSeconds = 60
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write deadline to %s: %w", deadline, err)
	}
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)

	// sendBuf guarantees that the data batch will be either fully
	// read or fully discarded on the vmstorage side.
	// sendBuf is used for read optimization in vmstorage.
	sendBuf := func(data []byte) error {
		sizeBuf.B = encoding.MarshalUint64(sizeBuf.B[:0], uint64(len(data)))
		if _, err := bc.Write(sizeBuf.B); err != nil {
			return fmt.Errorf("cannot write data size %d: %w", len(data), err)
		}

		if _, err := bc.Write(data); err != nil {
			return fmt.Errorf("cannot write data with size %d: %w", len(data), err)
		}

		if err := bc.Flush(); err != nil {
			return fmt.Errorf("cannot flush data with size %d: %w", len(data), err)
		}
		return nil
	}

	if len(rpcName) > 0 {
		rpcNameBytes := bytesutil.ToUnsafeBytes(rpcName)
		if err := sendBuf(rpcNameBytes); err != nil {
			return fmt.Errorf("cannot write rpcName %q: %w", rpcName, err)
		}
	}

	if err := sendBuf(buf); err != nil {
		return fmt.Errorf("cannot write tsBuf with size %d: %w", len(buf), err)
	}

	// Wait for `ack` from vmstorage.
	// This guarantees that the message has been fully received by vmstorage.
	deadline = time.Now().Add(timeout)
	if err := bc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set read deadline for reading `ack` to vmstorage: %w", err)
	}
	if _, err := io.ReadFull(bc, sizeBuf.B[:1]); err != nil {
		return fmt.Errorf("cannot read `ack` from vmstorage: %w", err)
	}

	ackResp := sizeBuf.B[0]
	switch ackResp {
	case consts.StorageStatusAck:
		// ok response, data successfully accepted by vmstorage
	case consts.StorageStatusReadOnly:
		// vmstorage is in readonly mode
		return nil
	default:
		return fmt.Errorf("unexpected `ack` received from vmstorage; got %d; want 1 or 2", sizeBuf.B[0])
	}

	return nil
}

var sizeBufPool bytesutil.ByteBufferPool

type testStorage struct {
	isReadOnly atomic.Bool

	parsedRows     []storage.MetricRow
	parsedMetadata []metricsmetadata.Row
}

func (v *testStorage) WriteRows(rows []storage.MetricRow) error {
	v.parsedRows = append(v.parsedRows, rows...)
	return nil
}

func (v *testStorage) WriteMetadata(mrs []metricsmetadata.Row) error {
	v.parsedMetadata = append(v.parsedMetadata, mrs...)
	return nil
}

func (v *testStorage) IsReadOnly() bool {
	return v.isReadOnly.Load()
}
