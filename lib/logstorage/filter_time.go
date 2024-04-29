package logstorage

// filterTime filters by time.
//
// It is expressed as `_time:(start, end]` in LogsQL.
type filterTime struct {
	minTimestamp int64
	maxTimestamp int64

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
