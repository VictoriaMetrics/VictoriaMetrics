package prompbmarshal

import (
	"fmt"
)

// MarshalProtobuf marshals wr to dst and returns the result.
func (wr *WriteRequest) MarshalProtobuf(dst []byte) []byte {
	size := wr.Size()
	dstLen := len(dst)
	if n := size - (cap(dst) - dstLen); n > 0 {
		dst = append(dst[:cap(dst)], make([]byte, n)...)
	}
	dst = dst[:dstLen+size]
	n, err := wr.MarshalToSizedBuffer(dst[dstLen:])
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error when marshaling WriteRequest: %w", err))
	}
	return dst[:dstLen+n]
}

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	wr.Timeseries = ResetTimeSeries(wr.Timeseries)
}

// ResetTimeSeries clears all the GC references from tss and returns an empty tss ready for further use.
func ResetTimeSeries(tss []TimeSeries) []TimeSeries {
	for i := range tss {
		tss[i] = TimeSeries{}
	}
	return tss[:0]
}
