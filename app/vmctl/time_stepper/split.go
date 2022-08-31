package time_stepper

import (
	"fmt"
	"time"
)

type TimeRange struct {
	start time.Time
	end   time.Time
}

func (t TimeRange) Start() string {
	return t.start.Format(time.RFC3339)
}

func (t TimeRange) End() string {
	return t.end.Format(time.RFC3339)
}

const (
	GranularityMonth string = "month"
	GranularityDay   string = "day"
	GranularityHour  string = "hour"
)

// SplitDateRange splits range of dates in subset of ranges.
// Ranges with granularity of GranularityMonth are aligned to 1st of each month in order to improve export efficiency at block transfer level
func SplitDateRange(start, end time.Time, granularity string) ([]TimeRange, error) {

	if start.After(end) {
		return nil, fmt.Errorf("start time should be after end: start - %s, end - %s", start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	var st Stepper

	switch granularity {
	case GranularityMonth:
		st = &MonthStepper{}
	case GranularityDay:
		st = &DayStepper{}
	case GranularityHour:
		st = &HourStepper{}
	default:
		return nil, fmt.Errorf("failed to parse '--vm-native-filter-chunk', valid values are: '%s', '%s', '%s'. provided: '%s'", GranularityMonth, GranularityDay, GranularityHour, granularity)
	}

	currentStep := start

	ranges := make([]TimeRange, 0)

	for end.After(currentStep) {
		startOfStep, endOfStep := st.Next(currentStep)
		if endOfStep.After(end) {
			endOfStep = end
		}
		ranges = append(ranges, TimeRange{
			start: startOfStep,
			end:   endOfStep,
		})
		currentStep = endOfStep
	}

	return ranges, nil
}
