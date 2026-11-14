package vm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
)

func TestBlockAckWaitsForAllItems(t *testing.T) {
	im := &Importer{}
	b := im.NewBlockAck()
	for range 3 {
		if err := b.add(); err != nil {
			t.Fatal(err)
		}
		b.wg.Done()
	}
	done := make(chan error, 1)
	go func() {
		done <- im.WaitBlock(b)
	}()

	select {
	case <-done:
		t.Fatal("WaitBlock returned before items finished")
	case <-time.After(10 * time.Millisecond):
	}
	b.finish(nil)
	b.finish(errors.New("import failed"))
	select {
	case <-done:
		t.Fatal("WaitBlock returned before all items finished")
	case <-time.After(10 * time.Millisecond):
	}
	b.finish(nil)

	select {
	case err := <-done:
		if err == nil || err.Error() != "import failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitBlock did not return after all items finished")
	}
}

func TestBlockAckIndependent(t *testing.T) {
	im := &Importer{}
	b1, b2 := im.NewBlockAck(), im.NewBlockAck()

	if err := b1.add(); err != nil {
		t.Fatal(err)
	}
	b1.wg.Done()

	if err := b2.add(); err != nil {
		t.Fatal(err)
	}
	b2.wg.Done()

	var wg sync.WaitGroup
	wg.Add(2)

	type blockResult struct {
		blockID string
		err     error
	}

	got := make(chan blockResult, 2)

	go func() {
		defer wg.Done()
		got <- blockResult{
			blockID: "b1",
			err:     im.WaitBlock(b1),
		}
	}()

	go func() {
		defer wg.Done()
		got <- blockResult{
			blockID: "b2",
			err:     im.WaitBlock(b2),
		}
	}()

	b1.finish(nil)
	b2.finish(errors.New("block 2 failed"))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitBlock deadlocked")
	}

	close(got)

	results := make(map[string]error, 2)
	for result := range got {
		results[result.blockID] = result.err
	}

	if err := results["b1"]; err != nil {
		t.Errorf("WaitBlock(b1) returned unexpected error: %v; want nil", err)
	}

	if err := results["b2"]; err == nil || err.Error() != "block 2 failed" {
		t.Errorf("WaitBlock(b2) returned %v; want %q", err, "block 2 failed")
	}
}

func TestAddExtraLabelsToImportPath_Failure(t *testing.T) {
	f := func(path string, extraLabels []string) {
		t.Helper()

		_, err := AddExtraLabelsToImportPath(path, extraLabels)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// bad incorrect format for extra label
	f("/api/v1/import", []string{"label=value", "bad_label_wo_value"})
}

func TestAddExtraLabelsToImportPath_Success(t *testing.T) {
	f := func(path string, extraLabels []string, resultExpected string) {
		t.Helper()

		result, err := AddExtraLabelsToImportPath(path, extraLabels)
		if err != nil {
			t.Fatalf("AddExtraLabelsToImportPath() error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	// ok w/o extra labels
	f("/api/v1/import", nil, "/api/v1/import")

	// ok one extra label
	f("/api/v1/import", []string{"instance=host-1"}, "/api/v1/import?extra_label=instance%3Dhost-1")

	// ok two extra labels
	f("/api/v1/import", []string{"instance=host-2", "job=vmagent"}, "/api/v1/import?extra_label=instance%3Dhost-2&extra_label=job%3Dvmagent")

	// ok two extra with exist param
	f("/api/v1/import?timeout=50", []string{"instance=host-2", "job=vmagent"}, "/api/v1/import?timeout=50&extra_label=instance%3Dhost-2&extra_label=job%3Dvmagent")

	// ok special chars in label value
	f("/api/v1/import", []string{"team=a&b"}, "/api/v1/import?extra_label=team%3Da%26b")
}

func getTestImporter(t *testing.T, handler http.HandlerFunc) (*Importer, *httptest.Server) {
	t.Helper()
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", handler)
	srv := httptest.NewServer(mux)

	boff, err := backoff.New(3, 1.5, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create test backoff: %v", err)
	}

	cfg := Config{
		Addr:        srv.URL,
		Concurrency: 2,
		BatchSize:   10,
		Backoff:     boff,
	}

	im, err := NewImporter(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create test importer: %v", err)
	}
	return im, srv
}

func TestWaitBlockFlushesPartialBatch(t *testing.T) {
	im, srv := getTestImporter(t, nil)
	defer srv.Close()
	defer im.Close()

	b := im.NewBlockAck()
	err := im.InputBlock(b, &TimeSeries{Values: []float64{1.0}})
	if err != nil {
		t.Fatalf("unexpected error on InputBlock: %v", err)
	}

	if err := im.WaitBlock(b); err != nil {
		t.Fatalf("WaitBlock failed: %v", err)
	}
}

func TestWaitBlockFlushesAllWorkers(t *testing.T) {
	blockServer := make(chan struct{})
	serverReached := make(chan struct{}, 2)
	var mu sync.Mutex
	reqCount := 0

	handler := func(w http.ResponseWriter, r *http.Request) {
		serverReached <- struct{}{}
		<-blockServer
		mu.Lock()
		reqCount++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}
	im, srv := getTestImporter(t, handler)
	defer srv.Close()
	defer im.Close()

	b := im.NewBlockAck()

	for i := 0; i < 19; i++ {
		if err := im.InputBlock(b, &TimeSeries{Values: []float64{float64(i)}}); err != nil {
			t.Fatalf("InputBlock failed: %v", err)
		}
	}

	<-serverReached

	errCh := make(chan error, 1)
	go func() {
		errCh <- im.WaitBlock(b)
	}()

	<-serverReached

	close(blockServer)

	if err := <-errCh; err != nil {
		t.Fatalf("WaitBlock failed: %v", err)
	}

	mu.Lock()
	if reqCount != 2 {
		t.Fatalf("expected exactly 2 HTTP requests (one full batch, one partial), got %d", reqCount)
	}
	mu.Unlock()
}

func TestWaitBlockConcurrentSubmissions(t *testing.T) {
	im, srv := getTestImporter(t, nil)
	defer srv.Close()
	defer im.Close()

	b := im.NewBlockAck()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val float64) {
			defer wg.Done()
			err := im.InputBlock(b, &TimeSeries{Values: []float64{val}})
			if err != nil && err.Error() != "cannot add to a closed blockAck" {
				t.Errorf("unexpected error from InputBlock: %v", err)
			}
		}(float64(i))
	}

	if err := im.WaitBlock(b); err != nil {
		t.Fatalf("WaitBlock failed: %v", err)
	}
	wg.Wait()
}

func TestWaitBlockConcurrentBlocks(t *testing.T) {
	im, srv := getTestImporter(t, nil)
	defer srv.Close()
	defer im.Close()

	b1 := im.NewBlockAck()
	b2 := im.NewBlockAck()

	_ = im.InputBlock(b1, &TimeSeries{Values: []float64{1}})
	_ = im.InputBlock(b2, &TimeSeries{Values: []float64{2}})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := im.WaitBlock(b1); err != nil {
			t.Errorf("WaitBlock b1 failed: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := im.WaitBlock(b2); err != nil {
			t.Errorf("WaitBlock b2 failed: %v", err)
		}
	}()
	wg.Wait()
}

func TestWaitBlockImportFailure(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error mock"))
	}
	im, srv := getTestImporter(t, handler)
	defer srv.Close()
	defer im.Close()

	b := im.NewBlockAck()
	_ = im.InputBlock(b, &TimeSeries{Values: []float64{1}})

	err := im.WaitBlock(b)
	if err == nil {
		t.Fatal("expected error from failed import, got nil")
	}
	if !strings.Contains(err.Error(), "retry attempts") && !strings.Contains(err.Error(), "import failed") {
		t.Fatalf("expected error message to indicate retry exhaustion, got: %v", err)
	}
}

func TestWaitBlockRetrySuccess(t *testing.T) {
	var attempts int
	var mu sync.Mutex
	handler := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		curr := attempts
		mu.Unlock()

		if curr == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}

	im, srv := getTestImporter(t, handler)
	defer srv.Close()
	defer im.Close()

	b := im.NewBlockAck()
	_ = im.InputBlock(b, &TimeSeries{Values: []float64{1}})

	err := im.WaitBlock(b)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	mu.Lock()
	if attempts != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", attempts)
	}
	mu.Unlock()
}

func TestImporterFlushConcurrent(t *testing.T) {
	var mu sync.Mutex
	reqs := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}
	im, srv := getTestImporter(t, handler)
	defer srv.Close()
	defer im.Close()

	_ = im.Input(&TimeSeries{Values: []float64{1}})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			im.Flush()
		}()
	}
	wg.Wait()

	mu.Lock()
	if reqs == 0 {
		t.Fatalf("expected at least 1 request triggered by flush, got 0")
	}
	mu.Unlock()
}

func TestImporterFlushAndCloseRace(t *testing.T) {
	im, srv := getTestImporter(t, nil)
	defer srv.Close()

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			im.Flush()
		}()
		go func() {
			defer wg.Done()
			im.Close()
		}()
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Flush and Close race deadlocked")
	}
}

func TestImporterInputErrorDelivery(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}
	im, srv := getTestImporter(t, handler)
	defer srv.Close()

	errCh := make(chan *ImportError, 20)
	consumerDone := make(chan struct{})

	go func() {
		defer close(consumerDone)
		for err := range im.Errors() {
			errCh <- err
		}
	}()

	for range 20 {
		if err := im.Input(&TimeSeries{Values: []float64{1}}); err != nil {
			t.Fatalf("Input failed: %v", err)
		}
	}

	im.Flush()

	select {
	case err := <-errCh:
		if err == nil || err.Err == nil {
			t.Fatal("expected actual ImportError, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for asynchronous error delivery")
	}

	im.Close()

	select {
	case <-consumerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("error consumer did not stop after importer closed")
	}
}

func TestImporterNoGoroutineLeakOnErrors(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}
	im, srv := getTestImporter(t, handler)
	defer srv.Close()

	for i := 0; i < 50; i++ {
		_ = im.Input(&TimeSeries{Values: []float64{1}})
		im.Flush()
	}

	done := make(chan struct{})
	go func() {
		im.Close()
		close(done)
	}()

	var errCount int
	for range im.Errors() {
		errCount++
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() deadlocked waiting on leaked error-delivery goroutines")
	}

	if errCount == 0 {
		t.Fatal("expected errors, got none")
	}
}

func TestImporterReusableAfterWaitBlock(t *testing.T) {
	im, srv := getTestImporter(t, nil)
	defer srv.Close()
	defer im.Close()

	b1 := im.NewBlockAck()
	_ = im.InputBlock(b1, &TimeSeries{Values: []float64{1}})
	if err := im.WaitBlock(b1); err != nil {
		t.Fatalf("WaitBlock 1 failed: %v", err)
	}

	b2 := im.NewBlockAck()
	_ = im.InputBlock(b2, &TimeSeries{Values: []float64{2}})
	if err := im.WaitBlock(b2); err != nil {
		t.Fatalf("WaitBlock 2 failed: %v", err)
	}
}

func TestBlockAckRejectsSubmissionAfterSeal(t *testing.T) {
	im, srv := getTestImporter(t, nil)
	defer srv.Close()
	defer im.Close()

	b := im.NewBlockAck()
	_ = im.InputBlock(b, &TimeSeries{Values: []float64{1}})

	if err := im.WaitBlock(b); err != nil {
		t.Fatalf("WaitBlock failed: %v", err)
	}

	err := im.InputBlock(b, &TimeSeries{Values: []float64{2}})
	if err == nil {
		t.Fatal("expected error submitting to sealed block, got nil")
	}
	if err.Error() != "cannot add to a closed blockAck" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestImporterPrecisionSettings(t *testing.T) {
	tests := []struct {
		name               string
		significantFigures int
		roundDigits        int
	}{
		{
			name:               "default precision",
			significantFigures: 0,
			roundDigits:        0,
		},
		{
			name:               "significant figures only",
			significantFigures: 5,
			roundDigits:        0,
		},
		{
			name:               "round digits only",
			significantFigures: 0,
			roundDigits:        3,
		},
		{
			name:               "both precision settings",
			significantFigures: 5,
			roundDigits:        3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/health" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			boff, err := backoff.New(3, 1.5, 10*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}

			im, err := NewImporter(context.Background(), Config{
				Addr:               srv.URL,
				Concurrency:        1,
				BatchSize:          10,
				Backoff:            boff,
				SignificantFigures: tc.significantFigures,
				RoundDigits:        tc.roundDigits,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer im.Close()

			if got := im.SignificantFigures(); got != tc.significantFigures {
				t.Errorf("SignificantFigures() = %d; want %d", got, tc.significantFigures)
			}

			if got := im.RoundDigits(); got != tc.roundDigits {
				t.Errorf("RoundDigits() = %d; want %d", got, tc.roundDigits)
			}
		})
	}
}
