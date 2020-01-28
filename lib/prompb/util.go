package prompb

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	for i := range wr.Timeseries {
		ts := &wr.Timeseries[i]
		ts.Labels = nil
		ts.Samples = nil
	}
	wr.Timeseries = wr.Timeseries[:0]

	for i := range wr.labelsPool {
		lb := &wr.labelsPool[i]
		lb.Name = nil
		lb.Value = nil
	}
	wr.labelsPool = wr.labelsPool[:0]

	for i := range wr.samplesPool {
		s := &wr.samplesPool[i]
		s.Value = 0
		s.Timestamp = 0
	}
	wr.samplesPool = wr.samplesPool[:0]
}
