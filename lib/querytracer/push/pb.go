package push

// This file contains marshal-only protobuf types for OTLP trace export.
// Copied and trimmed from VictoriaTraces/lib/protoparser/opentelemetry/pb/
// Only marshaling is needed since this package only pushes traces outward.

import (
	"encoding/hex"

	"github.com/VictoriaMetrics/easyproto"
)

var mp easyproto.MarshalerPool

// exportTraceServiceRequest is the top-level OTLP protobuf message.
type exportTraceServiceRequest struct {
	ResourceSpans []*resourceSpans
}

func (r *exportTraceServiceRequest) marshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	mm := m.MessageMarshaler()
	for _, rs := range r.ResourceSpans {
		rs.marshalProtobuf(mm.AppendMessage(1))
	}
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

// resourceSpans groups spans from a single resource (e.g. service instance).
type resourceSpans struct {
	Resource   resource
	ScopeSpans []*scopeSpans
}

func (rs *resourceSpans) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	rs.Resource.marshalProtobuf(mm.AppendMessage(1))
	for _, ss := range rs.ScopeSpans {
		ss.marshalProtobuf(mm.AppendMessage(2))
	}
}

// resource holds resource-level attributes (service.name, etc.).
type resource struct {
	Attributes []*keyValue
}

func (r *resource) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, a := range r.Attributes {
		a.marshalProtobuf(mm.AppendMessage(1))
	}
}

// scopeSpans groups spans from a single instrumentation scope.
type scopeSpans struct {
	Scope instrumentationScope
	Spans []*span
}

func (ss *scopeSpans) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	ss.Scope.marshalProtobuf(mm.AppendMessage(1))
	for _, s := range ss.Spans {
		s.marshalProtobuf(mm.AppendMessage(2))
	}
}

// instrumentationScope identifies the library that produced the spans.
type instrumentationScope struct {
	Name    string
	Version string
}

func (is *instrumentationScope) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, is.Name)
	mm.AppendString(2, is.Version)
}

// span represents a single operation within a trace.
type span struct {
	// TraceID is a 32-char lowercase hex string (16 bytes).
	TraceID string
	// SpanID is a 16-char lowercase hex string (8 bytes).
	SpanID string
	// ParentSpanID is a 16-char lowercase hex string; empty for root spans.
	ParentSpanID      string
	Name              string
	StartTimeUnixNano uint64
	EndTimeUnixNano   uint64
	Attributes        []*keyValue
	Events            []*spanEvent
}

func (s *span) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	traceID, err := hex.DecodeString(s.TraceID)
	if err != nil {
		traceID = []byte(s.TraceID)
	}
	mm.AppendBytes(1, traceID)

	spanID, err := hex.DecodeString(s.SpanID)
	if err != nil {
		spanID = []byte(s.SpanID)
	}
	mm.AppendBytes(2, spanID)

	// field 3: trace_state — omitted

	parentSpanID, err := hex.DecodeString(s.ParentSpanID)
	if err != nil {
		parentSpanID = []byte(s.ParentSpanID)
	}
	mm.AppendBytes(4, parentSpanID)

	mm.AppendString(5, s.Name)
	// field 6: kind — omitted (INTERNAL=1 is default)
	mm.AppendFixed64(7, s.StartTimeUnixNano)
	mm.AppendFixed64(8, s.EndTimeUnixNano)
	for _, a := range s.Attributes {
		a.marshalProtobuf(mm.AppendMessage(9))
	}
	for _, e := range s.Events {
		e.marshalProtobuf(mm.AppendMessage(11))
	}
}

// spanEvent is a time-stamped annotation within a span.
type spanEvent struct {
	TimeUnixNano uint64
	Name         string
}

func (se *spanEvent) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendFixed64(1, se.TimeUnixNano)
	mm.AppendString(2, se.Name)
}

// keyValue is an OTLP attribute key-value pair.
type keyValue struct {
	Key   string
	Value anyValue
}

func (kv *keyValue) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, kv.Key)
	kv.Value.marshalProtobuf(mm.AppendMessage(2))
}

// anyValue holds a single string attribute value (sufficient for our use case).
type anyValue struct {
	StringValue string
}

func (av *anyValue) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, av.StringValue)
}
