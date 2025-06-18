package jaeger

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/google/go-cmp/cmp"
)

func TestFieldsToSpan(t *testing.T) {
	f := func(name string, input []logstorage.Field, want *span, wantErr bool) {
		t.Helper()
		got, err := fieldsToSpan(input)
		if (err != nil) != wantErr {
			t.Fatalf("fieldsToSpan() error = %v", err)
		}
		cmpOpts := cmp.AllowUnexported(span{}, process{}, spanRef{}, keyValue{}, log{})
		if !cmp.Equal(got, want, cmpOpts) {
			t.Fatalf("%s fieldsToSpan() diff = %v", name, cmp.Diff(got, want, cmpOpts))
		}
	}

	// case 1: empty
	f("empty fields", []logstorage.Field{}, nil, true)

	// case 2: without span_id
	fields := []logstorage.Field{
		{Name: pb.TraceIDField, Value: "1234567890"},
	}
	f("without span_id", fields, nil, true)

	// case 3: without trace_id
	fields = []logstorage.Field{
		{Name: pb.SpanIDField, Value: "12345"},
	}
	f("without trace_id", fields, nil, true)

	// case 4: with basic fields
	fields = []logstorage.Field{
		{Name: pb.TraceIDField, Value: "1234567890"},
		{Name: pb.SpanIDField, Value: "12345"},
	}
	sp := &span{
		traceID: "1234567890", spanID: "12345",
	}
	f("with basic fields", fields, sp, false)

	// case 5: with all fields
	// see: lib/protoparser/opentelemetry/pb/trace_fields.go
	fields = []logstorage.Field{
		{Name: pb.ResourceAttrServiceName, Value: "service_name_1"},
		{Name: pb.ResourceAttrPrefix + "resource_attr_1", Value: "resource_attr_1"},
		{Name: pb.ResourceAttrPrefix + "resource_attr_2", Value: "resource_attr_2"},

		{Name: pb.InstrumentationScopeName, Value: "scope_name_1"},
		{Name: pb.InstrumentationScopeVersion, Value: "scope_version_1"},
		{Name: pb.InstrumentationScopeAttrPrefix + "scope_attr_1", Value: "scope_attr_1"},
		{Name: pb.InstrumentationScopeAttrPrefix + "scope_attr_2", Value: "scope_attr_2"},

		{Name: pb.TraceIDField, Value: "1234567890"},
		{Name: pb.SpanIDField, Value: "12345"},
		{Name: pb.TraceStateField, Value: "trace_state_1"},
		{Name: pb.ParentSpanIDField, Value: "23456"},
		{Name: pb.FlagsField, Value: "0"},
		{Name: pb.NameField, Value: "span_name_1"},
		{Name: pb.KindField, Value: "1"},
		{Name: pb.StartTimeUnixNanoField, Value: "0"},
		{Name: pb.EndTimeUnixNanoField, Value: "123456789"},
		{Name: pb.SpanAttrPrefixField + "attr_1", Value: "attr_1"},
		{Name: pb.SpanAttrPrefixField + "attr_2", Value: "attr_2"},
		{Name: pb.DurationField, Value: "123456789"},

		{Name: pb.EventPrefix + "0:" + pb.EventTimeUnixNanoField, Value: "0"},
		{Name: pb.EventPrefix + "0:" + pb.EventNameField, Value: "event_0"},
		{Name: pb.EventPrefix + "0:" + pb.EventAttrPrefix + "event_attr_1", Value: "event_0_attr_1"},
		{Name: pb.EventPrefix + "0:" + pb.EventAttrPrefix + "event_attr_2", Value: "event_0_attr_2"},

		{Name: pb.EventPrefix + "1:" + pb.EventTimeUnixNanoField, Value: "1"},
		{Name: pb.EventPrefix + "1:" + pb.EventNameField, Value: "event_1"},
		{Name: pb.EventPrefix + "1:" + pb.EventAttrPrefix + "event_attr_1", Value: "event_1_attr_1"},
		{Name: pb.EventPrefix + "1:" + pb.EventAttrPrefix + "event_attr_2", Value: "event_1_attr_2"},

		{Name: pb.LinkPrefix + "0:" + pb.LinkTraceIDField, Value: "1234567890"},
		{Name: pb.LinkPrefix + "0:" + pb.LinkSpanIDField, Value: "23456"},
		{Name: pb.LinkPrefix + "0:" + pb.LinkTraceStateField, Value: "link_0_trace_state_1"},
		{Name: pb.LinkPrefix + "0:" + pb.LinkAttrPrefix + "link_attr_1", Value: "link_0_trace_attr_1"},
		{Name: pb.LinkPrefix + "0:" + pb.LinkAttrPrefix + "link_attr_2", Value: "link_0_trace_attr_2"},
		{Name: pb.LinkPrefix + "0:" + pb.LinkAttrPrefix + "opentracing.ref_type", Value: "child_of"},
		{Name: pb.LinkPrefix + "0:" + pb.LinkFlagsField, Value: "0"},
		{Name: pb.LinkPrefix + "1:" + pb.LinkTraceIDField, Value: "99999999999"},
		{Name: pb.LinkPrefix + "1:" + pb.LinkSpanIDField, Value: "98765"},
		{Name: pb.LinkPrefix + "1:" + pb.LinkTraceStateField, Value: "link_1_trace_state_1"},
		{Name: pb.LinkPrefix + "1:" + pb.LinkAttrPrefix + "link_attr_1", Value: "link_1_trace_attr_1"},
		{Name: pb.LinkPrefix + "1:" + pb.LinkAttrPrefix + "link_attr_2", Value: "link_1_trace_attr_2"},
		{Name: pb.LinkPrefix + "1:" + pb.LinkFlagsField, Value: "1"},

		{Name: pb.StatusMessageField, Value: "status_message_1"},
		{Name: pb.StatusCodeField, Value: "2"},
	}

	sp = &span{
		traceID:       "1234567890",
		spanID:        "12345",
		operationName: "span_name_1",
		references: []spanRef{
			{
				traceID: "1234567890",
				spanID:  "23456",
				refType: "CHILD_OF",
			},
			{
				traceID: "99999999999",
				spanID:  "98765",
				refType: "FOLLOW_FROM",
			},
		},
		startTime: 0,
		duration:  123456,
		tags: []keyValue{
			{"otel.scope.name", "scope_name_1"},
			{"otel.scope.version", "scope_version_1"},
			{"scope_attr:scope_attr_1", "scope_attr_1"},
			{"scope_attr:scope_attr_2", "scope_attr_2"},
			{"w3c.tracestate", "trace_state_1"},
			{"span.kind", "internal"},
			{"attr_1", "attr_1"},
			{"attr_2", "attr_2"},
			{"otel.status_description", "status_message_1"},
			{"error", "true"},
		},
		logs: []log{
			{
				timestamp: 0,
				fields: []keyValue{
					{"event", "event_0"},
					{"event_attr_1", "event_0_attr_1"},
					{"event_attr_2", "event_0_attr_2"},
				},
			},
			{
				timestamp: 0,
				fields: []keyValue{
					{"event", "event_1"},
					{"event_attr_1", "event_1_attr_1"},
					{"event_attr_2", "event_1_attr_2"},
				},
			},
		},
		process: process{
			serviceName: "service_name_1",
			tags: []keyValue{
				{"resource_attr_1", "resource_attr_1"},
				{"resource_attr_2", "resource_attr_2"},
			},
		},
	}
	f("with with all fields", fields, sp, false)

}
