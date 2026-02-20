package netutil

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/metrics"
)

func TestConnPoolStartStopSerial(t *testing.T) {
	ms := metrics.NewSet()
	testConnPoolStartStop(t, "foobar", ms)
	ms.UnregisterAllMetrics()
}

func TestConnPoolStartStopConcurrent(t *testing.T) {
	ms := metrics.NewSet()
	concurrency := 5
	ch := make(chan struct{})
	for i := range concurrency {
		name := fmt.Sprintf("foobar_%d", i)
		go func() {
			testConnPoolStartStop(t, name, ms)
			ch <- struct{}{}
		}()
	}
	tc := time.NewTimer(time.Second * 5)
	for range concurrency {
		select {
		case <-tc.C:
			t.Fatalf("timeout")
		case <-ch:
		}
	}
	tc.Stop()
	ms.UnregisterAllMetrics()
}

func testConnPoolStartStop(t *testing.T, name string, ms *metrics.Set) {
	dialTimeout := 5 * time.Second
	compressLevel := 1
	var cps []*ConnPool
	for i := range 5 {
		addr := fmt.Sprintf("host-%d", i)
		cp := NewConnPool(ms, name, addr, handshake.VMSelectClient, compressLevel, dialTimeout, 0, mockHealthCheck)
		cps = append(cps, cp)
	}
	for _, cp := range cps {
		cp.MustStop()
		// Make sure that Get works properly after MustStop()
		c, err := cp.Get()
		if err == nil {
			t.Fatalf("expecting non-nil error after MustStop()")
		}
		if c != nil {
			t.Fatalf("expecting nil conn after MustStop()")
		}
	}
}

// TestGetPutDialConnectionPool test the concurrent dial limit of the connection pool.
// It simulates 256 concurrent goroutine to get new connections, while the concurrent dial limit shouldn't be more than 64.
// After the first 64 goroutines got connections, the rest 192 goroutines will race for the `select-case`.
// Only 64 of them have chance to create new connections while the rest will go to the default case and try to obtain
// used connections.
//
// Without extra logging or metrics, it's hard to verify if all the paths are covered. But the coverage report shows it
// does.
func TestGetPutDialConnectionPool(t *testing.T) {
	mockSvr := newMockServer()
	addr, _ := url.Parse(mockSvr.URL)
	cp := NewConnPool(metrics.NewSet(), "test-pool", addr.Host, mockHandshake, 1, 5*time.Second, 0, mockHealthCheck)

	concurrency := 256
	connChan := make(chan *handshake.BufferedConn, concurrency)

	// concurrent create connections
	for range concurrency {
		go func() {
			conn, err := cp.Get()
			if err != nil {
				t.Errorf("get conn from connection pool err:%v", err)
				panic(err)
			}
			connChan <- conn
		}()
	}

	// concurrent return connections to pool.
	var wg sync.WaitGroup
	for range concurrency {
		wg.Go(func() {
			conn := <-connChan
			time.Sleep(time.Millisecond * 10)
			cp.Put(conn)
		})
	}
	wg.Wait()
}

func TestGetWithHealthCheckConnectionPool(t *testing.T) {
	mockSvr := newMockServer()
	addr, _ := url.Parse(mockSvr.URL)
	healthCheckCounter := atomic.Int32{}
	handshakeCounter := atomic.Int32{}

	mockHealthCheckWithCounter := func(_ *handshake.BufferedConn) error {
		healthCheckCounter.Add(1)
		return nil
	}
	mockFailHealthCheckWithCounter := func(_ *handshake.BufferedConn) error {
		healthCheckCounter.Add(1)
		return fmt.Errorf("fail health check")
	}
	mockHandshakeWithCounter := func(c net.Conn, n int) (*handshake.BufferedConn, error) {
		handshakeCounter.Add(1)
		return mockHandshake(c, n)
	}
	cp := NewConnPool(metrics.NewSet(), "test-pool", addr.Host, mockHandshakeWithCounter, 1, 5*time.Second, 0, mockHealthCheckWithCounter)
	cp.connKeepAliveInterval = 3
	// generate new connections and put it in the pool.
	conn, err := cp.Get()
	if err != nil {
		t.Errorf("get conn from connection pool err:%v", err)
		panic(err)
	}
	if handshakeCounter.Load() != 1 {
		t.Errorf("unexpected handshake counter value: %d", handshakeCounter.Load())
	}
	cp.Put(conn)

	// get an existing active connection
	conn, err = cp.Get()
	if conn == nil || err != nil {
		t.Errorf("failed to get connection from pool")
	}
	if conn != nil {
		cp.Put(conn)
	}
	if healthCheckCounter.Load() != 0 {
		t.Errorf("unexpected health check counter value: %d", healthCheckCounter.Load())
	}
	if handshakeCounter.Load() != 1 {
		t.Errorf("unexpected handshake counter value: %d", handshakeCounter.Load())
	}

	// wait for connection to be in-active
	// and then get an existing in-active connection
	time.Sleep(4 * time.Second)
	conn, err = cp.Get()
	if conn == nil || err != nil {
		t.Errorf("failed to get connection from pool")
	}
	if healthCheckCounter.Load() != 1 {
		t.Errorf("unexpected health check counter value: %d", healthCheckCounter.Load())
	}

	// health check fail, connection will dial a new connection to return.
	cp.healthCheckFunc = mockFailHealthCheckWithCounter

	time.Sleep(4 * time.Second)
	conn, err = cp.Get()
	if conn == nil || err != nil {
		t.Errorf("failed to get connection from pool")
	}
	if handshakeCounter.Load() != 2 {
		t.Errorf("unexpected handshake counter value: %d", handshakeCounter.Load())
	}
}

// mockServer does nothing. It only acts as a tcp server for connection test.
type mockServer struct {
	*httptest.Server
}

func newMockServer() *mockServer {
	var s mockServer
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(""))
	}))
	return &s
}

func mockHandshake(c net.Conn, _ int) (*handshake.BufferedConn, error) {
	bc := &handshake.BufferedConn{
		Conn: c,
	}
	return bc, nil
}

func mockHealthCheck(_ *handshake.BufferedConn) error {
	return nil
}
