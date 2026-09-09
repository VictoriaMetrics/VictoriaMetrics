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

func resetInsertLimiter(t *testing.T, limit int) {
	t.Helper()

	concurrencyLimitCh = nil
	concurrencyLimitChOnce = sync.Once{}
	prevLimit := *maxConcurrentInserts
	*maxConcurrentInserts = limit
	t.Cleanup(func() {
		*maxConcurrentInserts = prevLimit
		concurrencyLimitCh = nil
		concurrencyLimitChOnce = sync.Once{}
	})
}

func TestGetReaderInsertTimeoutReleasesStreamSlot(t *testing.T) {
	resetStreamReadersLimiter(t, 1, 10*time.Millisecond)
	resetInsertLimiter(t, 1)

	if err := IncConcurrency(); err != nil {
		t.Fatalf("cannot occupy insert token: %v", err)
	}
	r, err := GetReader(strings.NewReader("test"))
	if err == nil {
		PutReader(r)
		t.Error("expecting an error when insert admission times out")
	} else if esc, ok := err.(*httpserver.ErrorWithStatusCode); !ok || esc.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("unexpected insert admission error: %v", err)
	}
	DecConcurrency()
	if n := len(streamReadersCh); n != 0 {
		t.Fatalf("stream reader reservation leaked after insert timeout: %d", n)
	}

	r, err = GetReader(strings.NewReader("test"))
	if err != nil {
		t.Fatalf("cannot obtain reader after insert token is released: %v", err)
	}
	PutReader(r)
}

func TestGetReaderStreamWaitDoesNotBlockExistingReader(t *testing.T) {
	resetStreamReadersLimiter(t, 1, time.Second)
	resetInsertLimiter(t, 1)

	br := &blockingReader{
		started: make(chan struct{}),
		ready:   make(chan struct{}),
	}
	r, err := GetReader(br)
	if err != nil {
		t.Fatalf("cannot obtain first reader: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 4)
		_, err := r.Read(buf)
		readDone <- err
	}()
	<-br.started

	// The existing reader holds the only stream slot, but releases its insert
	// token while waiting for data. A new reader must not take that token
	// while waiting for the stream slot.
	limitReached := streamReadersLimitReached.Get()
	getDone := make(chan error, 1)
	go func() {
		r, err := GetReader(strings.NewReader("test"))
		if err == nil {
			PutReader(r)
		}
		getDone <- err
	}()
	deadline := time.Now().Add(time.Second)
	for streamReadersLimitReached.Get() == limitReached && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if streamReadersLimitReached.Get() == limitReached {
		t.Error("new reader did not wait for the occupied stream slot")
	}
	if n := len(concurrencyLimitCh); n != 0 {
		t.Errorf("stream slot waiter holds %d insert tokens; want 0", n)
	}

	close(br.ready)
	if err := <-readDone; err != nil {
		t.Errorf("existing reader cannot reacquire insert token: %v", err)
	}
	PutReader(r)
	if err := <-getDone; err != nil {
		t.Errorf("cannot obtain reader after stream slot is released: %v", err)
	}
	if n := len(streamReadersCh); n != 0 {
		t.Errorf("stream reader slots leaked: %d", n)
	}
	if n := len(concurrencyLimitCh); n != 0 {
		t.Errorf("insert tokens leaked: %d", n)
	}
}

type blockingReader struct {
	started chan struct{}
	ready   chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	close(r.started)
	<-r.ready
	return copy(p, "test"), nil
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

			// Decrement before releasing the slot, so current tracks only
			// goroutines that actually hold a stream-reader slot. Otherwise a
			// worker that acquires the freed slot could increment current
			// before this goroutine decrements it, transiently pushing
			// maxCurrent above limit and flaking the assertion below.
			mu.Lock()
			current--
			mu.Unlock()

			PutReader(r)
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
