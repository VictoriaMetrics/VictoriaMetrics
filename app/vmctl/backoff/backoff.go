package backoff

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	backoffRetries     = 10
	backoffFactor      = 1.8
	backoffMinDuration = time.Second * 2
)

// retryableFunc describes call back which will repeat on errors
type retryableFunc func() error

// ErrBadRequest is an error returned on bad request
var ErrBadRequest = errors.New("bad request")

// Backoff describes object with backoff policy params
type Backoff struct {
	retries     int
	factor      float64
	minDuration time.Duration
}

// New initialize backoff object
func New() *Backoff {
	return &Backoff{
		retries:     backoffRetries,
		factor:      backoffFactor,
		minDuration: backoffMinDuration,
	}
}

// Retry process retries until all attempts are completed
func (b *Backoff) Retry(ctx context.Context, cb retryableFunc) (uint64, error) {
	var attempt uint64
	for i := 0; i < b.retries; i++ {
		err := cb()
		if err == nil {
			return attempt, nil
		}
		if errors.Is(err, ErrBadRequest) || errors.Is(err, context.Canceled) {
			logger.Errorf("unrecoverable error: %s", err)
			return attempt, err // fail fast if not recoverable
		}
		attempt++
		backoff := float64(b.minDuration) * math.Pow(b.factor, float64(i))
		dur := time.Duration(backoff)
		logger.Errorf("got error: %s on attempt: %d; will retry in %v", err, attempt, dur)

		t := time.NewTimer(dur)
		select {
		case <-t.C:
			// duration elapsed, loop
		case <-ctx.Done():
			// context cancelled, kill the timer if it hasn't fired, and return
			// the last error we got
			if !t.Stop() {
				<-t.C
			}
			return attempt, err
		}
	}
	return attempt, fmt.Errorf("execution failed after %d retry attempts", b.retries)
}
