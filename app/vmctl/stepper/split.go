package stepper

import (
	"fmt"
	"time"
)

const (
	// StepMonth is representing month value for flag
	StepMonth string = "month"
	// StepDay is representing day value for flag
	StepDay string = "day"
	// StepHour is representing hour value for flag
	StepHour string = "hour"
)

// SplitDateRange splits range of dates in subset of ranges.
// Ranges with granularity of StepMonth are aligned to 1st of each month in order to improve export efficiency at block transfer level
func SplitDateRange(start, end time.Time, granularity string) ([][]time.Time, error) {

	if start.After(end) {
		return nil, fmt.Errorf("start time should be after end: start - %s, end - %s", start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	var step func(time.Time) (time.Time, time.Time)

	switch granularity {
	case StepMonth:
		step = func() func(time.Time) (time.Time, time.Time) {
			generatedFirst := false
			return func(t time.Time) (time.Time, time.Time) {
				if !generatedFirst {
					generatedFirst = true
					endOfCurrentMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)

					return t, endOfCurrentMonth
				}

				endOfNextMonth := time.Date(t.Year(), t.Month()+2, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)
				startOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())

				return startOfNextMonth, endOfNextMonth
			}
		}()
	case StepDay:
		step = func(t time.Time) (time.Time, time.Time) {
			return t, t.AddDate(0, 0, 1)
		}
	case StepHour:
		step = func(t time.Time) (time.Time, time.Time) {
			return t, t.Add(time.Hour * 1)
		}
	default:
		return nil, fmt.Errorf("failed to parse '--vm-native-filter-chunk', valid values are: '%s', '%s', '%s'. provided: '%s'", StepMonth, StepDay, StepHour, granularity)
	}

	currentStep := start

	ranges := make([][]time.Time, 0)

	for end.After(currentStep) {
		s, e := step(currentStep)
		if e.After(end) {
			e = end
		}
		ranges = append(ranges, []time.Time{
			s,
			e,
		})
		currentStep = e
	}

	return ranges, nil
}
