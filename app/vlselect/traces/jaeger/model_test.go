package jaeger

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/google/go-cmp/cmp"
)

func Test_fieldsToSpan(t *testing.T) {
	f := func(name string, input []logstorage.Field, want *span, wantErr bool) {
		t.Helper()
		got, err := fieldsToSpan(input)
		if (err != nil) != wantErr {
			t.Fatalf("fieldsToSpan() error = %v", err)
		}
		cmpOpts := cmp.AllowUnexported(span{}, process{}, spanRef{}, keyValue{}, log{})
		if !cmp.Equal(got, want, cmpOpts) {
			t.Fatalf("fieldsToSpan() diff = %v", cmp.Diff(got, want, cmpOpts))
		}
	}

	// case 1: empty
	f("empty fields", []logstorage.Field{}, nil, true)

	// case 2: without span_id
	fields := []logstorage.Field{
		{pb.TraceIDField, "1234567890"},
	}
	f("without span_id", fields, nil, true)

	// case 3: without trace_id
	fields = []logstorage.Field{
		{pb.SpanIDField, "12345"},
	}
	f("without trace_id", fields, nil, true)

	// case 4: with basic fields
	fields = []logstorage.Field{
		{pb.TraceIDField, "1234567890"},
		{pb.SpanIDField, "12345"},
	}
	sp := &span{
		traceID: "1234567890", spanID: "12345",
	}
	f("with basic fields", fields, sp, false)

	// case 5: with all fields
	// see: lib/protoparser/opentelemetry/pb/trace_fields.go
	fields = []logstorage.Field{
		{pb.ResourceAttrServiceName, "service_name_1"},
		{pb.ResourceAttrPrefix + "resource_attr_1", "resource_attr_1"},
		{pb.ResourceAttrPrefix + "resource_attr_2", "resource_attr_2"},

		{pb.InstrumentationScopeName, "scope_name_1"},
		{pb.InstrumentationScopeVersion, "scope_version_1"},
		{pb.InstrumentationScopeAttrPrefix + "scope_attr_1", "scope_attr_1"},
		{pb.InstrumentationScopeAttrPrefix + "scope_attr_2", "scope_attr_2"},

		{pb.TraceIDField, "1234567890"},
		{pb.SpanIDField, "12345"},
		{pb.TraceStateField, "trace_state_1"},
		{pb.ParentSpanIDField, "23456"},
		{pb.FlagsField, "0"},
		{pb.NameField, "span_name_1"},
		{pb.KindField, "1"},
		{pb.StartTimeUnixNanoField, "0"},
		{pb.EndTimeUnixNanoField, "123456789"},
		{pb.SpanAttrPrefixField + "attr_1", "attr_1"},
		{pb.SpanAttrPrefixField + "attr_2", "attr_2"},
		{pb.DurationField, "123456789"},

		{pb.EventPrefix + "0:" + pb.EventTimeUnixNanoField, "0"},
		{pb.EventPrefix + "0:" + pb.EventNameField, "event_0"},
		{pb.EventPrefix + "0:" + pb.EventAttrPrefix + "event_attr_1", "event_0_attr_1"},
		{pb.EventPrefix + "0:" + pb.EventAttrPrefix + "event_attr_2", "event_0_attr_2"},

		{pb.EventPrefix + "1:" + pb.EventTimeUnixNanoField, "1"},
		{pb.EventPrefix + "1:" + pb.EventNameField, "event_1"},
		{pb.EventPrefix + "1:" + pb.EventAttrPrefix + "event_attr_1", "event_1_attr_1"},
		{pb.EventPrefix + "1:" + pb.EventAttrPrefix + "event_attr_2", "event_1_attr_2"},

		{pb.LinkPrefix + "0:" + pb.LinkTraceIDField, "1234567890"},
		{pb.LinkPrefix + "0:" + pb.LinkSpanIDField, "23456"},
		{pb.LinkPrefix + "0:" + pb.LinkTraceStateField, "link_0_trace_state_1"},
		{pb.LinkPrefix + "0:" + pb.LinkAttrPrefix + "link_attr_1", "link_0_trace_attr_1"},
		{pb.LinkPrefix + "0:" + pb.LinkAttrPrefix + "link_attr_2", "link_0_trace_attr_2"},
		{pb.LinkPrefix + "0:" + pb.LinkAttrPrefix + "opentracing.ref_type", "child_of"},
		{pb.LinkPrefix + "0:" + pb.LinkFlagsField, "0"},
		{pb.LinkPrefix + "1:" + pb.LinkTraceIDField, "99999999999"},
		{pb.LinkPrefix + "1:" + pb.LinkSpanIDField, "98765"},
		{pb.LinkPrefix + "1:" + pb.LinkTraceStateField, "link_1_trace_state_1"},
		{pb.LinkPrefix + "1:" + pb.LinkAttrPrefix + "link_attr_1", "link_1_trace_attr_1"},
		{pb.LinkPrefix + "1:" + pb.LinkAttrPrefix + "link_attr_2", "link_1_trace_attr_2"},
		{pb.LinkPrefix + "1:" + pb.LinkFlagsField, "1"},

		{pb.StatusMessageField, "status_message_1"},
		{pb.StatusCodeField, "2"},
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
