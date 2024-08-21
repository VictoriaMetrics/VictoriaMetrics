package netutil

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

// NewStatDialFuncWithDial returns dialer function that registers stats metrics for conns.
func NewStatDialFuncWithDial(metricPrefix string, dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newStatDialFunc(metricPrefix, dialFunc)
}

// NewStatDialFunc returns dialer function that supports DNS SRV records and registers stats metrics for conns.
func NewStatDialFunc(metricPrefix string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newStatDialFunc(metricPrefix, DialMaybeSRV)
}

func newStatDialFunc(metricPrefix string, dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		sc := &statDialConn{
			dialsTotal: metrics.GetOrCreateCounter(fmt.Sprintf(`%s_dials_total`, metricPrefix)),
			dialErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`%s_dial_errors_total`, metricPrefix)),
			conns:      metrics.GetOrCreateGauge(fmt.Sprintf(`%s_conns`, metricPrefix), nil),

			readsTotal:        metrics.GetOrCreateCounter(fmt.Sprintf(`%s_conn_reads_total`, metricPrefix)),
			writesTotal:       metrics.GetOrCreateCounter(fmt.Sprintf(`%s_conn_writes_total`, metricPrefix)),
			readErrorsTotal:   metrics.GetOrCreateCounter(fmt.Sprintf(`%s_conn_read_errors_total`, metricPrefix)),
			writeErrorsTotal:  metrics.GetOrCreateCounter(fmt.Sprintf(`%s_conn_write_errors_total`, metricPrefix)),
			bytesReadTotal:    metrics.GetOrCreateCounter(fmt.Sprintf(`%s_conn_bytes_read_total`, metricPrefix)),
			bytesWrittenTotal: metrics.GetOrCreateCounter(fmt.Sprintf(`%s_conn_bytes_written_total`, metricPrefix)),
		}

		network := GetTCPNetwork()
		conn, err := dialFunc(ctx, network, addr)
		sc.dialsTotal.Inc()
		if err != nil {
			sc.dialErrors.Inc()
			if !TCP6Enabled() && !isTCPv4Addr(addr) {
				err = fmt.Errorf("%w; try -enableTCP6 command-line flag for dialing ipv6 addresses", err)
			}
			return nil, err
		}
		sc.Conn = conn
		sc.conns.Inc()
		return sc, nil
	}
}

type statDialConn struct {
	closed atomic.Int32
	net.Conn

	dialsTotal *metrics.Counter
	dialErrors *metrics.Counter
	conns      *metrics.Gauge

	readsTotal        *metrics.Counter
	writesTotal       *metrics.Counter
	readErrorsTotal   *metrics.Counter
	writeErrorsTotal  *metrics.Counter
	bytesReadTotal    *metrics.Counter
	bytesWrittenTotal *metrics.Counter
}

func (sc *statDialConn) Read(p []byte) (int, error) {
	n, err := sc.Conn.Read(p)
	sc.readsTotal.Inc()
	if err != nil {
		sc.readErrorsTotal.Inc()
	}
	sc.bytesReadTotal.Add(n)
	return n, err
}

func (sc *statDialConn) Write(p []byte) (int, error) {
	n, err := sc.Conn.Write(p)
	sc.writesTotal.Inc()
	if err != nil {
		sc.writeErrorsTotal.Inc()
	}
	sc.bytesWrittenTotal.Add(n)
	return n, err
}

func (sc *statDialConn) Close() error {
	err := sc.Conn.Close()
	if sc.closed.Add(1) == 1 {
		sc.conns.Dec()
	}
	return err
}

func isTCPv4Addr(addr string) bool {
	s := addr
	for i := 0; i < 3; i++ {
		n := strings.IndexByte(s, '.')
		if n < 0 {
			return false
		}
		if !isUint8NumString(s[:n]) {
			return false
		}
		s = s[n+1:]
	}
	n := strings.IndexByte(s, ':')
	if n < 0 {
		return false
	}
	if !isUint8NumString(s[:n]) {
		return false
	}
	s = s[n+1:]

	// Verify TCP port
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return n >= 0 && n < (1<<16)
}

func isUint8NumString(s string) bool {
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return n >= 0 && n < (1<<8)
}
