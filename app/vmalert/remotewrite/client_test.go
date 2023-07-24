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
	const rowsN = 1e4
	var sent int
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
		if err == nil {
			sent++
		}
		err = faultyClient.Push(s)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
	}
	if sent == 0 {
		t.Fatalf("0 series sent")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	if err := faultyClient.Close(); err != nil {
		t.Fatalf("failed to close faulty client: %s", err)
	}
	got := testSrv.accepted()
	if got != sent {
		t.Fatalf("expected to have %d series; got %d", sent, got)
	}
	got = faultySrv.accepted()
	if got != sent {
		t.Fatalf("expected to have %d series for faulty client; got %d", sent, got)
	}
}

func newRWServer() *rwServer {
	rw := &rwServer{}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	return rw
}

type rwServer struct {
	// WARN: ordering of fields is important for alignment!
	// see https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	acceptedRows uint64
	*httptest.Server
}

func (rw *rwServer) accepted() int {
	return int(atomic.LoadUint64(&rw.acceptedRows))
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
	if err := wr.Unmarshal(b); err != nil {
		rw.err(w, fmt.Errorf("unmarhsal err: %w", err))
		return
	}
	atomic.AddUint64(&rw.acceptedRows, uint64(len(wr.Timeseries)))
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
