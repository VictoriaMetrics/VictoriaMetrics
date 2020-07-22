package pacelimiter

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestPacelimiter(t *testing.T) {
	t.Run("nonblocking", func(t *testing.T) {
		pl := New()
		ch := make(chan struct{}, 10)
		for i := 0; i < cap(ch); i++ {
			go func() {
				for j := 0; j < 10; j++ {
					pl.WaitIfNeeded()
					runtime.Gosched()
				}
				ch <- struct{}{}
			}()
		}

		// Check that all the goroutines are finished.
		timeoutCh := time.After(5 * time.Second)
		for i := 0; i < cap(ch); i++ {
			select {
			case <-ch:
			case <-timeoutCh:
				t.Fatalf("timeout")
			}
		}
		if n := pl.DelaysTotal(); n > 0 {
			t.Fatalf("unexpected non-zero number of delays: %d", n)
		}
	})
	t.Run("blocking", func(t *testing.T) {
		pl := New()
		pl.Inc()
		ch := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i := 0; i < cap(ch); i++ {
			wg.Add(1)
			go func() {
				wg.Done()
				for j := 0; j < 10; j++ {
					pl.WaitIfNeeded()
				}
				ch <- struct{}{}
			}()
		}

		// Check that all the goroutines created above are started and blocked in WaitIfNeeded
		wg.Wait()
		select {
		case <-ch:
			t.Fatalf("the pl must be blocked")
		default:
		}

		// Unblock goroutines and check that they are unblocked.
		pl.Dec()
		timeoutCh := time.After(5 * time.Second)
		for i := 0; i < cap(ch); i++ {
			select {
			case <-ch:
			case <-timeoutCh:
				t.Fatalf("timeout")
			}
		}
		if n := pl.DelaysTotal(); n == 0 {
			t.Fatalf("expecting non-zero number of delays")
		}
		// Verify that the pl is unblocked now.
		pl.WaitIfNeeded()
	})
	t.Run("concurrent_inc_dec", func(t *testing.T) {
		pl := New()
		ch := make(chan struct{}, 10)
		for i := 0; i < cap(ch); i++ {
			go func() {
				for j := 0; j < 10; j++ {
					pl.Inc()
					runtime.Gosched()
					pl.Dec()
				}
				ch <- struct{}{}
			}()
		}

		// Verify that all the goroutines are finished
		timeoutCh := time.After(5 * time.Second)
		for i := 0; i < cap(ch); i++ {
			select {
			case <-ch:
			case <-timeoutCh:
				t.Fatalf("timeout")
			}
		}
		// Verify that the pl is unblocked.
		pl.WaitIfNeeded()
		if n := pl.DelaysTotal(); n > 0 {
			t.Fatalf("expecting zer number of delays; got %d", n)
		}
	})
}
