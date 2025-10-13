package vminsertapi

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/clusternative/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// VMInsertServer processes connections from vminsert.
type VMInsertServer struct {
	api API

	// ln is the listener for incoming connections to the server.
	ln net.Listener

	handshakeFunc handshake.Func

	// connsMap is a map of currently established connections to the server.
	// It is used for closing the connections when MustStop() is called.
	connsMap ingestserver.ConnsMap

	// wg is used for waiting for worker goroutines to stop when MustStop() is called.
	wg sync.WaitGroup

	// stopFlag is set to true when the server needs to stop.
	stopFlag atomic.Bool

	connectionTimeout time.Duration

	vminsertConns      *metrics.Counter
	vminsertConnErrors *metrics.Counter

	vminsertMetricsRead  *metrics.Counter
	vminsertMetadataRead *metrics.Counter
}

// NewVMInsertServer starts VMInsertServer at the given addr serving the given storage.
func NewVMInsertServer(addr string, connectionTimeout time.Duration, listenerName string, api API) (*VMInsertServer, error) {
	ln, err := netutil.NewTCPListener(listenerName, addr, false, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen vminsertAddr %s: %w", addr, err)
	}
	logger.Infof("started TCP %s server at %q", listenerName, ln.Addr())

	s := &VMInsertServer{
		api:               api,
		ln:                ln,
		handshakeFunc:     handshake.VMInsertServer,
		connectionTimeout: connectionTimeout,

		vminsertConns:        metrics.GetOrCreateCounter(`vm_vminsert_conns`),
		vminsertConnErrors:   metrics.GetOrCreateCounter(`vm_vminsert_conn_errors_total`),
		vminsertMetricsRead:  metrics.GetOrCreateCounter(`vm_vminsert_metrics_read_total`),
		vminsertMetadataRead: metrics.GetOrCreateCounter(`vm_vminsert_metadata_read_total`),
	}
	s.connsMap.Init(listenerName)
	s.wg.Add(1)
	go func() {
		s.run()
		s.wg.Done()
	}()
	return s, nil
}

func (s *VMInsertServer) run() {
	logger.Infof("accepting vminsert conns at %s", s.ln.Addr())
	for {
		c, err := s.ln.Accept()
		if err != nil {
			if pe, ok := err.(net.Error); ok && pe.Temporary() {
				continue
			}
			if s.isStopping() {
				return
			}
			logger.Panicf("FATAL: cannot process vminsert conns at %s: %s", s.ln.Addr(), err)
		}

		if !s.connsMap.Add(c) {
			// The server is closed.
			_ = c.Close()
			return
		}
		s.vminsertConns.Inc()
		s.wg.Add(1)
		go func() {
			defer func() {
				s.connsMap.Delete(c)
				s.vminsertConns.Dec()
				s.wg.Done()
			}()

			// There is no need in response compression, since
			// vmstorage sends only small packets to vminsert.
			compressionLevel := 0
			bc, err := s.handshakeFunc(c, compressionLevel)
			if err != nil {
				if s.isStopping() {
					// c is stopped inside VMInsertServer.MustStop
					return
				}
				if handshake.IsTimeoutNetworkError(err) {
					logger.Warnf("cannot complete vminsert handshake due to network timeout error with client %q: %s. "+
						"If errors are transient and infrequent increase -rpc.handshakeTimeout and -vmstorageDialTimeout on client and server side. Check vminsert logs for errors", c.RemoteAddr(), err)
				} else if handshake.IsClientNetworkError(err) {
					logger.Warnf("cannot complete vminsert handshake due to network error with client %q: %s. "+
						"Check vminsert logs for errors", c.RemoteAddr(), err)
				} else if !handshake.IsTCPHealthcheck(err) {
					logger.Errorf("cannot perform vminsert handshake with client %q: %s", c.RemoteAddr(), err)
				}
				_ = c.Close()
				return
			}
			defer func() {
				if !s.isStopping() {
					logger.Infof("closing vminsert conn from %s", c.RemoteAddr())
				}
				_ = bc.Close()
			}()

			logger.Infof("processing vminsert conn from %s", c.RemoteAddr())
			if err := s.processConn(bc); err != nil {
				if s.isStopping() {
					// c is stopped inside VMInsertServer.MustStop
					return
				}
				s.vminsertConnErrors.Inc()
				logger.Errorf("cannot process vminsert conn from %s: %s", c.RemoteAddr(), err)
				return
			}
		}()
	}
}

// MustStop gracefully stops s so it no longer touches s.storage after returning.
func (s *VMInsertServer) MustStop() {
	// Mark the server as stopping.
	s.setIsStopping()

	// Stop accepting new connections from vminsert.
	if err := s.ln.Close(); err != nil {
		logger.Panicf("FATAL: cannot close vminsert listener: %s", err)
	}

	// Close existing connections from vminsert, so the goroutines
	// processing these connections are finished.
	s.connsMap.CloseAll(s.connectionTimeout)

	// Wait until all the goroutines processing vminsert conns are finished.
	s.wg.Wait()
}

func (s *VMInsertServer) setIsStopping() {
	s.stopFlag.Store(true)
}

func (s *VMInsertServer) isStopping() bool {
	return s.stopFlag.Load()
}

func (s *VMInsertServer) processConn(bc *handshake.BufferedConn) error {
	ctx := NewRequestCtx(bc)
	for {
		if err := s.processRequest(ctx); err != nil {
			if errors.Is(err, io.EOF) {
				// Remote client gracefully closed the connection.
				return nil
			}
			return fmt.Errorf("cannot process vminsert request: %w", err)
		}
		if err := bc.Flush(); err != nil {
			return fmt.Errorf("cannot flush compressed buffers: %w", err)
		}
	}
}

const maxRPCNameSize = 128

func (s *VMInsertServer) processRequest(ctx *RequestCtx) error {
	// Read rpcName
	// Do not set deadline on reading rpcName, since it may take a
	// lot of time for idle connection.
	ctx.dataBuf = ctx.dataBuf[:0]
	if ctx.bc.IsLegacy {
		// fallback to prev API without RPC
		return s.processWriteRowsLegacy(ctx)
	}

	rpcName, err := ctx.ReadRPCName()
	if err != nil {
		return fmt.Errorf("cannot read rpcName: %w", err)
	}

	// Process the rpc call.
	if err := s.processRPC(ctx, string(rpcName)); err != nil {
		return fmt.Errorf("cannot execute %q: %w", rpcName, err)
	}

	return nil
}

func (s *VMInsertServer) processRPC(ctx *RequestCtx, rpcName string) error {
	switch rpcName {
	case MetricRowsRpcCall.VersionedName:
		if err := s.processWriteRows(ctx); err != nil {
			if writeErr := ctx.WriteErrorMessage(err); writeErr != nil {
				return fmt.Errorf("cannot write error message: %s: %w", err, writeErr)
			}
			if errors.Is(err, storage.ErrReadOnly) {
				return nil
			}
			return fmt.Errorf("cannot process writeRows: %w", err)
		}
		// return empty errror
		return ctx.WriteString("")
	case MetricMetadataRpcCall.VersionedName:
		if err := s.processWriteMetadata(ctx); err != nil {
			if writeErr := ctx.WriteErrorMessage(err); writeErr != nil {
				return fmt.Errorf("cannot write error message: %s: %w", err, writeErr)
			}
			if errors.Is(err, storage.ErrReadOnly) {
				return nil
			}
			return fmt.Errorf("cannot process writeMetadata: %w", err)
		}
		// return empty errror
		return ctx.WriteString("")

	default:
		// reply to client unsupported rpc
		// so it should handle this error
		err := fmt.Errorf("unsupported rpcName: %q", rpcName)
		if writeErr := ctx.WriteErrorMessage(err); writeErr != nil {
			return fmt.Errorf("cannot write error message: %s: %w", err, writeErr)
		}
		return err
	}
}

func (s *VMInsertServer) processWriteRowsLegacy(ctx *RequestCtx) error {
	return stream.Parse(ctx.bc, func(rows []storage.MetricRow) error {
		s.vminsertMetricsRead.Add(len(rows))
		return s.api.WriteRows(rows)
	}, s.api.IsReadOnly)
}

func (s *VMInsertServer) processWriteRows(ctx *RequestCtx) error {
	return stream.ParseBlock(ctx.bc, func(rows []storage.MetricRow) error {
		s.vminsertMetricsRead.Add(len(rows))
		return s.api.WriteRows(rows)
	}, s.api.IsReadOnly)
}

func (s *VMInsertServer) processWriteMetadata(_ *RequestCtx) error {
	// TODO: implement
	return nil
}

// RequestCtx defines server request context
type RequestCtx struct {
	bc      *handshake.BufferedConn
	sizeBuf []byte
	dataBuf []byte
}

// NewRequestCtx returns new server request context for given BufferedConn
func NewRequestCtx(bc *handshake.BufferedConn) *RequestCtx {
	return &RequestCtx{
		bc:      bc,
		sizeBuf: make([]byte, 0, 8),
	}
}

// ReadRPCName reads rpc call name from the client
//
// Returned bytes slice is valid until any ctx method call
func (ctx *RequestCtx) ReadRPCName() ([]byte, error) {
	ctx.sizeBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.sizeBuf, 8)
	if _, err := io.ReadFull(ctx.bc, ctx.sizeBuf); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("cannot read data size: %w", err)
	}
	dataSize := encoding.UnmarshalUint64(ctx.sizeBuf)
	if dataSize > uint64(maxRPCNameSize) {
		return nil, fmt.Errorf("too big data size: %d; it mustn't exceed %d bytes", dataSize, maxRPCNameSize)
	}

	ctx.dataBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.dataBuf, int(dataSize))
	if dataSize == 0 {
		return nil, nil
	}
	if n, err := io.ReadFull(ctx.bc, ctx.dataBuf); err != nil {
		return nil, fmt.Errorf("cannot read data with size %d: %w; read only %d bytes", dataSize, err, n)
	}
	return ctx.dataBuf, nil
}

// maxErrorMessageSize is the maximum size of error message to send to clients.
const maxErrorMessageSize = 64 * 1024

// WriteErrorMessage sends given error to the client
func (ctx *RequestCtx) WriteErrorMessage(err error) error {
	errMsg := err.Error()
	if len(errMsg) > maxErrorMessageSize {
		// Trim too long error message.
		errMsg = errMsg[:maxErrorMessageSize]
	}
	if err := ctx.WriteString(errMsg); err != nil {
		return fmt.Errorf("cannot send error message %q to client: %w", errMsg, err)
	}
	return nil
}

// WriteString writes provided string to the client
func (ctx *RequestCtx) WriteString(s string) error {
	ctx.dataBuf = append(ctx.dataBuf[:0], s...)
	if err := ctx.writeDataBufBytes(); err != nil {
		return fmt.Errorf("cannot writeString: %w", err)
	}
	return nil
}

func (ctx *RequestCtx) writeDataBufBytes() error {
	if err := ctx.writeUint64(uint64(len(ctx.dataBuf))); err != nil {
		return fmt.Errorf("cannot write data size: %w", err)
	}
	if len(ctx.dataBuf) == 0 {
		return nil
	}
	if _, err := ctx.bc.Write(ctx.dataBuf); err != nil {
		return fmt.Errorf("cannot write data with size %d: %w", len(ctx.dataBuf), err)
	}
	return nil
}

func (ctx *RequestCtx) writeUint64(n uint64) error {
	ctx.sizeBuf = encoding.MarshalUint64(ctx.sizeBuf[:0], n)
	if _, err := ctx.bc.Write(ctx.sizeBuf); err != nil {
		return fmt.Errorf("cannot write uint64 %d: %w", n, err)
	}
	return nil
}
