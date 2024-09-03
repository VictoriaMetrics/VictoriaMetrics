package pb

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/easyproto"
)

// ExportMetricsServiceRequest represents the corresponding OTEL protobuf message
type ExportMetricsServiceRequest struct {
	ResourceMetrics []*ResourceMetrics
}

// UnmarshalProtobuf unmarshals r from protobuf message at src.
func (r *ExportMetricsServiceRequest) UnmarshalProtobuf(src []byte) error {
	r.ResourceMetrics = nil
	return r.unmarshalProtobuf(src)
}

// MarshalProtobuf marshals r to protobuf message, appends it to dst and returns the result.
func (r *ExportMetricsServiceRequest) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	r.marshalProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

var mp easyproto.MarshalerPool

func (r *ExportMetricsServiceRequest) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, rm := range r.ResourceMetrics {
		rm.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (r *ExportMetricsServiceRequest) unmarshalProtobuf(src []byte) (err error) {
	// message ExportMetricsServiceRequest {
	//   repeated ResourceMetrics resource_metrics = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ExportMetricsServiceRequest: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ResourceMetrics data")
			}
			r.ResourceMetrics = append(r.ResourceMetrics, &ResourceMetrics{})
			rm := r.ResourceMetrics[len(r.ResourceMetrics)-1]
			if err := rm.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ResourceMetrics: %w", err)
			}
		}
	}
	return nil
}

// ResourceMetrics represents the corresponding OTEL protobuf message
type ResourceMetrics struct {
	Resource     *Resource
	ScopeMetrics []*ScopeMetrics
}

func (rm *ResourceMetrics) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if rm.Resource != nil {
		rm.Resource.marshalProtobuf(mm.AppendMessage(1))
	}
	for _, sm := range rm.ScopeMetrics {
		sm.marshalProtobuf(mm.AppendMessage(2))
	}
}

func (rm *ResourceMetrics) unmarshalProtobuf(src []byte) (err error) {
	// message ResourceMetrics {
	//   Resource resource = 1;
	//   repeated ScopeMetrics scope_metrics = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ResourceMetrics: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Resource data")
			}
			rm.Resource = &Resource{}
			if err := rm.Resource.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot umarshal Resource: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ScopeMetrics data")
			}
			rm.ScopeMetrics = append(rm.ScopeMetrics, &ScopeMetrics{})
			sm := rm.ScopeMetrics[len(rm.ScopeMetrics)-1]
			if err := sm.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ScopeMetrics: %w", err)
			}
		}
	}
	return nil
}

// ScopeMetrics represents the corresponding OTEL protobuf message
type ScopeMetrics struct {
	Metrics []*Metric
}

func (sm *ScopeMetrics) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, m := range sm.Metrics {
		m.marshalProtobuf(mm.AppendMessage(2))
	}
}

func (sm *ScopeMetrics) unmarshalProtobuf(src []byte) (err error) {
	// message ScopeMetrics {
	//   repeated Metric metrics = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ScopeMetrics: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Metric data")
			}
			sm.Metrics = append(sm.Metrics, &Metric{})
			m := sm.Metrics[len(sm.Metrics)-1]
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Metric: %w", err)
			}
		}
	}
	return nil
}

// Metric represents the corresponding OTEL protobuf message
type Metric struct {
	Name      string
	Unit      string
	Gauge     *Gauge
	Sum       *Sum
	Histogram *Histogram
	Summary   *Summary
}

func (m *Metric) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, m.Name)
	mm.AppendString(3, m.Unit)
	switch {
	case m.Gauge != nil:
		m.Gauge.marshalProtobuf(mm.AppendMessage(5))
	case m.Sum != nil:
		m.Sum.marshalProtobuf(mm.AppendMessage(7))
	case m.Histogram != nil:
		m.Histogram.marshalProtobuf(mm.AppendMessage(9))
	case m.Summary != nil:
		m.Summary.marshalProtobuf(mm.AppendMessage(11))
	}
}

func (m *Metric) unmarshalProtobuf(src []byte) (err error) {
	// message Metric {
	//   string name = 1;
	//   string unit = 3;
	//   oneof data {
	//     Gauge gauge = 5;
	//     Sum sum = 7;
	//     Histogram histogram = 9;
	//     Summary summary = 11;
	//   }
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Metric: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read metric name")
			}
			m.Name = strings.Clone(name)
		case 3:
			unit, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read metric unit")
			}
			m.Unit = strings.Clone(unit)
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Gauge data")
			}
			m.Gauge = &Gauge{}
			if err := m.Gauge.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Gauge: %w", err)
			}
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Sum data")
			}
			m.Sum = &Sum{}
			if err := m.Sum.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Sum: %w", err)
			}
		case 9:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Histogram data")
			}
			m.Histogram = &Histogram{}
			if err := m.Histogram.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Histogram: %w", err)
			}
		case 11:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Summary data")
			}
			m.Summary = &Summary{}
			if err := m.Summary.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Summary: %w", err)
			}
		}
	}
	return nil
}

// Gauge represents the corresponding OTEL protobuf message
type Gauge struct {
	DataPoints []*NumberDataPoint
}

func (g *Gauge) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range g.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (g *Gauge) unmarshalProtobuf(src []byte) (err error) {
	// message Gauge {
	//   repeated NumberDataPoint data_points = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Gauge")
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read DataPoint data")
			}
			g.DataPoints = append(g.DataPoints, &NumberDataPoint{})
			dp := g.DataPoints[len(g.DataPoints)-1]
			if err := dp.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal DataPoint: %w", err)
			}
		}
	}
	return nil
}

// NumberDataPoint represents the corresponding OTEL protobuf message
type NumberDataPoint struct {
	Attributes   []*KeyValue
	TimeUnixNano uint64
	DoubleValue  *float64
	IntValue     *int64
	Flags        uint32
}

func (ndp *NumberDataPoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, a := range ndp.Attributes {
		a.marshalProtobuf(mm.AppendMessage(7))
	}
	mm.AppendFixed64(3, ndp.TimeUnixNano)
	switch {
	case ndp.DoubleValue != nil:
		mm.AppendDouble(4, *ndp.DoubleValue)
	case ndp.IntValue != nil:
		mm.AppendSfixed64(6, *ndp.IntValue)
	}
	mm.AppendUint32(8, ndp.Flags)
}

func (ndp *NumberDataPoint) unmarshalProtobuf(src []byte) (err error) {
	// message NumberDataPoint {
	//   repeated KeyValue attributes = 7;
	//   fixed64 time_unix_nano = 3;
	//   oneof value {
	//     double as_double = 4;
	//     sfixed64 as_int = 6;
	//   }
	//   uint32 flags = 8;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in NumberDataPoint: %w", err)
		}
		switch fc.FieldNum {
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attribute")
			}
			ndp.Attributes = append(ndp.Attributes, &KeyValue{})
			a := ndp.Attributes[len(ndp.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Attribute: %w", err)
			}
		case 3:
			timeUnixNano, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
			ndp.TimeUnixNano = timeUnixNano
		case 4:
			doubleValue, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read DoubleValue")
			}
			ndp.DoubleValue = &doubleValue
		case 6:
			intValue, ok := fc.Sfixed64()
			if !ok {
				return fmt.Errorf("cannot read IntValue")
			}
			ndp.IntValue = &intValue
		case 8:
			flags, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
			ndp.Flags = flags
		}
	}
	return nil
}

// Sum represents the corresponding OTEL protobuf message
type Sum struct {
	DataPoints             []*NumberDataPoint
	AggregationTemporality AggregationTemporality
	IsMonotonic            bool
}

// AggregationTemporality represents the corresponding OTEL protobuf enum
type AggregationTemporality int

const (
	// AggregationTemporalityUnspecified is enum value for AggregationTemporality
	AggregationTemporalityUnspecified = AggregationTemporality(0)
	// AggregationTemporalityDelta is enum value for AggregationTemporality
	AggregationTemporalityDelta = AggregationTemporality(1)
	// AggregationTemporalityCumulative is enum value for AggregationTemporality
	AggregationTemporalityCumulative = AggregationTemporality(2)
)

func (s *Sum) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range s.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
	mm.AppendInt64(2, int64(s.AggregationTemporality))
	mm.AppendBool(3, s.IsMonotonic)
}

func (s *Sum) unmarshalProtobuf(src []byte) (err error) {
	// message Sum {
	//   repeated NumberDataPoint data_points = 1;
	//   AggregationTemporality aggregation_temporality = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Sum: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read DataPoint data")
			}
			s.DataPoints = append(s.DataPoints, &NumberDataPoint{})
			dp := s.DataPoints[len(s.DataPoints)-1]
			if err := dp.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal DataPoint: %w", err)
			}
		case 2:
			at, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read AggregationTemporality")
			}
			s.AggregationTemporality = AggregationTemporality(at)
		case 3:
			im, ok := fc.Bool()
			if !ok {
				return fmt.Errorf("cannot read IsMonotonic")
			}
			s.IsMonotonic = im
		}
	}
	return nil
}

// Histogram represents the corresponding OTEL protobuf message
type Histogram struct {
	DataPoints             []*HistogramDataPoint
	AggregationTemporality AggregationTemporality
}

func (h *Histogram) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range h.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
	mm.AppendInt64(2, int64(h.AggregationTemporality))
}

func (h *Histogram) unmarshalProtobuf(src []byte) (err error) {
	// message Histogram {
	//   repeated HistogramDataPoint data_points = 1;
	//   AggregationTemporality aggregation_temporality = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Histogram: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read DataPoint")
			}
			h.DataPoints = append(h.DataPoints, &HistogramDataPoint{})
			dp := h.DataPoints[len(h.DataPoints)-1]
			if err := dp.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal DataPoint: %w", err)
			}
		case 2:
			at, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read AggregationTemporality")
			}
			h.AggregationTemporality = AggregationTemporality(at)
		}
	}
	return nil
}

// Summary represents the corresponding OTEL protobuf message
type Summary struct {
	DataPoints []*SummaryDataPoint
}

func (s *Summary) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range s.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (s *Summary) unmarshalProtobuf(src []byte) (err error) {
	// message Summary {
	//   repeated SummaryDataPoint data_points = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Summary: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read DataPoint")
			}
			s.DataPoints = append(s.DataPoints, &SummaryDataPoint{})
			dp := s.DataPoints[len(s.DataPoints)-1]
			if err := dp.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal DataPoint: %w", err)
			}
		}
	}
	return nil
}

// HistogramDataPoint represents the corresponding OTEL protobuf message
type HistogramDataPoint struct {
	Attributes     []*KeyValue
	TimeUnixNano   uint64
	Count          uint64
	Sum            *float64
	BucketCounts   []uint64
	ExplicitBounds []float64
	Flags          uint32
}

func (dp *HistogramDataPoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, a := range dp.Attributes {
		a.marshalProtobuf(mm.AppendMessage(9))
	}
	mm.AppendFixed64(3, dp.TimeUnixNano)
	mm.AppendFixed64(4, dp.Count)
	if dp.Sum != nil {
		mm.AppendDouble(5, *dp.Sum)
	}
	mm.AppendFixed64s(6, dp.BucketCounts)
	mm.AppendDoubles(7, dp.ExplicitBounds)
	mm.AppendUint32(10, dp.Flags)
}

func (dp *HistogramDataPoint) unmarshalProtobuf(src []byte) (err error) {
	// message HistogramDataPoint {
	//   repeated KeyValue attributes = 9;
	//   fixed64 time_unix_nano = 3;
	//   fixed64 count = 4;
	//   optional double sum = 5;
	//   repeated fixed64 bucket_counts = 6;
	//   repeated double explicit_bounds = 7;
	//   uint32 flags = 10;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in HistogramDataPoint: %w", err)
		}
		switch fc.FieldNum {
		case 9:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attribute")
			}
			dp.Attributes = append(dp.Attributes, &KeyValue{})
			a := dp.Attributes[len(dp.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Attribute: %w", err)
			}
		case 3:
			timeUnixNano, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
			dp.TimeUnixNano = timeUnixNano
		case 4:
			count, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read Count")
			}
			dp.Count = count
		case 5:
			sum, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Sum")
			}
			dp.Sum = &sum
		case 6:
			bucketCounts, ok := fc.UnpackFixed64s(dp.BucketCounts)
			if !ok {
				return fmt.Errorf("cannot read BucketCounts")
			}
			dp.BucketCounts = bucketCounts
		case 7:
			explicitBounds, ok := fc.UnpackDoubles(dp.ExplicitBounds)
			if !ok {
				return fmt.Errorf("cannot read ExplicitBounds")
			}
			dp.ExplicitBounds = explicitBounds
		case 10:
			flags, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
			dp.Flags = flags
		}
	}
	return nil
}

// SummaryDataPoint represents the corresponding OTEL protobuf message
type SummaryDataPoint struct {
	Attributes     []*KeyValue
	TimeUnixNano   uint64
	Count          uint64
	Sum            float64
	QuantileValues []*ValueAtQuantile
	Flags          uint32
}

func (dp *SummaryDataPoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, a := range dp.Attributes {
		a.marshalProtobuf(mm.AppendMessage(7))
	}
	mm.AppendFixed64(3, dp.TimeUnixNano)
	mm.AppendFixed64(4, dp.Count)
	mm.AppendDouble(5, dp.Sum)
	for _, v := range dp.QuantileValues {
		v.marshalProtobuf(mm.AppendMessage(6))
	}
	mm.AppendUint32(8, dp.Flags)
}

func (dp *SummaryDataPoint) unmarshalProtobuf(src []byte) (err error) {
	// message SummaryDataPoint {
	//   repeated KeyValue attributes = 7;
	//   fixed64 time_unix_nano = 3;
	//   fixed64 count = 4;
	//   double sum = 5;
	//   repeated ValueAtQuantile quantile_values = 6;
	//   uint32 flags = 8;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in SummaryDataPoint: %w", err)
		}
		switch fc.FieldNum {
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attribute")
			}
			dp.Attributes = append(dp.Attributes, &KeyValue{})
			a := dp.Attributes[len(dp.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Attribute: %w", err)
			}
		case 3:
			timeUnixNano, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
			dp.TimeUnixNano = timeUnixNano
		case 4:
			count, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read Count")
			}
			dp.Count = count
		case 5:
			sum, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Sum")
			}
			dp.Sum = sum
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read QuantileValue")
			}
			dp.QuantileValues = append(dp.QuantileValues, &ValueAtQuantile{})
			v := dp.QuantileValues[len(dp.QuantileValues)-1]
			if err := v.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal QuantileValue: %w", err)
			}
		case 8:
			flags, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
			dp.Flags = flags
		}
	}
	return nil
}

// ValueAtQuantile represents the corresponding OTEL protobuf message
type ValueAtQuantile struct {
	Quantile float64
	Value    float64
}

func (v *ValueAtQuantile) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendDouble(1, v.Quantile)
	mm.AppendDouble(2, v.Value)
}

func (v *ValueAtQuantile) unmarshalProtobuf(src []byte) (err error) {
	// message ValueAtQuantile {
	//   double quantile = 1;
	//   double value = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ValueAtQuantile: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			quantile, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Quantile")
			}
			v.Quantile = quantile
		case 2:
			value, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Value")
			}
			v.Value = value
		}
	}
	return nil
}
