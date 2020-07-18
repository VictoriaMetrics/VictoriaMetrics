package remotewrite

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestClient_Push(t *testing.T) {
	testSrv := newRWServer()
	cfg := Config{
		Addr:         testSrv.URL,
		MaxBatchSize: 100,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %s", err)
	}
	const rowsN = 1e4
	var sent int
	for i := 0; i < rowsN; i++ {
		s := prompbmarshal.TimeSeries{
			Samples: []prompbmarshal.Sample{{
				Value:     rand.Float64(),
				Timestamp: time.Now().Unix(),
			}},
		}
		err := client.Push(s)
		if err == nil {
			sent++
		}
	}
	if sent == 0 {
		t.Fatalf("0 series sent")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	got := testSrv.accepted()
	if got != sent {
		t.Fatalf("expected to have %d series; got %d", sent, got)
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
	data, err := ioutil.ReadAll(r.Body)
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
