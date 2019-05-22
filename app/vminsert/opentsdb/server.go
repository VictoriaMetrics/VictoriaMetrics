package opentsdb

import (
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequestsTCP = metrics.NewCounter(`vm_opentsdb_requests_total{name="write", net="tcp"}`)
	writeErrorsTCP   = metrics.NewCounter(`vm_opentsdb_request_errors_total{name="write", net="tcp"}`)

	writeRequestsUDP = metrics.NewCounter(`vm_opentsdb_requests_total{name="write", net="udp"}`)
	writeErrorsUDP   = metrics.NewCounter(`vm_opentsdb_request_errors_total{name="write", net="udp"}`)
)

// Serve starts OpenTSDB collector on the given addr.
func Serve(addr string) {
	logger.Infof("starting TCP OpenTSDB collector at %q", addr)
	lnTCP, err := net.Listen("tcp4", addr)
	if err != nil {
		logger.Fatalf("cannot start TCP OpenTSDB collector at %q: %s", addr, err)
	}
	listenerTCP = lnTCP

	logger.Infof("starting UDP OpenTSDB collector at %q", addr)
	lnUDP, err := net.ListenPacket("udp4", addr)
	if err != nil {
		logger.Fatalf("cannot start UDP OpenTSDB collector at %q: %s", addr, err)
	}
	listenerUDP = lnUDP

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		serveTCP(listenerTCP)
		logger.Infof("stopped TCP OpenTSDB collector at %q", addr)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		serveUDP(listenerUDP)
		logger.Infof("stopped UDP OpenTSDB collector at %q", addr)
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

func serveUDP(ln net.PacketConn) {
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
					if ne, ok := err.(net.Error); ok {
						if ne.Temporary() {
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

var (
	listenerTCP net.Listener
	listenerUDP net.PacketConn
)

// Stop stops the server.
func Stop() {
	logger.Infof("stopping TCP OpenTSDB server at %q...", listenerTCP.Addr())
	if err := listenerTCP.Close(); err != nil {
		logger.Errorf("cannot close TCP OpenTSDB server: %s", err)
	}
	logger.Infof("stopping UDP OpenTSDB server at %q...", listenerUDP.LocalAddr())
	if err := listenerUDP.Close(); err != nil {
		logger.Errorf("cannot close UDP OpenTSDB server: %s", err)
	}
}
