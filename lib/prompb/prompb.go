package prompb

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/VictoriaMetrics/easyproto"
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

// WriteRequestUnmarshaler is reusable unmarshaler for WriteRequest protobuf messages.
// It maintains internal pools for labels and samples to reduce memory allocations.
// See UnmarshalProtobuf for details on how to use it.
type WriteRequestUnmarshaler struct {
	wr WriteRequest

	labelsPool  []Label
	samplesPool []Sample
}

func (wru *WriteRequestUnmarshaler) Reset() {
	wru.wr.Reset()

	clear(wru.labelsPool)
	wru.labelsPool = wru.labelsPool[:0]

	clear(wru.samplesPool)
	wru.samplesPool = wru.samplesPool[:0]
}

// UnmarshalProtobuf parses the given Protobuf-encoded `src` into an internal WriteRequest instance
// and returns a pointer to it. This method avoids allocations by reusing preallocated slices and pools.
//
// Notes:
//   - The `src` slice must remain unchanged for the lifetime of the returned WriteRequest,
//     as the WriteRequest retain references to it.
//   - The returned WriteRequest is only valid until the next call to UnmarshalProtobuf,
//     which reuses internal buffers and structs.
func (wru *WriteRequestUnmarshaler) UnmarshalProtobuf(src []byte) (*WriteRequest, error) {
	wru.Reset()

	var err error

	// message WriteRequest {
	//    repeated TimeSeries timeseries = 1;
	//    reserved 2;
	//    repeated Metadata metadata = 3;
	// }
	tss := wru.wr.Timeseries
	mds := wru.wr.Metadata
	labelsPool := wru.labelsPool
	samplesPool := wru.samplesPool
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read timeseries data")
			}
			if len(tss) < cap(tss) {
				tss = tss[:len(tss)+1]
			} else {
				tss = append(tss, TimeSeries{})
			}
			ts := &tss[len(tss)-1]
			labelsPool, samplesPool, err = ts.unmarshalProtobuf(data, labelsPool, samplesPool)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read metricMetadata data")
			}
			if len(mds) < cap(mds) {
				mds = mds[:len(mds)+1]
			} else {
				mds = append(mds, MetricMetadata{})
			}
			md := &mds[len(mds)-1]
			if err := md.unmarshalProtobuf(data); err != nil {
				return nil, fmt.Errorf("cannot unmarshal metricMetadata: %w", err)
			}

		}
	}
	wru.wr.Timeseries = tss
	wru.wr.Metadata = mds
	wru.labelsPool = labelsPool
	wru.samplesPool = samplesPool
	return &wru.wr, nil
}

func (ts *TimeSeries) unmarshalProtobuf(src []byte, labelsPool []Label, samplesPool []Sample) ([]Label, []Sample, error) {
	// message TimeSeries {
	//   repeated Label labels   = 1;
	//   repeated Sample samples = 2;
	// }
	labelsPoolLen := len(labelsPool)
	samplesPoolLen := len(samplesPool)
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return labelsPool, samplesPool, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, samplesPool, fmt.Errorf("cannot read label data")
			}
			if len(labelsPool) < cap(labelsPool) {
				labelsPool = labelsPool[:len(labelsPool)+1]
			} else {
				labelsPool = append(labelsPool, Label{})
			}
			label := &labelsPool[len(labelsPool)-1]
			if err := label.unmarshalProtobuf(data); err != nil {
				return labelsPool, samplesPool, fmt.Errorf("cannot unmarshal label: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, samplesPool, fmt.Errorf("cannot read the sample data")
			}
			if len(samplesPool) < cap(samplesPool) {
				samplesPool = samplesPool[:len(samplesPool)+1]
			} else {
				samplesPool = append(samplesPool, Sample{})
			}
			sample := &samplesPool[len(samplesPool)-1]
			if err := sample.unmarshalProtobuf(data); err != nil {
				return labelsPool, samplesPool, fmt.Errorf("cannot unmarshal sample: %w", err)
			}
		}
	}
	ts.Labels = labelsPool[labelsPoolLen:]
	ts.Samples = samplesPool[samplesPoolLen:]
	return labelsPool, samplesPool, nil
}

func (lbl *Label) unmarshalProtobuf(src []byte) (err error) {
	// message Label {
	//   string name  = 1;
	//   string value = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read label name")
			}
			lbl.Name = name
		case 2:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read label value")
			}
			lbl.Value = value
		}
	}
	return nil
}

func (s *Sample) unmarshalProtobuf(src []byte) (err error) {
	// message Sample {
	//   double value    = 1;
	//   int64 timestamp = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			value, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sample value")
			}
			s.Value = value
		case 2:
			timestamp, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read sample timestamp")
			}
			s.Timestamp = timestamp
		}
	}
	return nil
}

// MetricMetadataType represents the Prometheus type of a metric.
// https://github.com/prometheus/prometheus/blob/c5282933765ec322a0664d0a0268f8276e83b156/prompb/types.pb.go#L28C1-L39C2
// https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#metric-types
type MetricMetadataType int32

const (
	// MetricMetadataUNKNOWN represents a Prometheus Unknown-typed metric
	MetricMetadataUNKNOWN MetricMetadataType = 0
	// MetricMetadataCOUNTER represents a Prometheus Counter
	MetricMetadataCOUNTER MetricMetadataType = 1
	// MetricMetadataGAUGE represents a Prometheus Gauge
	MetricMetadataGAUGE MetricMetadataType = 2
	// MetricMetadataHISTOGRAM represents a Prometheus Histogram
	MetricMetadataHISTOGRAM MetricMetadataType = 3
	// MetricMetadataGAUGEHISTOGRAM represents a Prometheus GaugeHistogram
	MetricMetadataGAUGEHISTOGRAM MetricMetadataType = 4
	// MetricMetadataSUMMARY represents a Prometheus Summary
	MetricMetadataSUMMARY MetricMetadataType = 5
	// MetricMetadataINFO represents a Prometheus Info metric
	MetricMetadataINFO MetricMetadataType = 6
	// MetricMetadataSTATESET represents a Prometheus StateSet metric
	MetricMetadataSTATESET MetricMetadataType = 7
)

// MetricMetadata represents additional meta information for specific MetricFamilyName
// Refer to https://github.com/prometheus/prometheus/blob/c5282933765ec322a0664d0a0268f8276e83b156/prompb/types.proto#L21
type MetricMetadata struct {
	// Represents the metric type, these match the set from Prometheus.
	// Refer to https://github.com/prometheus/common/blob/95acce133ca2c07a966a71d475fb936fc282db18/model/metadata.go for details.
	Type             uint32
	MetricFamilyName string
	Help             string
	Unit             string

	// Additional fields to allow storing and querying metadata in multitenancy.
	AccountID uint32
	ProjectID uint32
}

func (mm *MetricMetadata) unmarshalProtobuf(src []byte) (err error) {
	// message MetricMetadata {
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
	//
	//   MetricType type = 1;
	//   string metric_family_name = 2;
	//   string help = 4;
	//   string unit = 5;
	//
	//   uint32 AccountID = 11;
	//   uint32 ProjectID = 12;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			value, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read metric type")
			}
			mm.Type = value
		case 2:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read metric family name")
			}
			mm.MetricFamilyName = value
		case 4:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read help")
			}
			mm.Help = value
		case 5:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read unit")
			}
			mm.Unit = value
		case 11:
			value, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read AccountID")
			}
			mm.AccountID = value
		case 12:
			value, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read ProjectID")
			}
			mm.ProjectID = value
		}
	}
	return nil
}

// IsEmpty checks if the WriteRequest has data to push.
func (m *WriteRequest) IsEmpty() bool {
	return m == nil || (len(m.Timeseries) == 0 && len(m.Metadata) == 0)
}
