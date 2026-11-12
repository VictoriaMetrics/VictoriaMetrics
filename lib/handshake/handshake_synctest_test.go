//go:build synctest

package handshake

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"testing/synctest"
)

func TestVMInsertClientWithDialer(t *testing.T) {

	f := func(serverFunc Func, wantLegacyBC bool) {
		t.Helper()
		synctest.Test(t, func(t *testing.T) {
			dial, serverErrors := newVMInsertTestDialer(t, serverFunc, serverFunc)
			bc, err := VMInsertClientWithDialer(dial, 0)
			if err != nil {
				t.Fatalf("error on client handshake: %s", err)
			}
			if bc.IsLegacy != wantLegacyBC {
				t.Fatalf("unexpected legacy mode; got %v; want %v", bc.IsLegacy, wantLegacyBC)
			}
			if wantLegacyBC && <-serverErrors == nil {
				t.Fatalf("expected legacy server to reject the new protocol")
			}
			if err := <-serverErrors; err != nil {
				t.Fatalf("error on server handshake: %s", err)
			}
		})
	}
	// server with only current protocol
	f(VMInsertServer, false)
	// server with only legacy protocol
	f(VMInsertServerWithLegacyHello, true)
}

func TestVMInsertClientWithDialerTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Withhold the hello response until the client closes the timed-out connection.
		stalledServer := func(c net.Conn, _ int) (*BufferedConn, error) {
			if err := readMessage(c, vminsertHello); err != nil {
				return nil, err
			}
			_, err := io.Copy(io.Discard, c)
			return nil, err
		}
		dial, serverErrors := newVMInsertTestDialer(t, stalledServer, VMInsertServer)
		if _, err := VMInsertClientWithDialer(dial, 0); !IsTimeoutNetworkError(err) {
			t.Fatalf("unexpected error; got %v; want network timeout", err)
		}
		if err := <-serverErrors; err != nil {
			t.Fatalf("error on stalled server: %s", err)
		}

		bc, err := VMInsertClientWithDialer(dial, 0)
		if err != nil {
			t.Fatalf("error on client handshake after retry: %s", err)
		}
		if bc.IsLegacy {
			t.Fatalf("unexpected legacy mode after retry")
		}
		if err := <-serverErrors; err != nil {
			t.Fatalf("error on server handshake after retry: %s", err)
		}
	})
}

// newVMInsertTestDialer pairs each connection attempt with the next server function.
func newVMInsertTestDialer(t *testing.T, serverFuncs ...Func) (func() (net.Conn, error), <-chan error) {
	t.Helper()

	var conns []net.Conn
	var wg sync.WaitGroup
	t.Cleanup(func() {
		for _, c := range conns {
			_ = c.Close()
		}
		wg.Wait()
	})
	serverErrors := make(chan error, len(serverFuncs))
	dial := func() (net.Conn, error) {
		if len(serverFuncs) == 0 {
			return nil, fmt.Errorf("unexpected connection attempt")
		}
		serverFunc := serverFuncs[0]
		serverFuncs = serverFuncs[1:]
		c, s := net.Pipe()
		conns = append(conns, c, s)
		wg.Go(func() {
			_, err := serverFunc(s, 0)
			serverErrors <- err
			if err != nil {
				_ = s.Close()
			}
		})
		return c, nil
	}
	return dial, serverErrors
}
