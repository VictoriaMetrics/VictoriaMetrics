package backoff

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestBackoffRetry_Failure(t *testing.T) {
	f := func(backoffFactor float64, backoffRetries int, cancelTimeout time.Duration, retryFunc func() error, resultExpected int) {
		t.Helper()

		r := &Backoff{
			retries:     backoffRetries,
			factor:      backoffFactor,
			minDuration: time.Millisecond * 10,
		}
		ctx := context.Background()
		if cancelTimeout != 0 {
			newCtx, cancelFn := context.WithTimeout(context.Background(), cancelTimeout)
			ctx = newCtx
			defer cancelFn()
		}

		result, err := r.Retry(ctx, retryFunc)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if result != uint64(resultExpected) {
			t.Fatalf("unexpected result: got %d; want %d", result, resultExpected)
		}
	}

	// return bad request
	retryFunc := func() error {
		return ErrBadRequest
	}
	f(0, 0, 0, retryFunc, 0)

	// empty retries values
	retryFunc = func() error {
		time.Sleep(time.Millisecond * 100)
		return nil
	}
	f(0, 0, 0, retryFunc, 0)

	// all retries failed test
	backoffFactor := 0.1
	backoffRetries := 5
	cancelTimeout := time.Second * 0
	retryFunc = func() error {
		t := time.NewTicker(time.Millisecond * 5)
		defer t.Stop()
		for range t.C {
			return fmt.Errorf("got some error")
		}
		return nil
	}
	resultExpected := 5
	f(backoffFactor, backoffRetries, cancelTimeout, retryFunc, resultExpected)

	// cancel context
	backoffFactor = 1.7
	backoffRetries = 5
	cancelTimeout = time.Millisecond * 40
	retryFunc = func() error {
		return fmt.Errorf("got some error")
	}
	resultExpected = 3
	f(backoffFactor, backoffRetries, cancelTimeout, retryFunc, resultExpected)
}

func TestBackoffRetry_Success(t *testing.T) {
	f := func(retryFunc func() error, resultExpected int) {
		t.Helper()

		r := &Backoff{
			retries:     5,
			factor:      1.7,
			minDuration: time.Millisecond * 10,
		}
		ctx := context.Background()

		result, err := r.Retry(ctx, retryFunc)
		if err != nil {
			t.Fatalf("Retry() error: %s", err)
		}
		if result != uint64(resultExpected) {
			t.Fatalf("unexpected result: got %d; want %d", result, resultExpected)
		}
	}

	// only one retry test
	counter := 0
	retryFunc := func() error {
		t := time.NewTicker(time.Millisecond * 5)
		defer t.Stop()
		for range t.C {
			counter++
			if counter%2 == 0 {
				return fmt.Errorf("got some error")
			}
			if counter%3 == 0 {
				return nil
			}
		}
		return nil
	}
	resultExpected := 1
	f(retryFunc, resultExpected)
}
