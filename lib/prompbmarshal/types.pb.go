package prompbmarshal

import (
	"encoding/binary"
	"math"
	"sort"
	"strconv"
)

// Sample is a metric sample
type Sample struct {
	Value     float64
	Timestamp int64
}

// TimeSeries represents samples and labels for a single time series.
type TimeSeries struct {
	Labels  []Label
	Samples []Sample
}

// Label is a key-value label pair
type Label struct {
	Name  string
	Value string
}

func (m *Sample) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	if m.Timestamp != 0 {
		i = encodeVarint(dst, i, uint64(m.Timestamp))
		i--
		dst[i] = 0x10
	}
	if m.Value != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dst[i:], uint64(math.Float64bits(float64(m.Value))))
		i--
		dst[i] = 0x9
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
		dst[i] = 0x12
	}
	for j := len(m.Labels) - 1; j >= 0; j-- {
		size, err := m.Labels[j].marshalToSizedBuffer(dst[:i])
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

func (m *Label) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	if len(m.Value) > 0 {
		i -= len(m.Value)
		copy(dst[i:], m.Value)
		i = encodeVarint(dst, i, uint64(len(m.Value)))
		i--
		dst[i] = 0x12
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dst[i:], m.Name)
		i = encodeVarint(dst, i, uint64(len(m.Name)))
		i--
		dst[i] = 0xa
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

// LabelsToString converts labels to Prometheus-compatible string
func LabelsToString(labels []Label) string {
	labelsCopy := append([]Label{}, labels...)
	sort.Slice(labelsCopy, func(i, j int) bool {
		return string(labelsCopy[i].Name) < string(labelsCopy[j].Name)
	})
	var b []byte
	b = append(b, '{')
	for i, label := range labelsCopy {
		if len(label.Name) == 0 {
			b = append(b, "__name__"...)
		} else {
			b = append(b, label.Name...)
		}
		b = append(b, '=')
		b = strconv.AppendQuote(b, label.Value)
		if i < len(labels)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return string(b)
}

// MetricMetadata represents additional meta information for specific MetricFamilyName
// Refer to https://github.com/prometheus/prometheus/blob/c5282933765ec322a0664d0a0268f8276e83b156/prompb/types.proto#L21
type MetricMetadata struct {
	// Represents the metric type, these match the set from Prometheus.
	// Refer to https://github.com/prometheus/common/blob/95acce133ca2c07a966a71d475fb936fc282db18/model/metadata.go for details.
	Type             uint32
	MetricFamilyName string
	Help             string
	Unit             string
}

func (m *MetricMetadata) marshalToSizedBuffer(dst []byte) (int, error) {
	i := len(dst)
	if len(m.Unit) > 0 {
		i -= len(m.Unit)
		copy(dst[i:], m.Unit)
		i = encodeVarint(dst, i, uint64(len(m.Unit)))
		i--
		dst[i] = 0x2a
	}
	if len(m.Help) > 0 {
		i -= len(m.Help)
		copy(dst[i:], m.Help)
		i = encodeVarint(dst, i, uint64(len(m.Help)))
		i--
		dst[i] = 0x22
	}
	if len(m.MetricFamilyName) > 0 {
		i -= len(m.MetricFamilyName)
		copy(dst[i:], m.MetricFamilyName)
		i = encodeVarint(dst, i, uint64(len(m.MetricFamilyName)))
		i--
		dst[i] = 0x12
	}
	if m.Type != 0 {
		i = encodeVarint(dst, i, uint64(m.Type))
		i--
		dst[i] = 0x8
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
	return n
}
