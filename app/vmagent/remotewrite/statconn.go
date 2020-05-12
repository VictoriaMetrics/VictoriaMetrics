package remotewrite

import (
	"net"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/fasthttp"
	"github.com/VictoriaMetrics/metrics"
)

func statDial(addr string) (conn net.Conn, err error) {
	if netutil.TCP6Enabled() {
		conn, err = fasthttp.DialDualStack(addr)
	} else {
		conn, err = fasthttp.Dial(addr)
	}
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

var (
	dialsTotal = metrics.NewCounter(`vmagent_remotewrite_dials_total`)
	dialErrors = metrics.NewCounter(`vmagent_remotewrite_dial_errors_total`)
	conns      = metrics.NewCounter(`vmagent_remotewrite_conns`)
)

type statConn struct {
	closed uint64
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
	if atomic.AddUint64(&sc.closed, 1) == 1 {
		conns.Dec()
	}
	return err
}

var (
	connReadsTotal   = metrics.NewCounter(`vmagent_remotewrite_conn_reads_total`)
	connWritesTotal  = metrics.NewCounter(`vmagent_remotewrite_conn_writes_total`)
	connReadErrors   = metrics.NewCounter(`vmagent_remotewrite_conn_read_errors_total`)
	connWriteErrors  = metrics.NewCounter(`vmagent_remotewrite_conn_write_errors_total`)
	connBytesRead    = metrics.NewCounter(`vmagent_remotewrite_conn_bytes_read_total`)
	connBytesWritten = metrics.NewCounter(`vmagent_remotewrite_conn_bytes_written_total`)
)
