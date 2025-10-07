package vminsertapi

import (
	"fmt"
	"io"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
)

// SendToConn sends given buf over provided bc with given rpc name
func SendToConn(bc *handshake.BufferedConn, rpcName string, buf []byte) error {
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

	if !bc.IsNotRPCCompatible {
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
	case StorageStatusAck:
		// ok response, data successfully accepted by vmstorage
	case StorageStatusReadOnly:
		// vmstorage is in readonly mode
		return nil
	default:
		return fmt.Errorf("unexpected `ack` received from vmstorage; got %d; want 1 or 2", sizeBuf.B[0])
	}

	return nil
}

var sizeBufPool bytesutil.ByteBufferPool
