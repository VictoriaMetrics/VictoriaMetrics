package influx

import (
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequestsTCP = metrics.NewCounter(`vm_ingestserver_requests_total{type="influx", name="write", net="tcp"}`)
	writeErrorsTCP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="influx", name="write", net="tcp"}`)

	writeRequestsUDP = metrics.NewCounter(`vm_ingestserver_requests_total{type="influx", name="write", net="udp"}`)
	writeErrorsUDP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="influx", name="write", net="udp"}`)
)

// Server accepts Influx line protocol over TCP and UDP.
type Server struct {
	addr  string
	lnTCP net.Listener
	lnUDP net.PacketConn
	wg    sync.WaitGroup
}

// MustStart starts Influx server on the given addr.
//
// The incoming connections are processed with insertHandler.
//
// MustStop must be called on the returned server when it is no longer needed.
func MustStart(addr string, insertHandler func(r io.Reader) error) *Server {
	logger.Infof("starting TCP Influx server at %q", addr)
	lnTCP, err := netutil.NewTCPListener("influx", addr)
	if err != nil {
		logger.Fatalf("cannot start TCP Influx server at %q: %s", addr, err)
	}

	logger.Infof("starting UDP Influx server at %q", addr)
	lnUDP, err := net.ListenPacket("udp4", addr)
	if err != nil {
		logger.Fatalf("cannot start UDP Influx server at %q: %s", addr, err)
	}

	s := &Server{
		addr:  addr,
		lnTCP: lnTCP,
		lnUDP: lnUDP,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		serveTCP(lnTCP, insertHandler)
		logger.Infof("stopped TCP Influx server at %q", addr)
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		serveUDP(lnUDP, insertHandler)
		logger.Infof("stopped UDP Influx server at %q", addr)
	}()
	return s
}

// MustStop stops the server.
func (s *Server) MustStop() {
	logger.Infof("stopping TCP Influx server at %q...", s.addr)
	if err := s.lnTCP.Close(); err != nil {
		logger.Errorf("cannot close TCP Influx server: %s", err)
	}
	logger.Infof("stopping UDP Influx server at %q...", s.addr)
	if err := s.lnUDP.Close(); err != nil {
		logger.Errorf("cannot close UDP Influx server: %s", err)
	}
	s.wg.Wait()
	logger.Infof("TCP and UDP Influx servers at %q have been stopped", s.addr)
}

func serveTCP(ln net.Listener, insertHandler func(r io.Reader) error) {
	for {
		c, err := ln.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) {
				if ne.Temporary() {
					logger.Errorf("influx: temporary error when listening for TCP addr %q: %s", ln.Addr(), err)
					time.Sleep(time.Second)
					continue
				}
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				logger.Fatalf("unrecoverable error when accepting TCP Influx connections: %s", err)
			}
			logger.Fatalf("unexpected error when accepting TCP Influx connections: %s", err)
		}
		go func() {
			writeRequestsTCP.Inc()
			if err := insertHandler(c); err != nil {
				writeErrorsTCP.Inc()
				logger.Errorf("error in TCP Influx conn %q<->%q: %s", c.LocalAddr(), c.RemoteAddr(), err)
			}
			_ = c.Close()
		}()
	}
}

func serveUDP(ln net.PacketConn, insertHandler func(r io.Reader) error) {
	gomaxprocs := runtime.GOMAXPROCS(-1)
	var wg sync.WaitGroup
	for i := 0; i < gomaxprocs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var bb bytesutil.ByteBuffer
			bb.B = bytesutil.Resize(bb.B, 64*1024)
			for {
				bb.Reset()
				bb.B = bb.B[:cap(bb.B)]
				n, addr, err := ln.ReadFrom(bb.B)
				if err != nil {
					writeErrorsUDP.Inc()
					var ne net.Error
					if errors.As(err, &ne) {
						if ne.Temporary() {
							logger.Errorf("influx: temporary error when listening for UDP addr %q: %s", ln.LocalAddr(), err)
							time.Sleep(time.Second)
							continue
						}
						if strings.Contains(err.Error(), "use of closed network connection") {
							break
						}
					}
					logger.Errorf("cannot read Influx UDP data: %s", err)
					continue
				}
				bb.B = bb.B[:n]
				writeRequestsUDP.Inc()
				if err := insertHandler(bb.NewReader()); err != nil {
					writeErrorsUDP.Inc()
					logger.Errorf("error in UDP Influx conn %q<->%q: %s", ln.LocalAddr(), addr, err)
					continue
				}
			}
		}()
	}
	wg.Wait()
}
