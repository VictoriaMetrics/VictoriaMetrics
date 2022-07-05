package servers

import (
	"flag"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/clusternative"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss")

// VMInsertServer processes connections from vminsert.
type VMInsertServer struct {
	// storage is a pointer to the underlying storage.
	storage *storage.Storage

	// ln is the listener for incoming connections to the server.
	ln net.Listener

	// connsMap is a map of currently established connections to the server.
	// It is used for closing the connections when MustStop() is called.
	connsMap ingestserver.ConnsMap

	// wg is used for waiting for worker goroutines to stop when MustStop() is called.
	wg sync.WaitGroup

	// stopFlag is set to true when the server needs to stop.
	stopFlag uint32
}

// NewVMInsertServer starts VMInsertServer at the given addr serving the given storage.
func NewVMInsertServer(addr string, storage *storage.Storage) (*VMInsertServer, error) {
	ln, err := netutil.NewTCPListener("vminsert", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen vminsertAddr %s: %w", addr, err)
	}
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		return nil, fmt.Errorf("invalid -precisionBits: %w", err)
	}
	s := &VMInsertServer{
		storage: storage,
		ln:      ln,
	}
	s.connsMap.Init()
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
		logger.Infof("accepted vminsert conn from %s", c.RemoteAddr())

		if !s.connsMap.Add(c) {
			// The server is closed.
			_ = c.Close()
			return
		}
		vminsertConns.Inc()
		s.wg.Add(1)
		go func() {
			defer func() {
				s.connsMap.Delete(c)
				vminsertConns.Dec()
				s.wg.Done()
			}()

			// There is no need in response compression, since
			// vmstorage sends only small packets to vminsert.
			compressionLevel := 0
			bc, err := handshake.VMInsertServer(c, compressionLevel)
			if err != nil {
				if s.isStopping() {
					// c is stopped inside VMInsertServer.MustStop
					return
				}
				logger.Errorf("cannot perform vminsert handshake with client %q: %s", c.RemoteAddr(), err)
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
			err = clusternative.ParseStream(bc, func(rows []storage.MetricRow) error {
				vminsertMetricsRead.Add(len(rows))
				return s.storage.AddRows(rows, uint8(*precisionBits))
			}, s.storage.IsReadOnly)
			if err != nil {
				if s.isStopping() {
					return
				}
				vminsertConnErrors.Inc()
				logger.Errorf("cannot process vminsert conn from %s: %s", c.RemoteAddr(), err)
			}
		}()
	}
}

var (
	vminsertConns       = metrics.NewCounter("vm_vminsert_conns")
	vminsertConnErrors  = metrics.NewCounter("vm_vminsert_conn_errors_total")
	vminsertMetricsRead = metrics.NewCounter("vm_vminsert_metrics_read_total")
)

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
	s.connsMap.CloseAll()

	// Wait until all the goroutines processing vminsert conns are finished.
	s.wg.Wait()
}

func (s *VMInsertServer) setIsStopping() {
	atomic.StoreUint32(&s.stopFlag, 1)
}

func (s *VMInsertServer) isStopping() bool {
	return atomic.LoadUint32(&s.stopFlag) != 0
}
