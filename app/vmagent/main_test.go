package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestStartConfigReloader(t *testing.T) {
	const interval = 10 * time.Millisecond

	var reloadCount atomic.Int64
	stop := startConfigReloader(interval, func() {
		reloadCount.Add(1)
	})

	deadline := time.After(time.Second)
	for reloadCount.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("config reload callback wasn't called")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	stop()
	countAfterStop := reloadCount.Load()
	time.Sleep(interval * 2)
	if count := reloadCount.Load(); count != countAfterStop {
		t.Fatalf("config reload callback was called after stop: got %d calls, want %d", count, countAfterStop)
	}
}
