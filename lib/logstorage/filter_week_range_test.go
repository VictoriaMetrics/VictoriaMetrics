package logstorage

import (
	"testing"
	"time"
)

func TestFilterWeekRange(t *testing.T) {
	timestamps := []int64{
		0,
		1 * nsecsPerDay,
		2 * nsecsPerDay,
		4 * nsecsPerDay,
		6 * nsecsPerDay,
	}

	// match
	ft := &filterWeekRange{
		startDay: time.Sunday,
		endDay:   time.Sunday,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterWeekRange{
		startDay: time.Sunday,
		endDay:   time.Monday,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0, 1})

	ft = &filterWeekRange{
		startDay: time.Monday,
		endDay:   time.Monday,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1})

	ft = &filterWeekRange{
		startDay: time.Monday,
		endDay:   time.Monday,
		offset:   2 * nsecsPerDay,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2})

	ft = &filterWeekRange{
		startDay: time.Monday,
		endDay:   time.Monday,
		offset:   -2 * nsecsPerDay,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1})

	ft = &filterWeekRange{
		startDay: time.Sunday,
		endDay:   time.Saturday,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0, 1, 2, 3, 4})

	// mismatch
	ft = &filterWeekRange{
		startDay: time.Friday,
		endDay:   time.Friday,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)

	ft = &filterWeekRange{
		startDay: time.Thursday,
		endDay:   time.Thursday,
		offset:   2 * nsecsPerHour,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)

	ft = &filterWeekRange{
		startDay: time.Saturday,
		endDay:   time.Saturday,
		offset:   -2 * nsecsPerHour,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)
}
