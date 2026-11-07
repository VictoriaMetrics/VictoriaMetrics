package vmselect

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestRequestHandlerSlowQueryLogIncludesWaitDuration(t *testing.T) {
	origMaxConcurrentRequests := maxConcurrentRequests
	origMaxQueueDuration := maxQueueDuration
	origConcurrencyLimitCh := concurrencyLimitCh
	origLogSlowQueryDuration := *logSlowQueryDuration
	defer func() {
		maxConcurrentRequests = origMaxConcurrentRequests
		maxQueueDuration = origMaxQueueDuration
		concurrencyLimitCh = origConcurrencyLimitCh
		*logSlowQueryDuration = origLogSlowQueryDuration
	}()

	maxConcurrentRequests = 1
	maxQueueDuration = 5 * time.Second
	concurrencyLimitCh = make(chan struct{}, maxConcurrentRequests)
	*logSlowQueryDuration = 40 * time.Millisecond

	var logBuf bytes.Buffer
	logger.SetOutputForTests(&logBuf)
	defer logger.ResetOutputForTest()

	// Occupy the single concurrency slot for 60ms so the incoming request waits in the queue.
	concurrencyLimitCh <- struct{}{}
	go func() {
		time.Sleep(60 * time.Millisecond)
		<-concurrencyLimitCh
	}()

	req := httptest.NewRequest(http.MethodPost, "/tags/tagSeries", nil)
	rec := httptest.NewRecorder()
	if !RequestHandler(rec, req) {
		t.Fatalf("expected RequestHandler to handle /tags/tagSeries")
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "slow query according to -search.logSlowQueryDuration=") {
		t.Fatalf("expected slow query log line, got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "totalDuration=") {
		t.Fatalf("expected totalDuration in slow query log line, got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "waitDuration=") {
		t.Fatalf("expected waitDuration in slow query log line, got: %q", logOutput)
	}
	if strings.Contains(logOutput, "waitDuration=0.000 seconds") {
		t.Fatalf("expected non-zero waitDuration in slow query log line when request waited in queue, got: %q", logOutput)
	}
}
