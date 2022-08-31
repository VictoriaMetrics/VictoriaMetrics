package time_stepper

import "time"

type HourStepper struct{}

func (HourStepper) Next(t time.Time) (time.Time, time.Time) {
	return t, t.Add(time.Hour * 1)
}
