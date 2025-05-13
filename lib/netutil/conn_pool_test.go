package netutil

import (
	"fmt"
	"sync"
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
	for i := 0; i < concurrency; i++ {
		name := fmt.Sprintf("foobar_%d", i)
		go func() {
			testConnPoolStartStop(t, name, ms)
			ch <- struct{}{}
		}()
	}
	tc := time.NewTimer(time.Second * 5)
	for i := 0; i < concurrency; i++ {
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
	for i := 0; i < 5; i++ {
		addr := fmt.Sprintf("host-%d", i)
		cp := NewConnPool(ms, name, addr, handshake.VMSelectClient, compressLevel, dialTimeout, 0)
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

func TestGetPutDialConnectionPool(t *testing.T) {
	t.Skip()
	ms := metrics.NewSet()
	dialTimeout := 5 * time.Second
	compressLevel := 1
	cp := NewConnPool(ms, "test-pool", "127.0.0.1:8401", handshake.VMSelectClient, compressLevel, dialTimeout, 0)

	connChan := make(chan *handshake.BufferedConn, 50)
	wg := sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			conn, err := cp.Get()
			if err != nil {
				t.Errorf("expecting non-nil error after MustStop(), err:%v", err)
			}
			connChan <- conn
			wg.Done()
		}()
	}

	for i := 0; i < 50; i++ {
		cp.Put(<-connChan)
	}
	wg.Wait()
}
