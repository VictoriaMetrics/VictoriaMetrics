package pb

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/easyproto"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// MetricPusher must push the parsed samples and metric metadata to the underlying storage.
type MetricPusher interface {
	// PushSample must store a sample with the given args.
	//
	// The PushSample must copy labels, since they become invalid after returning from the func.
	PushSample(mm *MetricMetadata, suffix string, ls *promutil.Labels, timestampNsecs uint64, value float64, flags uint32)

	// PushMetricMetadata must store mm.
	//
	// The PushMetricMetadata must copy mm contents, since it becomes invalid after returning from the func.
	PushMetricMetadata(mm *MetricMetadata)
}

// MetricMetadata contains metric metadata
type MetricMetadata struct {
	// Name is metric name
	Name string

	// Unit is metric unit
	Unit string

	// Description is metric description
	Description string

	// Type is metric type
	Type prompb.MetricType
}

func (mm *MetricMetadata) reset() {
	mm.Name = ""
	mm.Unit = ""
	mm.Type = 0
}

// MetricsData represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L56
type MetricsData struct {
	ResourceMetrics []*ResourceMetrics
}

// MarshalProtobuf marshals r to protobuf message, appends it to dst and returns the result.
func (r *MetricsData) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	r.marshalProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

var mp easyproto.MarshalerPool

func (r *MetricsData) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, rm := range r.ResourceMetrics {
		rm.marshalProtobuf(mm.AppendMessage(1))
	}
}

// DecodeMetricsData decodes metricsData from src and sends the decoded data to mp.
func DecodeMetricsData(src []byte, mp MetricPusher) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L56
	//
	// message MetricsData {
	//   repeated ResourceMetrics resource_metrics = 1;
	// }

	dctx := getDecoderContext(mp)
	defer putDecoderContext(dctx)

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
				return fmt.Errorf("cannot read ResourceMetrics data")
			}
			if err := dctx.decodeResourceMetrics(data); err != nil {
				return fmt.Errorf("cannot unmarshal ResourceMetrics: %w", err)
			}
		}
	}
	return nil
}

// ResourceMetrics represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L66
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

func (dctx *decoderContext) decodeResourceMetrics(src []byte) error {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L66
	//
	// message ResourceMetrics {
	//   Resource resource = 1;
	//   repeated ScopeMetrics scope_metrics = 2;
	// }

	dctx.ls.Reset()
	dctx.fb.reset()

	resourceData, ok, err := easyproto.GetMessageData(src, 1)
	if err != nil {
		return fmt.Errorf("cannot read Resource data: %w", err)
	}
	if ok {
		if err := dctx.decodeResource(resourceData); err != nil {
			return fmt.Errorf("cannot decode Resource: %w", err)
		}
	}

	dctxSnapshot := dctx.getSnapshot()

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ScopeMetrics data")
			}

			if err := dctx.decodeScopeMetrics(data); err != nil {
				return fmt.Errorf("cannot unmarshal ScopeMetrics: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// Resource represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/resource/v1/resource.proto#L28
type Resource struct {
	Attributes []*KeyValue
}

func (r *Resource) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, a := range r.Attributes {
		a.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (dctx *decoderContext) decodeResource(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/resource/v1/resource.proto#L28
	//
	// message Resource {
	//   repeated KeyValue attributes = 1;
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
				return fmt.Errorf("cannot read Attributes")
			}
			if err := decodeKeyValue(data, &dctx.ls, &dctx.fb, ""); err != nil {
				return fmt.Errorf("cannot unmarshal Attributes: %w", err)
			}
		}
	}
	return nil
}

// KeyValue represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L66
type KeyValue struct {
	Key   string
	Value *AnyValue
}

func (kv *KeyValue) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if kv.Key != "" {
		mm.AppendString(1, kv.Key)
	}
	if kv.Value != nil {
		kv.Value.marshalProtobuf(mm.AppendMessage(2))
	}
}

func decodeKeyValue(src []byte, ls *promutil.Labels, fb *fmtBuffer, keyPrefix string) error {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L66
	//
	// message KeyValue {
	//   string key = 1;
	//   AnyValue value = 2;
	// }

	// Decode key
	keySuffix, ok, err := easyproto.GetString(src, 1)
	if err != nil {
		return fmt.Errorf("cannot find Key in KeyValue: %w", err)
	}
	if !ok {
		// Key is missing, skip it.
		// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/869#issuecomment-3631307996
		return nil
	}
	key := fb.formatSubFieldName(keyPrefix, keySuffix)

	// Decode value
	valueData, ok, err := easyproto.GetMessageData(src, 2)
	if err != nil {
		return fmt.Errorf("cannot find Value in KeyValue: %w", err)
	}
	if !ok {
		// Value is null, skip it.
		return nil
	}

	if err := decodeAnyValue(valueData, ls, fb, key); err != nil {
		return fmt.Errorf("cannot decode AnyValue: %w", err)
	}

	return nil
}

// AnyValue represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L28
type AnyValue struct {
	StringValue  *string
	BoolValue    *bool
	IntValue     *int64
	DoubleValue  *float64
	ArrayValue   *ArrayValue
	KeyValueList *KeyValueList
	BytesValue   *[]byte
}

func (av *AnyValue) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if av == nil {
		return
	}
	switch {
	case av.StringValue != nil:
		mm.AppendString(1, *av.StringValue)
	case av.BoolValue != nil:
		mm.AppendBool(2, *av.BoolValue)
	case av.IntValue != nil:
		mm.AppendInt64(3, *av.IntValue)
	case av.DoubleValue != nil:
		mm.AppendDouble(4, *av.DoubleValue)
	case av.ArrayValue != nil:
		av.ArrayValue.marshalProtobuf(mm.AppendMessage(5))
	case av.KeyValueList != nil:
		av.KeyValueList.marshalProtobuf(mm.AppendMessage(6))
	case av.BytesValue != nil:
		mm.AppendBytes(7, *av.BytesValue)
	}
}

func decodeAnyValue(src []byte, ls *promutil.Labels, fb *fmtBuffer, key string) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L28
	//
	// message AnyValue {
	//   oneof value {
	//     string string_value = 1;
	//     bool bool_value = 2;
	//     int64 int_value = 3;
	//     double double_value = 4;
	//     ArrayValue array_value = 5;
	//     KeyValueList kvlist_value = 6;
	//     bytes bytes_value = 7;
	//   }
	// }

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			stringValue, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read StringValue")
			}
			ls.Add(key, stringValue)
		case 2:
			boolValue, ok := fc.Bool()
			if !ok {
				return fmt.Errorf("cannot read BoolValue")
			}
			boolValueStr := strconv.FormatBool(boolValue)
			ls.Add(key, boolValueStr)
		case 3:
			intValue, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read IntValue")
			}
			intValueStr := fb.formatInt(intValue)
			ls.Add(key, intValueStr)
		case 4:
			doubleValue, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read DoubleValue")
			}
			doubleValueStr := fb.formatFloat(doubleValue)
			ls.Add(key, doubleValueStr)
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ArrayValue")
			}
			a := jsonArenaPool.Get()
			// Encode arrays as JSON to match the behavior of /insert/jsonline
			arr, err := decodeArrayValueToJSON(data, a, fb)
			if err != nil {
				jsonArenaPool.Put(a)
				return fmt.Errorf("cannot decode ArrayValue: %w", err)
			}

			encodedArr := fb.encodeJSONValue(arr)
			jsonArenaPool.Put(a)

			ls.Add(key, encodedArr)
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read KeyValueList")
			}
			if err := decodeKeyValueList(data, ls, fb, key); err != nil {
				return fmt.Errorf("cannot decode KeyValueList: %w", err)
			}
		case 7:
			bytesValue, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read BytesValue")
			}
			v := fb.formatBase64(bytesValue)
			ls.Add(key, v)
		}
	}
	return nil
}

// ArrayValue represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L44
type ArrayValue struct {
	Values []*AnyValue
}

func (av *ArrayValue) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, v := range av.Values {
		v.marshalProtobuf(mm.AppendMessage(1))
	}
}

// KeyValueList represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L54
type KeyValueList struct {
	Values []*KeyValue
}

func (kvl *KeyValueList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, v := range kvl.Values {
		v.marshalProtobuf(mm.AppendMessage(1))
	}
}

func decodeKeyValueList(src []byte, ls *promutil.Labels, fb *fmtBuffer, keyPrefix string) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L54
	//
	// message KeyValueList {
	//   repeated KeyValue values = 1;
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
				return fmt.Errorf("cannot read KeyValue data")
			}
			if err := decodeKeyValue(data, ls, fb, keyPrefix); err != nil {
				return fmt.Errorf("cannot decode KeyValue: %w", err)
			}
		}
	}
	return nil
}

// ScopeMetrics represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L86
type ScopeMetrics struct {
	Scope   *InstrumentationScope
	Metrics []*Metric
}

func (sm *ScopeMetrics) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if sm.Scope != nil {
		sm.Scope.marshalProtobuf(mm.AppendMessage(1))
	}
	for _, m := range sm.Metrics {
		m.marshalProtobuf(mm.AppendMessage(2))
	}
}

func (dctx *decoderContext) decodeScopeMetrics(src []byte) error {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L86
	//
	// message ScopeMetrics {
	//   InstrumentationScope scope = 1;
	//   repeated Metric metrics = 2;
	// }

	scopeData, ok, err := easyproto.GetMessageData(src, 1)
	if err != nil {
		return fmt.Errorf("cannot read InstrumentationScope: %w", err)
	}
	if ok {
		if err := dctx.decodeInstrumentationScope(scopeData); err != nil {
			return fmt.Errorf("cannot decode InstrumentationScope: %w", err)
		}
	}

	dctxSnapshot := dctx.getSnapshot()

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Metric data")
			}

			if err := dctx.decodeMetric(data); err != nil {
				return fmt.Errorf("cannot unmarshal Metric: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// InstrumentationScope represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/common/v1/common.proto#L76
type InstrumentationScope struct {
	Name       *string
	Version    *string
	Attributes []*KeyValue
}

func (is *InstrumentationScope) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if is.Name != nil {
		mm.AppendString(1, *is.Name)
	}
	if is.Version != nil {
		mm.AppendString(2, *is.Version)
	}
	for _, a := range is.Attributes {
		a.marshalProtobuf(mm.AppendMessage(3))
	}
}

func (dctx *decoderContext) decodeInstrumentationScope(src []byte) error {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/a5f0eac5b802f7ae51dfe41e5116fe5548955e64/opentelemetry/proto/common/v1/common.proto#L76
	//
	// message InstrumentationScope {
	//   string name = 1;
	//   string version = 2;
	//   repeated KeyValue attributes = 3;
	// }

	nameStr, ok, err := easyproto.GetString(src, 1)
	if err != nil {
		return fmt.Errorf("cannot read name: %w", err)
	}
	name := "unknown"
	if ok {
		name = nameStr
	}
	dctx.ls.Add("scope.name", name)

	versionStr, ok, err := easyproto.GetString(src, 2)
	if err != nil {
		return fmt.Errorf("cannot read version: %w", err)
	}
	version := "unknown"
	if ok {
		version = versionStr
	}
	dctx.ls.Add("scope.version", version)

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 3:
			attributesData, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attributes data")
			}
			if err := decodeKeyValue(attributesData, &dctx.ls, &dctx.fb, "scope.attributes"); err != nil {
				return fmt.Errorf("cannot decode Attributes: %w", err)
			}
		}
	}

	return nil
}

// Metric represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L188
type Metric struct {
	Name                 string
	Description          string
	Unit                 string
	Gauge                *Gauge
	Sum                  *Sum
	Histogram            *Histogram
	ExponentialHistogram *ExponentialHistogram
	Summary              *Summary

	Metadata []*KeyValue
}

func (m *Metric) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, m.Name)
	mm.AppendString(2, m.Description)
	mm.AppendString(3, m.Unit)
	switch {
	case m.Gauge != nil:
		m.Gauge.marshalProtobuf(mm.AppendMessage(5))
	case m.Sum != nil:
		m.Sum.marshalProtobuf(mm.AppendMessage(7))
	case m.Histogram != nil:
		m.Histogram.marshalProtobuf(mm.AppendMessage(9))
	case m.ExponentialHistogram != nil:
		m.ExponentialHistogram.marshalProtobuf(mm.AppendMessage(10))
	case m.Summary != nil:
		m.Summary.marshalProtobuf(mm.AppendMessage(11))
	}
	for _, md := range m.Metadata {
		md.marshalProtobuf(mm.AppendMessage(12))
	}
}

func (dctx *decoderContext) decodeMetric(src []byte) error {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L188
	//
	// message Metric {
	//   string name = 1;
	//   string description = 2;
	//   string unit = 3;
	//   oneof data {
	//     Gauge gauge = 5;
	//     Sum sum = 7;
	//     Histogram histogram = 9;
	//     ExponentialHistogram exponential_histogram = 10;
	//     Summary summary = 11;
	//   }
	//   repeated opentelemetry.proto.common.v1.KeyValue metadata = 12;
	// }

	metricName, ok, err := easyproto.GetString(src, 1)
	if err != nil {
		return fmt.Errorf("cannot read metric name: %w", err)
	}
	if !ok {
		return fmt.Errorf("missing metric name")
	}
	dctx.mm.Name = metricName

	unit, ok, err := easyproto.GetString(src, 3)
	if err != nil {
		return fmt.Errorf("cannot read metric unit: %w", err)
	}
	if ok {
		dctx.mm.Unit = unit
	}

	dctx.mm.Type = prompb.MetricTypeUnknown

	lsMetadata := promutil.GetLabels()
	defer promutil.PutLabels(lsMetadata)

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 2:
			dctx.mm.Description, ok = fc.String()
			if !ok {
				return fmt.Errorf("cannot read metric description")
			}
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Gauge data")
			}
			if err := dctx.decodeGauge(data); err != nil {
				return fmt.Errorf("cannot unmarshal Gauge: %w", err)
			}
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Sum data")
			}
			if err := dctx.decodeSum(data); err != nil {
				return fmt.Errorf("cannot unmarshal Sum: %w", err)
			}
		case 9:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Histogram data")
			}
			if err := dctx.decodeHistogram(data); err != nil {
				return fmt.Errorf("cannot unmarshal Histogram: %w", err)
			}
		case 10:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ExponentialHistogram data")
			}
			if err := dctx.decodeExponentialHistogram(data); err != nil {
				return fmt.Errorf("cannot unmarshal ExponentialHistogram: %w", err)
			}
		case 11:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Summary data")
			}
			if err := dctx.decodeSummary(data); err != nil {
				return fmt.Errorf("cannot unmarshal Summary: %w", err)
			}
		case 12:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read metadata attributes")
			}
			if err := decodeKeyValue(data, lsMetadata, &dctx.fb, ""); err != nil {
				return fmt.Errorf("cannot unmarshal metadata attributes: %w", err)
			}
		}
	}

	switch lsMetadata.Get("prometheus.type") {
	case "unknown":
		dctx.mm.Type = prompb.MetricTypeUnknown
	case "info":
		dctx.mm.Type = prompb.MetricTypeInfo
	case "stateset":
		dctx.mm.Type = prompb.MetricTypeStateset
	}

	dctx.mp.PushMetricMetadata(&dctx.mm)

	return nil
}

// Gauge represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L232
type Gauge struct {
	DataPoints []*NumberDataPoint
}

func (g *Gauge) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range g.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (dctx *decoderContext) decodeGauge(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L232
	//
	// message Gauge {
	//   repeated NumberDataPoint data_points = 1;
	// }

	dctx.mm.Type = prompb.MetricTypeGauge

	dctxSnapshot := dctx.getSnapshot()

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
				return fmt.Errorf("cannot read NumberDataPoint data")
			}
			if err := dctx.decodeNumberDataPoint(data); err != nil {
				return fmt.Errorf("cannot unmarshal NumberDataPoint: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// NumberDataPoint represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L385
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

func (dctx *decoderContext) decodeNumberDataPoint(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L385
	//
	// message NumberDataPoint {
	//   repeated KeyValue attributes = 7;
	//   fixed64 time_unix_nano = 3;
	//   oneof value {
	//     double as_double = 4;
	//     sfixed64 as_int = 6;
	//   }
	//   uint32 flags = 8;
	// }

	var (
		timestamp uint64
		value     float64
		flags     uint32
	)

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attributes")
			}
			if err := decodeKeyValue(data, &dctx.ls, &dctx.fb, ""); err != nil {
				return fmt.Errorf("cannot unmarshal Attributes: %w", err)
			}
		case 3:
			timestamp, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
		case 4:
			value, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read DoubleValue")
			}
		case 6:
			intValue, ok := fc.Sfixed64()
			if !ok {
				return fmt.Errorf("cannot read IntValue")
			}
			value = float64(intValue)
		case 8:
			flags, ok = fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
		}
	}

	dctx.mp.PushSample(&dctx.mm, "", &dctx.ls, timestamp, value, flags)

	return nil
}

// Sum represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L240
type Sum struct {
	DataPoints  []*NumberDataPoint
	IsMonotonic bool
}

func (s *Sum) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range s.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
	mm.AppendBool(3, s.IsMonotonic)
}

func (dctx *decoderContext) decodeSum(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L240
	//
	// message Sum {
	//   repeated NumberDataPoint data_points = 1;
	//   bool is_monotonic = 3;
	// }

	isMonotonic, _, err := easyproto.GetBool(src, 3)
	if err != nil {
		return fmt.Errorf("cannot obtain isMonotonic: %w", err)
	}
	if isMonotonic {
		dctx.mm.Type = prompb.MetricTypeCounter
	} else {
		dctx.mm.Type = prompb.MetricTypeGauge
	}

	dctxSnapshot := dctx.getSnapshot()

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
				return fmt.Errorf("cannot read NumberDataPoint data")
			}
			if err := dctx.decodeNumberDataPoint(data); err != nil {
				return fmt.Errorf("cannot unmarshal NumberDataPoint: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// Histogram represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L255
type Histogram struct {
	DataPoints []*HistogramDataPoint
}

func (h *Histogram) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range h.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (dctx *decoderContext) decodeHistogram(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L255
	//
	// message Histogram {
	//   repeated HistogramDataPoint data_points = 1;
	// }

	dctx.mm.Type = prompb.MetricTypeHistogram

	dctxSnapshot := dctx.getSnapshot()

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
				return fmt.Errorf("cannot read HistogramDataPoint")
			}
			if err := dctx.decodeHistogramDataPoint(data); err != nil {
				return fmt.Errorf("cannot unmarshal HistogramDataPoint: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// ExponentialHistogram represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L267
type ExponentialHistogram struct {
	DataPoints []*ExponentialHistogramDataPoint
}

func (h *ExponentialHistogram) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range h.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (dctx *decoderContext) decodeExponentialHistogram(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L267
	//
	// message ExponentialHistogram {
	//   repeated ExponentialHistogramDataPoint data_points = 1;
	// }

	dctx.mm.Type = prompb.MetricTypeHistogram

	dctxSnapshot := dctx.getSnapshot()

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
				return fmt.Errorf("cannot read ExponentialHistogramDataPoint")
			}
			if err := dctx.decodeExponentialHistogramDataPoint(data); err != nil {
				return fmt.Errorf("cannot unmarshal ExponentialHistogramDataPoint: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// Summary represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L286
type Summary struct {
	DataPoints []*SummaryDataPoint
}

func (s *Summary) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, dp := range s.DataPoints {
		dp.marshalProtobuf(mm.AppendMessage(1))
	}
}

func (dctx *decoderContext) decodeSummary(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L286
	//
	// message Summary {
	//   repeated SummaryDataPoint data_points = 1;
	// }

	dctx.mm.Type = prompb.MetricTypeSummary

	dctxSnapshot := dctx.getSnapshot()

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
				return fmt.Errorf("cannot read SummaryDataPoint")
			}
			if err := dctx.decodeSummaryDataPoint(data); err != nil {
				return fmt.Errorf("cannot unmarshal SummaryDataPoint: %w", err)
			}

			dctx.restoreFromSnapshot(dctxSnapshot)
		}
	}
	return nil
}

// HistogramDataPoint represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L434
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

func (dctx *decoderContext) decodeHistogramDataPoint(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L434
	//
	// message HistogramDataPoint {
	//   repeated KeyValue attributes = 9;
	//   fixed64 time_unix_nano = 3;
	//   fixed64 count = 4;
	//   optional double sum = 5;
	//   repeated fixed64 bucket_counts = 6;
	//   repeated double explicit_bounds = 7;
	//   uint32 flags = 10;
	// }

	hctx := getHistogramDataPointContext()
	defer putHistogramDataPointContext(hctx)

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}

		var ok bool
		switch fc.FieldNum {
		case 9:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attributes")
			}
			if err := decodeKeyValue(data, &dctx.ls, &dctx.fb, ""); err != nil {
				return fmt.Errorf("cannot unmarshal Attributes: %w", err)
			}
		case 3:
			hctx.timestamp, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
		case 4:
			hctx.count, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read Count")
			}
		case 5:
			hctx.sum, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Sum")
			}
			hctx.hasSum = true
		case 6:
			hctx.bucketCounts, ok = fc.UnpackFixed64s(hctx.bucketCounts)
			if !ok {
				return fmt.Errorf("cannot read BucketCounts")
			}
		case 7:
			hctx.explicitBounds, ok = fc.UnpackDoubles(hctx.explicitBounds)
			if !ok {
				return fmt.Errorf("cannot read ExplicitBounds")
			}
		case 10:
			hctx.flags, ok = fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
		}
	}

	hctx.pushSamples(dctx)

	return nil
}

type histogramDataPointContext struct {
	timestamp      uint64
	count          uint64
	sum            float64
	hasSum         bool
	bucketCounts   []uint64
	explicitBounds []float64
	flags          uint32
}

func (hctx *histogramDataPointContext) reset() {
	hctx.timestamp = 0
	hctx.count = 0
	hctx.sum = 0
	hctx.hasSum = false
	hctx.bucketCounts = hctx.bucketCounts[:0]
	hctx.explicitBounds = hctx.explicitBounds[:0]
	hctx.flags = 0
}

func (hctx *histogramDataPointContext) pushSamples(dctx *decoderContext) {
	if len(hctx.bucketCounts) == 0 {
		return
	}

	if len(hctx.bucketCounts) != len(hctx.explicitBounds)+1 {
		skippedSampleLogger.Warnf("opentelemetry: unexpected number of buckets for %s; got %d; want %d", &dctx.ls, len(hctx.bucketCounts), len(hctx.explicitBounds)+1)
		return
	}

	dctx.mp.PushSample(&dctx.mm, "_count", &dctx.ls, hctx.timestamp, float64(hctx.count), hctx.flags)

	if hctx.hasSum {
		dctx.mp.PushSample(&dctx.mm, "_sum", &dctx.ls, hctx.timestamp, float64(hctx.sum), hctx.flags)
	}

	dctx.ls.Add("le", "")
	leValueP := &dctx.ls.Labels[len(dctx.ls.Labels)-1].Value

	var cumulative uint64
	for index, bound := range hctx.explicitBounds {
		cumulative += hctx.bucketCounts[index]
		*leValueP = dctx.fb.formatFloat(bound)
		dctx.mp.PushSample(&dctx.mm, "_bucket", &dctx.ls, hctx.timestamp, float64(cumulative), hctx.flags)
	}
	cumulative += hctx.bucketCounts[len(hctx.bucketCounts)-1]
	*leValueP = "+Inf"
	dctx.mp.PushSample(&dctx.mm, "_bucket", &dctx.ls, hctx.timestamp, float64(cumulative), hctx.flags)
}

var skippedSampleLogger = logger.WithThrottler("otlp_skipped_sample", 5*time.Second)

func getHistogramDataPointContext() *histogramDataPointContext {
	v := hctxPool.Get()
	if v == nil {
		return &histogramDataPointContext{}
	}
	return v.(*histogramDataPointContext)
}

func putHistogramDataPointContext(hctx *histogramDataPointContext) {
	hctx.reset()
	hctxPool.Put(hctx)
}

var hctxPool sync.Pool

// ExponentialHistogramDataPoint represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L521
type ExponentialHistogramDataPoint struct {
	Attributes    []*KeyValue
	TimeUnixNano  uint64
	Count         uint64
	Sum           *float64
	Scale         int32
	ZeroCount     uint64
	Positive      *Buckets
	Flags         uint32
	Min           *float64
	Max           *float64
	ZeroThreshold float64
}

func (dp *ExponentialHistogramDataPoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, a := range dp.Attributes {
		a.marshalProtobuf(mm.AppendMessage(1))
	}
	mm.AppendFixed64(3, dp.TimeUnixNano)
	mm.AppendFixed64(4, dp.Count)
	if dp.Sum != nil {
		mm.AppendDouble(5, *dp.Sum)
	}
	mm.AppendSint32(6, dp.Scale)
	mm.AppendFixed64(7, dp.ZeroCount)
	if dp.Positive != nil {
		dp.Positive.marshalProtobuf(mm.AppendMessage(8))
	}
	mm.AppendUint32(10, dp.Flags)
	if dp.Min != nil {
		mm.AppendDouble(12, *dp.Min)
	}
	if dp.Max != nil {
		mm.AppendDouble(13, *dp.Max)
	}
	mm.AppendDouble(14, dp.ZeroThreshold)
}

func (dctx *decoderContext) decodeExponentialHistogramDataPoint(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L521
	//
	// message ExponentialHistogramDataPoint {
	//   repeated KeyValue attributes = 1;
	//   fixed64 time_unix_nano = 3;
	//   fixed64 count = 4;
	//   optional double sum = 5;
	//   sint32 scale = 6;
	//   fixed64 zero_count = 7;
	//   Buckets positive = 8;
	//   uint32 flags = 10;
	//   optional double min = 12;
	//   optional double max = 13;
	//   double zero_threshold = 14;
	// }

	ehctx := getExponentialHistogramDataPointContext()
	defer putExponentialHistogramDataPointContext(ehctx)

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attributes")
			}
			if err := decodeKeyValue(data, &dctx.ls, &dctx.fb, ""); err != nil {
				return fmt.Errorf("cannot unmarshal Attributes: %w", err)
			}
		case 3:
			ehctx.timestamp, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
		case 4:
			ehctx.count, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read Count")
			}
		case 5:
			ehctx.sum, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Sum")
			}
		case 6:
			ehctx.scale, ok = fc.Sint32()
			if !ok {
				return fmt.Errorf("cannot read Scale")
			}
		case 7:
			ehctx.zeroCount, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read ZeroCount")
			}
		case 8:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Positive buckets")
			}
			if err := ehctx.positive.decodeBuckets(data); err != nil {
				return fmt.Errorf("cannot unmarshal Positive: %w", err)
			}
		case 10:
			ehctx.flags, ok = fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
		case 12:
			ehctx.min, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Min")
			}
		case 13:
			ehctx.max, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Max")
			}
		case 14:
			ehctx.zeroThreshold, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read ZeroThreshold")
			}
		}
	}

	ehctx.pushSamples(dctx)

	return nil
}

type exponentialHistogramDataPointContext struct {
	timestamp     uint64
	count         uint64
	sum           float64
	scale         int32
	zeroCount     uint64
	positive      buckets
	flags         uint32
	min           float64
	max           float64
	zeroThreshold float64
}

func (ehctx *exponentialHistogramDataPointContext) reset() {
	ehctx.timestamp = 0
	ehctx.count = 0
	ehctx.sum = 0
	ehctx.scale = 0
	ehctx.zeroCount = 0
	ehctx.positive.reset()
	ehctx.flags = 0
	ehctx.min = 0
	ehctx.max = 0
	ehctx.zeroThreshold = 0
}

type buckets struct {
	offset       int32
	bucketCounts []uint64
}

func (b *buckets) reset() {
	b.offset = 0
	b.bucketCounts = b.bucketCounts[:0]
}

func (ehctx *exponentialHistogramDataPointContext) pushSamples(dctx *decoderContext) {
	dctx.mp.PushSample(&dctx.mm, "_count", &dctx.ls, ehctx.timestamp, float64(ehctx.count), ehctx.flags)
	dctx.mp.PushSample(&dctx.mm, "_sum", &dctx.ls, ehctx.timestamp, float64(ehctx.sum), ehctx.flags)

	dctx.ls.Add("vmrange", "")
	vmrangeValueP := &dctx.ls.Labels[len(dctx.ls.Labels)-1].Value

	if ehctx.zeroCount > 0 {
		*vmrangeValueP = dctx.fb.formatVmrange(0.0, ehctx.zeroThreshold)
		dctx.mp.PushSample(&dctx.mm, "_bucket", &dctx.ls, ehctx.timestamp, float64(ehctx.zeroCount), ehctx.flags)
	}

	ratio := math.Pow(2, -float64(ehctx.scale))
	base := math.Pow(2, ratio)
	bound := math.Pow(2, float64(ehctx.positive.offset)*ratio)
	for i, count := range ehctx.positive.bucketCounts {
		if count <= 0 {
			continue
		}

		lowerBound := bound * math.Pow(base, float64(i))
		upperBound := lowerBound * base
		*vmrangeValueP = dctx.fb.formatVmrange(lowerBound, upperBound)
		dctx.mp.PushSample(&dctx.mm, "_bucket", &dctx.ls, ehctx.timestamp, float64(count), ehctx.flags)
	}
}

func getExponentialHistogramDataPointContext() *exponentialHistogramDataPointContext {
	v := ehctxPool.Get()
	if v == nil {
		return &exponentialHistogramDataPointContext{}
	}
	return v.(*exponentialHistogramDataPointContext)
}

func putExponentialHistogramDataPointContext(ehctx *exponentialHistogramDataPointContext) {
	ehctx.reset()
	ehctxPool.Put(ehctx)
}

var ehctxPool sync.Pool

// Buckets represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L592
type Buckets struct {
	Offset       int32
	BucketCounts []uint64
}

func (b *Buckets) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendSint32(1, b.Offset)
	for _, bc := range b.BucketCounts {
		mm.AppendUint64(2, bc)
	}
}

func (b *buckets) decodeBuckets(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L592
	//
	// message Buckets {
	//   sint32 offset = 1;
	//   repeated uint64 bucket_counts = 2;
	// }

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 1:
			b.offset, ok = fc.Sint32()
			if !ok {
				return fmt.Errorf("cannot read Offset")
			}
		case 2:
			b.bucketCounts, ok = fc.UnpackUint64s(b.bucketCounts)
			if !ok {
				return fmt.Errorf("cannot read BucketCounts")
			}
		}
	}
	return nil
}

// SummaryDataPoint represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L636
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

func (dctx *decoderContext) decodeSummaryDataPoint(src []byte) (err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L636
	//
	// message SummaryDataPoint {
	//   repeated KeyValue attributes = 7;
	//   fixed64 time_unix_nano = 3;
	//   fixed64 count = 4;
	//   double sum = 5;
	//   repeated ValueAtQuantile quantile_values = 6;
	//   uint32 flags = 8;
	// }

	sctx := getSummaryDataPointContext()
	defer putSummaryDataPointContext(sctx)

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Attributes")
			}
			if err := decodeKeyValue(data, &dctx.ls, &dctx.fb, ""); err != nil {
				return fmt.Errorf("cannot unmarshal Attributes: %w", err)
			}
		case 3:
			sctx.timestamp, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read TimeUnixNano")
			}
		case 4:
			sctx.count, ok = fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read Count")
			}
		case 5:
			sctx.sum, ok = fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Sum")
			}
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ValueAtQuantile")
			}
			quantile, value, err := decodeValueAtQuantile(data)
			if err != nil {
				return fmt.Errorf("cannot unmarshal ValueAtQuantile: %w", err)
			}
			sctx.quantileValues = append(sctx.quantileValues, quantileValue{
				quantile: quantile,
				value:    value,
			})
		case 8:
			sctx.flags, ok = fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read Flags")
			}
		}
	}

	sctx.pushSamples(dctx)

	return nil
}

type summaryDataPointContext struct {
	timestamp      uint64
	count          uint64
	sum            float64
	quantileValues []quantileValue
	flags          uint32
}

type quantileValue struct {
	quantile float64
	value    float64
}

func (sctx *summaryDataPointContext) reset() {
	sctx.timestamp = 0
	sctx.count = 0
	sctx.sum = 0
	sctx.quantileValues = sctx.quantileValues[:0]
	sctx.flags = 0
}

func getSummaryDataPointContext() *summaryDataPointContext {
	v := sctxPool.Get()
	if v == nil {
		return &summaryDataPointContext{}
	}
	return v.(*summaryDataPointContext)
}

func putSummaryDataPointContext(sctx *summaryDataPointContext) {
	sctx.reset()
	sctxPool.Put(sctx)
}

var sctxPool sync.Pool

func (sctx *summaryDataPointContext) pushSamples(dctx *decoderContext) {
	dctx.mp.PushSample(&dctx.mm, "_count", &dctx.ls, sctx.timestamp, float64(sctx.count), sctx.flags)
	dctx.mp.PushSample(&dctx.mm, "_sum", &dctx.ls, sctx.timestamp, sctx.sum, sctx.flags)

	dctx.ls.Add("quantile", "")
	quantileValueP := &dctx.ls.Labels[len(dctx.ls.Labels)-1].Value

	for _, qv := range sctx.quantileValues {
		*quantileValueP = dctx.fb.formatFloat(qv.quantile)
		dctx.mp.PushSample(&dctx.mm, "", &dctx.ls, sctx.timestamp, qv.value, sctx.flags)
	}
}

// ValueAtQuantile represents the corresponding OTEL protobuf message
//
// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L680
type ValueAtQuantile struct {
	Quantile float64
	Value    float64
}

func (v *ValueAtQuantile) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendDouble(1, v.Quantile)
	mm.AppendDouble(2, v.Value)
}

func decodeValueAtQuantile(src []byte) (quantile float64, value float64, err error) {
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L680
	//
	// message ValueAtQuantile {
	//   double quantile = 1;
	//   double value = 2;
	// }

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot read the next field: %w", err)
		}
		var ok bool
		switch fc.FieldNum {
		case 1:
			quantile, ok = fc.Double()
			if !ok {
				return 0, 0, fmt.Errorf("cannot read Quantile")
			}
		case 2:
			value, ok = fc.Double()
			if !ok {
				return 0, 0, fmt.Errorf("cannot read Value")
			}
		}
	}
	return quantile, value, nil
}

type decoderContext struct {
	ls promutil.Labels
	fb fmtBuffer

	mm MetricMetadata

	mp MetricPusher
}

func (dctx *decoderContext) reset() {
	// Explicitly clear all the ls.Labels up to its' capacity in order to remove possible references
	// to the original byte slices, so they could be cleared by Go GC.
	clear(dctx.ls.Labels[:cap(dctx.ls.Labels)])
	dctx.ls.Labels = dctx.ls.Labels[:0]

	dctx.ls.Reset()
	dctx.fb.reset()

	dctx.mm.reset()

	dctx.mp = nil
}

func (dctx *decoderContext) getSnapshot() decoderContextSnapshot {
	return decoderContextSnapshot{
		labelsLen: len(dctx.ls.Labels),
		fbLen:     len(dctx.fb.buf),
	}
}

func (dctx *decoderContext) restoreFromSnapshot(snapshot decoderContextSnapshot) {
	dctx.ls.Labels = dctx.ls.Labels[:snapshot.labelsLen]
	dctx.fb.buf = dctx.fb.buf[:snapshot.fbLen]
}

type decoderContextSnapshot struct {
	labelsLen int
	fbLen     int
}

func getDecoderContext(mp MetricPusher) *decoderContext {
	v := dctxPool.Get()
	if v == nil {
		v = &decoderContext{}
	}
	dctx := v.(*decoderContext)
	dctx.mp = mp

	return dctx
}

func putDecoderContext(dctx *decoderContext) {
	dctx.reset()
	dctxPool.Put(dctx)
}

var dctxPool sync.Pool
