package utils

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type callback func() error

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
func (r *Retry) Do(cb callback) (uint64, error) {
	var attempt uint64
	for i := 0; i < r.backoffRetries; i++ {
		err := cb()
		if err == nil {
			return attempt, nil
		}
		if errors.Is(err, ErrBadRequest) {
			logger.Errorf("got bad request try to call callback, attempt: %d", attempt)
			return attempt, err // fail fast if not recoverable
		}
		attempt++
		backoff := float64(r.backoffMinDuration) * math.Pow(r.backoffFactor, float64(i))
		dur := time.Duration(backoff)
		logger.Errorf("error trying to call callback: %s, attempt: %d", err, attempt)
		logger.Infof("next try after: %s", dur.Round(time.Millisecond))
		time.Sleep(time.Duration(backoff))
	}
	return attempt, fmt.Errorf("retry failed after %d retries", r.backoffRetries)
}
