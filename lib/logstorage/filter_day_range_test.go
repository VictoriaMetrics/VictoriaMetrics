package logstorage

import (
	"testing"
)

func TestFilterDayRange(t *testing.T) {
	t.Parallel()

	timestamps := []int64{
		1,
		9,
		123,
		456,
		789,
	}

	// match
	ft := &filterDayRange{
		start: 0,
		end:   1,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterDayRange{
		start: 0,
		end:   10,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0, 1})

	ft = &filterDayRange{
		start: 1,
		end:   1,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterDayRange{
		start:  1,
		end:    1,
		offset: 8,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1})

	ft = &filterDayRange{
		start:  10,
		end:    10,
		offset: -9,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterDayRange{
		start: 2,
		end:   456,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1, 2, 3})

	ft = &filterDayRange{
		start: 2,
		end:   457,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1, 2, 3})

	ft = &filterDayRange{
		start: 120,
		end:   788,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3})

	ft = &filterDayRange{
		start: 120,
		end:   789,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3, 4})

	ft = &filterDayRange{
		start: 120,
		end:   10000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3, 4})

	ft = &filterDayRange{
		start: 789,
		end:   1000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{4})

	// mismatch
	ft = &filterDayRange{
		start:  1,
		end:    1,
		offset: 10,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)

	ft = &filterDayRange{
		start:  0,
		end:    1000,
		offset: 10_000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)

	ft = &filterDayRange{
		start: 790,
		end:   1000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)
}
