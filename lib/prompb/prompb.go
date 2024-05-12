package prompb

import (
	"fmt"

	"github.com/VictoriaMetrics/easyproto"
)

// WriteRequest represents Prometheus remote write API request.
type WriteRequest struct {
	// Timeseries is a list of time series in the given WriteRequest
	Timeseries         []TimeSeries
	labelsPool         []Label
	exemplarLabelsPool []Label
	samplesPool        []Sample
	exemplarsPool      []Exemplar
}

// Reset resets wr for subsequent re-use.
func (wr *WriteRequest) Reset() {
	tss := wr.Timeseries
	for i := range tss {
		tss[i] = TimeSeries{}
	}
	wr.Timeseries = tss[:0]

	labelsPool := wr.labelsPool
	for i := range labelsPool {
		labelsPool[i] = Label{}
	}
	wr.labelsPool = labelsPool[:0]

	exemplarLabelsPool := wr.exemplarLabelsPool
	for i := range exemplarLabelsPool {
		exemplarLabelsPool[i] = Label{}
	}
	wr.labelsPool = labelsPool[:0]
	samplesPool := wr.samplesPool
	for i := range samplesPool {
		samplesPool[i] = Sample{}
	}
	wr.samplesPool = samplesPool[:0]
	exemplarsPool := wr.exemplarsPool
	for i := range exemplarsPool {
		exemplarsPool[i] = Exemplar{}
	}
	wr.exemplarsPool = exemplarsPool[:0]
}

// Exemplar is an exemplar
type Exemplar struct {
	// Labels a list of labels that uniquely identifies exemplar
	// Optional, can be empty.
	Labels []Label
	// Value: the value of the exemplar
	Value float64
	// timestamp is in ms format, see model/timestamp/timestamp.go for
	// conversion from time.Time to Prometheus timestamp.
	Timestamp int64
}

// TimeSeries is a timeseries.
type TimeSeries struct {
	// Labels is a list of labels for the given TimeSeries
	Labels []Label

	// Samples is a list of samples for the given TimeSeries
	Samples   []Sample
	Exemplars []Exemplar
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
	// }
	tss := wr.Timeseries
	labelsPool := wr.labelsPool
	exemplarLabelsPool := wr.exemplarLabelsPool
	samplesPool := wr.samplesPool
	exemplarsPool := wr.exemplarsPool

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
			labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, err = ts.unmarshalProtobuf(data, labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool)
			if err != nil {
				return fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		}
	}
	wr.Timeseries = tss
	wr.labelsPool = labelsPool
	wr.samplesPool = samplesPool
	wr.exemplarsPool = exemplarsPool
	return nil
}

func (ts *TimeSeries) unmarshalProtobuf(src []byte, labelsPool []Label, exemplarLabelsPool []Label, samplesPool []Sample, exemplarsPool []Exemplar) ([]Label, []Label, []Sample, []Exemplar, error) {
	// message TimeSeries {
	//   repeated Label labels   = 1;
	//   repeated Sample samples = 2;
	//   repeated Exemplar exemplars = 3
	// }
	labelsPoolLen := len(labelsPool)
	samplesPoolLen := len(samplesPool)
	exemplarsPoolLen := len(exemplarsPool)
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot read label data")
			}
			if len(labelsPool) < cap(labelsPool) {
				labelsPool = labelsPool[:len(labelsPool)+1]
			} else {
				labelsPool = append(labelsPool, Label{})
			}
			label := &labelsPool[len(labelsPool)-1]
			if err := label.unmarshalProtobuf(data); err != nil {
				return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot unmarshal label: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot read the sample data")
			}
			if len(samplesPool) < cap(samplesPool) {
				samplesPool = samplesPool[:len(samplesPool)+1]
			} else {
				samplesPool = append(samplesPool, Sample{})
			}
			sample := &samplesPool[len(samplesPool)-1]
			if err := sample.unmarshalProtobuf(data); err != nil {
				return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot unmarshal sample: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot read the exemplar data")
			}
			if len(exemplarsPool) < cap(exemplarsPool) {
				exemplarsPool = exemplarsPool[:len(exemplarsPool)+1]
			} else {
				exemplarsPool = append(exemplarsPool, Exemplar{})
			}
			exemplar := &exemplarsPool[len(exemplarsPool)-1]
			if exemplarLabelsPool, err = exemplar.unmarshalProtobuf(data, exemplarLabelsPool); err != nil {
				return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, fmt.Errorf("cannot unmarshal exemplar: %w", err)
			}
		}
	}
	ts.Labels = labelsPool[labelsPoolLen:]
	ts.Samples = samplesPool[samplesPoolLen:]
	ts.Exemplars = exemplarsPool[exemplarsPoolLen:]
	return labelsPool, exemplarLabelsPool, samplesPool, exemplarsPool, nil
}

func (exemplar *Exemplar) unmarshalProtobuf(src []byte, labelsPool []Label) ([]Label, error) {
	// message Exemplar {
	//   repeated Label Labels  = 1;
	//   float64 Value = 2;
	//   int64 Timestamp = 3;
	// }
	var fc easyproto.FieldContext

	labelsPoolLen := len(labelsPool)

	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return labelsPool, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, fmt.Errorf("cannot read label data")
			}
			if len(labelsPool) < cap(labelsPool) {
				labelsPool = labelsPool[:len(labelsPool)+1]
			} else {
				labelsPool = append(labelsPool, Label{})
			}
			label := &labelsPool[len(labelsPool)-1]
			if err := label.unmarshalProtobuf(data); err != nil {
				return labelsPool, fmt.Errorf("cannot unmarshal label: %w", err)
			}
		case 2:
			value, ok := fc.Double()
			if !ok {
				return labelsPool, fmt.Errorf("cannot read exemplar value")
			}
			exemplar.Value = value
		case 3:
			timestamp, ok := fc.Int64()
			if !ok {
				return labelsPool, fmt.Errorf("cannot read exemplar timestamp")
			}
			exemplar.Timestamp = timestamp
		}
	}
	exemplar.Labels = labelsPool[labelsPoolLen:]
	return labelsPool, nil
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
