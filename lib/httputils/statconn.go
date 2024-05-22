package httputils

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var statConnMetricsRegistry sync.Map

type statConnMetrics struct {
	dialsTotal *metrics.Counter
	dialErrors *metrics.Counter
	conns      *metrics.Counter

	connReadsTotal   *metrics.Counter
	connWritesTotal  *metrics.Counter
	connReadErrors   *metrics.Counter
	connWriteErrors  *metrics.Counter
	connBytesRead    *metrics.Counter
	connBytesWritten *metrics.Counter
}

func newStatConnMetrics(metricPrefix string) statConnMetrics {
	scm := statConnMetrics{}

	scm.dialsTotal = metrics.NewCounter(fmt.Sprintf(`%s_dials_total`, metricPrefix))
	scm.dialErrors = metrics.NewCounter(fmt.Sprintf(`%s_dial_errors_total`, metricPrefix))
	scm.conns = metrics.NewCounter(fmt.Sprintf(`%s_conns`, metricPrefix))

	scm.connReadsTotal = metrics.NewCounter(fmt.Sprintf(`%s_conn_reads_total`, metricPrefix))
	scm.connWritesTotal = metrics.NewCounter(fmt.Sprintf(`%s_conn_writes_total`, metricPrefix))
	scm.connReadErrors = metrics.NewCounter(fmt.Sprintf(`%s_conn_read_errors_total`, metricPrefix))
	scm.connWriteErrors = metrics.NewCounter(fmt.Sprintf(`%s_conn_write_errors_total`, metricPrefix))
	scm.connBytesRead = metrics.NewCounter(fmt.Sprintf(`%s_conn_bytes_read_total`, metricPrefix))
	scm.connBytesWritten = metrics.NewCounter(fmt.Sprintf(`%s_conn_bytes_written_total`, metricPrefix))

	return scm
}

// GetStatDialFunc returns dial function that supports DNS SRV records,
// and register stats metrics for conns.
func GetStatDialFunc(metricPrefix string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	v, ok := statConnMetricsRegistry.Load(metricPrefix)
	if !ok {
		v = newStatConnMetrics(metricPrefix)
		statConnMetricsRegistry.Store(metricPrefix, v)
	}
	sm := v.(statConnMetrics)
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		network := netutil.GetTCPNetwork()
		conn, err := netutil.DialMaybeSRV(ctx, network, addr)
		sm.dialsTotal.Inc()
		if err != nil {
			sm.dialErrors.Inc()
			if !netutil.TCP6Enabled() && !isTCPv4Addr(addr) {
				err = fmt.Errorf("%w; try -enableTCP6 command-line flag for dialing ipv6 addresses", err)
			}
			return nil, err
		}
		sm.conns.Inc()
		sc := &statConn{
			Conn:            conn,
			statConnMetrics: sm,
		}
		return sc, nil
	}
}

type statConn struct {
	closed atomic.Int32
	net.Conn
	statConnMetrics
}

func (sc *statConn) Read(p []byte) (int, error) {
	n, err := sc.Conn.Read(p)
	sc.connReadsTotal.Inc()
	if err != nil {
		sc.connReadErrors.Inc()
	}
	sc.connBytesRead.Add(n)
	return n, err
}

func (sc *statConn) Write(p []byte) (int, error) {
	n, err := sc.Conn.Write(p)
	sc.connWritesTotal.Inc()
	if err != nil {
		sc.connWriteErrors.Inc()
	}
	sc.connBytesWritten.Add(n)
	return n, err
}

func (sc *statConn) Close() error {
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
