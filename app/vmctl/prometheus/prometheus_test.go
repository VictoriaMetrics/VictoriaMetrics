package prometheus

import (
	"testing"
)

func TestInRange(t *testing.T) {
	f := func(filterMin, filterMax, blockMin, blockMax int64, resultExpected bool) {
		t.Helper()

		f := filter{
			min: filterMin,
			max: filterMax,
		}
		result := f.inRange(blockMin, blockMax)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f(0, 0, 1, 2, true)
	f(0, 3, 1, 2, true)
	f(0, 3, 4, 5, false)
	f(3, 0, 1, 2, false)
	f(3, 0, 2, 4, true)
	f(3, 10, 1, 2, false)
	f(3, 10, 1, 4, true)
	f(3, 10, 5, 9, true)
	f(3, 10, 9, 12, true)
	f(3, 10, 12, 15, false)
}
