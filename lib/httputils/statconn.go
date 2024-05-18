package httputils

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	dialsTotal *metrics.Counter
	dialErrors *metrics.Counter
	conns      *metrics.Counter

	connReadsTotal   *metrics.Counter
	connWritesTotal  *metrics.Counter
	connReadErrors   *metrics.Counter
	connWriteErrors  *metrics.Counter
	connBytesRead    *metrics.Counter
	connBytesWritten *metrics.Counter
)

var metricSetRegister sync.Once

// GetStatDialFunc returns dial function that supports DNS SRV records,
// and register stats metrics for conns.
func GetStatDialFunc(metricPrefix string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	metricSetRegister.Do(func() {
		dialsTotal = metrics.NewCounter(fmt.Sprintf(`%s_dials_total`, metricPrefix))
		dialErrors = metrics.NewCounter(fmt.Sprintf(`%s_dial_errors_total`, metricPrefix))
		conns = metrics.NewCounter(fmt.Sprintf(`%s_conns`, metricPrefix))

		connReadsTotal = metrics.NewCounter(fmt.Sprintf(`%s_conn_reads_total`, metricPrefix))
		connWritesTotal = metrics.NewCounter(fmt.Sprintf(`%s_conn_writes_total`, metricPrefix))
		connReadErrors = metrics.NewCounter(fmt.Sprintf(`%s_conn_read_errors_total`, metricPrefix))
		connWriteErrors = metrics.NewCounter(fmt.Sprintf(`%s_conn_write_errors_total`, metricPrefix))
		connBytesRead = metrics.NewCounter(fmt.Sprintf(`%s_conn_bytes_read_total`, metricPrefix))
		connBytesWritten = metrics.NewCounter(fmt.Sprintf(`%s_conn_bytes_written_total`, metricPrefix))
	})
	return statDial
}

func statDial(ctx context.Context, _, addr string) (conn net.Conn, err error) {
	network := netutil.GetTCPNetwork()
	conn, err = netutil.DialMaybeSRV(ctx, network, addr)
	dialsTotal.Inc()
	if err != nil {
		dialErrors.Inc()
		return nil, err
	}
	conns.Inc()
	sc := &statConn{
		Conn: conn,
	}
	return sc, nil
}

type statConn struct {
	closed atomic.Int32
	net.Conn
}

func (sc *statConn) Read(p []byte) (int, error) {
	n, err := sc.Conn.Read(p)
	connReadsTotal.Inc()
	if err != nil {
		connReadErrors.Inc()
	}
	connBytesRead.Add(n)
	return n, err
}

func (sc *statConn) Write(p []byte) (int, error) {
	n, err := sc.Conn.Write(p)
	connWritesTotal.Inc()
	if err != nil {
		connWriteErrors.Inc()
	}
	connBytesWritten.Add(n)
	return n, err
}

func (sc *statConn) Close() error {
	err := sc.Conn.Close()
	if sc.closed.Add(1) == 1 {
		conns.Dec()
	}
	return err
}
