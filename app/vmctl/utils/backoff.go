package utils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type retryableFunc func() error

var ErrBadRequest = errors.New("bad request")

// Retry describes object with backoff policy params
type Retry struct {
	backoffRetries     int
	backoffFactor      float64
	backoffMinDuration time.Duration
}

// NewRetry initialize retry object
func NewRetry(backoffRetries int, backoffFactor float64, backoffMinDuration time.Duration) *Retry {
	return &Retry{
		backoffRetries:     backoffRetries,
		backoffFactor:      backoffFactor,
		backoffMinDuration: backoffMinDuration,
	}
}

// Do process retries until all attempts are completed
func (r *Retry) Do(ctx context.Context, cb retryableFunc) (uint64, error) {
	var attempt uint64
	for i := 0; i < r.backoffRetries; i++ {
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
			logger.Errorf("got error: %s on attempt: %d", err, attempt)
			return attempt, err // fail fast if not recoverable
		}
		attempt++
		backoff := float64(r.backoffMinDuration) * math.Pow(r.backoffFactor, float64(i))
		dur := time.Duration(backoff)
		logger.Errorf("got error: %s on attempt: %d", err, attempt)
		logger.Infof("next attempt will start after: %s", dur.Round(time.Millisecond))
		time.Sleep(time.Duration(backoff))
	}
	return attempt, fmt.Errorf("retry failed after %d retries", r.backoffRetries)
}
