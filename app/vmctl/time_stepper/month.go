package time_stepper

import "time"

type MonthStepper struct {
	generatedFirst bool
}

func (ms *MonthStepper) Next(t time.Time) (time.Time, time.Time) {
	if !ms.generatedFirst {
		ms.generatedFirst = true
		endOfCurrentMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)

		return t, endOfCurrentMonth
	}

	endOfNextMonth := time.Date(t.Year(), t.Month()+2, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)
	startOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())

	return startOfNextMonth, endOfNextMonth
}
