package writeconcurrencylimiter

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
)

func resetStreamReadersLimiter(t *testing.T, limit int, queueDuration time.Duration) {
	t.Helper()

	streamReadersCh = nil
	streamReadersChOnce = sync.Once{}

	prevLimit := *maxConcurrentStreamReaders
	*maxConcurrentStreamReaders = limit
	prevQueueDuration := *maxQueueDuration
	*maxQueueDuration = queueDuration

	t.Cleanup(func() {
		*maxConcurrentStreamReaders = prevLimit
		*maxQueueDuration = prevQueueDuration
		streamReadersCh = nil
		streamReadersChOnce = sync.Once{}
	})
}

func TestGetReaderStreamReadersLimit(t *testing.T) {
	resetStreamReadersLimiter(t, 1, 100*time.Millisecond)

	r, err := GetReader(strings.NewReader("test"))
	if err != nil {
		t.Fatalf("unexpected error when obtaining the first reader: %v", err)
	}

	// The second reader must fail with a 503 status code, since the single
	// stream readers slot is occupied by the first reader.
	if _, err := GetReader(strings.NewReader("test")); err == nil {
		t.Fatalf("expecting non-nil error when the stream readers limit is exceeded")
	} else if esc, ok := err.(*httpserver.ErrorWithStatusCode); !ok {
		t.Fatalf("unexpected error type: %v", err)
	} else if esc.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: %d; want %d", esc.StatusCode, http.StatusServiceUnavailable)
	}

	// The slot must be released after PutReader().
	PutReader(r)
	r2, err := GetReader(strings.NewReader("test"))
	if err != nil {
		t.Fatalf("unexpected error after releasing the first reader: %v", err)
	}
	PutReader(r2)
}

func TestGetReaderStreamReadersLimitConcurrent(t *testing.T) {
	const limit = 3
	const workers = 20
	resetStreamReadersLimiter(t, limit, 50*time.Millisecond)

	var mu sync.Mutex
	current := 0
	maxCurrent := 0

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			r, err := GetReader(strings.NewReader("test"))
			if err != nil {
				// The limit was exceeded, which is expected for some of the workers.
				return
			}

			mu.Lock()
			current++
			if current > maxCurrent {
				maxCurrent = current
			}
			mu.Unlock()

			PutReader(r)

			mu.Lock()
			current--
			mu.Unlock()
		}()
	}
	wg.Wait()

	if maxCurrent > limit {
		t.Fatalf("too many concurrent stream readers: %d; mustn't exceed %d", maxCurrent, limit)
	}
	if maxCurrent == 0 {
		t.Fatalf("unexpected zero concurrent stream readers")
	}
}

func TestStreamReadersDefaultLimit(t *testing.T) {
	streamReadersCh = nil
	streamReadersChOnce = sync.Once{}

	prevLimit := *maxConcurrentStreamReaders
	*maxConcurrentStreamReaders = 0
	t.Cleanup(func() {
		*maxConcurrentStreamReaders = prevLimit
		streamReadersCh = nil
		streamReadersChOnce = sync.Once{}
	})

	initStreamReadersCh()
	if cap(streamReadersCh) < 64 {
		t.Fatalf("unexpectedly low default stream readers limit: %d; want at least 64", cap(streamReadersCh))
	}
}
