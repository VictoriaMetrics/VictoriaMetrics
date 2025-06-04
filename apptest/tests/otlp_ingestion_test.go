package tests

import (
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/traces/query"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestSingleOTLPIngestionJaegerQuery test data ingestion of `/insert/opentelemetry/v1/traces` API
// and queries of various `/select/jaeger/api/*` APIs for vl-single.
func TestSingleOTLPIngestionJaegerQuery(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVlsingle()

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
	testTag := []*pb.KeyValue{
		{
			Key: "testTag",
			Value: &pb.AnyValue{
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

	req := &pb.ExportTraceServiceRequest{
		ResourceSpans: []*pb.ResourceSpans{
			{
				Resource: pb.Resource{
					Attributes: []*pb.KeyValue{
						{
							Key: "service.name",
							Value: &pb.AnyValue{
								StringValue: &serviceName,
							},
						},
					},
				},
				ScopeSpans: []*pb.ScopeSpans{
					{
						Scope: pb.InstrumentationScope{
							Name:                   "testInstrumentation",
							Version:                "1.0",
							Attributes:             testTag,
							DroppedAttributesCount: 999,
						},
						Spans: []*pb.Span{
							{
								TraceID:           traceID,
								SpanID:            spanID,
								TraceState:        "trace_state",
								ParentSpanID:      spanID,
								Flags:             1,
								Name:              spanName,
								Kind:              pb.SpanKind(1),
								StartTimeUnixNano: uint64(spanTime.UnixNano()),
								EndTimeUnixNano:   uint64(spanTime.UnixNano()),
								Attributes:        testTag,
								Events: []*pb.SpanEvent{
									{
										TimeUnixNano: uint64(spanTime.UnixNano()),
										Name:         "test event",
										Attributes:   testTag,
									},
								},
								Links: []*pb.SpanLink{
									{
										TraceID:    traceID,
										SpanID:     spanID,
										TraceState: "trace_state",
										Attributes: testTag,
										Flags:      1,
									},
								},
								Status: pb.Status{
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
							RefType: "CHILD_OF",
						},
					},
					StartTime: spanTime.UnixMicro(),
					Tags: []at.Tag{
						{Key: "span.kind", Type: "string", Value: "internal"},
						{Key: "testTag", Type: "string", Value: "testValue"},
						{Key: "otel.scope.name", Type: "string", Value: "testInstrumentation"},
						{Key: "otel.scope.version", Type: "string", Value: "1.0"},
						{Key: "testTag", Type: "string", Value: "testValue"},
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
