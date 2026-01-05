package ce

import "time"

type ResetSchedule struct {
	ResetInterval time.Duration `json:"resetInterval"`
	Quantum       time.Duration `json:"quantum"`
	Index         int           `json:"index"`
}

func NewResetSchedule(ceLookbackWindow time.Duration, ceNodeCount int, ceNodeIndex int) ResetSchedule {
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
