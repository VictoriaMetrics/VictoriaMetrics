package bytesutil

import (
	"fmt"
	"testing"
	"time"
)

func TestInternStringSerial(t *testing.T) {
	if err := testInternString(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestInternStringConcurrent(t *testing.T) {
	concurrency := 5
	resultCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			resultCh <- testInternString()
		}()
	}
	timer := time.NewTimer(5 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-resultCh:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-timer.C:
			t.Fatalf("timeout")
		}
	}
}

func testInternString() error {
	for i := 0; i < 1000; i++ {
		s := fmt.Sprintf("foo_%d", i)
		s1 := InternString(s)
		if s != s1 {
			return fmt.Errorf("unexpected string returned from internString; got %q; want %q", s1, s)
		}
	}
	return nil
}
