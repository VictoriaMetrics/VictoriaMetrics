package writeconcurrencylimiter

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxConcurrentInserts = flag.Int("maxConcurrentInserts", 2*cgroup.AvailableCPUs(), "The maximum number of concurrent insert requests. "+
		"Set higher value when clients send data over slow networks. "+
		"Default value depends on the number of available CPU cores. It should work fine in most cases since it minimizes resource usage. "+
		"See also -insert.maxQueueDuration")
	maxQueueDuration = flag.Duration("insert.maxQueueDuration", time.Minute, "The maximum duration to wait in the queue when -maxConcurrentInserts "+
		"concurrent insert requests are executed")
)

// Reader is a reader, which decreases the concurrency before every Read() call
// and increases the concurrency after Read() call.
//
// It effectively limits the number of concurrent goroutines,
// which may process results returned by concurrently processed Reader structs.
//
// The Reader must be obtained via GetReader() call.
type Reader struct {
	r io.Reader
}

// GetReader returns the Reader for r.
//
// The PutReader() must be called when the returned Reader is no longer needed.
func GetReader(r io.Reader) (*Reader, error) {
	if err := IncConcurrency(); err != nil {
		return nil, err
	}

	v := readerPool.Get()
	if v == nil {
		v = &Reader{}
	}
	rr := v.(*Reader)
	rr.r = r

	return rr, nil
}

// PutReader returns the r to the pool.
//
// It decreases the concurrency.
func PutReader(r *Reader) {
	r.r = nil
	readerPool.Put(r)

	DecConcurrency()
}

var readerPool sync.Pool

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	DecConcurrency()

	n, err := r.r.Read(p)

	if errC := IncConcurrency(); errC != nil {
		return n, errC
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8704
		err = fmt.Errorf("%w: while reading the request body. This might be caused by a timeout on the client side. "+
			"Possible solutions: to lower -insert.maxQueueDuration below the client’s timeout; to increase the client-side timeout; "+
			"to increase compute resources at the server; to increase -maxConcurrentInserts", err)
	}

	return n, err
}

func initConcurrencyLimitCh() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrentInserts)
}

var (
	concurrencyLimitCh     chan struct{}
	concurrencyLimitChOnce sync.Once
)

// IncConcurrency obtains a concurrency token from -maxConcurrentInserts.
//
// The obtained token must be returned back via DecConcurrency() call.
func IncConcurrency() error {
	concurrencyLimitChOnce.Do(initConcurrencyLimitCh)

	select {
	case concurrencyLimitCh <- struct{}{}:
		return nil
	default:
	}

	concurrencyLimitReached.Inc()
	t := timerpool.Get(*maxQueueDuration)
	defer timerpool.Put(t)
	select {
	case concurrencyLimitCh <- struct{}{}:
		return nil
	case <-t.C:
		concurrencyLimitTimeout.Inc()
		return &httpserver.ErrorWithStatusCode{
			Err: fmt.Errorf("cannot process insert request for %.3f seconds because %d concurrent insert requests are executed. "+
				"Possible solutions: to reduce workload; to increase compute resources at the server; "+
				"to increase -insert.maxQueueDuration; to increase -maxConcurrentInserts",
				maxQueueDuration.Seconds(), *maxConcurrentInserts),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
}

// DecConcurrency returns the token obtained via IncConcurrency(), so other goroutines could obtain it.
func DecConcurrency() {
	<-concurrencyLimitCh
}

var (
	concurrencyLimitReached = metrics.NewCounter(`vm_concurrent_insert_limit_reached_total`)
	concurrencyLimitTimeout = metrics.NewCounter(`vm_concurrent_insert_limit_timeout_total`)

	_ = metrics.NewGauge(`vm_concurrent_insert_capacity`, func() float64 {
		concurrencyLimitChOnce.Do(initConcurrencyLimitCh)
		return float64(cap(concurrencyLimitCh))
	})
	_ = metrics.NewGauge(`vm_concurrent_insert_current`, func() float64 {
		concurrencyLimitChOnce.Do(initConcurrencyLimitCh)
		return float64(len(concurrencyLimitCh))
	})
)
