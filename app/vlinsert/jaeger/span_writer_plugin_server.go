package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jaegertracing/jaeger/model"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// A SpanWriterPluginServer represents plugin Jaeger interface to write gRPC storage backend
type SpanWriterPluginServer struct {
}

// WriteSpan writes spans
func (s *SpanWriterPluginServer) WriteSpan(ctx context.Context, span *model.Span) error {
	if span == nil {
		return fmt.Errorf("span not found")
	}

	jsonSpan, err := json.Marshal(span)
	if err != nil {
		return err
	}
	cp, err := insertutils.GetJaegerCommonParams()
	if err != nil {
		return err
	}
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
		case model.ValueType_STRING:
			value = tags[i].GetVStr()
		case model.ValueType_BOOL:
			value = strconv.FormatBool(tags[i].GetVBool())
		case model.ValueType_INT64:
			value = strconv.FormatInt(tags[i].GetVInt64(), 10)
		case model.ValueType_FLOAT64:
			value = strconv.FormatFloat(tags[i].GetVFloat64(), 'f', -1, 64)
		case model.ValueType_BINARY:
			value = string(tags[i].GetVBinary())
		}
		fields = append(fields, logstorage.Field{
			Name:  tags[i].GetKey(),
			Value: value,
		})
	}
	fields = append(fields, logstorage.Field{
		Name:  "trace_id",
		Value: span.TraceID.String(),
	}, logstorage.Field{
		Name:  "span_id",
		Value: span.SpanID.String(),
	})
	processTags := span.GetProcess().GetTags()
	streamFields := make([]logstorage.Field, 0, 2+len(processTags))
	streamFields = append(streamFields,
		logstorage.Field{Name: "service_name", Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: "operation_name", Value: span.GetOperationName()},
	)

	for i := range processTags {
		value := "process_tag_"
		switch processTags[i].GetVType() {
		case model.ValueType_STRING:
			value += processTags[i].GetVStr()
		case model.ValueType_BOOL:
			value += strconv.FormatBool(processTags[i].GetVBool())
		case model.ValueType_INT64:
			value += strconv.FormatInt(processTags[i].GetVInt64(), 10)
		case model.ValueType_FLOAT64:
			value += strconv.FormatFloat(processTags[i].GetVFloat64(), 'f', -1, 64)
		case model.ValueType_BINARY:
			value += string(processTags[i].GetVBinary())
		}
		fields = append(fields, logstorage.Field{
			Name:  processTags[i].GetKey(),
			Value: value,
		})
	}
	lmp.AddRow(span.StartTime.UnixNano(), fields, streamFields)
	return nil
}

func (s *SpanWriterPluginServer) Close() error {
	return nil
}
