package vminsertapi

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/clusternative/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
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
	// Mark the server as stoping.
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
	ctx := &vminsertRequestCtx{
		bc:      bc,
		sizeBuf: make([]byte, 8),
	}
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

func (s *VMInsertServer) processRequest(ctx *vminsertRequestCtx) error {
	// Read rpcName
	// Do not set deadline on reading rpcName, since it may take a
	// lot of time for idle connection.
	ctx.dataBuf = ctx.dataBuf[:0]
	if ctx.bc.IsNotRPCCompatible {
		// fallback to prev API without RPC
		return s.processWriteRows(ctx)
	}
	if err := ctx.readDataBufBytes(maxRPCNameSize); err != nil {
		return fmt.Errorf("cannot read rpcName: %w", err)
	}
	rpcName := string(ctx.dataBuf)

	// Process the rpcName call.
	if err := s.processRPC(ctx, rpcName); err != nil {
		return fmt.Errorf("cannot execute %q: %w", rpcName, err)
	}

	return nil
}

func (s *VMInsertServer) processRPC(ctx *vminsertRequestCtx, rpcName string) error {
	switch rpcName {
	case MetricRowsRpcCall.VersionedName:
		return s.processWriteRows(ctx)
	case MetricMetadataRpcCall.VersionedName:
		return s.processWriteMetadata(ctx)
	case CheckReadonlyRpcCall.VersionedName:
		return s.processHealthcheck(ctx)
	default:
		return fmt.Errorf("unsupported rpcName: %q", rpcName)
	}
}

func (s *VMInsertServer) processWriteRows(ctx *vminsertRequestCtx) error {
	return stream.Parse(ctx.bc, func(rows []storage.MetricRow) error {
		s.vminsertMetricsRead.Add(len(rows))
		return s.api.WriteRows(rows)
	}, s.api.IsReadOnly)
}

func (s *VMInsertServer) processWriteMetadata(_ *vminsertRequestCtx) error {
	// TODO: implement
	return nil
}

func (s *VMInsertServer) processHealthcheck(ctx *vminsertRequestCtx) error {
	if err := ctx.readDataBufBytes(0); err != nil {
		if err == io.EOF {
			// Remote client gracefully closed the connection.
			return err
		}
		return fmt.Errorf("cannot read healthcheck_v1 data: %w", err)
	}

	status := StorageStatusAck
	if s.api.IsReadOnly() {
		status = StorageStatusReadOnly
	}

	if err := sendAck(ctx.bc, byte(status)); err != nil {
		return fmt.Errorf("cannot send ack for healthcheck_v1: %w", err)
	}

	return nil
}

type vminsertRequestCtx struct {
	bc      *handshake.BufferedConn
	sizeBuf []byte
	dataBuf []byte
}

func (ctx *vminsertRequestCtx) readDataBufBytes(maxDataSize int) error {
	ctx.sizeBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.sizeBuf, 8)
	if _, err := io.ReadFull(ctx.bc, ctx.sizeBuf); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("cannot read data size: %w", err)
	}
	dataSize := encoding.UnmarshalUint64(ctx.sizeBuf)
	if dataSize > uint64(maxDataSize) {
		return fmt.Errorf("too big data size: %d; it mustn't exceed %d bytes", dataSize, maxDataSize)
	}

	ctx.dataBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.dataBuf, int(dataSize))
	if dataSize == 0 {
		return nil
	}
	if n, err := io.ReadFull(ctx.bc, ctx.dataBuf); err != nil {
		return fmt.Errorf("cannot read data with size %d: %w; read only %d bytes", dataSize, err, n)
	}
	return nil
}

func sendAck(bc *handshake.BufferedConn, status byte) error {
	deadline := time.Now().Add(5 * time.Second)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write deadline: %w", err)
	}
	b := auxBufPool.Get()
	defer auxBufPool.Put(b)
	b.B = append(b.B[:0], status)
	if _, err := bc.Write(b.B); err != nil {
		return err
	}
	return bc.Flush()
}

var auxBufPool bytesutil.ByteBufferPool
