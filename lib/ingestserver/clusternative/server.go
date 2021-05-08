package clusternative

import (
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequestsTCP = metrics.NewCounter(`vm_ingestserver_requests_total{type="clusternative", net="tcp"}`)
	writeErrorsTCP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="clusternative", net="tcp"}`)
)

// Server accepts data from vminsert over TCP in the same way as vmstorage does.
type Server struct {
	addr  string
	lnTCP net.Listener
	wg    sync.WaitGroup
}

// MustStart starts clusternative server on the given addr.
//
// The incoming connections are processed with insertHandler.
//
// MustStop must be called on the returned server when it is no longer needed.
func MustStart(addr string, insertHandler func(c net.Conn) error) *Server {
	logger.Infof("starting TCP clusternative server at %q", addr)
	lnTCP, err := netutil.NewTCPListener("clusternative", addr)
	if err != nil {
		logger.Fatalf("cannot start TCP clusternative server at %q: %s", addr, err)
	}
	s := &Server{
		addr:  addr,
		lnTCP: lnTCP,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		serveTCP(lnTCP, insertHandler)
		logger.Infof("stopped TCP clusternative server at %q", addr)
	}()
	return s
}

// MustStop stops the server.
func (s *Server) MustStop() {
	logger.Infof("stopping TCP clusternative server at %q...", s.addr)
	if err := s.lnTCP.Close(); err != nil {
		logger.Errorf("cannot close TCP clusternative server: %s", err)
	}
	s.wg.Wait()
	logger.Infof("TCP clusternative server at %q has been stopped", s.addr)
}

func serveTCP(ln net.Listener, insertHandler func(c net.Conn) error) {
	for {
		c, err := ln.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) {
				if ne.Temporary() {
					logger.Errorf("clusternative: temporary error when listening for TCP addr %q: %s", ln.Addr(), err)
					time.Sleep(time.Second)
					continue
				}
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				logger.Fatalf("unrecoverable error when accepting TCP clusternative connections: %s", err)
			}
			logger.Fatalf("unexpected error when accepting TCP clusternative connections: %s", err)
		}
		go func() {
			writeRequestsTCP.Inc()
			if err := insertHandler(c); err != nil {
				writeErrorsTCP.Inc()
				logger.Errorf("error in TCP clusternative conn %q<->%q: %s", c.LocalAddr(), c.RemoteAddr(), err)
			}
			_ = c.Close()
		}()
	}
}
