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

// NewStatDialFuncWithDial returns a dialer function that registers stats metrics for conns.
func NewStatDialFuncWithDial(metricPrefix string, dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newStatDialFunc(metricPrefix, "", dialFunc)
}

// NewStatDialFunc returns a dialer function that supports DNS SRV records and registers stats metrics for conns.
func NewStatDialFunc(metricPrefix string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newStatDialFunc(metricPrefix, "", DialMaybeSRV)
}

// NewStatDialFuncWithLabels returns a dialer function that supports DNS SRV records and registers stats metrics for conns.
//
// metricLabels are appended to each metric name and must have the form `{label1="value1", ...}`.
func NewStatDialFuncWithLabels(metricPrefix, metricLabels string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newStatDialFunc(metricPrefix, metricLabels, DialMaybeSRV)
}

func newStatDialFunc(metricPrefix, metricLabels string, dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	sm := &statDialMetrics{
		dialsTotal: metrics.GetOrCreateCounter(metricPrefix + `_dials_total` + metricLabels),
		dialErrors: metrics.GetOrCreateCounter(metricPrefix + `_dial_errors_total` + metricLabels),
		conns:      metrics.GetOrCreateGauge(metricPrefix+`_conns`+metricLabels, nil),

		readsTotal:        metrics.GetOrCreateCounter(metricPrefix + `_conn_reads_total` + metricLabels),
		writesTotal:       metrics.GetOrCreateCounter(metricPrefix + `_conn_writes_total` + metricLabels),
		readErrorsTotal:   metrics.GetOrCreateCounter(metricPrefix + `_conn_read_errors_total` + metricLabels),
		writeErrorsTotal:  metrics.GetOrCreateCounter(metricPrefix + `_conn_write_errors_total` + metricLabels),
		bytesReadTotal:    metrics.GetOrCreateCounter(metricPrefix + `_conn_bytes_read_total` + metricLabels),
		bytesWrittenTotal: metrics.GetOrCreateCounter(metricPrefix + `_conn_bytes_written_total` + metricLabels),
	}

	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		network := GetTCPNetwork()
		conn, err := dialFunc(ctx, network, addr)
		sm.dialsTotal.Inc()
		if err != nil {
			sm.dialErrors.Inc()
			if !TCP6Enabled() && !isTCPv4Addr(addr) {
				err = fmt.Errorf("%w; try -enableTCP6 command-line flag for dialing ipv6 addresses", err)
			}
			return nil, err
		}
		sc := &statDialConn{
			Conn: conn,
			sm:   sm,
		}
		sm.conns.Inc()
		return sc, nil
	}
}

type statDialMetrics struct {
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

type statDialConn struct {
	closed atomic.Int32
	net.Conn

	sm *statDialMetrics
}

func (sc *statDialConn) Read(p []byte) (int, error) {
	n, err := sc.Conn.Read(p)
	sc.sm.readsTotal.Inc()
	if err != nil {
		sc.sm.readErrorsTotal.Inc()
	}
	sc.sm.bytesReadTotal.Add(n)
	return n, err
}

func (sc *statDialConn) Write(p []byte) (int, error) {
	n, err := sc.Conn.Write(p)
	sc.sm.writesTotal.Inc()
	if err != nil {
		sc.sm.writeErrorsTotal.Inc()
	}
	sc.sm.bytesWrittenTotal.Add(n)
	return n, err
}

func (sc *statDialConn) Close() error {
	err := sc.Conn.Close()
	if sc.closed.Add(1) == 1 {
		sc.sm.conns.Dec()
	}
	return err
}

func isTCPv4Addr(addr string) bool {
	s := addr
	for range 3 {
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
