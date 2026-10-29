package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
)

func TestBuildMatchWithFilter_Failure(t *testing.T) {
	f := func(filter, metricName string) {
		t.Helper()

		_, err := buildMatchWithFilter(filter, metricName)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// match with error
	f(`{cluster~=".*"}`, "http_request_count_total")
}

func TestBuildMatchWithFilter_Success(t *testing.T) {
	f := func(filter, metricName, resultExpected string) {
		t.Helper()

		result, err := buildMatchWithFilter(filter, metricName)
		if err != nil {
			t.Fatalf("buildMatchWithFilter() error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// parsed metric with label
	f(`{__name__="http_request_count_total",cluster="kube1"}`, "http_request_count_total", `{cluster="kube1",__name__="http_request_count_total"}`)

	// metric name with label
	f(`http_request_count_total{cluster="kube1"}`, "http_request_count_total", `{cluster="kube1",__name__="http_request_count_total"}`)

	// parsed metric with regexp value
	f(`{__name__="http_request_count_total",cluster=~"kube.*"}`, "http_request_count_total", `{cluster=~"kube.*",__name__="http_request_count_total"}`)

	// only label with regexp
	f(`{cluster=~".*"}`, "http_request_count_total", `{cluster=~".*",__name__="http_request_count_total"}`)

	// only label with regexp, empty metric name
	f(`{cluster=~".*"}`, "", `{cluster=~".*"}`)

	// many labels in filter with regexp
	f(`{cluster=~".*",job!=""}`, "http_request_count_total", `{cluster=~".*",job!="",__name__="http_request_count_total"}`)

	// all names
	f(`{__name__!=""}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// with many underscores labels
	f(`{__name__!="", __meta__!=""}`, "http_request_count_total", `{__meta__!="",__name__="http_request_count_total"}`)

	// metric name has regexp
	f(`{__name__=~".*"}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// metric name has negative regexp
	f(`{__name__!~".*"}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// metric name has negative regex and metric name is empty
	f(`{__name__!~".*"}`, "", `{__name__!~".*"}`)
}

// newExportServer returns a server, which streams chunks of data.
// It will abort the response if abort is set.
func newExportServer(t *testing.T, chunks int, abort bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		buf := make([]byte, 32*1024)
		for range chunks {
			if _, err := w.Write(buf); err != nil {
				return
			}
		}
		w.(http.Flusher).Flush()
		if abort {
			// http.ErrAbortHandler closes the connection without logging the stack trace
			panic(http.ErrAbortHandler)
		}
	}))
}

func TestVMNativeProcessorRunSingle_ExportFailureDoesntHang(t *testing.T) {
	var importsStarted, importsInFlight atomic.Int64

	src := newExportServer(t, 4, true)
	defer src.Close()

	// The destination reads the request body until it is closed, like vminsert does.
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		importsStarted.Add(1)
		importsInFlight.Add(1)
		defer importsInFlight.Add(-1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer dst.Close()

	p := &vmNativeProcessor{
		s:   &stats{startTime: time.Now()},
		src: &native.Client{Addr: src.URL, HTTPClient: &http.Client{}},
		dst: &native.Client{Addr: dst.URL, HTTPClient: &http.Client{}},
	}

	const attempts = 10
	for i := range attempts {
		if err := p.runSingle(context.Background(), native.Filter{}, src.URL, dst.URL, nil); err == nil {
			t.Fatalf("expecting non-nil error on attempt %d", i)
		}
	}

	if n := importsStarted.Load(); n == 0 {
		t.Fatalf("no import request reached the destination; the test doesn't exercise the checked code path")
	}

	// Every failed attempt must abort its import request at the destination.
	// Otherwise, the requests pile up there for the whole lifetime of the migration.
	deadline := time.Now().Add(5 * time.Second)
	for importsInFlight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := importsInFlight.Load(); n != 0 {
		t.Fatalf("%d import requests are still in flight at the destination after %d failed attempts", n, attempts)
	}
}

func TestVMNativeProcessorRunSingle_Success(t *testing.T) {
	const chunks = 8
	var got atomic.Int64

	src := newExportServer(t, chunks, false)
	defer src.Close()

	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		got.Add(n)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer dst.Close()

	p := &vmNativeProcessor{
		s:   &stats{startTime: time.Now()},
		src: &native.Client{Addr: src.URL, HTTPClient: &http.Client{}},
		dst: &native.Client{Addr: dst.URL, HTTPClient: &http.Client{}},
	}

	if err := p.runSingle(context.Background(), native.Filter{}, src.URL, dst.URL, nil); err != nil {
		t.Fatalf("unexpected runSingle() error: %s", err)
	}

	want := int64(chunks * 32 * 1024)
	if got.Load() != want {
		t.Fatalf("unexpected number of bytes at the destination; got %d; want %d", got.Load(), want)
	}
	if p.s.bytes != uint64(want) {
		t.Fatalf("unexpected stats.bytes; got %d; want %d", p.s.bytes, want)
	}
}
