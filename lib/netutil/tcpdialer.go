package netutil

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

// A dialer is a means to establish a connection.
type dialer interface {
	Dial(network, addr string) (c net.Conn, err error)
}

// NewTCPDialer returns new dialer for dialing the given addr.
//
// The name is used in metric tags for the returned dialer.
// The name must be unique among dialers.
func NewTCPDialer(ms *metrics.Set, name, addr string, dialTimeout, userTimeout time.Duration) *TCPDialer {
	nd := &net.Dialer{
		Timeout: dialTimeout,

		// How frequently to send keep-alive packets over established TCP connections.
		KeepAlive: time.Second,
	}
	d := &TCPDialer{
		d:    nd,
		addr: addr,

		dials:      ms.NewCounter(fmt.Sprintf(`vm_tcpdialer_dials_total{name=%q, addr=%q}`, name, addr)),
		dialErrors: ms.NewCounter(fmt.Sprintf(`vm_tcpdialer_errors_total{name=%q, addr=%q, type="dial"}`, name, addr)),
	}
	d.connMetrics.init(ms, "vm_tcpdialer", name, addr)
	if userTimeout > 0 {
		nd.Control = func(network, address string, c syscall.RawConn) error {
			var err error
			controlErr := c.Control(func(fd uintptr) {
				err = setTCPUserTimeout(fd, userTimeout)
			})
			if controlErr != nil {
				return controlErr
			}
			return err
		}
	}
	return d
}

// TCPDialer is used for dialing the addr passed to NewTCPDialer.
//
// It also gathers various stats for dialed connections.
type TCPDialer struct {
	d dialer

	addr string

	dials      *metrics.Counter
	dialErrors *metrics.Counter

	connMetrics
}

// Dial dials the addr passed to NewTCPDialer.
func (d *TCPDialer) Dial() (net.Conn, error) {
	d.dials.Inc()
	network := GetTCPNetwork()
	c, err := d.d.Dial(network, d.addr)
	if err != nil {
		d.dialErrors.Inc()
		return nil, err
	}
	d.conns.Inc()
	sc := &statConn{
		Conn: c,
		cm:   &d.connMetrics,
	}
	return sc, err
}

// Addr returns the address the dialer dials to.
func (d *TCPDialer) Addr() string {
	return d.addr
}
