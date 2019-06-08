package concurrencylimiter

import (
	"flag"
	"fmt"
	"runtime"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var maxConcurrentInserts = flag.Int("maxConcurrentInserts", runtime.GOMAXPROCS(-1)*4, "The maximum number of concurrent inserts")

var (
	// ch is the channel for limiting concurrent calls to Do.
	ch chan struct{}

	// waitDuration is the amount of time to wait until at least a single
	// concurrent Do call out of cap(ch) inserts is complete.
	waitDuration = time.Second * 30
)

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
	t := timerpool.Get(waitDuration)
	select {
	case ch <- struct{}{}:
		timerpool.Put(t)
		err := f()
		<-ch
		return err
	case <-t.C:
		timerpool.Put(t)
		concurrencyLimitErrors.Inc()
		return fmt.Errorf("the server is overloaded with %d concurrent inserts; either increase -maxConcurrentInserts or reduce the load", cap(ch))
	}
}

var concurrencyLimitErrors = metrics.NewCounter(`vm_concurrency_limit_errors_total`)
