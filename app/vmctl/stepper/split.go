package stepper

import (
	"fmt"
	"sort"
	"time"
)

const (
	// StepMonth represents a one month interval
	StepMonth string = "month"
	// StepWeek represents a one week interval
	StepWeek string = "week"
	// StepDay represents a one day interval
	StepDay string = "day"
	// StepHour represents a one hour interval
	StepHour string = "hour"
	// StepMinute represents a one minute interval
	StepMinute string = "minute"
)

// SplitDateRange splits start-end range in a subset of ranges respecting the given step
// Ranges with granularity of StepMonth are aligned to 1st of each month in order to improve export efficiency at block transfer level
func SplitDateRange(start, end time.Time, step string, timeReverse bool) ([][]time.Time, error) {

	if start.After(end) {
		return nil, fmt.Errorf("start time %q should come before end time %q", start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	var nextStep func(time.Time) (time.Time, time.Time)

	switch step {
	case StepMonth:
		nextStep = func(t time.Time) (time.Time, time.Time) {
			endOfMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)
			if t == endOfMonth {
				endOfMonth = time.Date(t.Year(), t.Month()+2, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)
				t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			}
			return t, endOfMonth
		}
	case StepDay:
		nextStep = func(t time.Time) (time.Time, time.Time) {
			return t, t.AddDate(0, 0, 1)
		}
	case StepWeek:
		nextStep = func(t time.Time) (time.Time, time.Time) {
			return t, t.Add(7 * 24 * time.Hour)
		}
	case StepHour:
		nextStep = func(t time.Time) (time.Time, time.Time) {
			return t, t.Add(time.Hour * 1)
		}
	case StepMinute:
		nextStep = func(t time.Time) (time.Time, time.Time) {
			return t, t.Add(time.Minute * 1)
		}
	default:
		return nil, fmt.Errorf("failed to parse step value, valid values are: '%s', '%s', '%s', '%s'. provided: '%s'",
			StepMonth, StepDay, StepHour, StepMinute, step)
	}

	currentStep := start

	ranges := make([][]time.Time, 0)

	for end.After(currentStep) {
		s, e := nextStep(currentStep)
		if e.After(end) {
			e = end
		}
		ranges = append(ranges, []time.Time{s, e})
		currentStep = e
	}
	if timeReverse {
		sort.SliceStable(ranges, func(i, j int) bool {
			return ranges[i][0].After(ranges[j][0])
		})
	}

	return ranges, nil
}
