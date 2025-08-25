package prompb

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"

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

func (m *Sample) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	if m.Timestamp != 0 {
		i = encodeVarint(dst, i, uint64(m.Timestamp))
		i--
		dst[i] = (2 << 3)
	}
	if m.Value != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dst[i:], uint64(math.Float64bits(float64(m.Value))))
		i--
		dst[i] = (1 << 3) | 1
	}
	return len(dst) - i, nil
}

func (m *TimeSeries) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	for j := len(m.Samples) - 1; j >= 0; j-- {
		size, err := m.Samples[j].marshalToSizedBuffer(dst[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarint(dst, i, uint64(size))
		i--
		dst[i] = (2 << 3) | 2
	}
	for j := len(m.Labels) - 1; j >= 0; j-- {
		size, err := m.Labels[j].marshalToSizedBuffer(dst[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarint(dst, i, uint64(size))
		i--
		dst[i] = (1 << 3) | 2
	}
	return len(dst) - i, nil
}

func (m *Label) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	if len(m.Value) > 0 {
		i -= len(m.Value)
		copy(dst[i:], m.Value)
		i = encodeVarint(dst, i, uint64(len(m.Value)))
		i--
		dst[i] = (2 << 3) | 2
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dst[i:], m.Name)
		i = encodeVarint(dst, i, uint64(len(m.Name)))
		i--
		dst[i] = (1 << 3) | 2
	}
	return len(dst) - i, nil
}

func (m *Sample) size() (n int) {
	if m == nil {
		return 0
	}
	if m.Value != 0 {
		n += 9
	}
	if m.Timestamp != 0 {
		n += 1 + sov(uint64(m.Timestamp))
	}
	return n
}

func (m *TimeSeries) size() (n int) {
	if m == nil {
		return 0
	}
	for _, e := range m.Labels {
		l := e.size()
		n += 1 + l + sov(uint64(l))
	}
	for _, e := range m.Samples {
		l := e.size()
		n += 1 + l + sov(uint64(l))
	}
	return n
}

func (m *Label) size() (n int) {
	if m == nil {
		return 0
	}
	if l := len(m.Name); l > 0 {
		n += 1 + l + sov(uint64(l))
	}
	if l := len(m.Value); l > 0 {
		n += 1 + l + sov(uint64(l))
	}
	return n
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
		dst[i] = (3 << 3) | 2
	}
	for j := len(m.Timeseries) - 1; j >= 0; j-- {
		size, err := m.Timeseries[j].marshalToSizedBuffer(dst[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarint(dst, i, uint64(size))
		i--
		dst[i] = (1 << 3) | 2
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

func (m *MetricMetadata) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	if len(m.Unit) > 0 {
		i -= len(m.Unit)
		copy(dst[i:], m.Unit)
		i = encodeVarint(dst, i, uint64(len(m.Unit)))
		i--
		dst[i] = (5 << 3) | 2
	}
	if len(m.Help) > 0 {
		i -= len(m.Help)
		copy(dst[i:], m.Help)
		i = encodeVarint(dst, i, uint64(len(m.Help)))
		i--
		dst[i] = (4 << 3) | 2
	}
	if len(m.MetricFamilyName) > 0 {
		i -= len(m.MetricFamilyName)
		copy(dst[i:], m.MetricFamilyName)
		i = encodeVarint(dst, i, uint64(len(m.MetricFamilyName)))
		i--
		dst[i] = (2 << 3) | 2
	}
	if m.Type != 0 {
		i = encodeVarint(dst, i, uint64(m.Type))
		i--
		dst[i] = (1 << 3)
	}
	if m.AccountID != 0 {
		i = encodeVarint(dst, i, uint64(m.AccountID))
		i--
		dst[i] = (11 << 3)
	}
	if m.ProjectID != 0 {
		i = encodeVarint(dst, i, uint64(m.ProjectID))
		i--
		dst[i] = (12 << 3)
	}
	return len(dst) - i, nil
}

func (m *MetricMetadata) size() (n int) {
	if m == nil {
		return 0
	}
	if m.Type != 0 {
		n += 1 + sov(uint64(m.Type))
	}
	if l := len(m.MetricFamilyName); l > 0 {
		n += 1 + l + sov(uint64(l))
	}
	if l := len(m.Help); l > 0 {
		n += 1 + l + sov(uint64(l))
	}
	if l := len(m.Unit); l > 0 {
		n += 1 + l + sov(uint64(l))
	}
	if m.AccountID != 0 {
		n += 1 + sov(uint64(m.AccountID))
	}
	if m.ProjectID != 0 {
		n += 1 + sov(uint64(m.ProjectID))
	}
	return n
}
