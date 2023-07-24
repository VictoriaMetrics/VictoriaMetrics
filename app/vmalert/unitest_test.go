package main

import (
	"testing"
)

func TestUnitRule(t *testing.T) {
	testCases := []struct {
		name              string
		disableGroupLabel bool
		files             []string
		failed            bool
	}{
		{
			name:   "run multi files",
			files:  []string{"./unittest/testdata/test1.yaml", "./unittest/testdata/test2.yaml"},
			failed: false,
		},
		{
			name:              "disable group label",
			disableGroupLabel: true,
			files:             []string{"./unittest/testdata/disable-group-label.yaml"},
			failed:            false,
		},
		{
			name:   "failing test",
			files:  []string{"./unittest/testdata/failed-test.yaml"},
			failed: true,
		},
	}
	for _, tc := range testCases {
		oldFlag := *disableAlertGroupLabel
		*disableAlertGroupLabel = tc.disableGroupLabel
		fail := unitRule(tc.files...)
		if fail != tc.failed {
			t.Fatalf("failed to test %s, expect %t, got %t", tc.name, tc.failed, fail)
		}
		*disableAlertGroupLabel = oldFlag
	}
}
