package netutil

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var enableTCP6 = flag.Bool("enableTCP6", false, "Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP is used")

// NewTCPListener returns new TCP listener for the given addr.
//
// name is used for exported metrics. Each listener in the program must have
// distinct name.
func NewTCPListener(name, addr string) (*TCPListener, error) {
	network := getNetwork()
	ln, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	tln := &TCPListener{
		Listener: ln,

		accepts:      metrics.NewCounter(fmt.Sprintf(`vm_tcplistener_accepts_total{name=%q, addr=%q}`, name, addr)),
		acceptErrors: metrics.NewCounter(fmt.Sprintf(`vm_tcplistener_errors_total{name=%q, addr=%q, type="accept"}`, name, addr)),
	}
	tln.connMetrics.init("vm_tcplistener", name, addr)
	return tln, err
}

// TCP6Enabled returns true if dialing and listening for IPv4 TCP is enabled.
func TCP6Enabled() bool {
	return *enableTCP6
}

func getNetwork() string {
	if *enableTCP6 {
		// Enable both tcp4 and tcp6
		return "tcp"
	}
	return "tcp4"
}

// TCPListener listens for the addr passed to NewTCPListener.
//
// It also gathers various stats for the accepted connections.
type TCPListener struct {
	net.Listener

	accepts      *metrics.Counter
	acceptErrors *metrics.Counter

	connMetrics
}

// Accept accepts connections from the addr passed to NewTCPListener.
func (ln *TCPListener) Accept() (net.Conn, error) {
	for {
		conn, err := ln.Listener.Accept()
		ln.accepts.Inc()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				logger.Errorf("temporary error when listening for TCP addr %q: %s", ln.Addr(), err)
				time.Sleep(time.Second)
				continue
			}
			ln.acceptErrors.Inc()
			return nil, err
		}
		ln.conns.Inc()
		sc := &statConn{
			Conn: conn,
			cm:   &ln.connMetrics,
		}
		return sc, nil
	}
}
