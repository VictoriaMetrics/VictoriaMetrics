package graphite

import (
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequestsTCP = metrics.NewCounter(`vm_graphite_requests_total{name="write", net="tcp"}`)
	writeErrorsTCP   = metrics.NewCounter(`vm_graphite_request_errors_total{name="write", net="tcp"}`)

	writeRequestsUDP = metrics.NewCounter(`vm_graphite_requests_total{name="write", net="udp"}`)
	writeErrorsUDP   = metrics.NewCounter(`vm_graphite_request_errors_total{name="write", net="udp"}`)
)

// Serve starts graphite server on the given addr.
func Serve(addr string) {
	logger.Infof("starting TCP Graphite server at %q", addr)
	lnTCP, err := netutil.NewTCPListener("graphite", addr)
	if err != nil {
		logger.Fatalf("cannot start TCP Graphite server at %q: %s", addr, err)
	}
	listenerTCP = lnTCP

	logger.Infof("starting UDP Graphite server at %q", addr)
	lnUDP, err := net.ListenPacket("udp4", addr)
	if err != nil {
		logger.Fatalf("cannot start UDP Graphite server at %q: %s", addr, err)
	}
	listenerUDP = lnUDP

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		serveTCP(listenerTCP)
		logger.Infof("stopped TCP Graphite server at %q", addr)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		serveUDP(listenerUDP)
		logger.Infof("stopped UDP Graphite server at %q", addr)
	}()
	wg.Wait()
}

func serveTCP(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
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
		go func() {
			writeRequestsTCP.Inc()
			var at auth.Token // TODO: properly initialize auth token
			if err := insertHandler(&at, c); err != nil {
				writeErrorsTCP.Inc()
				logger.Errorf("error in TCP Graphite conn %q<->%q: %s", c.LocalAddr(), c.RemoteAddr(), err)
			}
			_ = c.Close()
		}()
	}
}

func serveUDP(ln net.PacketConn) {
	gomaxprocs := runtime.GOMAXPROCS(-1)
	var wg sync.WaitGroup
	for i := 0; i < gomaxprocs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var bb bytesutil.ByteBuffer
			bb.B = bytesutil.Resize(bb.B, 64*1024)
			var at auth.Token // TODO: properly initialize auth token
			for {
				bb.Reset()
				bb.B = bb.B[:cap(bb.B)]
				n, addr, err := ln.ReadFrom(bb.B)
				if err != nil {
					writeErrorsUDP.Inc()
					if ne, ok := err.(net.Error); ok {
						if ne.Temporary() {
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
				if err := insertHandler(&at, bb.NewReader()); err != nil {
					writeErrorsUDP.Inc()
					logger.Errorf("error in UDP Graphite conn %q<->%q: %s", ln.LocalAddr(), addr, err)
					continue
				}
			}
		}()
	}
	wg.Wait()
}

var (
	listenerTCP net.Listener
	listenerUDP net.PacketConn
)

// Stop stops the server.
func Stop() {
	logger.Infof("stopping TCP Graphite server at %q...", listenerTCP.Addr())
	if err := listenerTCP.Close(); err != nil {
		logger.Errorf("cannot close TCP Graphite server: %s", err)
	}
	logger.Infof("stopping UDP Graphite server at %q...", listenerUDP.LocalAddr())
	if err := listenerUDP.Close(); err != nil {
		logger.Errorf("cannot close UDP Graphite server: %s", err)
	}
}
