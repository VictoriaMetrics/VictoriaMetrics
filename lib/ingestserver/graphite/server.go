package graphite

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequestsTCP = metrics.NewCounter(`vm_ingestserver_requests_total{type="graphite", name="write", net="tcp"}`)
	writeErrorsTCP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="graphite", name="write", net="tcp"}`)

	writeRequestsUDP = metrics.NewCounter(`vm_ingestserver_requests_total{type="graphite", name="write", net="udp"}`)
	writeErrorsUDP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="graphite", name="write", net="udp"}`)
)

// Server accepts Graphite plaintext lines over TCP and UDP.
type Server struct {
	addr  string
	lnTCP net.Listener
	lnUDP net.PacketConn
	wg    sync.WaitGroup
	cm    ingestserver.ConnsMap
}

// MustStart starts graphite server on the given addr.
//
// The incoming connections are processed with insertHandler.
//
// If useProxyProtocol is set to true, then the incoming connections are accepted via proxy protocol.
// See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
//
// MustStop must be called on the returned server when it is no longer needed.
func MustStart(addr string, useProxyProtocol bool, insertHandler func(r io.Reader) error) *Server {
	logger.Infof("starting TCP Graphite server at %q", addr)
	lnTCP, err := netutil.NewTCPListener("graphite", addr, useProxyProtocol, nil)
	if err != nil {
		logger.Fatalf("cannot start TCP Graphite server at %q: %s", addr, err)
	}

	logger.Infof("starting UDP Graphite server at %q", addr)
	lnUDP, err := net.ListenPacket(netutil.GetUDPNetwork(), addr)
	if err != nil {
		logger.Fatalf("cannot start UDP Graphite server at %q: %s", addr, err)
	}

	s := &Server{
		addr:  addr,
		lnTCP: lnTCP,
		lnUDP: lnUDP,
	}
	s.cm.Init("graphite")
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.serveTCP(insertHandler)
		logger.Infof("stopped TCP Graphite server at %q", addr)
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.serveUDP(insertHandler)
		logger.Infof("stopped UDP Graphite server at %q", addr)
	}()
	return s
}

// MustStop stops the server.
func (s *Server) MustStop() {
	logger.Infof("stopping TCP Graphite server at %q...", s.addr)
	if err := s.lnTCP.Close(); err != nil {
		logger.Errorf("cannot close TCP Graphite server: %s", err)
	}
	logger.Infof("stopping UDP Graphite server at %q...", s.addr)
	if err := s.lnUDP.Close(); err != nil {
		logger.Errorf("cannot close UDP Graphite server: %s", err)
	}
	s.cm.CloseAll(0)
	s.wg.Wait()
	logger.Infof("TCP and UDP Graphite servers at %q have been stopped", s.addr)
}

func (s *Server) serveTCP(insertHandler func(r io.Reader) error) {
	var wg sync.WaitGroup
	for {
		c, err := s.lnTCP.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) {
				if ne.Temporary() {
					logger.Errorf("graphite: temporary error when listening for TCP addr %q: %s", s.lnTCP.Addr(), err)
					time.Sleep(time.Second)
					continue
				}
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				logger.Fatalf("unrecoverable error when accepting TCP Graphite connections: %s", err)
			}
			logger.Fatalf("unexpected error when accepting TCP Graphite connections: %s", err)
		}
		if !s.cm.Add(c) {
			_ = c.Close()
			break
		}
		wg.Add(1)
		go func() {
			defer func() {
				s.cm.Delete(c)
				_ = c.Close()
				wg.Done()
			}()
			writeRequestsTCP.Inc()
			if err := insertHandler(c); err != nil {
				writeErrorsTCP.Inc()
				logger.Errorf("error in TCP Graphite conn %q<->%q: %s", c.LocalAddr(), c.RemoteAddr(), err)
			}
		}()
	}
	wg.Wait()
}

func (s *Server) serveUDP(insertHandler func(r io.Reader) error) {
	gomaxprocs := cgroup.AvailableCPUs()
	var wg sync.WaitGroup
	for i := 0; i < gomaxprocs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var bb bytesutil.ByteBuffer
			bb.B = bytesutil.ResizeNoCopyNoOverallocate(bb.B, 64*1024)
			for {
				bb.Reset()
				bb.B = bb.B[:cap(bb.B)]
				n, addr, err := s.lnUDP.ReadFrom(bb.B)
				if err != nil {
					writeErrorsUDP.Inc()
					var ne net.Error
					if errors.As(err, &ne) {
						if ne.Temporary() {
							logger.Errorf("graphite: temporary error when listening for UDP addr %q: %s", s.lnUDP.LocalAddr(), err)
							time.Sleep(time.Second)
							continue
						}
						if strings.Contains(err.Error(), "use of closed network connection") {
							break
						}
					}
					logger.Errorf("cannot read Graphite UDP data: %s", err)
					continue
				}
				bb.B = bb.B[:n]
				writeRequestsUDP.Inc()
				if err := insertHandler(bb.NewReader()); err != nil {
					writeErrorsUDP.Inc()
					logger.Errorf("error in UDP Graphite conn %q<->%q: %s", s.lnUDP.LocalAddr(), addr, err)
					continue
				}
			}
		}()
	}
	wg.Wait()
}
