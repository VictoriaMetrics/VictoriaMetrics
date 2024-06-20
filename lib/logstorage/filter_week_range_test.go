package logstorage

import (
	"testing"
	"time"
)

func TestFilterWeekRange(t *testing.T) {
	t.Parallel()

	sunday := time.Date(2024, 6, 9, 1, 0, 0, 0, time.UTC).UnixNano()
	timestamps := []int64{
		sunday,
		sunday + 1*nsecsPerDay,
		sunday + 2*nsecsPerDay,
		sunday + 4*nsecsPerDay,
		sunday + 6*nsecsPerDay,
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
		offset:   3 * nsecsPerDay,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{3})

	ft = &filterWeekRange{
		startDay: time.Monday,
		endDay:   time.Monday,
		offset:   -2 * nsecsPerDay,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{4})

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
		startDay: time.Friday,
		endDay:   time.Friday,
		offset:   -1 * nsecsPerHour,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)
}
