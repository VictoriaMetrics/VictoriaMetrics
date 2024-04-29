package logstorage

import (
	"testing"
)

func TestFilterTime(t *testing.T) {
	timestamps := []int64{
		1,
		9,
		123,
		456,
		789,
	}

	// match
	ft := &filterTime{
		minTimestamp: -10,
		maxTimestamp: 1,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterTime{
		minTimestamp: -10,
		maxTimestamp: 10,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0, 1})

	ft = &filterTime{
		minTimestamp: 1,
		maxTimestamp: 1,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterTime{
		minTimestamp: 2,
		maxTimestamp: 456,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1, 2, 3})

	ft = &filterTime{
		minTimestamp: 2,
		maxTimestamp: 457,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1, 2, 3})

	ft = &filterTime{
		minTimestamp: 120,
		maxTimestamp: 788,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3})

	ft = &filterTime{
		minTimestamp: 120,
		maxTimestamp: 789,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3, 4})

	ft = &filterTime{
		minTimestamp: 120,
		maxTimestamp: 10000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3, 4})

	ft = &filterTime{
		minTimestamp: 789,
		maxTimestamp: 1000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{4})

	// mismatch
	ft = &filterTime{
		minTimestamp: -1000,
		maxTimestamp: 0,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)

	ft = &filterTime{
		minTimestamp: 790,
		maxTimestamp: 1000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)
}
