package opentsdb

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequestsTCP = metrics.NewCounter(`vm_ingestserver_requests_total{type="opentsdb", name="write", net="tcp"}`)
	writeErrorsTCP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="opentsdb", name="write", net="tcp"}`)

	writeRequestsUDP = metrics.NewCounter(`vm_ingestserver_requests_total{type="opentsdb", name="write", net="udp"}`)
	writeErrorsUDP   = metrics.NewCounter(`vm_ingestserver_request_errors_total{type="opentsdb", name="write", net="udp"}`)
)

// Server is a server for collecting OpenTSDB TCP and UDP metrics.
//
// It accepts simultaneously Telnet put requests and HTTP put requests over TCP.
type Server struct {
	addr       string
	ls         *listenerSwitch
	httpServer *opentsdbhttp.Server
	lnUDP      net.PacketConn
	wg         sync.WaitGroup
	cm         ingestserver.ConnsMap
}

// MustStart starts OpenTSDB collector on the given addr.
//
// If useProxyProtocol is set to true, then the incoming connections are accepted via proxy protocol.
// See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
//
// MustStop must be called on the returned server when it is no longer needed.
func MustStart(addr string, useProxyProtocol bool, telnetInsertHandler func(r io.Reader) error, httpInsertHandler func(req *http.Request) error) *Server {
	logger.Infof("starting TCP OpenTSDB collector at %q", addr)
	lnTCP, err := netutil.NewTCPListener("opentsdb", addr, useProxyProtocol, nil)
	if err != nil {
		logger.Fatalf("cannot start TCP OpenTSDB collector at %q: %s", addr, err)
	}
	ls := newListenerSwitch(lnTCP)
	lnHTTP := ls.newHTTPListener()
	lnTelnet := ls.newTelnetListener()
	httpServer := opentsdbhttp.MustServe(lnHTTP, httpInsertHandler)

	logger.Infof("starting UDP OpenTSDB collector at %q", addr)
	lnUDP, err := net.ListenPacket(netutil.GetUDPNetwork(), addr)
	if err != nil {
		logger.Fatalf("cannot start UDP OpenTSDB collector at %q: %s", addr, err)
	}

	s := &Server{
		addr:       addr,
		ls:         ls,
		httpServer: httpServer,
		lnUDP:      lnUDP,
	}
	s.cm.Init("opentsdb")
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.serveTelnet(lnTelnet, telnetInsertHandler)
		logger.Infof("stopped TCP telnet OpenTSDB server at %q", addr)
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		httpServer.Wait()
		// Do not log when httpServer is stopped, since this is logged by the server itself.
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.serveUDP(telnetInsertHandler)
		logger.Infof("stopped UDP OpenTSDB server at %q", addr)
	}()
	return s
}

// MustStop stops the server.
func (s *Server) MustStop() {
	// Stop HTTP server. Do not emit log message, since it is emitted by the httpServer.
	s.httpServer.MustStop()

	logger.Infof("stopping TCP telnet OpenTSDB server at %q...", s.addr)
	if err := s.ls.stop(); err != nil {
		logger.Errorf("cannot stop TCP telnet OpenTSDB server: %s", err)
	}

	logger.Infof("stopping UDP OpenTSDB server at %q...", s.addr)
	if err := s.lnUDP.Close(); err != nil {
		logger.Errorf("cannot stop UDP OpenTSDB server: %s", err)
	}
	s.cm.CloseAll(0)
	s.wg.Wait()
	logger.Infof("TCP and UDP OpenTSDB servers at %q have been stopped", s.addr)
}

func (s *Server) serveTelnet(ln net.Listener, insertHandler func(r io.Reader) error) {
	var wg sync.WaitGroup
	for {
		c, err := ln.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) {
				if ne.Temporary() {
					logger.Errorf("opentsdb: temporary error when listening for TCP addr %q: %s", ln.Addr(), err)
					time.Sleep(time.Second)
					continue
				}
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				logger.Fatalf("unrecoverable error when accepting TCP OpenTSDB connections: %s", err)
			}
			logger.Fatalf("unexpected error when accepting TCP OpenTSDB connections: %s", err)
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
				logger.Errorf("error in TCP OpenTSDB conn %q<->%q: %s", c.LocalAddr(), c.RemoteAddr(), err)
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
							logger.Errorf("opentsdb: temporary error when listening for UDP addr %q: %s", s.lnUDP.LocalAddr(), err)
							time.Sleep(time.Second)
							continue
						}
						if strings.Contains(err.Error(), "use of closed network connection") {
							break
						}
					}
					logger.Errorf("cannot read OpenTSDB UDP data: %s", err)
					continue
				}
				bb.B = bb.B[:n]
				writeRequestsUDP.Inc()
				if err := insertHandler(bb.NewReader()); err != nil {
					writeErrorsUDP.Inc()
					logger.Errorf("error in UDP OpenTSDB conn %q<->%q: %s", s.lnUDP.LocalAddr(), addr, err)
					continue
				}
			}
		}()
	}
	wg.Wait()
}
