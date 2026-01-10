package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var readDuration = metrics.NewSummaryExt(`vmauth_request_read_duration_seconds`, time.Second*30, []float64{0.5, 0.8, 0.97, 0.99})
var writeDuration = metrics.NewSummaryExt(`vmauth_response_write_duration_seconds`, time.Second*30, []float64{0.5, 0.8, 0.97, 0.99})

var readTimeout = flag.Duration("readTimeout", 0, "The maximum duration for a single read call when exceeded the connection is closed. Zero disables request read timeout. "+
	"See also -writeTimeout")
var writeTimeout = flag.Duration("writeTimeout", 0, "The maximum duration for a single write call when exceeded the connection is closed. Zero disables response write timeout. "+
	"See also -readTimeout")

var errReadTimeout = fmt.Errorf("request read timeout")
var errWriteTimeout = fmt.Errorf("response write timeout")

type measureReadDurationBody struct {
	r io.ReadCloser

	err error
}

func (r *measureReadDurationBody) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}

	start := time.Now()

	n, err = r.r.Read(p)

	dur := time.Since(start)
	readDuration.Update(dur.Seconds())

	if err == nil && *readTimeout > 0 && dur > *readTimeout {
		r.err = errReadTimeout
	}

	return n, err
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

	err error
}

func (rw *measureWriteDurationResponseWriter) Header() http.Header {
	return rw.rw.Header()
}

func (rw *measureWriteDurationResponseWriter) Write(b []byte) (int, error) {
	if rw.err != nil {
		return 0, rw.err
	}

	start := time.Now()

	n, err := rw.rw.Write(b)

	dur := time.Since(start)
	writeDuration.Update(dur.Seconds())

	if err == nil && *writeTimeout > 0 && dur > *writeTimeout {
		rw.err = errWriteTimeout
	}

	return n, err
}

func (rw *measureWriteDurationResponseWriter) WriteHeader(statusCode int) {
	rw.rw.WriteHeader(statusCode)
}

func (rw *measureWriteDurationResponseWriter) Flush() {
	start := time.Now()

	rw.f.Flush()

	dur := time.Since(start)
	writeDuration.Update(dur.Seconds())
	if *writeTimeout > 0 && dur > *writeTimeout {
		rw.err = errWriteTimeout
	}
}
