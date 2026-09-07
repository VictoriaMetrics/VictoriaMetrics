package remotewrite

import (
	"net/http"
	"testing"
)

// TestDebugClient_IdleConns makes sure DebugClient keeps enough idle connections
// to -remoteWrite.url. Every series is pushed in a separate request, so with the
// two idle connections per host of http.DefaultTransport most of the concurrent
// requests would open a new connection and leave a socket in TIME_WAIT state.
func TestDebugClient_IdleConns(t *testing.T) {
	f := func(maxIdle int) {
		t.Helper()

		oldAddr, oldMaxIdle := *addr, *maxIdleConnections
		*addr, *maxIdleConnections = "http://localhost:8428", maxIdle
		defer func() {
			*addr, *maxIdleConnections = oldAddr, oldMaxIdle
		}()

		client, err := NewDebugClient()
		if err != nil {
			t.Fatalf("failed to create debug client: %s", err)
		}
		tr, ok := client.c.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("unexpected transport type %T", client.c.Transport)
		}
		if tr.MaxIdleConnsPerHost != maxIdle {
			t.Fatalf("unexpected MaxIdleConnsPerHost; got %d; want %d", tr.MaxIdleConnsPerHost, maxIdle)
		}
		if tr.MaxIdleConns != 0 && tr.MaxIdleConns < maxIdle {
			t.Fatalf("MaxIdleConns=%d is lower than MaxIdleConnsPerHost=%d", tr.MaxIdleConns, maxIdle)
		}
		if tr.IdleConnTimeout != *idleConnectionTimeout {
			t.Fatalf("unexpected IdleConnTimeout; got %s; want %s", tr.IdleConnTimeout, *idleConnectionTimeout)
		}
	}

	f(100)

	// the number of idle connections must be raised together with the total limit
	f(1000)
}
