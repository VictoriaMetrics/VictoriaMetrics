package netutil

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

// NewTCPDialer returns new dialer for dialing the given addr.
//
// The name is used in metric tags for the returned dialer.
// The name must be unique among dialers.
func NewTCPDialer(ms *metrics.Set, name, addr string, dialTimeout time.Duration) *TCPDialer {
	d := &TCPDialer{
		d: &net.Dialer{
			Timeout: dialTimeout,

			// How frequently to send keep-alive packets over established TCP connections.
			KeepAlive: time.Second,
		},

		addr: addr,

		dials:      ms.NewCounter(fmt.Sprintf(`vm_tcpdialer_dials_total{name=%q, addr=%q}`, name, addr)),
		dialErrors: ms.NewCounter(fmt.Sprintf(`vm_tcpdialer_errors_total{name=%q, addr=%q, type="dial"}`, name, addr)),
	}
	d.connMetrics.init(ms, "vm_tcpdialer", name, addr)
	d.d.Control = func(network, address string, c syscall.RawConn) (err error) {
		controlErr := c.Control(func(fd uintptr) {
			err = setTCPUserTimeout(fd, dialTimeout)
		})
		if controlErr != nil {
			return controlErr
		}
		return err
	}

	return d
}

// TCPDialer is used for dialing the addr passed to NewTCPDialer.
//
// It also gathers various stats for dialed connections.
type TCPDialer struct {
	d *net.Dialer

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
