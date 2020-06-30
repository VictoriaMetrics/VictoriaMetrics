package handshake

import (
	"fmt"
	"io"
	"net"
	"time"
)

const (
	vminsertHello = "vminsert.02"
	vmselectHello = "vmselect.01"

	successResponse = "ok"
)

// Func must perform handshake on the given c using the given compressionLevel.
//
// It must return BufferedConn wrapper for c on successful handshake.
type Func func(c net.Conn, compressionLevel int) (*BufferedConn, error)

// VMInsertClient performs client-side handshake for vminsert protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the server.
// compressionLevel <= 0 means 'no compression'
func VMInsertClient(c net.Conn, compressionLevel int) (*BufferedConn, error) {
	return genericClient(c, vminsertHello, compressionLevel)
}

// VMInsertServer performs server-side handshake for vminsert protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the client.
// compressionLevel <= 0 means 'no compression'
func VMInsertServer(c net.Conn, compressionLevel int) (*BufferedConn, error) {
	return genericServer(c, vminsertHello, compressionLevel)
}

// VMSelectClient performs client-side handshake for vmselect protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the server.
// compressionLevel <= 0 means 'no compression'
func VMSelectClient(c net.Conn, compressionLevel int) (*BufferedConn, error) {
	return genericClient(c, vmselectHello, compressionLevel)
}

// VMSelectServer performs server-side handshake for vmselect protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the client.
// compressionLevel <= 0 means 'no compression'
func VMSelectServer(c net.Conn, compressionLevel int) (*BufferedConn, error) {
	return genericServer(c, vmselectHello, compressionLevel)
}

func genericServer(c net.Conn, msg string, compressionLevel int) (*BufferedConn, error) {
	if err := readMessage(c, msg); err != nil {
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
	bc := newBufferedConn(c, compressionLevel, isRemoteCompressed)
	return bc, nil
}

func genericClient(c net.Conn, msg string, compressionLevel int) (*BufferedConn, error) {
	if err := writeMessage(c, msg); err != nil {
		return nil, fmt.Errorf("cannot write hello: %w", err)
	}
	if err := readMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot read success response after sending hello: %w", err)
	}
	if err := writeIsCompressed(c, compressionLevel > 0); err != nil {
		return nil, fmt.Errorf("cannot write isCompressed flag: %w", err)
	}
	if err := readMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot read success response on isCompressed: %w", err)
	}
	isRemoteCompressed, err := readIsCompressed(c)
	if err != nil {
		return nil, fmt.Errorf("cannot read isCompressed flag: %w", err)
	}
	if err := writeMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot write success response on isCompressed: %w", err)
	}
	bc := newBufferedConn(c, compressionLevel, isRemoteCompressed)
	return bc, nil
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
	if err := c.SetWriteDeadline(zeroTime); err != nil {
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

func readData(c net.Conn, dataLen int) ([]byte, error) {
	if err := c.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return nil, fmt.Errorf("cannot set read deadline: %w", err)
	}
	data := make([]byte, dataLen)
	if n, err := io.ReadFull(c, data); err != nil {
		return nil, fmt.Errorf("cannot read message with size %d: %w; read only %d bytes", dataLen, err, n)
	}
	if err := c.SetReadDeadline(zeroTime); err != nil {
		return nil, fmt.Errorf("cannot reset read deadline: %w", err)
	}
	return data, nil
}

var zeroTime time.Time
