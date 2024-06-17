package handshake

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

const (
	vminsertHello = "vminsert.02"
	vmselectHello = "vmselect.01"

	successResponse = "ok"
)

// ClientFunc must perform handshake on the given c using the given compressionLevel.
//
// It must return BufferedConn wrapper for c on successful handshake.
type ClientFunc func(c net.Conn, compressionLevel int) (*BufferedConn, uint64, error)

// ServerFunc must perform handshake on the given c using the given compressionLevel and id.
//
// It must return BufferedConn wrapper for c on successful handshake.
type ServerFunc func(c net.Conn, compressionLevel int, id uint64) (*BufferedConn, error)

// VMInsertClient performs client-side handshake for vminsert protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the server.
// compressionLevel <= 0 means 'no compression'
func VMInsertClient(c net.Conn, compressionLevel int) (*BufferedConn, uint64, error) {
	return genericClient(c, vminsertHello, compressionLevel)
}

// VMInsertServer performs server-side handshake for vminsert protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the client.
// compressionLevel <= 0 means 'no compression'
func VMInsertServer(c net.Conn, compressionLevel int, id uint64) (*BufferedConn, error) {
	return genericServer(c, vminsertHello, compressionLevel, id)
}

// VMSelectClient performs client-side handshake for vmselect protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the server.
// compressionLevel <= 0 means 'no compression'
func VMSelectClient(c net.Conn, compressionLevel int) (*BufferedConn, uint64, error) {
	return genericClient(c, vmselectHello, compressionLevel)
}

// VMSelectServer performs server-side handshake for vmselect protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the client.
// compressionLevel <= 0 means 'no compression'
func VMSelectServer(c net.Conn, compressionLevel int, id uint64) (*BufferedConn, error) {
	return genericServer(c, vmselectHello, compressionLevel, id)
}

// ErrIgnoreHealthcheck means the TCP healthckeck, which must be ignored.
//
// The TCP healthcheck is performed by opening and then immediately closing the connection.
var ErrIgnoreHealthcheck = fmt.Errorf("TCP healthcheck - ignore it")

func genericServer(c net.Conn, msg string, compressionLevel int, id uint64) (*BufferedConn, error) {
	if err := readMessage(c, msg); err != nil {
		if errors.Is(err, io.EOF) {
			// This is TCP healthcheck, which must be ignored in order to prevent from logs pollution.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1762
			return nil, ErrIgnoreHealthcheck
		}
		return nil, fmt.Errorf("cannot read hello: %w", err)
	}
	if err := writeMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot write success response on hello: %w", err)
	}
	isRemoteCompressed, err := readIsCompressed(c)
	if err != nil {
		return nil, fmt.Errorf("cannot read isCompressed flag: %w", err)
	}
	if err := writeMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot write success response on isCompressed: %w", err)
	}
	if err := writeIsCompressed(c, compressionLevel > 0); err != nil {
		return nil, fmt.Errorf("cannot write isCompressed flag: %w", err)
	}
	if err := readMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot read success response on isCompressed: %w", err)
	}
	if err := writeNodeID(c, id); err != nil {
		return nil, fmt.Errorf("cannot write nodeID: %w", err)
	}
	if err := readMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot read success response on nodeID: %w", err)
	}
	bc := newBufferedConn(c, compressionLevel, isRemoteCompressed)
	return bc, nil
}

func genericClient(c net.Conn, msg string, compressionLevel int) (*BufferedConn, uint64, error) {
	if err := writeMessage(c, msg); err != nil {
		return nil, 0, fmt.Errorf("cannot write hello: %w", err)
	}
	if err := readMessage(c, successResponse); err != nil {
		return nil, 0, fmt.Errorf("cannot read success response after sending hello: %w", err)
	}
	if err := writeIsCompressed(c, compressionLevel > 0); err != nil {
		return nil, 0, fmt.Errorf("cannot write isCompressed flag: %w", err)
	}
	if err := readMessage(c, successResponse); err != nil {
		return nil, 0, fmt.Errorf("cannot read success response on isCompressed: %w", err)
	}
	isRemoteCompressed, err := readIsCompressed(c)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot read isCompressed flag: %w", err)
	}
	if err := writeMessage(c, successResponse); err != nil {
		return nil, 0, fmt.Errorf("cannot write success response on isCompressed: %w", err)
	}
	nodeID, err := readNodeID(c)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot read nodeID: %w", err)
	}
	if err := writeMessage(c, successResponse); err != nil {
		return nil, 0, fmt.Errorf("cannot write success response on nodeID: %w", err)
	}

	bc := newBufferedConn(c, compressionLevel, isRemoteCompressed)
	return bc, nodeID, nil
}

func writeIsCompressed(c net.Conn, isCompressed bool) error {
	var buf [1]byte
	if isCompressed {
		buf[0] = 1
	}
	return writeMessage(c, string(buf[:]))
}

func readIsCompressed(c net.Conn) (bool, error) {
	buf, err := readData(c, 1)
	if err != nil {
		return false, err
	}
	isCompressed := (buf[0] != 0)
	return isCompressed, nil
}

func writeMessage(c net.Conn, msg string) error {
	if err := c.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return fmt.Errorf("cannot set write deadline: %w", err)
	}
	if _, err := io.WriteString(c, msg); err != nil {
		return fmt.Errorf("cannot write %q to server: %w", msg, err)
	}
	if fc, ok := c.(flusher); ok {
		if err := fc.Flush(); err != nil {
			return fmt.Errorf("cannot flush %q to server: %w", msg, err)
		}
	}
	if err := c.SetWriteDeadline(time.Time{}); err != nil {
		return fmt.Errorf("cannot reset write deadline: %w", err)
	}
	return nil
}

type flusher interface {
	Flush() error
}

func readMessage(c net.Conn, msg string) error {
	buf, err := readData(c, len(msg))
	if err != nil {
		return err
	}
	if string(buf) != msg {
		return fmt.Errorf("unexpected message obtained; got %q; want %q", buf, msg)
	}
	return nil
}

func readNodeID(c net.Conn) (uint64, error) {
	buf, err := readData(c, int(unsafe.Sizeof(uint64(0))))
	if err != nil {
		return 0, err
	}
	return encoding.UnmarshalUint64(buf), nil
}

func writeNodeID(c net.Conn, id uint64) error {
	buf := encoding.MarshalUint64(nil, id)
	return writeMessage(c, string(buf[:]))
}

func readData(c net.Conn, dataLen int) ([]byte, error) {
	if err := c.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return nil, fmt.Errorf("cannot set read deadline: %w", err)
	}
	data := make([]byte, dataLen)
	if n, err := io.ReadFull(c, data); err != nil {
		return nil, fmt.Errorf("cannot read message with size %d: %w; read only %d bytes", dataLen, err, n)
	}
	if err := c.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("cannot reset read deadline: %w", err)
	}
	return data, nil
}
