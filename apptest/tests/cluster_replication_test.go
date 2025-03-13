package tests

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// TestClusterReplicationLatency checks ingestion perfomance in various scenarios
// and data consistency
//
// It wraps network connections to the storage with proxy
// which could emualte network errors and timeouts
func TestClusterReplicationLatency(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	const (
		replicationFactor = 1
		cycles            = 10
		rps               = 10_000
		wantIngestedRows  = rps * cycles * replicationFactor
	)

	// spin up 1 cluster and wrap it with proxy that emulate storage nodes
	// do not use multiple storage nodes
	// since it add delay for tsid registrations
	// and may have negative impact on test results with extra delays
	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	laggingStorage0 := newLaggingProxyWrap(t, vmstorage.VminsertAddr())
	defer laggingStorage0.close()
	laggingStorage1 := newLaggingProxyWrap(t, vmstorage.VminsertAddr())
	defer laggingStorage1.close()
	laggingStorage2 := newLaggingProxyWrap(t, vmstorage.VminsertAddr())
	defer laggingStorage2.close()

	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + fmt.Sprintf("%s,%s,%s", laggingStorage0.listenAddr(), laggingStorage1.listenAddr(), laggingStorage2.listenAddr()),
		fmt.Sprintf("-replicationFactor=%d", replicationFactor),
		"-memory.allowedBytes=1500000",
		"-disableRerouting=false",
	})

	// emulate network delays and disconnects here
	// uncomment or add needed lines
	laggingStorage0.startLag()
	ds := genDataset(rps)
	var (
		requestsInTime   int
		requestsTimeouts int
	)

	// start ingestion with configure requests per seconds
	for range cycles {
		ct := time.Now()
		// use non blocking mode for ingestion
		// since it may introduce timeouts
		vminsert.PrometheusAPIV1ImportPrometheus(t, ds, apptest.QueryOpts{Tenant: "1", IsNonBlocking: true})
		since := time.Since(ct)
		if since > time.Second {
			requestsTimeouts++
		} else {
			requestsInTime++
		}
		toSleep := time.Second - since
		if toSleep > 0 {
			time.Sleep(toSleep)
		}
	}
	vmstorage.ForceFlush(t)
	if requestsTimeouts > 0 {
		t.Errorf("unexpected result, got requests timeouts=%d out of %d, in time %d", requestsTimeouts, cycles, requestsInTime)
	}
	// verify that metrics actually ingested
	ct := time.Now()
	var ingestedTotal int
	// use 10 seconds as timeout
	for range 100 {
		ingestedTotal = 0
		ingested := vminsert.GetMetricsByPrefix(t, "vm_rpc_rows_sent_total")
		for _, v := range ingested {
			ingestedTotal += int(v)
		}
		if ingestedTotal >= wantIngestedRows {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	since := time.Since(ct)
	if since > time.Second {
		t.Logf("WARN: data ingestion check took: %s", since)
	}
	if ingestedTotal != wantIngestedRows {
		t.Fatalf("unexpected ingested metrics=%d want=%d, it took time=%s", ingestedTotal, wantIngestedRows, since)
	}
}

func genDataset(size int) []string {
	var ds []string
	for i := range size {
		ds = append(ds, fmt.Sprintf("metric_%d 15\n", i))
	}
	return ds
}

type laggingProxy struct {
	l                net.Listener
	dstAddr          string
	shouldLag        atomic.Bool
	shouldDisconnect atomic.Bool
	wg               sync.WaitGroup
}

func newLaggingProxyWrap(t *testing.T, dstAddr string) *laggingProxy {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot start proxy: %s", err)
	}
	lp := &laggingProxy{
		l:       l,
		dstAddr: dstAddr,
	}
	lp.wg.Add(1)
	go lp.run()
	return lp
}

func (lp *laggingProxy) listenAddr() string {
	return lp.l.Addr().String()
}

func (lp *laggingProxy) run() {
	defer lp.wg.Done()
	for {
		src, err := lp.l.Accept()
		if err != nil {
			println("exiting at err: ", err.Error())
			break
		}
		if lp.shouldDisconnect.Load() {
			src.Close()
			continue
		}
		go func() {
			dst, err := net.Dial("tcp", lp.dstAddr)
			if err != nil {
				println("err dial: ", err.Error())
				return
			}
			laggingDst := wrapConnWithLag(dst, &lp.shouldLag, &lp.shouldDisconnect)
			go io.Copy(src, laggingDst)
			io.Copy(laggingDst, src)
			laggingDst.Close()
			src.Close()
		}()
	}
}

func (lp *laggingProxy) startLag() {
	lp.shouldLag.Store(true)
}

func (lp *laggingProxy) stopLag() {
	lp.shouldLag.Store(false)
}

func (lp *laggingProxy) rejectConnections() {
	lp.shouldDisconnect.Store(true)
}

func (lp *laggingProxy) stopRejectConnections() {
	lp.shouldDisconnect.Store(false)
}

func (lp *laggingProxy) close() {
	lp.l.Close()
	lp.wg.Wait()
}

type laggingWriteReader struct {
	origin           net.Conn
	shouldLag        *atomic.Bool
	shouldDisconnect *atomic.Bool
	mu               sync.Mutex
	lagDelay         int
	maxDelay         int
}

func wrapConnWithLag(origin net.Conn, lagOn *atomic.Bool, disconnectOn *atomic.Bool) *laggingWriteReader {
	return &laggingWriteReader{origin: origin, maxDelay: 5, shouldLag: lagOn, shouldDisconnect: disconnectOn}
}

func (lwr *laggingWriteReader) lag() {
	if lwr.shouldDisconnect.Load() {
		lwr.origin.Close()
		return
	}
	if !lwr.shouldLag.Load() {
		return
	}
	time.Sleep(time.Second * time.Duration(lwr.lagDelay))
	lwr.mu.Lock()
	defer lwr.mu.Unlock()
	lwr.lagDelay++
	if lwr.lagDelay > lwr.maxDelay {
		lwr.lagDelay = 0
	}
}
func (lwr *laggingWriteReader) Read(p []byte) (n int, err error) {
	lwr.lag()
	return lwr.origin.Read(p)
}

func (lwr *laggingWriteReader) Write(p []byte) (n int, err error) {
	lwr.lag()
	return lwr.origin.Write(p)
}

func (lwr *laggingWriteReader) Close() error {
	return lwr.origin.Close()
}
