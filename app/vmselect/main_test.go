package vmselect

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	const waitPrefix = "waitDuration="
	waitIdx := strings.Index(logOutput, waitPrefix)
	if waitIdx < 0 {
		t.Fatalf("expected waitDuration in slow query log line, got: %q", logOutput)
	}
	waitRest := logOutput[waitIdx+len(waitPrefix):]
	waitFields := strings.Fields(waitRest)
	if len(waitFields) == 0 {
		t.Fatalf("expected numeric value after waitDuration= in slow query log line, got: %q", logOutput)
	}
	waitSec, err := strconv.ParseFloat(waitFields[0], 64)
	if err != nil {
		t.Fatalf("cannot parse waitDuration=%q as float: %v (log: %q)", waitFields[0], err, logOutput)
	}
	if waitSec < 0.05 {
		t.Fatalf("expected waitDuration >= 0.05 seconds when request waited ~60ms in queue, got %.3f seconds (log: %q)", waitSec, logOutput)
	}
}
