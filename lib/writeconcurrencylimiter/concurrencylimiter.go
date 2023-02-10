package writeconcurrencylimiter

import (
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
		"Default value should work for most cases, since it minimizes the memory usage. The default value can be increased when clients send data over slow networks. "+
		"See also -insert.maxQueueDuration")
	maxQueueDuration = flag.Duration("insert.maxQueueDuration", time.Minute, "The maximum duration to wait in the queue when -maxConcurrentInserts "+
		"concurrent insert requests are executed")
)

// Reader is a reader, which increases the concurrency after the first Read() call
//
// The concurrency can be reduced by calling DecConcurrency().
// Then the concurrency is increased after the next Read() call.
type Reader struct {
	r                    io.Reader
	increasedConcurrency bool
}

// GetReader returns the Reader for r.
//
// The PutReader() must be called when the returned Reader is no longer needed.
func GetReader(r io.Reader) *Reader {
	v := readerPool.Get()
	if v == nil {
		return &Reader{
			r: r,
		}
	}
	rr := v.(*Reader)
	rr.r = r
	return rr
}

// PutReader returns the r to the pool.
//
// It decreases the concurrency if r has increased concurrency.
func PutReader(r *Reader) {
	r.DecConcurrency()
	r.r = nil
	readerPool.Put(r)
}

var readerPool sync.Pool

// Read implements io.Reader.
//
// It increases concurrency after the first call or after the next call after DecConcurrency() call.
func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if !r.increasedConcurrency {
		if !incConcurrency() {
			err = &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("cannot process insert request for %.3f seconds because %d concurrent insert requests are executed. "+
					"Possible solutions: to reduce workload; to increase compute resources at the server; "+
					"to increase -insert.maxQueueDuration; to increase -maxConcurrentInserts",
					maxQueueDuration.Seconds(), *maxConcurrentInserts),
				StatusCode: http.StatusServiceUnavailable,
			}
			return 0, err
		}
		r.increasedConcurrency = true
	}
	return n, err
}

// DecConcurrency decreases the concurrency, so it could be increased again after the next Read() call.
func (r *Reader) DecConcurrency() {
	if r.increasedConcurrency {
		decConcurrency()
		r.increasedConcurrency = false
	}
}

func initConcurrencyLimitCh() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrentInserts)
}

var (
	concurrencyLimitCh     chan struct{}
	concurrencyLimitChOnce sync.Once
)

func incConcurrency() bool {
	concurrencyLimitChOnce.Do(initConcurrencyLimitCh)

	select {
	case concurrencyLimitCh <- struct{}{}:
		return true
	default:
	}

	concurrencyLimitReached.Inc()
	t := timerpool.Get(*maxQueueDuration)
	select {
	case concurrencyLimitCh <- struct{}{}:
		timerpool.Put(t)
		return true
	case <-t.C:
		timerpool.Put(t)
		concurrencyLimitTimeout.Inc()
		return false
	}
}

func decConcurrency() {
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
