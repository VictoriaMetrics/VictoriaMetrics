package opentsdb

import (
	"errors"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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
}

// MustStart starts OpenTSDB collector on the given addr.
//
// MustStop must be called on the returned server when it is no longer needed.
func MustStart(addr string, telnetInsertHandler func(r io.Reader) error, httpInsertHandler func(req *http.Request) error) *Server {
	logger.Infof("starting TCP OpenTSDB collector at %q", addr)
	lnTCP, err := netutil.NewTCPListener("opentsdb", addr)
	if err != nil {
		logger.Fatalf("cannot start TCP OpenTSDB collector at %q: %s", addr, err)
	}
	ls := newListenerSwitch(lnTCP)
	lnHTTP := ls.newHTTPListener()
	lnTelnet := ls.newTelnetListener()
	httpServer := opentsdbhttp.MustServe(lnHTTP, httpInsertHandler)

	logger.Infof("starting UDP OpenTSDB collector at %q", addr)
	lnUDP, err := net.ListenPacket("udp4", addr)
	if err != nil {
		logger.Fatalf("cannot start UDP OpenTSDB collector at %q: %s", addr, err)
	}

	s := &Server{
		addr:       addr,
		ls:         ls,
		httpServer: httpServer,
		lnUDP:      lnUDP,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		serveTelnet(lnTelnet, telnetInsertHandler)
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
		serveUDP(lnUDP, telnetInsertHandler)
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

	// Wait until all the servers are stopped.
	s.wg.Wait()
	logger.Infof("TCP and UDP OpenTSDB servers at %q have been stopped", s.addr)
}

func serveTelnet(ln net.Listener, insertHandler func(r io.Reader) error) {
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
		go func() {
			writeRequestsTCP.Inc()
			if err := insertHandler(c); err != nil {
				writeErrorsTCP.Inc()
				logger.Errorf("error in TCP OpenTSDB conn %q<->%q: %s", c.LocalAddr(), c.RemoteAddr(), err)
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
							logger.Errorf("opentsdb: temporary error when listening for UDP addr %q: %s", ln.LocalAddr(), err)
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
					logger.Errorf("error in UDP OpenTSDB conn %q<->%q: %s", ln.LocalAddr(), addr, err)
					continue
				}
			}
		}()
	}
	wg.Wait()
}
