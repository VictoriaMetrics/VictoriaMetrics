package backoff

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRetry_Do(t *testing.T) {
	counter := 0
	tests := []struct {
		name               string
		backoffRetries     int
		backoffFactor      float64
		backoffMinDuration time.Duration
		retryableFunc      retryableFunc
		ctx                context.Context
		withCancel         bool
		want               uint64
		wantErr            bool
	}{
		{
			name: "return bad request",
			retryableFunc: func() error {
				return ErrBadRequest
			},
			ctx:     context.Background(),
			want:    0,
			wantErr: true,
		},
		{
			name: "empty retries values",
			retryableFunc: func() error {
				time.Sleep(time.Millisecond * 100)
				return nil
			},
			ctx:     context.Background(),
			want:    0,
			wantErr: false,
		},
		{
			name:               "only one retry test",
			backoffRetries:     5,
			backoffFactor:      1.7,
			backoffMinDuration: time.Millisecond * 10,
			retryableFunc: func() error {
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
			},
			ctx:     context.Background(),
			want:    1,
			wantErr: false,
		},
		{
			name:               "all retries failed test",
			backoffRetries:     5,
			backoffFactor:      0.1,
			backoffMinDuration: time.Millisecond * 10,
			retryableFunc: func() error {
				t := time.NewTicker(time.Millisecond * 5)
				defer t.Stop()
				for range t.C {
					return fmt.Errorf("got some error")
				}
				return nil
			},
			ctx:     context.Background(),
			want:    5,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			got, err := r.Retry(tt.ctx, tt.retryableFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Retry() got = %v, want %v", got, tt.want)
			}
		})
	}
}
