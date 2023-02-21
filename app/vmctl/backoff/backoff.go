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
	backoffRetries     = 5
	backoffFactor      = 1.7
	backoffMinDuration = time.Second
)

// retryableFunc describes call back which will repeat on errors
type retryableFunc func() error

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
		select {
		case <-ctx.Done():
			return attempt, nil
		default:
		}
		err := cb()
		if err == nil {
			return attempt, nil
		}
		if errors.Is(err, ErrBadRequest) {
			logger.Errorf("got error: %s on attempt: %d \n", err, attempt)
			return attempt, err // fail fast if not recoverable
		}
		attempt++
		backoff := float64(b.minDuration) * math.Pow(b.factor, float64(i))
		dur := time.Duration(backoff)
		logger.Errorf("got error: %s on attempt: %d \n", err, attempt)
		logger.Infof("next attempt will start after: %s \n", dur.Round(time.Millisecond))
		time.Sleep(time.Duration(backoff))
	}
	return attempt, fmt.Errorf("retry failed after %d retries", b.retries)
}
