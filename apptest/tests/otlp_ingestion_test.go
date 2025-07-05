package tests

import (
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/traces/query"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	otelpb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestSingleOTLPIngestionJaegerQuery test data ingestion of `/insert/opentelemetry/v1/traces` API
// and queries of various `/select/jaeger/api/*` APIs for vl-single.
func TestSingleOTLPIngestionJaegerQuery(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVtsingle()

	testOTLPIngestionJaegerQuery(tc, sut)
}

func testOTLPIngestionJaegerQuery(tc *at.TestCase, sut at.VictoriaTracesWriteQuerier) {
	t := tc.T()

	// prepare test data for ingestion and assertion.
	serviceName := "testKeyIngestQueryService"
	spanName := "testKeyIngestQuerySpan"
	traceID := "123456789"
	spanID := "987654321"
	testTagValue := "testValue"
	testTag := []*otelpb.KeyValue{
		{
			Key: "testTag",
			Value: &otelpb.AnyValue{
				StringValue: &testTagValue,
			},
		},
	}
	assertTag := []at.Tag{
		{
			Key:   "testTag",
			Type:  "string",
			Value: "testValue",
		},
	}
	spanTime := time.Now()

	req := &otelpb.ExportTraceServiceRequest{
		ResourceSpans: []*otelpb.ResourceSpans{
			{
				Resource: otelpb.Resource{
					Attributes: []*otelpb.KeyValue{
						{
							Key: "service.name",
							Value: &otelpb.AnyValue{
								StringValue: &serviceName,
							},
						},
					},
				},
				ScopeSpans: []*otelpb.ScopeSpans{
					{
						Scope: otelpb.InstrumentationScope{
							Name:                   "testInstrumentation",
							Version:                "1.0",
							Attributes:             testTag,
							DroppedAttributesCount: 999,
						},
						Spans: []*otelpb.Span{
							{
								TraceID:           traceID,
								SpanID:            spanID,
								TraceState:        "trace_state",
								ParentSpanID:      spanID,
								Flags:             1,
								Name:              spanName,
								Kind:              otelpb.SpanKind(1),
								StartTimeUnixNano: uint64(spanTime.UnixNano()),
								EndTimeUnixNano:   uint64(spanTime.UnixNano()),
								Attributes:        testTag,
								Events: []*otelpb.SpanEvent{
									{
										TimeUnixNano: uint64(spanTime.UnixNano()),
										Name:         "test event",
										Attributes:   testTag,
									},
								},
								Links: []*otelpb.SpanLink{
									{
										TraceID:    traceID,
										SpanID:     spanID,
										TraceState: "trace_state",
										Attributes: testTag,
										Flags:      1,
									},
								},
								Status: otelpb.Status{
									Message: "success",
									Code:    0,
								},
							},
						},
					},
				},
			},
		},
	}

	// ingest data via /insert/opentelemetry/v1/traces
	sut.OTLPExportTraces(t, req, at.QueryOpts{})
	sut.ForceFlush(t)

	// check services via /select/jaeger/api/services
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /select/jaeger/api/services response",
		Got: func() any {
			return sut.JaegerAPIServices(t, at.QueryOpts{})
		},
		Want: &at.JaegerAPIServicesResponse{
			Data: []string{serviceName},
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(at.JaegerAPIServicesResponse{}, "Errors", "Limit", "Offset", "Total"),
		},
	})

	// check span name via /select/jaeger/api/services/*/operations
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /select/jaeger/api/services/*/operations response",
		Got: func() any {
			return sut.JaegerAPIOperations(t, serviceName, at.QueryOpts{})
		},
		Want: &at.JaegerAPIOperationsResponse{
			Data: []string{spanName},
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(at.JaegerAPIOperationsResponse{}, "Errors", "Limit", "Offset", "Total"),
		},
	})

	expectTraceData := []at.TracesResponseData{
		{
			Processes: map[string]at.Process{"p1": {ServiceName: "testKeyIngestQueryService", Tags: []at.Tag{}}},
			Spans: []at.Span{
				{
					Duration: 0,
					TraceID:  hex.EncodeToString([]byte(traceID)),
					SpanID:   hex.EncodeToString([]byte(spanID)),
					Logs: []at.Log{
						{
							Timestamp: spanTime.UnixMicro(),
							Fields: append(assertTag, at.Tag{
								Key:   "event",
								Type:  "string",
								Value: "test event",
							}),
						},
					},
					OperationName: spanName,
					ProcessID:     "p1",
					References: []at.Reference{
						{
							TraceID: hex.EncodeToString([]byte(traceID)),
							SpanID:  hex.EncodeToString([]byte(spanID)),
							RefType: "FOLLOWS_FROM",
						},
					},
					StartTime: spanTime.UnixMicro(),
					Tags: []at.Tag{
						{Key: "span.kind", Type: "string", Value: "internal"},
						{Key: "scope_attr:testTag", Type: "string", Value: "testValue"},
						{Key: "otel.scope.name", Type: "string", Value: "testInstrumentation"},
						{Key: "otel.scope.version", Type: "string", Value: "1.0"},
						{Key: "testTag", Type: "string", Value: "testValue"},
						{Key: "error", Type: "string", Value: "unset"},
						{Key: "otel.status_description", Type: "string", Value: "success"},
						{Key: "w3c.tracestate", Type: "string", Value: "trace_state"},
					},
				},
			},
			TraceID: hex.EncodeToString([]byte(traceID)),
		},
	}

	// check traces data via /select/jaeger/api/traces
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /select/jaeger/api/traces response",
		Got: func() any {
			return sut.JaegerAPITraces(t, at.JaegerQueryParam{
				TraceQueryParam: query.TraceQueryParam{
					ServiceName:  serviceName,
					StartTimeMin: spanTime.Add(-10 * time.Minute),
					StartTimeMax: spanTime.Add(10 * time.Minute),
				},
			}, at.QueryOpts{})
		},
		Want: &at.JaegerAPITracesResponse{
			Data: expectTraceData,
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(at.JaegerAPITracesResponse{}, "Errors", "Limit", "Offset", "Total"),
		},
	})
	// check single trace data via /select/jaeger/api/traces/<trace_id>
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /select/jaeger/api/traces/<trace_id> response",
		Got: func() any {
			return sut.JaegerAPITrace(t, hex.EncodeToString([]byte(traceID)), at.QueryOpts{})
		},
		Want: &at.JaegerAPITraceResponse{
			Data: expectTraceData,
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(at.JaegerAPITraceResponse{}, "Errors", "Limit", "Offset", "Total"),
		},
	})
}
