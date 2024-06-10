package utils

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrGroup(t *testing.T) {
	testCases := []struct {
		errs []error
		exp  string
	}{
		{nil, ""},
		{[]error{errors.New("timeout")}, "errors(1): timeout"},
		{
			[]error{errors.New("timeout"), errors.New("deadline")},
			"errors(2): timeout\ndeadline",
		},
	}
	for _, tc := range testCases {
		eg := new(ErrGroup)
		for _, err := range tc.errs {
			eg.Add(err)
		}
		if len(tc.errs) == 0 {
			if eg.Err() != nil {
				t.Fatalf("expected to get nil error")
			}
			continue
		}
		if eg.Err() == nil {
			t.Fatalf("expected to get non-nil error")
		}
		if eg.Error() != tc.exp {
			t.Fatalf("expected to have: \n%q\ngot:\n%q", tc.exp, eg.Error())
		}
	}
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
