package prometheus

import (
	"testing"
)

func TestInRange(t *testing.T) {
	testCases := []struct {
		filterMin, filterMax int64
		blockMin, blockMax   int64
		expected             bool
	}{
		{0, 0, 1, 2, true},
		{0, 3, 1, 2, true},
		{0, 3, 4, 5, false},
		{3, 0, 1, 2, false},
		{3, 0, 2, 4, true},
		{3, 10, 1, 2, false},
		{3, 10, 1, 4, true},
		{3, 10, 5, 9, true},
		{3, 10, 9, 12, true},
		{3, 10, 12, 15, false},
	}
	for _, tc := range testCases {
		f := filter{
			min: tc.filterMin,
			max: tc.filterMax,
		}
		got := f.inRange(tc.blockMin, tc.blockMax)
		if got != tc.expected {
			t.Fatalf("got %v; expected %v: %v", got, tc.expected, tc)
		}
	}
}
