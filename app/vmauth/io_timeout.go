package main

import (
	"flag"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var readDuration = metrics.NewSummaryExt(`vmauth_request_read_duration_seconds`, time.Second*30, []float64{0.8, 0.97, 0.99})
var writeDuration = metrics.NewSummaryExt(`vmauth_response_write_duration_seconds`, time.Second*30, []float64{0.8, 0.97, 0.99})

var readOpTimeout = flag.Duration("readOpTimeout", 0, "TODO")
var writeOpTimeout = flag.Duration("writeOpTimeout", 0, "TODO")

type measureReadDurationBody struct {
	r io.ReadCloser
}

func (r *measureReadDurationBody) Read(p []byte) (n int, err error) {
	start := time.Now()
	defer readDuration.UpdateDuration(start)

	return r.r.Read(p)
}

func (r *measureReadDurationBody) Close() error {
	return r.r.Close()
}

func wrapResponseWriter(rw http.ResponseWriter) http.ResponseWriter {
	f, ok := rw.(http.Flusher)
	if !ok {
		logger.Panicf("BUG: client must implement net/http.Flusher interface; got %T", rw)
	}

	return &measureWriteDurationResponseWriter{
		rw: rw,
		f:  f,
	}
}

type measureWriteDurationResponseWriter struct {
	rw http.ResponseWriter
	f  http.Flusher
}

func (rw *measureWriteDurationResponseWriter) Header() http.Header {
	return rw.rw.Header()
}

func (rw *measureWriteDurationResponseWriter) Write(b []byte) (int, error) {
	start := time.Now()
	defer writeDuration.UpdateDuration(start)

	n, err := rw.rw.Write(b)
	return n, err
}

func (rw *measureWriteDurationResponseWriter) WriteHeader(statusCode int) {
	start := time.Now()
	defer writeDuration.UpdateDuration(start)

	rw.rw.WriteHeader(statusCode)
}

func (rw *measureWriteDurationResponseWriter) Flush() {
	start := time.Now()
	defer writeDuration.UpdateDuration(start)

	rw.f.Flush()
}
