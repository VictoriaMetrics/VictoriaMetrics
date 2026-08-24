package netutil

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/VictoriaMetrics/metrics"
)

// NewUnixListener returns a new unix listener for the given addr.
//
// name is used for metrics. Each listener in the program must have a distinct name.
func NewUnixListener(name, addr string) (*UnixListener, error) {
	// Call unlink before socket bind to prevent possible stale socket issue.
	// In case if app was stopped ungracefully, e.g. OOM or SIGKILL,
	// ListenUnix returns bind: address already in use error
	// which requires manual intervention
	if err := os.Remove(addr); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("cannot remove exist socket path: %q: %w", addr, err)
	}
	ul, err := net.ListenUnix("unix", &net.UnixAddr{Name: addr, Net: "unix"})
	if err != nil {
		return nil, err
	}
	// Default permissions are 0777.
	// Override them to restrict file permissions with current user.
	perm := os.FileMode(0600)
	if err := os.Chmod(addr, perm); err != nil {
		return nil, fmt.Errorf("cannot set permissions for socket path %q: %w", addr, err)
	}

	ms := metrics.GetDefaultSet()
	uln := &UnixListener{
		UnixListener: ul,

		accepts:      ms.NewCounter(fmt.Sprintf(`vm_unixlistener_accepts_total{name=%q, addr=%q}`, name, addr)),
		acceptErrors: ms.NewCounter(fmt.Sprintf(`vm_unixlistener_errors_total{name=%q, addr=%q, type="accept"}`, name, addr)),
	}
	uln.cm.init(ms, "vm_unixlistener", name, addr)
	return uln, nil
}

// UnixListener listens for the addr passed to NewUnixListener.
//
// It also gathers various stats for the accepted connections.
type UnixListener struct {
	*net.UnixListener

	accepts      *metrics.Counter
	acceptErrors *metrics.Counter

	cm connMetrics
}

// Accept accepts connections from the addr passed to NewUnixListener.
func (ul *UnixListener) Accept() (net.Conn, error) {
	conn, err := ul.UnixListener.Accept()
	ul.accepts.Inc()
	if err != nil {
		ul.acceptErrors.Inc()
		return nil, err
	}
	ul.cm.conns.Inc()
	sc := &statConn{
		Conn: conn,
		cm:   &ul.cm,
	}
	return sc, nil
}
