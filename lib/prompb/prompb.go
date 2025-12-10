package prompb

import (
	"fmt"
	"sort"
	"strconv"
)

// WriteRequest represents Prometheus remote write API request.
type WriteRequest struct {
	// Timeseries is a list of time series in the given WriteRequest
	Timeseries []TimeSeries

	// Metadata is a list of metadata info in the given WriteRequest
	Metadata []MetricMetadata
}

// Reset resets wr for subsequent reuse.
func (wr *WriteRequest) Reset() {
	wr.Timeseries = ResetTimeSeries(wr.Timeseries)
	wr.Metadata = ResetMetadata(wr.Metadata)
}

// TimeSeries is a timeseries.
type TimeSeries struct {
	// Labels is a list of labels for the given TimeSeries
	Labels []Label

	// Samples is a list of samples for the given TimeSeries
	Samples []Sample
}

// Sample is a timeseries sample.
type Sample struct {
	// Value is sample value.
	Value float64

	// Timestamp is unix timestamp for the sample in milliseconds.
	Timestamp int64
}

// Label is a timeseries label.
type Label struct {
	// Name is label name.
	Name string

	// Value is label value.
	Value string
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

// MetricMetadata represents additional meta information for specific MetricFamilyName.
//
// Refer to https://github.com/prometheus/prometheus/blob/c5282933765ec322a0664d0a0268f8276e83b156/prompb/types.proto#L21
type MetricMetadata struct {
	// Represents the metric type, these match the set from Prometheus.
	// Refer to https://github.com/prometheus/common/blob/95acce133ca2c07a966a71d475fb936fc282db18/model/metadata.go for details.
	Type             MetricType
	MetricFamilyName string
	Help             string
	Unit             string

	// Additional fields to allow storing and querying metadata in multitenancy.
	AccountID uint32
	ProjectID uint32
}

// MetricType represents the Prometheus type of a metric.
//
// https://github.com/prometheus/prometheus/blob/c5282933765ec322a0664d0a0268f8276e83b156/prompb/types.pb.go#L28C1-L39C2
// https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#metric-types
type MetricType uint32

const (
	// MetricTypeUnknown represents a Prometheus Unknown-typed metric
	MetricTypeUnknown MetricType = 0
	// MetricTypeCounter represents a Prometheus Counter
	MetricTypeCounter MetricType = 1
	// MetricTypeGauge represents a Prometheus Gauge
	MetricTypeGauge MetricType = 2
	// MetricTypeHistogram represents a Prometheus Histogram
	MetricTypeHistogram MetricType = 3
	// MetricTypeGaugeHistogram represents a Prometheus GaugeHistogram
	MetricTypeGaugeHistogram MetricType = 4
	// MetricTypeSummary represents a Prometheus Summary
	MetricTypeSummary MetricType = 5
	// MetricTypeInfo represents a Prometheus Info metric
	MetricTypeInfo MetricType = 6
	// MetricTypeStateset represents a Prometheus StateSet metric
	MetricTypeStateset MetricType = 7
)

// String returns human-readable string for mt.
func (mt MetricType) String() string {
	//   enum MetricType {
	//     UNKNOWN = 0;
	//     COUNTER = 1;
	//     GAUGE = 2;
	//     HISTOGRAM = 3;
	//     GAUGEHISTOGRAM = 4;
	//     SUMMARY = 5;
	//     INFO = 6;
	//     STATESET = 7;
	//   }
	// source https://github.com/prometheus/prometheus/blob/c5282933765ec322a0664d0a0268f8276e83b156/prompb/types.proto#L22
	switch mt {
	case 0:
		return "unknown"
	case 1:
		return "counter"
	case 2:
		return "gauge"
	case 3:
		return "histogram"
	case 4:
		return "gauge histogram"
	case 5:
		return "summary"
	case 6:
		return "info"
	case 7:
		return "stateset"
	default:
		return fmt.Sprintf("unknown(%d)", mt)
	}
}

// IsEmpty checks if the WriteRequest has data to push.
func (m *WriteRequest) IsEmpty() bool {
	return m == nil || (len(m.Timeseries) == 0 && len(m.Metadata) == 0)
}
