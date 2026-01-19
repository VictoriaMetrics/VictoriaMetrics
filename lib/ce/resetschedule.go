package ce

import (
	"log"
	"time"
)

type ResetSchedule struct {
	ResetInterval time.Duration `json:"resetInterval"`
	Quantum       time.Duration `json:"quantum"`
	Index         int           `json:"index"`
}

func NewResetSchedule(ceLookbackWindow time.Duration, ceNodeCount int, ceNodeIndex int) ResetSchedule {
	if ceNodeCount <= 0 {
		log.Panicf("node count cannot be leq 0")
	}
	if ceNodeIndex >= ceNodeCount {
		log.Panicf("node index cannot be geq to node count")
	}

	resetInterval := ceLookbackWindow
	quantum := resetInterval / time.Duration(ceNodeCount)

	return ResetSchedule{
		ResetInterval: resetInterval,
		Quantum:       quantum,
		Index:         ceNodeIndex,
	}
}

// ResetTime returns the reset time within the current reset interval
func (r *ResetSchedule) ResetTime() time.Time {
	return r.ResetIntervalStart().Add(r.Quantum * time.Duration(r.Index))
}

func (r *ResetSchedule) ResetIntervalStart() time.Time {
	return time.Now().Truncate(r.ResetInterval)
}
