package remotewrite

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
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
	for range rowsN {
		s := prompb.TimeSeries{
			Samples: []prompb.Sample{{
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
		for range pushCnt {
			if err = rwClient.Push(prompb.TimeSeries{
				Labels:  []prompb.Label{{Name: "__name__", Value: "m"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: 1000}},
			}); err != nil {
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
	if h != "zstd" {
		rw.err(w, fmt.Errorf("header read error: Content-Encoding is not zstd (%q)", h))
	}

	h = r.Header.Get("Content-Type")
	if h != "application/x-protobuf" {
		rw.err(w, fmt.Errorf("header read error: Content-Type is not x-protobuf (%q)", h))
	}

	h = r.Header.Get("X-VictoriaMetrics-Remote-Write-Version")
	if h != "1" {
		rw.err(w, fmt.Errorf("header read error: X-VictoriaMetrics-Remote-Write-Version is not 1 (%q)", h))
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		rw.err(w, fmt.Errorf("body read err: %w", err))
		return
	}
	defer func() { _ = r.Body.Close() }()

	b, err := zstd.Decompress(nil, data)
	if err != nil {
		rw.err(w, fmt.Errorf("decode err: %w", err))
		return
	}
	wru := &prompb.WriteRequestUnmarshaler{}
	wr, err := wru.UnmarshalProtobuf(b)
	if err != nil {
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

func TestClient_Push_fullQueue(t *testing.T) {
	srv := newGatedRWServer()
	defer srv.Close()
	defer srv.open()
	client := newSmallQueueClient(t, context.Background(), srv.URL, false)

	// The only writer waits for the server, so the queue fills up.
	var err error
	for range 100 {
		if err = client.Push(newTestSeries()); err != nil {
			break
		}
	}
	if err == nil || !strings.Contains(err.Error(), "queue is full") {
		t.Fatalf("expected queue is full error; got %v", err)
	}

	srv.open()
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
}

func TestClient_Push_blockOnFullQueue(t *testing.T) {
	srv := newGatedRWServer()
	defer srv.Close()
	defer srv.open()
	client := newSmallQueueClient(t, context.Background(), srv.URL, true)

	const rowsN = 100
	pushed := make(chan error, 1)
	go func() {
		for range rowsN {
			if err := client.Push(newTestSeries()); err != nil {
				pushed <- err
				return
			}
		}
		pushed <- nil
	}()

	// The only writer waits for the server, so the queue fills up and Push must wait.
	select {
	case err := <-pushed:
		t.Fatalf("expected Push to wait for free space in the full queue; got %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	srv.open()
	if err := <-pushed; err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	if got := srv.accepted(); got != rowsN {
		t.Fatalf("expected to have %d series; got %d", rowsN, got)
	}
}

func TestClient_Push_blockOnFullQueue_canceledContext(t *testing.T) {
	srv := newGatedRWServer()
	defer srv.Close()
	defer srv.open()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newSmallQueueClient(t, ctx, srv.URL, true)

	// The writer holds the first series and the queue holds the second,
	// so the third Push waits for free space.
	const rowsN = 3
	pushed := make(chan error, 1)
	go func() {
		for range rowsN {
			if err := client.Push(newTestSeries()); err != nil {
				pushed <- err
				return
			}
		}
		pushed <- nil
	}()
	select {
	case err := <-pushed:
		t.Fatalf("expected Push to wait for free space in the full queue; got %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// The writer keeps draining the queue after the client context is canceled,
	// even while the server does not respond, so the waiting Push completes.
	cancel()
	select {
	case err := <-pushed:
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("expected Push to complete after the client context is canceled")
	}

	srv.open()
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
}

func TestClient_Close_blockOnFullQueue(t *testing.T) {
	srv := newGatedRWServer()
	defer srv.Close()
	defer srv.open()
	client := newSmallQueueClient(t, context.Background(), srv.URL, true)

	// The only writer waits for the server, so the queue fills up and the pushes wait.
	const pushersN = 8
	var accepted, inFlight atomic.Int64
	pushed := make(chan error, pushersN)
	for range pushersN {
		go func() {
			for {
				inFlight.Add(1)
				err := client.Push(newTestSeries())
				inFlight.Add(-1)
				if err != nil {
					pushed <- err
					return
				}
				accepted.Add(1)
			}
		}()
	}

	// Wait until the writer is stuck in a request to the server, the queue is full and every pusher is in Push.
	// No Push can complete in this state, so Close must release blocked pushes.
	select {
	case <-srv.requested:
	case <-time.After(5 * time.Second):
		t.Fatalf("expected the writer to send a request to the server")
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(client.input) < cap(client.input) || inFlight.Load() < pushersN {
		if time.Now().After(deadline) {
			t.Fatalf("expected all the pushers to wait for free space in the full queue")
		}
		time.Sleep(time.Millisecond)
	}

	closed := make(chan error, 1)
	go func() {
		closed <- client.Close()
	}()

	// Close releases the waiting pushes while the server still does not respond.
	for range pushersN {
		select {
		case err := <-pushed:
			if err == nil || !strings.Contains(err.Error(), "client is closed") {
				t.Fatalf("expected client is closed error; got %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("expected Close to release the waiting pushes")
		}
	}

	srv.open()
	if err := <-closed; err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	if got, want := srv.accepted(), int(accepted.Load()); got != want {
		t.Fatalf("expected to have %d series; got %d", want, got)
	}
}

func TestClient_Push_blockOnFullQueue_duringClose(t *testing.T) {
	srv := newRWServer()
	defer srv.Close()
	client, err := NewClient(context.Background(), Config{
		Addr:             srv.URL,
		Concurrency:      2,
		MaxBatchSize:     10,
		MaxQueueSize:     10,
		BlockOnFullQueue: true,
		FlushInterval:    10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %s", err)
	}

	// Pushes run before, during and after Close. None of them may panic,
	// and every accepted series must reach the server.
	const pushersN = 8
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for range pushersN {
		wg.Go(func() {
			for {
				err := client.Push(newTestSeries())
				if err == nil {
					accepted.Add(1)
					continue
				}
				if !strings.Contains(err.Error(), "client is closed") {
					t.Errorf("expected client is closed error; got %s", err)
				}
				return
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	wg.Wait()

	for range 100 {
		if err := client.Push(newTestSeries()); err == nil || !strings.Contains(err.Error(), "client is closed") {
			t.Fatalf("expected client is closed error after Close; got %v", err)
		}
	}
	if got, want := srv.accepted(), int(accepted.Load()); got != want {
		t.Fatalf("expected to have %d series; got %d", want, got)
	}
}

func newSmallQueueClient(t *testing.T, ctx context.Context, addr string, blockOnFullQueue bool) *Client {
	t.Helper()
	client, err := NewClient(ctx, Config{
		Addr:             addr,
		Concurrency:      1,
		MaxBatchSize:     1,
		MaxQueueSize:     1,
		BlockOnFullQueue: blockOnFullQueue,
		// The writer starts after a jitter of up to FlushInterval.
		FlushInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %s", err)
	}
	return client
}

func newTestSeries() prompb.TimeSeries {
	return prompb.TimeSeries{
		Labels:  []prompb.Label{{Name: "__name__", Value: "m"}},
		Samples: []prompb.Sample{{Value: 1, Timestamp: 1000}},
	}
}

// gatedRWServer accepts requests only after open is called.
type gatedRWServer struct {
	*rwServer

	gate     chan struct{}
	openOnce sync.Once

	// requested is closed when the server receives the first request.
	requested     chan struct{}
	requestedOnce sync.Once
}

func newGatedRWServer() *gatedRWServer {
	rw := &gatedRWServer{
		rwServer:  &rwServer{},
		gate:      make(chan struct{}),
		requested: make(chan struct{}),
	}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	return rw
}

func (grw *gatedRWServer) open() {
	grw.openOnce.Do(func() { close(grw.gate) })
}

func (grw *gatedRWServer) handler(w http.ResponseWriter, r *http.Request) {
	grw.requestedOnce.Do(func() { close(grw.requested) })
	<-grw.gate
	grw.rwServer.handler(w, r)
}
