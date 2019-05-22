package storage

// rawBlock represents a raw block of a single time-series rows.
type rawBlock struct {
	TSID TSID

	Timestamps []int64
	Values     []float64
}

// Reset resets rb.
func (rb *rawBlock) Reset() {
	rb.TSID = TSID{}
	rb.Timestamps = rb.Timestamps[:0]
	rb.Values = rb.Values[:0]
}

// CopyFrom copies src to rb.
func (rb *rawBlock) CopyFrom(src *rawBlock) {
	rb.TSID = src.TSID
	rb.Timestamps = append(rb.Timestamps[:0], src.Timestamps...)
	rb.Values = append(rb.Values[:0], src.Values...)
}
