package netutil

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var enableTCP6 = flag.Bool("enableTCP6", false, "Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used")

// NewTCPListener returns new TCP listener for the given addr and optional tlsConfig.
//
// name is used for metrics. Each listener in the program must have a distinct name.
//
// If useProxyProtocol is set to true, then the returned listener accepts TCP connections via proxy protocol.
// See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
func NewTCPListener(name, addr string, useProxyProtocol bool, tlsConfig *tls.Config) (*TCPListener, error) {
	network := GetTCPNetwork()
	ln, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		ln = tls.NewListener(ln, tlsConfig)
	}
	ms := metrics.GetDefaultSet()
	tln := &TCPListener{
		Listener:         ln,
		useProxyProtocol: useProxyProtocol,

		accepts:      ms.NewCounter(fmt.Sprintf(`vm_tcplistener_accepts_total{name=%q, addr=%q}`, name, addr)),
		acceptErrors: ms.NewCounter(fmt.Sprintf(`vm_tcplistener_errors_total{name=%q, addr=%q, type="accept"}`, name, addr)),
	}
	tln.cm.init(ms, "vm_tcplistener", name, addr)
	return tln, err
}

// EnableIPv6 enables IPv6 for dialing and listening.
func EnableIPv6() {
	*enableTCP6 = true
}

// TCP6Enabled returns true if dialing and listening for IPv4 TCP is enabled.
func TCP6Enabled() bool {
	return *enableTCP6
}

// GetUDPNetwork returns current udp network.
func GetUDPNetwork() string {
	if *enableTCP6 {
		// Enable both udp4 and udp6
		return "udp"
	}
	return "udp4"
}

// GetTCPNetwork returns current tcp network.
func GetTCPNetwork() string {
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

	useProxyProtocol bool

	cm connMetrics
}

var proxyProtocolReadErrorLogger = logger.WithThrottler("proxyProtocolReadError", 5*time.Second)

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
		if ln.useProxyProtocol {
			pConn := newProxyProtocolConn(conn)
			conn = pConn
		}
		ln.cm.conns.Inc()
		sc := &statConn{
			Conn: conn,
			cm:   &ln.cm,
		}
		return sc, nil
	}
}
