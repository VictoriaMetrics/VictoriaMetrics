package stepper

import "time"

// Stepper generates time ranges in specific manner
type Stepper interface {
	Next(t time.Time) (time.Time, time.Time)
}
