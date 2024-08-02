package remotewrite

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestClient_Push(t *testing.T) {
	oldMinInterval := *retryMinInterval
	*retryMinInterval = time.Millisecond * 10
	defer func() {
		*retryMinInterval = oldMinInterval
	}()

	testSrv := newRWServer()
	client, err := NewClient(context.Background(), Config{
		Addr:         testSrv.URL,
		MaxBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %s", err)
	}

	faultySrv := newFaultyRWServer()
	faultyClient, err := NewClient(context.Background(), Config{
		Addr:         faultySrv.URL,
		MaxBatchSize: 50,
	})
	if err != nil {
		t.Fatalf("failed to create faulty client: %s", err)
	}

	r := rand.New(rand.NewSource(1))
	const rowsN = int(1e4)
	for i := 0; i < rowsN; i++ {
		s := prompbmarshal.TimeSeries{
			Samples: []prompbmarshal.Sample{{
				Value:     r.Float64(),
				Timestamp: time.Now().Unix(),
			}},
		}
		err := client.Push(s)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		err = faultyClient.Push(s)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
	}
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	if err := faultyClient.Close(); err != nil {
		t.Fatalf("failed to close faulty client: %s", err)
	}
	got := testSrv.accepted()
	if got != rowsN {
		t.Fatalf("expected to have %d series; got %d", rowsN, got)
	}
	got = faultySrv.accepted()
	if got != rowsN {
		t.Fatalf("expected to have %d series for faulty client; got %d", rowsN, got)
	}
}

func TestClient_run_maxBatchSizeDuringShutdown(t *testing.T) {
	const batchSize = 20

	f := func(pushCnt, batchCntExpected int) {
		t.Helper()

		// run new server
		bcServer := newBatchCntRWServer()

		// run new client
		rwClient, err := NewClient(context.Background(), Config{
			MaxBatchSize: batchSize,

			// Set everything to 1 to simplify the calculation.
			Concurrency:   1,
			MaxQueueSize:  1000,
			FlushInterval: time.Minute,

			// batch count server
			Addr: bcServer.URL,
		})
		if err != nil {
			t.Fatalf("cannot create remote write client: %s", err)
		}

		// push time series to the client.
		for i := 0; i < pushCnt; i++ {
			if err = rwClient.Push(prompbmarshal.TimeSeries{}); err != nil {
				t.Fatalf("cannot time series to the client: %s", err)
			}
		}

		// close the client so the rest ts will be flushed in `shutdown`
		if err = rwClient.Close(); err != nil {
			t.Fatalf("cannot shutdown client: %s", err)
		}

		// finally check how many batches is sent.
		if bcServer.acceptedBatches() != batchCntExpected {
			t.Fatalf("client sent batch count incorrect; got %d; want %d", bcServer.acceptedBatches(), batchCntExpected)
		}
		if pushCnt != bcServer.accepted() {
			t.Fatalf("client sent time series count incorrect; got %d; want %d", bcServer.accepted(), pushCnt)
		}
	}

	// pushCnt % batchSize == 0
	f(batchSize*40, 40)

	//pushCnt % batchSize != 0
	f(batchSize*40+1, 40+1)
}

func newRWServer() *rwServer {
	rw := &rwServer{}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	return rw
}

type rwServer struct {
	acceptedRows atomic.Uint64
	*httptest.Server
}

func (rw *rwServer) accepted() int {
	return int(rw.acceptedRows.Load())
}

func (rw *rwServer) err(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func (rw *rwServer) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.err(w, fmt.Errorf("bad method %q", r.Method))
		return
	}

	h := r.Header.Get("Content-Encoding")
	if h != "snappy" {
		rw.err(w, fmt.Errorf("header read error: Content-Encoding is not snappy (%q)", h))
	}

	h = r.Header.Get("Content-Type")
	if h != "application/x-protobuf" {
		rw.err(w, fmt.Errorf("header read error: Content-Type is not x-protobuf (%q)", h))
	}

	h = r.Header.Get("X-Prometheus-Remote-Write-Version")
	if h != "0.1.0" {
		rw.err(w, fmt.Errorf("header read error: X-Prometheus-Remote-Write-Version is not 0.1.0 (%q)", h))
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		rw.err(w, fmt.Errorf("body read err: %w", err))
		return
	}
	defer func() { _ = r.Body.Close() }()

	b, err := snappy.Decode(nil, data)
	if err != nil {
		rw.err(w, fmt.Errorf("decode err: %w", err))
		return
	}
	wr := &prompb.WriteRequest{}
	if err := wr.UnmarshalProtobuf(b); err != nil {
		rw.err(w, fmt.Errorf("unmarhsal err: %w", err))
		return
	}
	rw.acceptedRows.Add(uint64(len(wr.Timeseries)))
	w.WriteHeader(http.StatusNoContent)
}

// faultyRWServer sometimes respond with 5XX status code
// or just closes the connection. Is used for testing retries.
type faultyRWServer struct {
	*rwServer

	reqsMu sync.Mutex
	reqs   int
}

func newFaultyRWServer() *faultyRWServer {
	rw := &faultyRWServer{
		rwServer: &rwServer{},
	}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	return rw
}

func (frw *faultyRWServer) handler(w http.ResponseWriter, r *http.Request) {
	frw.reqsMu.Lock()
	reqs := frw.reqs
	frw.reqs++
	if frw.reqs > 5 {
		frw.reqs = 0
	}
	frw.reqsMu.Unlock()

	switch reqs {
	case 0, 1, 2, 3:
		frw.rwServer.handler(w, r)
	case 4:
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	case 5:
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server overloaded"))
	}
}

type batchCntRWServer struct {
	*rwServer

	batchCnt atomic.Int64 // accepted batch count, which also equals to request count
}

func newBatchCntRWServer() *batchCntRWServer {
	bc := &batchCntRWServer{
		rwServer: &rwServer{},
	}

	bc.Server = httptest.NewServer(http.HandlerFunc(bc.handler))
	return bc
}

func (bc *batchCntRWServer) handler(w http.ResponseWriter, r *http.Request) {
	bc.batchCnt.Add(1)
	bc.rwServer.handler(w, r)
}

func (bc *batchCntRWServer) acceptedBatches() int {
	return int(bc.batchCnt.Load())
}
