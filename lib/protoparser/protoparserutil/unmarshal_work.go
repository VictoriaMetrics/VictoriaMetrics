package protoparserutil

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ScheduleUnmarshalWork schedules uw to run in the worker pool.
//
// It is expected that StartUnmarshalWorkers is already called.
func ScheduleUnmarshalWork(uw UnmarshalWork) {
	unmarshalWorkCh <- uw
}

// UnmarshalWork is a unit of unmarshal work.
type UnmarshalWork interface {
	// Unmarshal must implement CPU-bound unmarshal work.
	Unmarshal()
}

// StartUnmarshalWorkers starts unmarshal workers.
func StartUnmarshalWorkers() {
	if unmarshalWorkCh != nil {
		logger.Panicf("BUG: it looks like startUnmarshalWorkers() has been already called without stopUnmarshalWorkers()")
	}
	gomaxprocs := cgroup.AvailableCPUs()
	unmarshalWorkCh = make(chan UnmarshalWork, gomaxprocs)
	for range gomaxprocs {
		unmarshalWorkersWG.Go(func() {
			for uw := range unmarshalWorkCh {
				uw.Unmarshal()
			}
		})
	}
}

// StopUnmarshalWorkers stops unmarshal workers.
//
// No more calls to ScheduleUnmarshalWork are allowed after calling stopUnmarshalWorkers
func StopUnmarshalWorkers() {
	close(unmarshalWorkCh)
	unmarshalWorkersWG.Wait()
	unmarshalWorkCh = nil
}

var (
	unmarshalWorkCh    chan UnmarshalWork
	unmarshalWorkersWG sync.WaitGroup
)
