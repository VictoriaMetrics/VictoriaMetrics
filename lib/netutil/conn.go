package netutil

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"

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
	// Move atomic counters to the top of struct in order to properly align them on 32-bit arch.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	closeCalls uint64

	net.Conn

	cm *connMetrics
}

func (sc *statConn) Read(p []byte) (int, error) {
	n, err := sc.Conn.Read(p)
	sc.cm.readCalls.Inc()
	sc.cm.readBytes.Add(n)
	if err != nil && err != io.EOF {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			sc.cm.readTimeouts.Inc()
		} else {
			sc.cm.readErrors.Inc()
		}
	}
	return n, err
}

func (sc *statConn) Write(p []byte) (int, error) {
	n, err := sc.Conn.Write(p)
	sc.cm.writeCalls.Inc()
	sc.cm.writtenBytes.Add(n)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			sc.cm.writeTimeouts.Inc()
		} else {
			sc.cm.writeErrors.Inc()
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
