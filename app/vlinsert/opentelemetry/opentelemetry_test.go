package opentelemetry

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestPushProtoOk(t *testing.T) {
	f := func(src []pb.ResourceLogs, timestampsExpected []int64, resultExpected string) {
		t.Helper()
		lr := pb.ExportLogsServiceRequest{
			ResourceLogs: src,
		}

		pData := lr.MarshalProtobuf(nil)
		tlp := &insertutils.TestLogMessageProcessor{}
		n, err := pushProtobufRequest(pData, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if err := tlp.Verify(n, timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}
	// single line without resource attributes
	f([]pb.ResourceLogs{
		{
			ScopeLogs: []pb.ScopeLogs{
				{
					LogRecords: []pb.LogRecord{
						{Attributes: []*pb.KeyValue{}, TimeUnixNano: 1234, SeverityNumber: 1, Body: pb.AnyValue{StringValue: ptrTo("log-line-message")}},
					},
				},
			},
		},
	},
		[]int64{1234},
		`{"_msg":"log-line-message","severity":"Trace"}`,
	)
	// multi-line with resource attributes
	f([]pb.ResourceLogs{
		{
			Resource: pb.Resource{
				Attributes: []*pb.KeyValue{
					{Key: "logger", Value: &pb.AnyValue{StringValue: ptrTo("context")}},
					{Key: "instance_id", Value: &pb.AnyValue{IntValue: ptrTo[int64](10)}},
					{Key: "node_taints", Value: &pb.AnyValue{KeyValueList: &pb.KeyValueList{
						Values: []*pb.KeyValue{
							{Key: "role", Value: &pb.AnyValue{StringValue: ptrTo("dev")}},
							{Key: "cluster_load_percent", Value: &pb.AnyValue{DoubleValue: ptrTo(0.55)}},
						},
					}}},
				},
			},
			ScopeLogs: []pb.ScopeLogs{
				{
					LogRecords: []pb.LogRecord{
						{Attributes: []*pb.KeyValue{}, TimeUnixNano: 1234, SeverityNumber: 1, Body: pb.AnyValue{StringValue: ptrTo("log-line-message")}},
						{Attributes: []*pb.KeyValue{}, TimeUnixNano: 1235, SeverityNumber: 21, Body: pb.AnyValue{StringValue: ptrTo("log-line-message-msg-2")}},
						{Attributes: []*pb.KeyValue{}, TimeUnixNano: 1236, SeverityNumber: -1, Body: pb.AnyValue{StringValue: ptrTo("log-line-message-msg-2")}},
					},
				},
			},
		},
	},
		[]int64{1234, 1235, 1236},
		`{"logger":"context","instance_id":"10","node_taints":"[{\"Key\":\"role\",\"Value\":{\"StringValue\":\"dev\",\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":null,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}},{\"Key\":\"cluster_load_percent\",\"Value\":{\"StringValue\":null,\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":0.55,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}}]","_msg":"log-line-message","severity":"Trace"}
{"logger":"context","instance_id":"10","node_taints":"[{\"Key\":\"role\",\"Value\":{\"StringValue\":\"dev\",\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":null,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}},{\"Key\":\"cluster_load_percent\",\"Value\":{\"StringValue\":null,\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":0.55,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}}]","_msg":"log-line-message-msg-2","severity":"Unspecified"}
{"logger":"context","instance_id":"10","node_taints":"[{\"Key\":\"role\",\"Value\":{\"StringValue\":\"dev\",\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":null,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}},{\"Key\":\"cluster_load_percent\",\"Value\":{\"StringValue\":null,\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":0.55,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}}]","_msg":"log-line-message-msg-2","severity":"Unspecified"}`,
	)

	// multi-scope with resource attributes and multi-line
	f([]pb.ResourceLogs{
		{
			Resource: pb.Resource{
				Attributes: []*pb.KeyValue{
					{Key: "logger", Value: &pb.AnyValue{StringValue: ptrTo("context")}},
					{Key: "instance_id", Value: &pb.AnyValue{IntValue: ptrTo[int64](10)}},
					{Key: "node_taints", Value: &pb.AnyValue{KeyValueList: &pb.KeyValueList{
						Values: []*pb.KeyValue{
							{Key: "role", Value: &pb.AnyValue{StringValue: ptrTo("dev")}},
							{Key: "cluster_load_percent", Value: &pb.AnyValue{DoubleValue: ptrTo(0.55)}},
						},
					}}},
				},
			},
			ScopeLogs: []pb.ScopeLogs{
				{
					LogRecords: []pb.LogRecord{
						{TimeUnixNano: 1234, SeverityNumber: 1, Body: pb.AnyValue{StringValue: ptrTo("log-line-message")}},
						{TimeUnixNano: 1235, SeverityNumber: 5, Body: pb.AnyValue{StringValue: ptrTo("log-line-message-msg-2")}},
					},
				},
			},
		},
		{
			ScopeLogs: []pb.ScopeLogs{
				{
					LogRecords: []pb.LogRecord{
						{TimeUnixNano: 2345, SeverityNumber: 10, Body: pb.AnyValue{StringValue: ptrTo("log-line-resource-scope-1-0-0")}},
						{TimeUnixNano: 2346, SeverityNumber: 10, Body: pb.AnyValue{StringValue: ptrTo("log-line-resource-scope-1-0-1")}},
					},
				},
				{
					LogRecords: []pb.LogRecord{
						{TimeUnixNano: 2347, SeverityNumber: 12, Body: pb.AnyValue{StringValue: ptrTo("log-line-resource-scope-1-1-0")}},
						{ObservedTimeUnixNano: 2348, SeverityNumber: 12, Body: pb.AnyValue{StringValue: ptrTo("log-line-resource-scope-1-1-1")}},
					},
				},
			},
		},
	},
		[]int64{1234, 1235, 2345, 2346, 2347, 2348},
		`{"logger":"context","instance_id":"10","node_taints":"[{\"Key\":\"role\",\"Value\":{\"StringValue\":\"dev\",\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":null,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}},{\"Key\":\"cluster_load_percent\",\"Value\":{\"StringValue\":null,\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":0.55,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}}]","_msg":"log-line-message","severity":"Trace"}
{"logger":"context","instance_id":"10","node_taints":"[{\"Key\":\"role\",\"Value\":{\"StringValue\":\"dev\",\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":null,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}},{\"Key\":\"cluster_load_percent\",\"Value\":{\"StringValue\":null,\"BoolValue\":null,\"IntValue\":null,\"DoubleValue\":0.55,\"ArrayValue\":null,\"KeyValueList\":null,\"BytesValue\":null}}]","_msg":"log-line-message-msg-2","severity":"Debug"}
{"_msg":"log-line-resource-scope-1-0-0","severity":"Info2"}
{"_msg":"log-line-resource-scope-1-0-1","severity":"Info2"}
{"_msg":"log-line-resource-scope-1-1-0","severity":"Info4"}
{"_msg":"log-line-resource-scope-1-1-1","severity":"Info4"}`,
	)
}

func ptrTo[T any](s T) *T {
	return &s
}
