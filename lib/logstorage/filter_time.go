package logstorage

// filterTime filters by time.
//
// It is expressed as `_time:(start, end]` in LogsQL.
type filterTime struct {
	// mintimestamp is the minimum timestamp in nanoseconds to find
	minTimestamp int64

	// maxTimestamp is the maximum timestamp in nanoseconds to find
	maxTimestamp int64

	// stringRepr is string representation of the filter
	stringRepr string
}

func (ft *filterTime) String() string {
	return "_time:" + ft.stringRepr
}

func (ft *filterTime) apply(bs *blockSearch, bm *bitmap) {
	minTimestamp := ft.minTimestamp
	maxTimestamp := ft.maxTimestamp

	if minTimestamp > maxTimestamp {
		bm.resetBits()
		return
	}

	th := bs.bsw.bh.timestampsHeader
	if minTimestamp > th.maxTimestamp || maxTimestamp < th.minTimestamp {
		bm.resetBits()
		return
	}
	if minTimestamp <= th.minTimestamp && maxTimestamp >= th.maxTimestamp {
		return
	}

	timestamps := bs.getTimestamps()
	bm.forEachSetBit(func(idx int) bool {
		ts := timestamps[idx]
		return ts >= minTimestamp && ts <= maxTimestamp
	})
}
