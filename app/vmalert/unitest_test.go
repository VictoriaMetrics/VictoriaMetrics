package main

import (
	"testing"
)

func TestUnitRule(t *testing.T) {
	testCases := []struct {
		name   string
		files  []string
		failed bool
	}{
		{
			name:   "run multi files",
			files:  []string{"./unittest/testdata/test1.yaml", "./unittest/testdata/test2.yaml"},
			failed: false,
		},
		{
			name:   "failing test",
			files:  []string{"./unittest/testdata/failed-test.yaml"},
			failed: true,
		},
		{
			// This test will take about 1 minute to run now.
			// todo may need to improve the performance
			name:   "long period",
			files:  []string{"./unittest/testdata/long-period.yaml"},
			failed: false,
		},
	}
	for _, tc := range testCases {
		fail := unitRule(tc.files...)
		if fail != tc.failed {
			t.Fatalf("failed to test %s, expect %t, got %t", tc.name, tc.failed, fail)
		}
	}
}
