package jaeger

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jaeger/proto"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

type SpanWriterPluginServer struct {
}

func (s *SpanWriterPluginServer) WriteSpan(ctx context.Context, req *proto.WriteSpanRequest) (*proto.WriteSpanResponse, error) {
	span := req.GetSpan()
	if span == nil {
		return nil, nil
	}

	jsonSpan, err := json.Marshal(span)
	if err != nil {
		return nil, err
	}
	cp, err := insertutils.GetJaegerCommonParams()
	lmp := cp.NewLogMessageProcessor("jaeger")
	defer lmp.MustClose()

	tags := span.GetTags()
	fields := make([]logstorage.Field, 0, 1+len(tags))
	fields = append(fields, logstorage.Field{
		Name:  "_msg",
		Value: string(jsonSpan),
	}, logstorage.Field{
		Name:  "duration",
		Value: strconv.FormatInt(span.Duration.Nanoseconds(), 10),
	})

	for i := range tags {
		var value string
		switch tags[i].GetVType() {
		case proto.ValueType_STRING:
			value = tags[i].GetVStr()
		case proto.ValueType_BOOL:
			value = strconv.FormatBool(tags[i].GetVBool())
		case proto.ValueType_INT64:
			value = strconv.FormatInt(tags[i].GetVInt64(), 10)
		case proto.ValueType_FLOAT64:
			value = strconv.FormatFloat(tags[i].GetVFloat64(), 'f', -1, 64)
		case proto.ValueType_BINARY:
			value = string(tags[i].GetVBinary())
		}
		fields = append(fields, logstorage.Field{
			Name:  tags[i].GetKey(),
			Value: value,
		})
	}
	processTags := span.GetProcess().GetTags()
	streamFields := make([]logstorage.Field, 0, 2+len(processTags))
	streamFields = append(streamFields,
		logstorage.Field{Name: "service_name", Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: "operation_name", Value: span.GetOperationName()},
	)

	for i := range processTags {
		value := "process_tag_"
		switch processTags[i].GetVType() {
		case proto.ValueType_STRING:
			value += processTags[i].GetVStr()
		case proto.ValueType_BOOL:
			value += strconv.FormatBool(processTags[i].GetVBool())
		case proto.ValueType_INT64:
			value += strconv.FormatInt(processTags[i].GetVInt64(), 10)
		case proto.ValueType_FLOAT64:
			value += strconv.FormatFloat(processTags[i].GetVFloat64(), 'f', -1, 64)
		case proto.ValueType_BINARY:
			value += string(processTags[i].GetVBinary())
		}
		streamFields = append(streamFields, logstorage.Field{
			Name:  processTags[i].GetKey(),
			Value: value,
		})
	}
	lmp.AddRow(span.StartTime.UnixNano(), fields, streamFields)
	resp := &proto.WriteSpanResponse{}
	return resp, nil
}
func (s *SpanWriterPluginServer) Close(ctx context.Context, req *proto.CloseWriterRequest) (*proto.CloseWriterResponse, error) {
	resp := &proto.CloseWriterResponse{}
	return resp, nil
}
