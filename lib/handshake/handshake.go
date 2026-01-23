package handshake

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var rpcHandshakeTimeout = flag.Duration("rpc.handshakeTimeout", 5*time.Second, "Timeout for RPC handshake between vminsert/vmselect and vmstorage. Increase this value if transient handshake failures occur. See https://docs.victoriametrics.com/victoriametrics/troubleshooting/#cluster-instability section for more details.")

const (
	vminsertHelloLegacyVersion = "vminsert.02"
	vminsertHello              = "vminsert.03"
	vmselectHello              = "vmselect.01"

	successResponse = "ok"
)

// Func must perform handshake on the given c using the given compressionLevel.
//
// It must return BufferedConn wrapper for c on successful handshake.
type Func func(c net.Conn, compressionLevel int) (*BufferedConn, error)

type HealthCheckFunc func(bc *BufferedConn) error

// VMInsertClientWithDialer performs client-side handshake for vminsert protocol.
//
// it uses provided dial func to establish connection to the server.
// compressionLevel is a legacy option which defines the level used for compression of the data sent
// to the server.
// compressionLevel <= 0 means 'no compression'
func VMInsertClientWithDialer(dial func() (net.Conn, error), compressionLevel int) (*BufferedConn, error) {
	c, err := dial()
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	bc, err := vminsertClient(c, 0)
	if err == nil {
		return bc, nil
	}
	_ = c.Close()
	if !strings.Contains(err.Error(), "cannot read success response after sending hello") {
		return nil, err
	}
	// try to fallback to the prev non-RPC API version
	// we cannot re-use exist connection, since vmstorage already closed it
	c, err = dial()
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	bc, err = genericClient(c, vminsertHelloLegacyVersion, compressionLevel)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("legacy handshake error: %w", err)
	}
	bc.IsLegacy = true
	logger.Infof("server=%q doesn't support new RPC version, fallback to the legacy format", c.RemoteAddr())
	return bc, nil
}

func vminsertClient(c net.Conn, compressionLevel int) (*BufferedConn, error) {
	return genericClient(c, vminsertHello, compressionLevel)
}

// VMInsertClientWithHello performs client-side handshake for vminsert protocol.
//
// should be used for testing only
func VMInsertClientWithHello(c net.Conn, helloMsg string, compressionLevel int) (*BufferedConn, error) {
	return genericClient(c, helloMsg, compressionLevel)
}

// VMInsertServer performs server-side handshake for vminsert protocol.
//
// compressionLevel is the level used for compression of the data sent
// to the client.
// compressionLevel <= 0 means 'no compression'
func VMInsertServer(c net.Conn, compressionLevel int) (*BufferedConn, error) {

	var isRPCSupported bool
	bc, err := genericServer(c, compressionLevel, func(c net.Conn) error {
		buf, err := readData(c, len(vminsertHello))
		if err != nil {
			if errors.Is(err, io.EOF) {
				// This is likely a TCP healthcheck, which must be ignored in order to prevent logs pollution.
				// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1762
				return errTCPHealthcheck
			}
			return fmt.Errorf("cannot read hello: %w", err)
		}
		isRPCSupported = string(buf) == vminsertHello
		if !isRPCSupported {
			// try to fallback to the previous protocol version
			if string(buf) != vminsertHelloLegacyVersion {
				return fmt.Errorf("unexpected message obtained; got %q; want %q", buf, vminsertHello)
			}
			logger.Infof("client=%q doesn't support new RPC version, fallback to the legacy format", c.RemoteAddr())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	bc.IsLegacy = !isRPCSupported
	return bc, nil
}

// VMInsertServerWithLegacyHello performs server-side handshake for vminsert protocol
// with legacy hello  message
//
// should be used for testing only
func VMInsertServerWithLegacyHello(c net.Conn, compressionLevel int) (*BufferedConn, error) {

	bc, err := genericServer(c, compressionLevel, func(c net.Conn) error {
		return readMessage(c, vminsertHelloLegacyVersion)
	})
	if err != nil {
		return nil, err
	}
	bc.IsLegacy = true
	return bc, nil
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
	return genericServer(c, compressionLevel, func(c net.Conn) error {
		return readMessage(c, vmselectHello)
	})
}

func HealthCheckToVmstroage(bc *BufferedConn) error {
	funcName := "healthCheck_v1"
	buf := []byte(funcName)
	sizeBuf := encoding.MarshalUint64(nil, uint64(len(buf)))
	if _, err := bc.Write(sizeBuf); err != nil {
		return fmt.Errorf("cannot write funcName size: %w", err)
	}
	_, err := bc.Write(buf)
	if err != nil {
		return fmt.Errorf("cannot write funcName: %w", err)
	}
	var trace [1]byte
	_, err = bc.Write(trace[:])
	if err != nil {
		return fmt.Errorf("cannot write traceEnable: %w", err)
	}

	timeout := encoding.MarshalUint32(nil, 5)
	_, err = bc.Write(timeout)
	if err != nil {
		return fmt.Errorf("cannot write timeout: %w", err)
	}
	if err = bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush buf: %w", err)
	}

	var resp [8]byte
	if _, err = io.ReadFull(bc, resp[:]); err != nil {
		return fmt.Errorf("cannot read health check response: %w", err)
	}
	_ = encoding.UnmarshalUint64(resp[:])

	if _, err = io.ReadFull(bc, resp[:]); err != nil {
		return fmt.Errorf("cannot read trace field: %w", err)
	}

	return nil
}

// errTCPHealthcheck indicates that the connection was opened as part of a TCP health check
// and was closed immediately after being established.
//
// This is expected behavior and can be safely ignored.
var errTCPHealthcheck = fmt.Errorf("TCP health check connection – safe to ignore")

// IsTCPHealthcheck determines whether the provided error is a TCP health check
func IsTCPHealthcheck(err error) bool {
	return errors.Is(err, errTCPHealthcheck)
}

// IsClientNetworkError determines whether the provided error is a client-side network error,
// such as io.EOF, io.ErrUnexpectedEOF, or a timeout.
// These errors typically occur when a client disconnects abruptly or fails during the handshake,
// and are generally non-actionable from the server point of view.
// This function helps distinguish such errors from critical ones during the handshake process
// and adjust logging accordingly.
//
// See: https://github.com/VictoriaMetrics/VictoriaMetrics-enterprise/pull/880
func IsClientNetworkError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if IsTimeoutNetworkError(err) {
		return true
	}

	if errMsg := err.Error(); strings.Contains(errMsg, "broken pipe") || strings.Contains(errMsg, "reset by peer") {
		return true
	}

	return false
}

// IsTimeoutNetworkError determines whether the provided error is a network error with a timeout.
func IsTimeoutNetworkError(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}

	return false
}

func genericServer(c net.Conn, compressionLevel int, readHelloMessage func(c net.Conn) error) (*BufferedConn, error) {
	if err := c.SetDeadline(time.Now().Add(*rpcHandshakeTimeout)); err != nil {
		return nil, fmt.Errorf("cannot set deadline: %w", err)
	}

	if err := readHelloMessage(c); err != nil {
		return nil, fmt.Errorf("cannot read hello message : %w", err)
	}
	if err := writeMessage(c, successResponse); err != nil {
		return nil, fmt.Errorf("cannot write success response on isCompressed: %w", err)
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

	if err := c.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("cannot reset deadline: %w", err)
	}

	bc := newBufferedConn(c, compressionLevel, isRemoteCompressed)
	return bc, nil
}

func genericClient(c net.Conn, msg string, compressionLevel int) (*BufferedConn, error) {
	if err := c.SetDeadline(time.Now().Add(*rpcHandshakeTimeout)); err != nil {
		return nil, fmt.Errorf("cannot set deadline: %w", err)
	}

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

	if err := c.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("cannot reset deadline: %w", err)
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
	isCompressed := buf[0] != 0
	return isCompressed, nil
}

func writeMessage(c net.Conn, msg string) error {
	if _, err := io.WriteString(c, msg); err != nil {
		return fmt.Errorf("cannot write %q to server: %w", msg, err)
	}
	if fc, ok := c.(flusher); ok {
		if err := fc.Flush(); err != nil {
			return fmt.Errorf("cannot flush %q to server: %w", msg, err)
		}
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
	data := make([]byte, dataLen)
	if n, err := io.ReadFull(c, data); err != nil {
		return nil, fmt.Errorf("cannot read message with size %d: %w; read only %d bytes", dataLen, err, n)
	}
	return data, nil
}
