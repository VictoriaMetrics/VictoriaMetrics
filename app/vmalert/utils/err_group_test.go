package utils

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrGroup(t *testing.T) {
	f := func(errs []error, resultExpected string) {
		t.Helper()

		eg := &ErrGroup{}
		for _, err := range errs {
			eg.Add(err)
		}
		if len(errs) == 0 {
			if eg.Err() != nil {
				t.Fatalf("expected to get nil error")
			}
			return
		}
		if eg.Err() == nil {
			t.Fatalf("expected to get non-nil error")
		}
		result := eg.Error()
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	f(nil, "")
	f([]error{errors.New("timeout")}, "errors(1): timeout")
	f([]error{errors.New("timeout"), errors.New("deadline")}, "errors(2): timeout\ndeadline")
}

// TestErrGroupConcurrent supposed to test concurrent
// use of error group.
// Should be executed with -race flag
func TestErrGroupConcurrent(_ *testing.T) {
	eg := new(ErrGroup)

	const writersN = 4
	payload := make(chan error, writersN)
	for i := 0; i < writersN; i++ {
		go func() {
			for err := range payload {
				eg.Add(err)
			}
		}()
	}

	const iterations = 500
	for i := 0; i < iterations; i++ {
		payload <- fmt.Errorf("error %d", i)
		if i%10 == 0 {
			_ = eg.Err()
		}
	}
	close(payload)
}
