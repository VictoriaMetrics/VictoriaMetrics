package prompbmarshal

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// MarshalProtobuf marshals wr to dst and returns the result.
func (wr *WriteRequest) MarshalProtobuf(dst []byte) []byte {
	size := wr.size()
	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+size)
	n, err := wr.marshalToSizedBuffer(dst[dstLen:])
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error when marshaling WriteRequest: %w", err))
	}
	return dst[:dstLen+n]
}

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	wr.Timeseries = ResetTimeSeries(wr.Timeseries)
	wr.Metadata = ResetMetadata(wr.Metadata)
}

// ResetTimeSeries clears all the GC references from tss and returns an empty tss ready for further use.
func ResetTimeSeries(tss []TimeSeries) []TimeSeries {
	clear(tss)
	return tss[:0]
}

// ResetMetadata clears all the GC references from mms and returns an empty mms ready for further use.
func ResetMetadata(mms []MetricMetadata) []MetricMetadata {
	clear(mms)
	return mms[:0]
}
