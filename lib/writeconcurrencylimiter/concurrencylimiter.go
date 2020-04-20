package writeconcurrencylimiter

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxConcurrentInserts = flag.Int("maxConcurrentInserts", runtime.GOMAXPROCS(-1)*4, "The maximum number of concurrent inserts. Default value should work for most cases, "+
		"since it minimizes the overhead for concurrent inserts. This option is tigthly coupled with -insert.maxQueueDuration")
	maxQueueDuration = flag.Duration("insert.maxQueueDuration", time.Minute, "The maximum duration for waiting in the queue for insert requests due to -maxConcurrentInserts")
)

// ch is the channel for limiting concurrent calls to Do.
var ch chan struct{}

// Init initializes concurrencylimiter.
//
// Init must be called after flag.Parse call.
func Init() {
	ch = make(chan struct{}, *maxConcurrentInserts)
}

// Do calls f with the limited concurrency.
func Do(f func() error) error {
	// Limit the number of conurrent f calls in order to prevent from excess
	// memory usage and CPU trashing.
	select {
	case ch <- struct{}{}:
		err := f()
		<-ch
		return err
	default:
	}

	// All the workers are busy.
	// Sleep for up to *maxQueueDuration.
	concurrencyLimitReached.Inc()
	t := timerpool.Get(*maxQueueDuration)
	select {
	case ch <- struct{}{}:
		timerpool.Put(t)
		err := f()
		<-ch
		return err
	case <-t.C:
		timerpool.Put(t)
		concurrencyLimitTimeout.Inc()
		return &httpserver.ErrorWithStatusCode{
			Err: fmt.Errorf("cannot handle more than %d concurrent inserts during %s; possible solutions: "+
				"increase `-insert.maxQueueDuration`, increase `-maxConcurrentInserts`, increase server capacity", *maxConcurrentInserts, *maxQueueDuration),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
}

var (
	concurrencyLimitReached = metrics.NewCounter(`vm_concurrent_insert_limit_reached_total`)
	concurrencyLimitTimeout = metrics.NewCounter(`vm_concurrent_insert_limit_timeout_total`)

	_ = metrics.NewGauge(`vm_concurrent_insert_capacity`, func() float64 {
		return float64(cap(ch))
	})
	_ = metrics.NewGauge(`vm_concurrent_insert_current`, func() float64 {
		return float64(len(ch))
	})
)
