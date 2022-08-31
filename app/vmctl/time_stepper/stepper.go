package time_stepper

import "time"

type Stepper interface {
	Next(t time.Time) (time.Time, time.Time)
}
