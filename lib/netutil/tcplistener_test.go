package netutil

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTCPListenerSetMaxConns(t *testing.T) {
	ln, err := NewTCPListener("test_max_conns", "127.0.0.1:0", false, nil)
	if err != nil {
		t.Fatalf("unexpected error in NewTCPListener: %s", err)
	}
	defer ln.Close()
	ln.SetMaxConns(1)

	acceptedCh := make(chan net.Conn, 10)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				close(acceptedCh)
				return
			}
			acceptedCh <- c
		}
	}()

	addr := ln.Addr().String()
	c1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("unexpected error when dialing the first connection: %s", err)
	}
	defer c1.Close()

	select {
	case c, ok := <-acceptedCh:
		if !ok {
			t.Fatalf("listener unexpectedly stopped accepting connections")
		}
		defer c.Close()
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for the first connection to be accepted")
	}

	c2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("unexpected error when dialing the second connection: %s", err)
	}
	defer c2.Close()

	// The second connection must be rejected, since the limit of 1 established connection is already reached.
	buf := make([]byte, 1)
	c2.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := c2.Read(buf); err == nil {
		t.Fatalf("expected the second connection to be closed by the server due to the max conns limit")
	}

	if n := int(ln.cm.conns.Get()); n != 1 {
		t.Fatalf("unexpected number of active connections; got %d; want 1", n)
	}
	if n := ln.cm.connsDropped.Get(); n != 1 {
		t.Fatalf("unexpected number of dropped connections; got %d; want 1", n)
	}
}

func TestTCPListenerSetMaxConnsConcurrentAccept(t *testing.T) {
	ln, err := NewTCPListener("test_max_conns_concurrent", "127.0.0.1:0", false, nil)
	if err != nil {
		t.Fatalf("unexpected error in NewTCPListener: %s", err)
	}
	defer ln.Close()

	const maxConns = 5
	ln.SetMaxConns(maxConns)

	var admitted atomic.Int64
	var maxObserved atomic.Int64

	// Run multiple goroutines calling Accept concurrently on the same listener,
	// so the maxConns enforcement must be safe against concurrent check-then-act races.
	var acceptorsWG sync.WaitGroup
	for range 4 {
		acceptorsWG.Add(1)
		go func() {
			defer acceptorsWG.Done()
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				n := admitted.Add(1)
				for {
					old := maxObserved.Load()
					if n <= old || maxObserved.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				admitted.Add(-1)
				c.Close()
			}
		}()
	}

	addr := ln.Addr().String()
	var dialersWG sync.WaitGroup
	for range 50 {
		dialersWG.Add(1)
		go func() {
			defer dialersWG.Done()
			c, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				return
			}
			defer c.Close()
			buf := make([]byte, 1)
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, _ = c.Read(buf)
		}()
	}
	dialersWG.Wait()
	ln.Close()
	acceptorsWG.Wait()

	if got := maxObserved.Load(); got > maxConns {
		t.Fatalf("more than maxConns connections were admitted concurrently; got %d; want at most %d", got, maxConns)
	}
}

func TestTCPListenerSetMaxConnsZeroDisablesLimit(t *testing.T) {
	ln, err := NewTCPListener("test_max_conns_disabled", "127.0.0.1:0", false, nil)
	if err != nil {
		t.Fatalf("unexpected error in NewTCPListener: %s", err)
	}
	defer ln.Close()
	// maxConns is 0 by default, meaning no limit.

	acceptedCh := make(chan net.Conn, 10)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				close(acceptedCh)
				return
			}
			acceptedCh <- c
		}
	}()

	addr := ln.Addr().String()
	for i := 0; i < 5; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("unexpected error when dialing connection %d: %s", i, err)
		}
		defer c.Close()
	}

	for i := 0; i < 5; i++ {
		select {
		case c, ok := <-acceptedCh:
			if !ok {
				t.Fatalf("listener unexpectedly stopped accepting connections")
			}
			defer c.Close()
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for connection %d to be accepted", i)
		}
	}

	if n := ln.cm.connsDropped.Get(); n != 0 {
		t.Fatalf("unexpected number of dropped connections; got %v; want 0", n)
	}
}
