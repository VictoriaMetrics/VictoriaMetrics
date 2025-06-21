package prompb

import (
	"fmt"

	"github.com/VictoriaMetrics/easyproto"
)

// WriteRequest represents Prometheus remote write API request.
type WriteRequest struct {
	// Timeseries is a list of time series in the given WriteRequest
	Timeseries []TimeSeries

	// Metadata is a list of metadata info in the given WriteRequest
	Metadata []MetricMetadata

	labelsPool  []Label
	samplesPool []Sample
}

// Reset resets wr for subsequent reuse.
func (wr *WriteRequest) Reset() {
	clear(wr.Timeseries)
	wr.Timeseries = wr.Timeseries[:0]

	clear(wr.Metadata)
	wr.Metadata = wr.Metadata[:0]

	clear(wr.labelsPool)
	wr.labelsPool = wr.labelsPool[:0]

	clear(wr.samplesPool)
	wr.samplesPool = wr.samplesPool[:0]
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

// UnmarshalProtobuf unmarshals wr from src.
//
// src mustn't change while wr is in use, since wr points to src.
func (wr *WriteRequest) UnmarshalProtobuf(src []byte) (err error) {
	wr.Reset()

	// message WriteRequest {
	//    repeated TimeSeries timeseries = 1;
	//    reserved 2;
	//    repeated Metadata metadata = 3;
	// }
	tss := wr.Timeseries
	mds := wr.Metadata
	labelsPool := wr.labelsPool
	samplesPool := wr.samplesPool
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read timeseries data")
			}
			if len(tss) < cap(tss) {
				tss = tss[:len(tss)+1]
			} else {
				tss = append(tss, TimeSeries{})
			}
			ts := &tss[len(tss)-1]
			labelsPool, samplesPool, err = ts.unmarshalProtobuf(data, labelsPool, samplesPool)
			if err != nil {
				return fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read metricMetadata data")
			}
			if len(mds) < cap(mds) {
				mds = mds[:len(mds)+1]
			} else {
				mds = append(mds, MetricMetadata{})
			}
			md := &mds[len(mds)-1]
			if err := md.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal metricMetadata: %w", err)
			}

		}
	}
	wr.Timeseries = tss
	wr.Metadata = mds
	wr.labelsPool = labelsPool
	wr.samplesPool = samplesPool
	return nil
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
		}
	}
	return nil
}
