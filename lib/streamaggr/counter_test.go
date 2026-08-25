package streamaggr

import "testing"

func TestCounterDelta(t *testing.T) {
	testCases := []struct {
		name          string
		prevValue     float64
		value         float64
		deltaExpected float64
		resetExpected bool
	}{
		{
			name:          "increase",
			prevValue:     100,
			value:         120,
			deltaExpected: 20,
		},
		{
			name:          "unchanged",
			prevValue:     100,
			value:         100,
			deltaExpected: 0,
		},
		{
			name:          "partial reset",
			prevValue:     100,
			value:         95,
			deltaExpected: 0,
			resetExpected: true,
		},
		{
			name:          "full reset at threshold",
			prevValue:     100,
			value:         87.5,
			deltaExpected: 87.5,
			resetExpected: true,
		},
		{
			name:          "full reset",
			prevValue:     100,
			value:         10,
			deltaExpected: 10,
			resetExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			delta, reset := counterDelta(tc.prevValue, tc.value)
			if delta != tc.deltaExpected {
				t.Fatalf("unexpected delta; got %v; want %v", delta, tc.deltaExpected)
			}
			if reset != tc.resetExpected {
				t.Fatalf("unexpected reset flag; got %v; want %v", reset, tc.resetExpected)
			}
		})
	}
}
