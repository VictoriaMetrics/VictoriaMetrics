package netutil

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

type connMetrics struct {
	readCalls    *metrics.Counter
	readBytes    *metrics.Counter
	readErrors   *metrics.Counter
	readTimeouts *metrics.Counter

	writeCalls    *metrics.Counter
	writtenBytes  *metrics.Counter
	writeErrors   *metrics.Counter
	writeTimeouts *metrics.Counter

	closeErrors *metrics.Counter

	conns *metrics.Counter
}

func (cm *connMetrics) init(group, name, addr string) {
	cm.readCalls = metrics.NewCounter(fmt.Sprintf(`%s_read_calls_total{name=%q, addr=%q}`, group, name, addr))
	cm.readBytes = metrics.NewCounter(fmt.Sprintf(`%s_read_bytes_total{name=%q, addr=%q}`, group, name, addr))
	cm.readErrors = metrics.NewCounter(fmt.Sprintf(`%s_errors_total{name=%q, addr=%q, type="read"}`, group, name, addr))
	cm.readTimeouts = metrics.NewCounter(fmt.Sprintf(`%s_read_timeouts_total{name=%q, addr=%q}`, group, name, addr))

	cm.writeCalls = metrics.NewCounter(fmt.Sprintf(`%s_write_calls_total{name=%q, addr=%q}`, group, name, addr))
	cm.writtenBytes = metrics.NewCounter(fmt.Sprintf(`%s_written_bytes_total{name=%q, addr=%q}`, group, name, addr))
	cm.writeErrors = metrics.NewCounter(fmt.Sprintf(`%s_errors_total{name=%q, addr=%q, type="write"}`, group, name, addr))
	cm.writeTimeouts = metrics.NewCounter(fmt.Sprintf(`%s_write_timeouts_total{name=%q, addr=%q}`, group, name, addr))

	cm.closeErrors = metrics.NewCounter(fmt.Sprintf(`%s_errors_total{name=%q, addr=%q, type="close"}`, group, name, addr))

	cm.conns = metrics.NewCounter(fmt.Sprintf(`%s_conns{name=%q, addr=%q}`, group, name, addr))
}

type statConn struct {
	readTimeout  time.Duration
	lastReadTime time.Time

	writeTimeout  time.Duration
	lastWriteTime time.Time

	net.Conn

	cm *connMetrics

	closeCalls uint64
}

func (sc *statConn) Read(p []byte) (int, error) {
	if sc.readTimeout > 0 {
		t := time.Now()
		if t.Sub(sc.lastReadTime) > sc.readTimeout>>4 {
			d := t.Add(sc.readTimeout)
			if err := sc.Conn.SetReadDeadline(d); err != nil {
				// This error may occur when the client closes the connection before setting the deadline
				return 0, err
			}
		}
	}

	n, err := sc.Conn.Read(p)
	sc.cm.readCalls.Inc()
	sc.cm.readBytes.Add(n)
	if err != nil && err != io.EOF {
		sc.cm.readErrors.Inc()
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			sc.cm.readTimeouts.Inc()
		}
	}
	return n, err
}

func (sc *statConn) Write(p []byte) (int, error) {
	if sc.writeTimeout > 0 {
		t := time.Now()
		if t.Sub(sc.lastWriteTime) > sc.writeTimeout>>4 {
			d := t.Add(sc.writeTimeout)
			if err := sc.Conn.SetWriteDeadline(d); err != nil {
				// This error may accour when the client closes the connection before setting the deadline
				return 0, err
			}
		}
	}

	n, err := sc.Conn.Write(p)
	sc.cm.writeCalls.Inc()
	sc.cm.writtenBytes.Add(n)
	if err != nil {
		sc.cm.writeErrors.Inc()
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			sc.cm.writeTimeouts.Inc()
		}
	}
	return n, err
}

func (sc *statConn) Close() error {
	n := atomic.AddUint64(&sc.closeCalls, 1)
	if n > 1 {
		// The connection has been already closed.
		return nil
	}
	err := sc.Conn.Close()
	sc.cm.conns.Dec()
	if err != nil {
		sc.cm.closeErrors.Inc()
	}
	return err
}
