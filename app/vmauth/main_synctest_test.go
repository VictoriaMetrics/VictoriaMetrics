//go:build synctest

package main

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
)

func TestBufferRequestBody_Timeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		defaultMaxQueueDuration := *maxQueueDuration
		defer func() {
			*maxQueueDuration = defaultMaxQueueDuration
		}()

		*maxQueueDuration = 100 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), *maxQueueDuration)
		defer cancel()

		_, err := bufferRequestBody(ctx, &timeoutBody{delay: *maxQueueDuration + 100*time.Millisecond}, "foo")
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}

		esc, ok := err.(*httpserver.ErrorWithStatusCode)
		if !ok {
			t.Fatalf("unexpected error type: %s", err)
		}
		if esc.StatusCode != http.StatusRequestTimeout {
			t.Fatalf("read request body timeout meet unexpected status code; got %d; want %d", esc.StatusCode, http.StatusRequestTimeout)
		}
	})
}

type timeoutBody struct {
	delay time.Duration
}

func (r *timeoutBody) Read(_ []byte) (int, error) {
	time.Sleep(r.delay)
	return 0, context.DeadlineExceeded
}

func (r *timeoutBody) Close() error {
	return nil
}
