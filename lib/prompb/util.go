package prompb

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	for i := range wr.Timeseries {
		wr.Timeseries[i] = TimeSeries{}
	}
	wr.Timeseries = wr.Timeseries[:0]

	for i := range wr.labelsPool {
		wr.labelsPool[i] = Label{}
	}
	wr.labelsPool = wr.labelsPool[:0]

	for i := range wr.samplesPool {
		wr.samplesPool[i] = Sample{}
	}
	wr.samplesPool = wr.samplesPool[:0]
}
