package httpserver

import (
	"strings"
	"testing"
	"time"
)

func setupLimiter(maxConcurrent, maxPending int, timeout time.Duration) *Limiter {
	old := connTimeout
	connTimeout = &timeout
	defer func() { connTimeout = old }()

	return NewLimiter(maxConcurrent, maxPending)
}

func TestLimiter_Acquire(t *testing.T) {
	t.Run("AcquireAndRelease", func(t *testing.T) {
		l := setupLimiter(1, 10, 10*time.Second)
		if err := l.Acquire(); err != nil {
			t.Fatalf("unexpected error on first acquire: %v", err)
		}
		if l.CurrentConcurrentRequests() == 0 {
			t.Errorf("pending should not be 0 after acquire, got %d", l.CurrentConcurrentRequests())
		}
		if l.CurrentPendingRequests() != 0 {
			t.Errorf("pending should be 0 after acquire, got %d", l.CurrentPendingRequests())
		}
		l.Release()
	})

	t.Run("Queue full", func(t *testing.T) {
		l := setupLimiter(1, 0, 10*time.Second)
		if err := l.Acquire(); err != nil {
			t.Fatalf("unexpected error on first acquire: %v", err)
		}
		defer l.Release()

		err := l.Acquire()
		if !strings.Contains(err.Error(), "queue") {
			t.Errorf("expected queue full error, got %v", err)
		}
	})

	t.Run("FailFast_QueueFull", func(t *testing.T) {
		l := setupLimiter(1, 1, 10*time.Second)
		if err := l.Acquire(); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		defer l.Release()
		err := l.Acquire()
		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("expected queue full error, got %v", err)
		}
	})
}
