package prompb

import (
	"fmt"

	"github.com/VictoriaMetrics/easyproto"
)

// WriteRequest represents Prometheus remote write API request.
type WriteRequest struct {
	// Timeseries is a list of time series in the given WriteRequest
	Timeseries []TimeSeries

	labelsPool     []Label
	samplesPool    []Sample
	exemplarsPool  []Exemplar
	histogramsPool []Histogram
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

	histogramsPool := wr.histogramsPool
	for i := range histogramsPool {
		histogramsPool[i] = Histogram{}
	}
	wr.histogramsPool = histogramsPool[:0]
}

// TimeSeries is a timeseries.
type TimeSeries struct {
	// Labels is a list of labels for the given TimeSeries
	Labels []Label

	// Samples is a list of samples for the given TimeSeries
	Samples []Sample

	// Exemplars is a list of exemplars for the given TimeSeries
	Exemplars []Exemplar

	// Histograms is a list of histograms for the given TimeSeries
	Histograms []Histogram
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

// Exemplar is a timeseries exemplar
type Exemplar struct {
	// Labels is a list of labels for a given exemplar
	Labels []Label

	// Value is an exemplar value
	Value float64

	// Timestamp is an exemplar timestamp
	Timestamp int64
}

type Histogram struct {
	// Count value of histogram
	Count uint64

	// CountFloat value of histogram
	CountFloat float64

	// Sum value of histogram
	Sum float64

	// Schema value of histogram
	Schema int32

	// ZeroThreshold value of histogram
	ZeroThreshold float64

	// ZeroCount value of histogram
	ZeroCount uint64

	// ZeroCountFloat value of histogram
	ZeroCountFloat float64

	// NegativeSpans value of histogram
	NegativeSpans []BucketSpan

	// NegativeDeltas value of histogram
	NegativeDeltas []int64

	// NegativeCounts value of histogram
	NegativeCounts []float64

	// PositiveSpans value of histogram
	PositiveSpans []BucketSpan

	// PositiveDeltas value of histogram
	PositiveDeltas []int64

	// PositiveCounts value of histogram
	PositiveCounts []float64

	// ResetHint value of histogram
	ResetHint ResetHint

	// Timestamp value of histogram
	Timestamp int64
}

type ResetHint int32

const (
	UnknownHint ResetHint = 0
	YesHint     ResetHint = 1
	NoHint      ResetHint = 2
	GaugeHint   ResetHint = 3
)

// BucketSpan defines a number of consecutive buckets with their offset
type BucketSpan struct {
	Offset int32
	Length uint32
}

func (s *BucketSpan) unmarshalProtobuf(src []byte) (err error) {
	// message BucketSpan {
	//   sint32 offset = 1;
	//   uint32 length = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			offset, ok := fc.Sint32()
			if !ok {
				return fmt.Errorf("cannot read bucket span offset")
			}
			s.Offset = offset
		case 2:
			length, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read bucket span length")
			}
			s.Length = length
		}
	}
	return nil
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
	samplesPool := wr.samplesPool
	exemplarsPool := wr.exemplarsPool
	histogramsPool := wr.histogramsPool
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
			err = ts.unmarshalProtobuf(data, &labelsPool, &samplesPool, &exemplarsPool, &histogramsPool)
			if err != nil {
				return fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		}
	}
	wr.Timeseries = tss
	wr.labelsPool = labelsPool
	wr.samplesPool = samplesPool
	wr.exemplarsPool = exemplarsPool
	wr.histogramsPool = histogramsPool
	return nil
}

func (ts *TimeSeries) unmarshalProtobuf(src []byte, labelsPool *[]Label, samplesPool *[]Sample, exemplarsPool *[]Exemplar, histogramsPool *[]Histogram) error {
	// message TimeSeries {
	//   repeated Label labels         = 1;
	//   repeated Sample samples       = 2;
	//   repeated Exemplar exemplars   = 3;
	//   repeated Histogram histograms = 4;
	// }
	labelsPoolLen := len(*labelsPool)
	samplesPoolLen := len(*samplesPool)
	exemplarsPoolLen := len(*exemplarsPool)
	histogramsPoolLen := len(*histogramsPool)
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read label data")
			}
			if len(*labelsPool) < cap(*labelsPool) {
				*labelsPool = (*labelsPool)[:len(*labelsPool)+1]
			} else {
				*labelsPool = append(*labelsPool, Label{})
			}
			label := &(*labelsPool)[len(*labelsPool)-1]
			if err := label.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal label: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read the sample data")
			}
			if len(*samplesPool) < cap(*samplesPool) {
				*samplesPool = (*samplesPool)[:len(*samplesPool)+1]
			} else {
				*samplesPool = append(*samplesPool, Sample{})
			}
			sample := &(*samplesPool)[len(*samplesPool)-1]
			if err := sample.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal sample: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read the exemplar data")
			}
			if len(*exemplarsPool) < cap(*exemplarsPool) {
				*exemplarsPool = (*exemplarsPool)[:len(*exemplarsPool)+1]
			} else {
				*exemplarsPool = append(*exemplarsPool, Exemplar{})
			}
			exemplar := &(*exemplarsPool)[len(*exemplarsPool)-1]
			if err := exemplar.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal exemplar: %w", err)
			}
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read the histogram data")
			}
			if len(*histogramsPool) < cap(*histogramsPool) {
				*histogramsPool = (*histogramsPool)[:len(*histogramsPool)+1]
			} else {
				*histogramsPool = append(*histogramsPool, Histogram{})
			}
			histogram := &(*histogramsPool)[len(*histogramsPool)-1]
			if err := histogram.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal histogram: %w", err)
			}
		}
	}
	ts.Labels = (*labelsPool)[labelsPoolLen:]
	ts.Samples = (*samplesPool)[samplesPoolLen:]
	ts.Exemplars = (*exemplarsPool)[exemplarsPoolLen:]
	ts.Histograms = (*histogramsPool)[histogramsPoolLen:]
	return nil
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

func (e *Exemplar) unmarshalProtobuf(src []byte) (err error) {
	// message Sample {
	//   repeated Label labels = 1;
	//   double value          = 2;
	//   int64 timestamp       = 3;
	// }
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
				return fmt.Errorf("cannot read Labels data")
			}
			var l Label
			if err := (&l).unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Label: %w", err)
			}
			e.Labels = append(e.Labels, l)
		case 2:
			value, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read exemplar value")
			}
			e.Value = value
		case 3:
			timestamp, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read exemplar timestamp")
			}
			e.Timestamp = timestamp
		}
	}
	return nil
}

func (h *Histogram) unmarshalProtobuf(src []byte) (err error) {
	// message Histogram {
	//   uint64              count_int        = 1;
	//   double              count_float      = 2;
	//   double              sum              = 3;
	//   sint32              schema           = 4;
	//   double              zero_threshold   = 5;
	//   uint64              zero_count_int   = 6;
	//   double              zero_count_float = 7;
	//   repeated BucketSpan negative_span    = 8;
	//   repeated sint64     negative_deltas  = 9;
	//   repeated double     negative_counts  = 10;
	//   repeated BucketSpan positive_spans   = 11;
	//   repeated sint64     positive_deltas  = 12;
	//   repeated double     positive_counts  = 13;
	//   ResetHint           reset_hint       = 14;
	//   int                 timestamp        = 15;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			count, ok := fc.Uint64()
			if !ok {
				return fmt.Errorf("cannot read count int value")
			}
			h.Count = count
		case 2:
			count, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read count float value")
			}
			h.CountFloat = count
		case 3:
			sum, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sum value")
			}
			h.Sum = sum
		case 4:
			schema, ok := fc.Sint32()
			if !ok {
				return fmt.Errorf("cannot read schema value")
			}
			h.Schema = schema
		case 5:
			zeroThreshold, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read zero threshold value")
			}
			h.ZeroThreshold = zeroThreshold
		case 6:
			count, ok := fc.Uint64()
			if !ok {
				return fmt.Errorf("cannot read zero count int value")
			}
			h.ZeroCount = count
		case 7:
			count, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read zero count float value")
			}
			h.ZeroCountFloat = count
		case 8:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read negative_span data")
			}
			var s BucketSpan
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal negative_span: %w", err)
			}
			h.NegativeSpans = append(h.NegativeSpans, s)
		case 9:
			var ok bool
			h.NegativeDeltas, ok = fc.UnpackSint64s(h.NegativeDeltas)
			if !ok {
				return fmt.Errorf("cannot read negative_delta")
			}
		case 10:
			var ok bool
			h.NegativeCounts, ok = fc.UnpackDoubles(h.NegativeCounts)
			if !ok {
				return fmt.Errorf("cannot read negative_count")
			}
		case 11:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read positive_span data")
			}
			var s BucketSpan
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal positive_span: %w", err)
			}
			h.PositiveSpans = append(h.PositiveSpans, s)
		case 12:
			var ok bool
			h.PositiveDeltas, ok = fc.UnpackSint64s(h.PositiveDeltas)
			if !ok {
				return fmt.Errorf("cannot read positive_delta")
			}
		case 13:
			var ok bool
			h.PositiveCounts, ok = fc.UnpackDoubles(h.PositiveCounts)
			if !ok {
				return fmt.Errorf("cannot read positive_count")
			}
		case 14:
			resetHint, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read reset hint")
			}
			h.ResetHint = ResetHint(resetHint)
		case 15:
			ts, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read timestamp value")
			}
			h.Timestamp = ts
		}
	}
	return nil
}
