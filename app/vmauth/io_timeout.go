package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

var readDuration = metrics.NewSummaryExt(`vmauth_request_read_duration_seconds`, time.Second*30, []float64{0.5, 0.8, 0.97, 0.99})

var readTimeout = flag.Duration("readTimeout", 0, "The maximum duration for a single read call when exceeded the connection is closed. Zero disables request read timeout. "+
	"See also -writeTimeout")

var errReadTimeout = fmt.Errorf("request read timeout")

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
