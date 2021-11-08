package promscrape

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/fasthttp"
	"github.com/VictoriaMetrics/metrics"
)

func statStdDial(ctx context.Context, networkUnused, addr string) (net.Conn, error) {
	d := getStdDialer()
	network := netutil.GetTCPNetwork()
	conn, err := d.DialContext(ctx, network, addr)
	dialsTotal.Inc()
	if err != nil {
		dialErrors.Inc()
		if !netutil.TCP6Enabled() {
			err = fmt.Errorf("%w; try -enableTCP6 command-line flag if you scrape ipv6 addresses", err)
		}
		return nil, err
	}
	conns.Inc()
	sc := &statConn{
		Conn: conn,
	}
	return sc, nil
}

func getStdDialer() *net.Dialer {
	stdDialerOnce.Do(func() {
		stdDialer = &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: netutil.TCP6Enabled(),
		}
	})
	return stdDialer
}

var (
	stdDialer     *net.Dialer
	stdDialerOnce sync.Once
)

func newStatDialFunc(proxyURL *proxy.URL, ac *promauth.Config) (fasthttp.DialFunc, error) {
	dialFunc, err := proxyURL.NewDialFunc(ac)
	if err != nil {
		return nil, err
	}
	statDialFunc := func(addr string) (net.Conn, error) {
		conn, err := dialFunc(addr)
		dialsTotal.Inc()
		if err != nil {
			dialErrors.Inc()
			if !netutil.TCP6Enabled() {
				err = fmt.Errorf("%w; try -enableTCP6 command-line flag if you scrape ipv6 addresses", err)
			}
			return nil, err
		}
		conns.Inc()
		sc := &statConn{
			Conn: conn,
		}
		return sc, nil
	}
	return statDialFunc, nil
}

var (
	dialsTotal = metrics.NewCounter(`vm_promscrape_dials_total`)
	dialErrors = metrics.NewCounter(`vm_promscrape_dial_errors_total`)
	conns      = metrics.NewCounter(`vm_promscrape_conns`)
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
	connReadsTotal   = metrics.NewCounter(`vm_promscrape_conn_reads_total`)
	connWritesTotal  = metrics.NewCounter(`vm_promscrape_conn_writes_total`)
	connReadErrors   = metrics.NewCounter(`vm_promscrape_conn_read_errors_total`)
	connWriteErrors  = metrics.NewCounter(`vm_promscrape_conn_write_errors_total`)
	connBytesRead    = metrics.NewCounter(`vm_promscrape_conn_bytes_read_total`)
	connBytesWritten = metrics.NewCounter(`vm_promscrape_conn_bytes_written_total`)
)
