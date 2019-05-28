package concurrencylimiter

import (
	"fmt"
	"runtime"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
)

var (
	// ch is the channel for limiting concurrent inserts.
	// Put an item into it before performing an insert and remove
	// the item after the insert is complete.
	ch = make(chan struct{}, runtime.GOMAXPROCS(-1)*2)

	// waitDuration is the amount of time to wait until at least a single
	// concurrent insert out of cap(Ch) inserts is complete.
	waitDuration = time.Second * 30
)

// Do calls f with the limited concurrency.
func Do(f func() error) error {
	// Limit the number of conurrent inserts in order to prevent from excess
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
		return fmt.Errorf("the server is overloaded with %d concurrent inserts; either increase the number of CPUs or reduce the load", cap(ch))
	}
}
