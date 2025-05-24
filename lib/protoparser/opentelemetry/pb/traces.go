package pb

import (
	"encoding/hex"
	"fmt"
	"github.com/VictoriaMetrics/easyproto"
	"strings"
)

type (
	Span_SpanKind     int32
	Status_StatusCode int32
)

// ExportTraceServiceRequest represent the OTLP protobuf message
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/collector/trace/v1/trace_service.proto#L36
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/collector/trace/v1/trace_service.pb.go#L33
type ExportTraceServiceRequest struct {
	ResourceSpans []*ResourceSpans
}

// MarshalProtobuf marshals r to protobuf message, appends it to dst and returns the result.
func (r *ExportTraceServiceRequest) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	r.marshalProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

func (r *ExportTraceServiceRequest) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message ExportTraceServiceRequest {
	//	repeated opentelemetry.proto.trace.v1.ResourceSpans resource_spans = 1;
	//}
	for _, rs := range r.ResourceSpans {
		rs.marshalProtobuf(mm.AppendMessage(1))
	}
}

// UnmarshalProtobuf unmarshals r from protobuf message at src.
func (r *ExportTraceServiceRequest) UnmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ExportTraceServiceRequest: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read resource spans data")
			}
			r.ResourceSpans = append(r.ResourceSpans, &ResourceSpans{})
			a := r.ResourceSpans[len(r.ResourceSpans)-1]
			if err = a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal resource span: %w", err)
			}
		}
	}
	return nil
}

// ResourceSpans represent a collection of ScopeSpans from a Resource.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/trace/v1/trace.proto#L48
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/trace/v1/trace.pb.go#L230
type ResourceSpans struct {
	Resource   Resource
	ScopeSpans []*ScopeSpans
	SchemaURL  string
}

func (rs *ResourceSpans) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message ResourceSpans {
	//	opentelemetry.proto.resource.v1.Resource resource = 1;
	//	repeated ScopeSpans scope_spans = 2;
	//	string schema_url = 3;
	//}
	rs.Resource.marshalProtobuf(mm.AppendMessage(1))
	for _, ss := range rs.ScopeSpans {
		ss.marshalProtobuf(mm.AppendMessage(2))
	}
	mm.AppendString(3, rs.SchemaURL)
}

func (rs *ResourceSpans) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read resource span resource data")
			}
			if err = rs.Resource.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal resource span resource: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read resource span scope span data")
			}
			rs.ScopeSpans = append(rs.ScopeSpans, &ScopeSpans{})
			a := rs.ScopeSpans[len(rs.ScopeSpans)-1]
			if err = a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal resource span scope span: %w", err)
			}
		case 3:
			schemaURL, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read resource span schema url")
			}
			rs.SchemaURL = strings.Clone(schemaURL)
		}
	}
	return nil
}

// ScopeSpans represent a collection of Spans produced by an InstrumentationScope.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/trace/v1/trace.proto#L68
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/trace/v1/trace.pb.go#L308
type ScopeSpans struct {
	Scope     InstrumentationScope
	Spans     []*Span
	SchemaURL string
}

func (ss *ScopeSpans) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message ScopeSpans {
	//	opentelemetry.proto.common.v1.InstrumentationScope scope = 1;
	//	repeated Span spans = 2;
	//	string schema_url = 3;
	//}
	ss.Scope.marshalProtobuf(mm.AppendMessage(1))
	for _, span := range ss.Spans {
		span.marshalProtobuf(mm.AppendMessage(2))
	}
	mm.AppendString(3, ss.SchemaURL)
}

func (ss *ScopeSpans) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read scope span scope data")
			}
			if err = ss.Scope.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal scope span scope: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read scope span span data")
			}
			ss.Spans = append(ss.Spans, &Span{})
			a := ss.Spans[len(ss.Spans)-1]
			if err = a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal scope span span: %w", err)
			}
		case 3:
			schemaURL, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read scope span schema url")
			}
			ss.SchemaURL = strings.Clone(schemaURL)
		}
	}
	return nil
}

// InstrumentationScope is a message representing the instrumentation scope information
// such as the fully qualified name and version.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/common/v1/common.proto#L71
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/common/v1/common.pb.go#L340
type InstrumentationScope struct {
	Name                   string
	Version                string
	Attributes             []*KeyValue
	DroppedAttributesCount uint32
}

func (is *InstrumentationScope) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message InstrumentationScope {
	//	string name = 1;
	//	string version = 2;
	//	repeated KeyValue attributes = 3;
	//	uint32 dropped_attributes_count = 4;
	//}
	mm.AppendString(1, is.Name)
	mm.AppendString(2, is.Version)
	for _, kv := range is.Attributes {
		kv.marshalProtobuf(mm.AppendMessage(3))
	}
	mm.AppendUint32(4, is.DroppedAttributesCount)
}

func (is *InstrumentationScope) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read scope name")
			}
			is.Name = strings.Clone(name)
		case 2:
			version, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read scope version")
			}
			is.Version = strings.Clone(version)
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read scope attributes data")
			}
			is.Attributes = append(is.Attributes, &KeyValue{})
			a := is.Attributes[len(is.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal scope attribute: %w", err)
			}
		case 4:
			droppedAttributesCount, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read scope dropped attributes count")
			}
			is.DroppedAttributesCount = droppedAttributesCount
		}
	}
	return nil
}

// Span represents a single operation performed by a single component of the system.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/trace/v1/trace.proto#L88
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/trace/v1/trace.pb.go#L380
type Span struct {
	TraceId                string
	SpanId                 string
	TraceState             string
	ParentSpanId           string
	Flags                  uint32
	Name                   string
	Kind                   Span_SpanKind
	StartTimeUnixNano      uint64
	EndTimeUnixNano        uint64
	Attributes             []*KeyValue
	DroppedAttributesCount uint32
	Events                 []*Span_Event
	DroppedEventsCount     uint32
	Links                  []*Span_Link
	DroppedLinksCount      uint32
	Status                 Status
}

func (s *Span) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message Span {
	//	bytes trace_id = 1;
	//	bytes span_id = 2;
	//	string trace_state = 3;
	//	bytes parent_span_id = 4;
	//	string name = 5;
	//	SpanKind kind = 6;
	//	fixed64 start_time_unix_nano = 7;
	//	fixed64 end_time_unix_nano = 8;
	//	repeated opentelemetry.proto.common.v1.KeyValue attributes = 9;
	//	uint32 dropped_attributes_count = 10;
	//	repeated Event events = 11;
	//	uint32 dropped_events_count = 12;
	//	repeated Link links = 13;
	//	uint32 dropped_links_count = 14;
	//	Status status = 15;
	//}
	traceID, err := hex.DecodeString(s.TraceId)
	if err != nil {
		traceID = []byte(s.TraceId)
	}
	mm.AppendBytes(1, traceID)

	spanID, err := hex.DecodeString(s.SpanId)
	if err != nil {
		spanID = []byte(s.SpanId)
	}
	mm.AppendBytes(2, spanID)

	mm.AppendString(3, s.TraceState)

	parentSpanID, err := hex.DecodeString(s.ParentSpanId)
	if err != nil {
		parentSpanID = []byte(s.ParentSpanId)
	}
	mm.AppendBytes(4, parentSpanID)

	mm.AppendString(5, s.Name)
	mm.AppendUint32(6, uint32(s.Kind))
	mm.AppendFixed64(7, s.StartTimeUnixNano)
	mm.AppendFixed64(8, s.EndTimeUnixNano)
	for _, a := range s.Attributes {
		a.marshalProtobuf(mm.AppendMessage(9))
	}
	mm.AppendUint32(10, s.DroppedAttributesCount)
	for _, e := range s.Events {
		e.marshalProtobuf(mm.AppendMessage(11))
	}
	mm.AppendUint32(12, s.DroppedEventsCount)
	for _, e := range s.Links {
		e.marshalProtobuf(mm.AppendMessage(13))
	}
	mm.AppendUint32(14, s.DroppedLinksCount)
	s.Status.marshalProtobuf(mm.AppendMessage(15))
}

func (s *Span) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			traceID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read span trace id")
			}
			s.TraceId = hex.EncodeToString(traceID)
		case 2:
			spanID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read span span id")
			}
			s.SpanId = hex.EncodeToString(spanID)
		case 3:
			traceState, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read span trace state")
			}
			s.TraceState = strings.Clone(traceState)
		case 4:
			parentSpanID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read span parent span id")
			}
			s.ParentSpanId = hex.EncodeToString(parentSpanID)
		case 5:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read span name")
			}
			s.Name = strings.Clone(name)
		case 6:
			kind, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read span kind")
			}
			s.Kind = Span_SpanKind(kind)
		case 7:
			startTimeUnixNano, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read span start timestamp")
			}
			s.StartTimeUnixNano = startTimeUnixNano
		case 8:
			endTimeUnixNano, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read span end timestamp")
			}
			s.EndTimeUnixNano = endTimeUnixNano
		case 9:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read span attributes data")
			}
			s.Attributes = append(s.Attributes, &KeyValue{})
			a := s.Attributes[len(s.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal span attribute: %w", err)
			}
		case 10:
			droppedAttributesCount, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read span dropped attributes count")
			}
			s.DroppedAttributesCount = droppedAttributesCount
		case 11:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read span event data")
			}
			s.Events = append(s.Events, &Span_Event{})
			a := s.Events[len(s.Events)-1]
			if err = a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal span event: %w", err)
			}
		case 12:
			droppedEventsCount, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read span dropped events count")
			}
			s.DroppedEventsCount = droppedEventsCount

		case 13:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read span links data")
			}
			s.Links = append(s.Links, &Span_Link{})
			a := s.Links[len(s.Links)-1]
			if err = a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal span link: %w", err)
			}
		case 14:
			droppedLinksCount, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read span dropped links count")
			}
			s.DroppedLinksCount = droppedLinksCount
		case 15:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read span status data")
			}
			if err = s.Status.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal span status: %w", err)
			}
		}
	}
	return nil
}

// Span_Event is a time-stamped annotation of the span, consisting of user-supplied
// text description and key-value pairs.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/trace/v1/trace.proto#L222
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/trace/v1/trace.pb.go#L613
type Span_Event struct {
	TimeUnixNano           uint64
	Name                   string
	Attributes             []*KeyValue
	DroppedAttributesCount uint32
}

func (se *Span_Event) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message Event {
	//	fixed64 time_unix_nano = 1;
	//	string name = 2;
	//	repeated opentelemetry.proto.common.v1.KeyValue attributes = 3;
	//	uint32 dropped_attributes_count = 4;
	//}
	mm.AppendFixed64(1, se.TimeUnixNano)
	mm.AppendString(2, se.Name)
	for _, a := range se.Attributes {
		a.marshalProtobuf(mm.AppendMessage(3))
	}
	mm.AppendUint32(4, se.DroppedAttributesCount)
}

func (se *Span_Event) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ts, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read span event timestamp")
			}
			se.TimeUnixNano = ts
		case 2:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read span event name")
			}
			se.Name = strings.Clone(name)
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read span event attributes data")
			}
			se.Attributes = append(se.Attributes, &KeyValue{})
			a := se.Attributes[len(se.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal span event attribute: %w", err)
			}
		case 4:
			droppedAttributesCount, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read span event dropped attributes count")
			}
			se.DroppedAttributesCount = droppedAttributesCount
		}
	}
	return nil
}

// Span_Link is a pointer from the current span to another span in the same trace or in a
// different trace. For example, this can be used in batching operations,
// where a single batch handler processes multiple requests from different
// traces or when the handler receives a request from a different project.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/trace/v1/trace.proto#L251
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/trace/v1/trace.pb.go#L693
type Span_Link struct {
	TraceId                string
	SpanId                 string
	TraceState             string
	Attributes             []*KeyValue
	DroppedAttributesCount uint32
	Flags                  uint32
}

func (sl *Span_Link) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message Link {
	//	bytes trace_id = 1;
	//	bytes span_id = 2;
	//	string trace_state = 3;
	//	repeated opentelemetry.proto.common.v1.KeyValue attributes = 4;
	//	uint32 dropped_attributes_count = 5;
	//	fixed32 flags = 6;
	//}
	traceID, err := hex.DecodeString(sl.TraceId)
	if err != nil {
		traceID = []byte(sl.TraceId)
	}
	mm.AppendBytes(1, traceID)

	spanID, err := hex.DecodeString(sl.SpanId)
	if err != nil {
		spanID = []byte(sl.SpanId)
	}
	mm.AppendBytes(2, spanID)

	mm.AppendString(3, sl.TraceState)

	for _, a := range sl.Attributes {
		a.marshalProtobuf(mm.AppendMessage(4))
	}
	mm.AppendUint32(5, sl.DroppedAttributesCount)
	mm.AppendFixed32(6, sl.Flags)
}

func (sl *Span_Link) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			traceID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read span link trace id")
			}
			sl.TraceId = hex.EncodeToString(traceID)
		case 2:
			spanID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read span link span id")
			}
			sl.SpanId = hex.EncodeToString(spanID)
		case 3:
			traceState, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read span link trace state")
			}
			sl.TraceState = strings.Clone(traceState)
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read aspan link ttributes data")
			}
			sl.Attributes = append(sl.Attributes, &KeyValue{})
			a := sl.Attributes[len(sl.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal span link attribute: %w", err)
			}
		case 5:
			droppedAttributesCount, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read span link dropped attributes count")
			}
			sl.DroppedAttributesCount = droppedAttributesCount
		case 6:
			flags, ok := fc.Fixed32()
			if !ok {
				return fmt.Errorf("cannot read span link flags")
			}
			sl.Flags = flags
		}
	}
	return nil
}

// The Status type defines a logical error model that is suitable for different
// programming environments, including REST APIs and RPC APIs.
//
// https://github.com/open-telemetry/opentelemetry-proto/blob/v1.5.0/opentelemetry/proto/trace/v1/trace.proto#L306
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.124.0/pdata/internal/data/protogen/trace/v1/trace.pb.go#L791
type Status struct {
	Message string
	Code    Status_StatusCode
}

func (s *Status) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	//message Status {
	//	reserved 1;
	//	string message = 2;
	//	StatusCode code = 3;
	//}
	mm.AppendString(2, s.Message)
	mm.AppendInt32(3, int32(s.Code))
}

func (s *Status) unmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Status: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			message, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read status message")
			}
			s.Message = strings.Clone(message)
		case 3:
			code, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read status code")
			}
			s.Code = Status_StatusCode(code)
		}
	}
	return nil
}
