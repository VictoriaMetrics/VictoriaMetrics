package prompbmarshal

import (
	"math/bits"
)

// WriteRequest represents Remote Write request
type WriteRequest struct {
	Timeseries []TimeSeries
	Metadata   []MetricMetadata
}

func (m *WriteRequest) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	for j := len(m.Metadata) - 1; j >= 0; j-- {
		size, err := m.Metadata[j].marshalToSizedBuffer(dst[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarint(dst, i, uint64(size))
		i--
		dst[i] = 0x1a
	}
	for j := len(m.Timeseries) - 1; j >= 0; j-- {
		size, err := m.Timeseries[j].marshalToSizedBuffer(dst[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarint(dst, i, uint64(size))
		i--
		dst[i] = 0xa
	}
	return len(dst) - i, nil
}

func encodeVarint(dst []byte, offset int, v uint64) int {
	offset -= sov(v)
	base := offset
	for v >= 1<<7 {
		dst[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dst[offset] = uint8(v)
	return base
}
func (m *WriteRequest) size() (n int) {
	if m == nil {
		return 0
	}
	for _, e := range m.Timeseries {
		l := e.size()
		n += 1 + l + sov(uint64(l))
	}
	for _, e := range m.Metadata {
		l := e.size()
		n += 1 + l + sov(uint64(l))
	}
	return n
}

func sov(x uint64) (n int) {
	return (bits.Len64(x|1) + 6) / 7
}
