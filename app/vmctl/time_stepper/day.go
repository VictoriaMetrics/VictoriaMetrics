package time_stepper

import "time"

type DayStepper struct{}

func (DayStepper) Next(t time.Time) (time.Time, time.Time) {
	return t, t.AddDate(0, 0, 1)
}
