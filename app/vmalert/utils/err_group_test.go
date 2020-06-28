package utils

import (
	"errors"
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
