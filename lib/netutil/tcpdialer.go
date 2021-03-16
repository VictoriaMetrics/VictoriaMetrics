package netutil

import (
	"fmt"
	"net"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

// NewTCPDialer returns new dialer for dialing the given addr.
//
// The name is used in metric tags for the returned dialer.
// The name must be unique among dialers.
func NewTCPDialer(name, addr string) *TCPDialer {
	d := &TCPDialer{
		d: &net.Dialer{
			Timeout:   time.Second,
			KeepAlive: time.Second,
		},

		addr: addr,

		dials:      metrics.NewCounter(fmt.Sprintf(`vm_tcpdialer_dials_total{name=%q, addr=%q}`, name, addr)),
		dialErrors: metrics.NewCounter(fmt.Sprintf(`vm_tcpdialer_errors_total{name=%q, addr=%q, type="dial"}`, name, addr)),
	}
	d.connMetrics.init("vm_tcpdialer", name, addr)
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
