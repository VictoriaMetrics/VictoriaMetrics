package prompb

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

type sharedPool struct {
	labels     []Label
	samples    []Sample
	histograms []Histogram
}

func (p *sharedPool) reset() {
	labelsPool := p.labels
	for i := range labelsPool {
		labelsPool[i] = Label{}
	}
	p.labels = labelsPool[:0]

	samplesPool := p.samples
	for i := range samplesPool {
		samplesPool[i] = Sample{}
	}
	p.samples = samplesPool[:0]

	histogramsPool := p.histograms
	for i := range histogramsPool {
		histogramsPool[i] = Histogram{}
	}
	p.histograms = histogramsPool[:0]
}

// WriteRequest represents Prometheus remote write API request.
type WriteRequest struct {
	// Timeseries is a list of time series in the given WriteRequest
	Timeseries []TimeSeries
	pool       *sharedPool
}

// Reset resets wr for subsequent re-use.
func (wr *WriteRequest) Reset() {
	tss := wr.Timeseries
	for i := range tss {
		tss[i] = TimeSeries{}
	}
	wr.Timeseries = tss[:0]
	if wr.pool == nil {
		wr.pool = &sharedPool{}
	}
	wr.pool.reset()
}

// TimeSeries is a timeseries.
type TimeSeries struct {
	// Labels is a list of labels for the given TimeSeries
	Labels []Label

	// Samples is a list of samples for the given TimeSeries
	Samples []Sample

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

// Histogram is a struct for histograms
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

	// Buckets value of histogram
	Buckets []*Bucket
}

// Bucket type for histograms
type Bucket struct {
	CumulativeCount      uint64
	CumulativeCountFloat float64
	UpperBound           float64
}

// ResetHint type for histograms
type ResetHint int32

const (
	// UnknownHint variable of type ResetHint
	UnknownHint ResetHint = 0
	// YesHint variable of type ResetHint
	YesHint ResetHint = 1
	// NoHint variable of type ResetHint
	NoHint ResetHint = 2
	// GaugeHint variable of type ResetHint
	GaugeHint ResetHint = 3
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
	pool := wr.pool
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
			err = ts.unmarshalProtobuf(data, pool)
			if err != nil {
				return fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		}
	}
	wr.Timeseries = tss
	wr.pool = pool
	return nil
}

func (ts *TimeSeries) unmarshalProtobuf(src []byte, p *sharedPool) error {
	// message TimeSeries {
	//   repeated Label labels   = 1;
	//   repeated Sample samples = 2;
	//   repeated Histogram histograms = 3;
	// }
	labelsPoolLen := len(p.labels)
	samplesPoolLen := len(p.samples)
	histogramsPoolLen := len(p.histograms)

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
			p.labels = slicesutil.SetLength(p.labels, len(p.labels)+1)
			label := &p.labels[len(p.labels)-1]
			if err := label.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal label: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read the sample data")
			}
			p.samples = slicesutil.SetLength(p.samples, len(p.samples)+1)
			sample := &p.samples[len(p.samples)-1]
			if err := sample.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal sample: %w", err)
			}
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read the histogram data")
			}
			p.histograms = slicesutil.SetLength(p.histograms, len(p.histograms)+1)
			histogram := &p.histograms[len(p.histograms)-1]
			if err := histogram.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal histogram: %w", err)
			}
		}
	}
	ts.Labels = p.labels[labelsPoolLen:]
	ts.Samples = p.samples[samplesPoolLen:]
	ts.Histograms = p.histograms[histogramsPoolLen:]
	return nil
}

func (l *Label) unmarshalProtobuf(src []byte) (err error) {
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
			l.Name = name
		case 2:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read label value")
			}
			l.Value = value
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
