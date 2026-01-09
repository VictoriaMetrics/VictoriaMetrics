package ce

import (
	"context"
	"flag"
	"log"
	"sync"
	"time"
)

var (
	estimatorResetInterval = flag.Duration("estimator.resetInterval", 24*time.Hour, "Interval for resetting the cardinality estimator. Intervals are offset from 0:00 UTC. Set to '0s' to disable.")
)

type ResetOperator struct {
	ce *CardinalityEstimator

	schedule *ResetSchedule
	lock     sync.Mutex
}

func NewResetOperator(ctx context.Context, ce *CardinalityEstimator) *ResetOperator {
	ret := &ResetOperator{
		ce: ce,
	}

	// Automatically clear the schedule every minute to avoid stale schedules
	// It is expected that schedules should be refreshed very frequently (many times per minute)
	go func() {
		for range time.Tick(time.Minute) {
			select {
			case <-ctx.Done():
				return
			default:
				ret.UpdateSchedule(nil)
			}
		}
	}()

	// Worker to reset the estimator as per schedule
	go func() {
		var curr time.Time
		var canReset bool

		for range time.Tick(1 * time.Second) {
			select {
			case <-ctx.Done():
				return
			default:
				func() {
					ret.lock.Lock()
					defer ret.lock.Unlock()

					if ret.schedule == nil {
						return
					}

					// update current interval
					if latest := ret.schedule.ResetIntervalStart(); latest != curr {
						curr = latest
						canReset = true
					}

					// try reset
					if canReset && time.Now().After(ret.schedule.ResetTime()) {
						canReset = false

						log.Printf("Resetting cardinality estimator as per schedule: %+v", *ret.schedule)
						ret.ce.Reset()
					}
				}()
			}
		}
	}()

	// Start job to automatically reset estimators
	go func() {
		if *estimatorResetInterval == 0 {
			log.Printf("Reset interval is set to 0 (disabled).")
			return
		}

		// Reset the cardinality estimator at fixed intervals offset from 0:00 UTC
		now := time.Now().UTC()
		next := now.Truncate(*estimatorResetInterval).Add(*estimatorResetInterval)

		time.Sleep(next.Sub(now))

		for {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("Resetting cardinality estimator at %s", time.Now().UTC().Format(time.RFC3339))
				ret.ce.Reset()

				time.Sleep(*estimatorResetInterval)
			}
		}
	}()

	return ret
}

// Can be called concurrently.
func (r *ResetOperator) UpdateSchedule(resetSchedule *ResetSchedule) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.schedule = resetSchedule
}
