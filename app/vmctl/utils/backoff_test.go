package utils

import (
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
		cb                 callback
		want               uint64
		wantErr            bool
	}{
		{
			name: "return bad request",
			cb: func() error {
				return ErrBadRequest
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "empty retries values",
			cb: func() error {
				time.Sleep(time.Millisecond * 100)
				return nil
			},
			want:    0,
			wantErr: true,
		},
		{
			name:               "first test",
			backoffRetries:     5,
			backoffFactor:      1.7,
			backoffMinDuration: time.Millisecond * 10,
			cb: func() error {
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
			want:    1,
			wantErr: false,
		},
		{
			name:               "first test",
			backoffRetries:     5,
			backoffFactor:      0.1,
			backoffMinDuration: time.Millisecond * 10,
			cb: func() error {
				t := time.NewTicker(time.Millisecond * 5)
				defer t.Stop()
				for range t.C {
					return fmt.Errorf("got some error")
				}
				return nil
			},
			want:    5,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Retry{
				backoffRetries:     tt.backoffRetries,
				backoffFactor:      tt.backoffFactor,
				backoffMinDuration: tt.backoffMinDuration,
			}
			got, err := r.Do(tt.cb)
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
