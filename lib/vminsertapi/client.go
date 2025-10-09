package vminsertapi

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
)

// ErrStorageReadOnly indicates that storage is in read-only mode
// and cannot accept write requests
var ErrStorageReadOnly = errors.New("storage node is read only")

// StartRPCRequest inits RPC method call
//
// BufferedConn in streaming mode cannot accept rpc calls
func StartRPCRequest(bc *handshake.BufferedConn, rpcName string) error {
	var err error
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)

	if bc.IsStreamingMode {
		return fmt.Errorf("BUG: connection in streaming mode cannot process RPC calls")
	}

	timeout := 5 * time.Second
	deadline := time.Now().Add(timeout)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write rpcName deadline to %s: %w", deadline, err)
	}
	rpcNameBytes := bytesutil.ToUnsafeBytes(rpcName)
	sizeBuf.B, err = sendDataOnBC(sizeBuf.B, bc, rpcNameBytes)
	if err != nil {
		return fmt.Errorf("cannot write rpcName %q: %w", rpcName, err)
	}

	deadline = time.Now().Add(timeout)
	if err := bc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set read deadline for reading `error` to vmstorage: %w", err)
	}
	// read error message
	sizeBuf.B, err = readBytes(sizeBuf.B[:0], bc, maxErrorMessageSize)
	if err != nil {
		return fmt.Errorf("cannot read error message: %w", err)
	}
	if len(sizeBuf.B) > 0 {
		return errors.New((string(sizeBuf.B)))
	}

	return nil
}

// SendToConn sends given buf over provided bc to the server
func SendToConn(bc *handshake.BufferedConn, buf []byte) error {

	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)
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
	var err error
	sizeBuf.B, err = sendDataOnBC(sizeBuf.B, bc, buf)
	if err != nil {
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
		return ErrStorageReadOnly
	default:
		return fmt.Errorf("unexpected `ack` received from vmstorage; got %d; want 1 or 2", sizeBuf.B[0])
	}

	return nil
}

func readBytes(buf []byte, bc *handshake.BufferedConn, maxDataSize int) ([]byte, error) {
	buf = bytesutil.ResizeNoCopyMayOverallocate(buf, 8)
	if n, err := io.ReadFull(bc, buf); err != nil {
		return buf, fmt.Errorf("cannot read %d bytes with data size: %w; read only %d bytes", len(buf), err, n)
	}
	dataSize := encoding.UnmarshalUint64(buf)
	if dataSize > uint64(maxDataSize) {
		return buf, fmt.Errorf("too big data size: %d; it mustn't exceed %d bytes", dataSize, maxDataSize)
	}
	buf = bytesutil.ResizeNoCopyMayOverallocate(buf, int(dataSize))
	if dataSize == 0 {
		return buf, nil
	}
	if n, err := io.ReadFull(bc, buf); err != nil {
		return buf, fmt.Errorf("cannot read data with size %d: %w; read only %d bytes", dataSize, err, n)
	}
	return buf, nil
}

func sendDataOnBC(sizeBuf []byte, bc *handshake.BufferedConn, data []byte) ([]byte, error) {
	sizeBuf = encoding.MarshalUint64(sizeBuf[:0], uint64(len(data)))
	if _, err := bc.Write(sizeBuf); err != nil {
		return sizeBuf, fmt.Errorf("cannot write data size %d: %w", len(data), err)
	}

	if _, err := bc.Write(data); err != nil {
		return sizeBuf, fmt.Errorf("cannot write data with size %d: %w", len(data), err)
	}

	if err := bc.Flush(); err != nil {
		return sizeBuf, fmt.Errorf("cannot flush data with size %d: %w", len(data), err)
	}
	return sizeBuf, nil

}

var sizeBufPool bytesutil.ByteBufferPool
