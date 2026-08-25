package streamaggr

// counterDelta returns the increase between two counter samples and reports
// whether the decrease was treated as a reset. This mirrors the reset
// threshold used by MetricsQL's removeCounterResets.
func counterDelta(prevValue, value float64) (float64, bool) {
	if value >= prevValue {
		return value - prevValue, false
	}
	d := value - prevValue
	if -d*8 < prevValue {
		// A small decrease is likely a partial reset. MetricsQL corrects the
		// series to the previous value, so this sample adds no increase.
		return 0, true
	}
	return value, true
}
