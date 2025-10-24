package vminsertapi

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// SendRPCRequestToConn sends given buf over provided bc to the server
func SendRPCRequestToConn(bc *handshake.BufferedConn, rpcName string, buf []byte) error {
	var err error
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)

	timeoutSeconds := len(buf) / 3e5
	if timeoutSeconds < 60 {
		timeoutSeconds = 60
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write deadline to %s: %w", deadline, err)
	}

	rpcNameBytes := bytesutil.ToUnsafeBytes(rpcName)
	sizeBuf.B, err = sendData(sizeBuf.B, bc, rpcNameBytes)
	if err != nil {
		return fmt.Errorf("cannot write rpcName %q: %w", rpcName, err)
	}

	zb := zbPool.Get()
	defer zbPool.Put(zb)

	zb.B = zstd.CompressLevel(zb.B[:0], buf, 1)
	sizeBuf.B, err = sendData(sizeBuf.B, bc, zb.B)
	if err != nil {
		return fmt.Errorf("cannot write buf with size %d: %w", len(zb.B), err)
	}
	if err := bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush data with size %d: %w", len(zb.B), err)
	}

	// Wait for error message from vmstorage.
	// This guarantees that the message has been fully received by vmstorage.
	deadline = time.Now().Add(timeout)
	if err := bc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set read deadline for reading `ack` to vmstorage: %w", err)
	}
	sizeBuf.B, err = readBytes(sizeBuf.B[:0], bc, maxErrorMessageSize)
	if err != nil {
		return fmt.Errorf("cannot read error message: %w", err)
	}
	if len(sizeBuf.B) > 0 {
		if string(sizeBuf.B) == storage.ErrReadOnly.Error() {
			return storage.ErrReadOnly
		}
		return errors.New(string(sizeBuf.B))
	}

	return nil
}

// SendToConn sends given buf over provided bc to the server
func SendToConn(bc *handshake.BufferedConn, buf []byte) error {
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)
	// if len(tsBuf) == 0, it must be sent to the vmstorage too in order to check for vmstorage health
	// See checkReadOnlyMode() and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4870

	// adjust timeout accordingly to the forwarded buf size
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
	sizeBuf.B, err = sendData(sizeBuf.B, bc, buf)
	if err != nil {
		return fmt.Errorf("cannot write tsBuf with size %d: %w", len(buf), err)
	}

	if err := bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush data with size %d: %w", len(buf), err)
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
		return storage.ErrReadOnly
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

func sendData(sizeBuf []byte, bc *handshake.BufferedConn, data []byte) ([]byte, error) {
	sizeBuf = encoding.MarshalUint64(sizeBuf[:0], uint64(len(data)))
	if _, err := bc.Write(sizeBuf); err != nil {
		return sizeBuf, fmt.Errorf("cannot write data size %d: %w", len(data), err)
	}

	if _, err := bc.Write(data); err != nil {
		return sizeBuf, fmt.Errorf("cannot write data with size %d: %w", len(data), err)
	}
	return sizeBuf, nil

}

var sizeBufPool bytesutil.ByteBufferPool
var zbPool bytesutil.ByteBufferPool
