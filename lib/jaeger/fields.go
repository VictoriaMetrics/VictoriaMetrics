package jaeger

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jaegertracing/jaeger/model"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// define the label name for span attributes
const (
	// attributes that can be stored in a single field.
	TraceID       string = "trace_id"
	SpanID        string = "span_id"
	OperationName string = "operation_name"
	Flags         string = "flags"
	StartTime     string = "start_time"
	EndTime       string = "end_time"
	Duration      string = "duration"
	ProcessID     string = "process_id"

	ProcessServiceName string = "process_service_name"
	ProcessTagKey      string = "process_tag:%s"
	ProcessTagVType    string = "process_tag:%s:v_type"

	// attributes that can be stored in a single field but does not need to be queried.
	Logs       string = "log"
	Warnings   string = "warnings"
	References string = "references"

	// attributes that cannot be stored in a single field.
	TagKey   string = "tag:%s"        // e.g.: span_tag:otel.scope.name
	TagVType string = "tag:%s:v_type" // e.g.: span_tag:otel.scope.name:v_type
)

func SpanToFieldsByteBuffer(bbuf *bytesutil.ByteBuffer, span *model.Span) ([]logstorage.Field, []logstorage.Field, error) {

	fields := make([]logstorage.Field, 0)

	fields = append(fields, logstorage.Field{Name: "_msg", Value: "span"},
		logstorage.Field{Name: TraceID, Value: span.TraceID.String()},
		logstorage.Field{Name: SpanID, Value: span.SpanID.String()},
		logstorage.Field{Name: Flags, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: StartTime, Value: strconv.FormatInt(span.StartTime.UnixNano(), 10)},
		logstorage.Field{Name: EndTime, Value: strconv.FormatInt(span.StartTime.Add(span.Duration).UnixNano(), 10)},
		logstorage.Field{Name: Duration, Value: strconv.FormatInt(span.Duration.Nanoseconds(), 10)}, logstorage.Field{Name: ProcessID, Value: span.ProcessID},
		logstorage.Field{Name: ProcessServiceName, Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: OperationName, Value: span.GetOperationName()},
	)

	if tags := span.GetProcess().GetTags(); len(tags) > 0 {
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
			startIdx := int64(len(bbuf.B))
			bbuf.Write([]byte("process_tag:"))
			bbuf.Write([]byte(tags[i].GetKey()))
			fields = append(fields, logstorage.Field{Name: string(bbuf.B[startIdx:]), Value: value})
			vType := int64(tags[i].GetVType())
			if vType > 0 {
				bbuf.Write([]byte(":v_type"))
				//fields = append(fields, logstorage.Field{Name: string(bbuf.B[startIdx:]), Value: strconv.FormatInt(int64(tags[i].GetVType()), 10)})
				fields = append(fields, logstorage.Field{Name: string(bbuf.B[startIdx:]), Value: "1"})
			}
		}
	}
	if len(span.Logs) > 0 {
		logs, err := json.Marshal(span.Logs)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  Logs,
			Value: string(logs),
		})
	}

	if len(span.Warnings) > 0 {
		warnings, err := json.Marshal(span.Warnings)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  Warnings,
			Value: string(warnings),
		})
	}

	if len(span.References) > 0 {
		refs, err := json.Marshal(span.References)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  References,
			Value: string(refs),
		})
	}

	tags := span.GetTags()
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
		startIdx := int64(len(bbuf.B))
		bbuf.Write([]byte("tag:"))
		bbuf.Write([]byte(tags[i].GetKey()))
		fields = append(fields, logstorage.Field{Name: string(bbuf.B[startIdx:]), Value: value})
		vtype := int64(tags[i].GetVType())
		if vtype > 0 {
			bbuf.Write([]byte(":v_type"))
			startVtype := int64(len(bbuf.B))
			bbuf.Write([]byte(strconv.FormatInt(int64(tags[i].GetVType()), 10)))
			fields = append(fields, logstorage.Field{Name: string(bbuf.B[startIdx:startVtype]), Value: string(bbuf.B[startVtype:])})
		}

	}
	streamFields := make([]logstorage.Field, 0, 2)
	streamFields = append(streamFields,
		logstorage.Field{Name: ProcessServiceName, Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: OperationName, Value: span.GetOperationName()},
	)

	return fields, streamFields, nil
}
func SpanToFieldsStringBuilder(sb *strings.Builder, span *model.Span) ([]logstorage.Field, []logstorage.Field, error) {
	fields := make([]logstorage.Field, 0)

	fields = append(fields, logstorage.Field{Name: "_msg", Value: "span"},
		logstorage.Field{Name: TraceID, Value: span.TraceID.String()},
		logstorage.Field{Name: SpanID, Value: span.SpanID.String()},
		logstorage.Field{Name: Flags, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: StartTime, Value: strconv.FormatInt(span.StartTime.UnixNano(), 10)},
		logstorage.Field{Name: EndTime, Value: strconv.FormatInt(span.StartTime.Add(span.Duration).UnixNano(), 10)},
		logstorage.Field{Name: Duration, Value: strconv.FormatInt(span.Duration.Nanoseconds(), 10)}, logstorage.Field{Name: ProcessID, Value: span.ProcessID},
		logstorage.Field{Name: ProcessServiceName, Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: OperationName, Value: span.GetOperationName()},
	)

	if tags := span.GetProcess().GetTags(); len(tags) > 0 {
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
			sb.WriteString("process_tag:")
			sb.WriteString(tags[i].GetKey())
			var f1 = logstorage.Field{Name: sb.String(), Value: value}
			sb.WriteString(":v_type")
			var f2 = logstorage.Field{Name: sb.String(), Value: strconv.FormatInt(int64(tags[i].GetVType()), 10)}
			sb.Reset()
			fields = append(fields, f1, f2)
		}
	}
	if len(span.Logs) > 0 {
		logs, err := json.Marshal(span.Logs)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  Logs,
			Value: string(logs),
		})
	}

	if len(span.Warnings) > 0 {
		warnings, err := json.Marshal(span.Warnings)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  Warnings,
			Value: string(warnings),
		})
	}

	if len(span.References) > 0 {
		refs, err := json.Marshal(span.References)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  References,
			Value: string(refs),
		})
	}

	tags := span.GetTags()
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
		sb.WriteString("tag:")
		sb.WriteString(tags[i].GetKey())
		var f1 = logstorage.Field{Name: sb.String(), Value: value}
		sb.WriteString(":v_type")
		var f2 = logstorage.Field{Name: sb.String(), Value: strconv.FormatInt(int64(tags[i].GetVType()), 10)}
		sb.Reset()
		fields = append(fields, f1, f2)
	}
	streamFields := make([]logstorage.Field, 0, 2)
	streamFields = append(streamFields,
		logstorage.Field{Name: ProcessServiceName, Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: OperationName, Value: span.GetOperationName()},
	)

	return fields, streamFields, nil
}
func SpanToFields(span *model.Span) ([]logstorage.Field, []logstorage.Field, error) {
	fields := make([]logstorage.Field, 0)

	fields = append(fields, logstorage.Field{Name: "_msg", Value: "span"},
		logstorage.Field{Name: TraceID, Value: span.TraceID.String()},
		logstorage.Field{Name: SpanID, Value: span.SpanID.String()},
		logstorage.Field{Name: Flags, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: StartTime, Value: strconv.FormatInt(span.StartTime.UnixNano(), 10)},
		logstorage.Field{Name: EndTime, Value: strconv.FormatInt(span.StartTime.Add(span.Duration).UnixNano(), 10)},
		logstorage.Field{Name: Duration, Value: strconv.FormatInt(span.Duration.Nanoseconds(), 10)}, logstorage.Field{Name: ProcessID, Value: span.ProcessID},
		logstorage.Field{Name: ProcessServiceName, Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: OperationName, Value: span.GetOperationName()},
	)

	if tags := span.GetProcess().GetTags(); len(tags) > 0 {
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
				Name:  fmt.Sprintf(ProcessTagVType, tags[i].GetKey()),
				Value: strconv.FormatInt(int64(tags[i].GetVType()), 10),
			}, logstorage.Field{
				Name:  fmt.Sprintf(ProcessTagKey, tags[i].GetKey()),
				Value: value,
			})
		}
	}
	if len(span.Logs) > 0 {
		logs, err := json.Marshal(span.Logs)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  Logs,
			Value: string(logs),
		})
	}

	if len(span.Warnings) > 0 {
		warnings, err := json.Marshal(span.Warnings)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  Warnings,
			Value: string(warnings),
		})
	}

	if len(span.References) > 0 {
		refs, err := json.Marshal(span.References)
		if err != nil {
			return nil, nil, err
		}
		fields = append(fields, logstorage.Field{
			Name:  References,
			Value: string(refs),
		})
	}

	tags := span.GetTags()
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
			Name:  fmt.Sprintf(TagVType, tags[i].GetKey()),
			Value: strconv.FormatInt(int64(tags[i].GetVType()), 10),
		}, logstorage.Field{
			Name:  fmt.Sprintf(TagKey, tags[i].GetKey()),
			Value: value,
		})
	}
	streamFields := make([]logstorage.Field, 0, 2)
	streamFields = append(streamFields,
		logstorage.Field{Name: ProcessServiceName, Value: span.GetProcess().GetServiceName()},
		logstorage.Field{Name: OperationName, Value: span.GetOperationName()},
	)

	return fields, streamFields, nil
}

func FieldsToSpan(fields []logstorage.Field) (*model.Span, error) {
	sp := &model.Span{
		Process: &model.Process{},
	}

	tagMap, tagVTypeMap, processTagMap, processVTypeMap := make(map[string]string), make(map[string]model.ValueType), make(map[string]string), make(map[string]model.ValueType)

	for _, field := range fields {
		switch field.Name {
		case "_stream":
			logstorage.GetStreamTags()
		case TraceID:
			traceID, err := model.TraceIDFromString(field.Value)
			if err != nil {
				return nil, err
			}
			sp.TraceID = traceID
		case SpanID:
			spanID, err := model.SpanIDFromString(field.Value)
			if err != nil {
				return nil, err
			}
			sp.SpanID = spanID
		case OperationName:
			sp.OperationName = field.Value
		case Flags:
			flags, err := strconv.ParseUint(field.Value, 10, 32)
			if err != nil {
				return nil, err
			}
			sp.Flags = model.Flags(uint32(flags))
		case StartTime:
			unixNano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.StartTime = time.Unix(0, unixNano)
		case Duration:
			nano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.Duration = time.Duration(nano)
		case ProcessID:
			sp.ProcessID = field.Value
		case ProcessServiceName:
			sp.Process.ServiceName = field.Value
		case Logs:
			var logs []model.Log
			err := json.Unmarshal([]byte(field.Value), &logs)
			if err != nil {
				return nil, err
			}
			sp.Logs = logs
		case Warnings:
			var warnings []string
			err := json.Unmarshal([]byte(field.Value), &warnings)
			if err != nil {
				return nil, err
			}
			sp.Warnings = warnings
		case References:
			var refs []model.SpanRef
			err := json.Unmarshal([]byte(field.Value), &refs)
			if err != nil {
				return nil, err
			}
			sp.References = refs
		default:
			if strings.HasPrefix(field.Name, "process_tag:") {
				if strings.HasSuffix(field.Name, ":v_type") {
					vType, err := strconv.ParseInt(field.Value, 10, 32)
					if err != nil {
						return nil, err
					}
					processVTypeMap[strings.TrimSuffix(strings.TrimPrefix(field.Name, "process_tag:"), ":v_type")] = model.ValueType(vType)
				} else {
					processTagMap[strings.TrimPrefix(field.Name, "process_tag:")] = field.Value
				}
			} else if strings.HasPrefix(field.Name, "tag:") {
				if strings.HasSuffix(field.Name, ":v_type") {
					vType, err := strconv.ParseInt(field.Value, 10, 32)
					if err != nil {
						return nil, err
					}
					tagVTypeMap[strings.TrimSuffix(strings.TrimPrefix(field.Name, "tag:"), ":v_type")] = model.ValueType(vType)
				} else {
					tagMap[strings.TrimPrefix(field.Name, "tag:")] = field.Value
				}
			}
		}
	}

	if len(tagMap) > 0 {
		tags, err := mapToTags(tagMap, tagVTypeMap)
		if err != nil {
			return nil, err
		}
		sp.Tags = tags
	}

	if len(processTagMap) > 0 {
		tags, err := mapToTags(processTagMap, processVTypeMap)
		if err != nil {
			return nil, err
		}
		sp.Process.Tags = tags
	}

	if sp.SpanID != 0 {
		return sp, nil
	}
	return nil, fmt.Errorf("invalid fields: %v", fields)
}

func mapToTags(tagMap map[string]string, tagVTypeMap map[string]model.ValueType) ([]model.KeyValue, error) {
	tags := make([]model.KeyValue, 0, len(tagMap))
	for name, vStr := range tagMap {
		vType, ok := tagVTypeMap[name]
		/*if !ok {
			return nil, fmt.Errorf("cannot find tag value type for tag [%s]", name)
		} */
		if !ok {
			// assume tag is of type string
			vType = model.ValueType(0)
		}
		tag := model.KeyValue{
			Key:   name,
			VType: vType,
		}
		switch vType {
		case model.ValueType_STRING:
			tag.VStr = vStr
		case model.ValueType_BOOL:
			v, err := strconv.ParseBool(vStr)
			if err != nil {
				return nil, err
			}
			tag.VBool = v
		case model.ValueType_INT64:
			v, err := strconv.ParseInt(vStr, 10, 64)
			if err != nil {
				return nil, err
			}
			tag.VInt64 = v
		case model.ValueType_FLOAT64:
			v, err := strconv.ParseFloat(vStr, 64)
			if err != nil {
				return nil, err
			}
			tag.VFloat64 = v
		case model.ValueType_BINARY:
			tag.VBinary = []byte(vStr)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}
