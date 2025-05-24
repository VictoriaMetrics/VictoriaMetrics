package jaeger

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/traceutil"
)

type Trace struct {
	Spans      []*Span
	ProcessMap []Trace_ProcessMapping
}

type Trace_ProcessMapping struct {
	ProcessID string
	Process   Process
}

type Process struct {
	ServiceName string
	Tags        []KeyValue
}

type Span struct {
	TraceID       string
	SpanID        string
	OperationName string
	References    []SpanRef
	Flags         uint32
	StartTime     int64
	Duration      int64
	Tags          []KeyValue
	Logs          []Log
	Process       *Process
	ProcessID     string
	Warnings      []string
}

type SpanRef struct {
	TraceID string
	SpanID  string
	RefType string
}

type KeyValue struct {
	Key  string
	VStr string
}

type Log struct {
	Timestamp int64
	Fields    []KeyValue
}

// FieldsToSpan convert OTLP spans in fields to Jaeger Spans.
func FieldsToSpan(fields []logstorage.Field) (*Span, error) {
	sp := &Span{
		Process: &Process{},
	}

	processTagList, spanTagList := make([]KeyValue, 0, len(fields)), make([]KeyValue, 0, len(fields))
	logsMap := make(map[string]*Log)     // idx -> *Log
	refsMap := make(map[string]*SpanRef) // idx -> *SpanRef

	parentSpanRef := SpanRef{}
	for _, field := range fields {
		switch field.Name {
		case "_stream":
			//logstorage.GetStreamTags()
		case traceutil.TraceId:
			sp.TraceID = field.Value
		case traceutil.SpanId:
			sp.SpanID = field.Value
		case traceutil.Name:
			sp.OperationName = field.Value
		case traceutil.ParentSpanId:
			parentSpanRef.SpanID = field.Value
			parentSpanRef.RefType = "CHILD_OF"
		case traceutil.Kind:
			if field.Value != "" {
				spanKind := ""
				switch field.Value {
				case "1":
					spanKind = "internal"
				case "2":
					spanKind = "server"
				case "3":
					spanKind = "client"
				case "4":
					spanKind = "producer"
				case "5":
					spanKind = "consumer"
				}
				spanTagList = append(spanTagList, KeyValue{Key: "span.kind", VStr: spanKind})
			}
		case traceutil.Flags:
			// todo trace does not contain "flag" in result
			//flagU64, err := strconv.ParseUint(field.Value, 10, 32)
			//if err != nil {
			//	return nil, err
			//}
			//sp.Flags = uint32(flagU64)
		case traceutil.StartTimeUnixNano:
			unixNano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.StartTime = unixNano / 1000
		case traceutil.Duration:
			nano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.Duration = nano / 1000
		case traceutil.StatusCode:
			if field.Value == "2" {
				spanTagList = append(spanTagList, KeyValue{Key: "error", VStr: "true"})
			}
		case traceutil.StatusMessage:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "otel.status_description", VStr: field.Value})
			}
		case traceutil.TraceState:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "w3c.tracestate", VStr: field.Value})
			}
		// resource level fields
		case traceutil.ResourceAttrServiceName:
			sp.Process.ServiceName = field.Value
		// scope level fields
		case traceutil.InstrumentationScopeName:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "otel.scope.name", VStr: field.Value})
			}
		case traceutil.InstrumentationScopeVersion:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "otel.scope.version", VStr: field.Value})
			}
		default:
			if strings.HasPrefix(field.Name, traceutil.ResourceAttrPrefix) {
				processTagList = append(processTagList, KeyValue{Key: strings.TrimPrefix(field.Name, traceutil.ResourceAttrPrefix), VStr: field.Value})
			} else if strings.HasPrefix(field.Name, traceutil.SpanAttrPrefix) {
				spanTagList = append(spanTagList, KeyValue{Key: strings.TrimPrefix(field.Name, traceutil.SpanAttrPrefix), VStr: field.Value})
			} else if strings.HasPrefix(field.Name, traceutil.InstrumentationScopeAttrPrefix) {
				spanTagList = append(spanTagList, KeyValue{Key: strings.TrimPrefix(field.Name, traceutil.InstrumentationScopeAttrPrefix), VStr: field.Value})
			} else if strings.HasPrefix(field.Name, traceutil.EventPrefix) {
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, traceutil.EventPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid event field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := logsMap[idx]; !ok {
					logsMap[idx] = &Log{}
				}
				log := logsMap[idx]
				switch fieldName {
				case traceutil.EventTimeUnixNano:
					unixNano, _ := strconv.ParseInt(field.Value, 10, 64)
					log.Timestamp = unixNano / 1000
				case traceutil.EventName:
					log.Fields = append(log.Fields, KeyValue{Key: "event", VStr: field.Value})
				case traceutil.EventDroppedAttributesCount:
					//no need to display
					//log.Fields = append(log.Fields, KeyValue{Key: fieldName, VStr: field.Value})
				default:
					log.Fields = append(log.Fields, KeyValue{Key: strings.TrimPrefix(fieldName, traceutil.EventAttrPrefix), VStr: field.Value})
				}
			} else if strings.HasPrefix(field.Name, traceutil.LinkAttrPrefix) {
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, traceutil.LinkAttrPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid link field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := refsMap[idx]; !ok {
					refsMap[idx] = &SpanRef{
						RefType: "FOLLOW_FROM", // default FOLLOW_FROM
					}
				}
				ref := refsMap[idx]
				switch fieldName {
				case traceutil.LinkTraceId:
					ref.TraceID = field.Value
				case traceutil.LinkSpanId:
					ref.SpanID = field.Value
				//case LinkTraceState:
				//case LinkFlags:
				//case LinkDroppedAttributesCount:
				default:
					if strings.TrimPrefix(field.Name, traceutil.LinkPrefix) == "opentracing.ref_type" && field.Value == "child_of" {
						ref.RefType = "CHILD_OF" // CHILD_OF
					}
				}
			}
		}
	}

	sp.Tags = spanTagList
	sp.Process.Tags = processTagList

	if parentSpanRef.SpanID != "" {
		parentSpanRef.TraceID = sp.TraceID
		sp.References = append(sp.References, parentSpanRef)
	}
	for i := 0; i < len(refsMap); i++ {
		idx := strconv.Itoa(i)
		if len(sp.References) > 0 && parentSpanRef.TraceID == refsMap[idx].TraceID && parentSpanRef.SpanID == refsMap[idx].SpanID {
			// We already added a reference to this span, but maybe with the wrong type, so override.
			sp.References[0].RefType = refsMap[idx].RefType
			continue
		}
		sp.References = append(sp.References, SpanRef{
			refsMap[idx].TraceID, refsMap[idx].SpanID, refsMap[idx].RefType,
		})
	}
	for i := 0; i < len(logsMap); i++ {
		idx := strconv.Itoa(i)
		sp.Logs = append(sp.Logs, Log{
			logsMap[idx].Timestamp, logsMap[idx].Fields,
		})
	}

	if sp.SpanID != "" {
		return sp, nil
	}
	return nil, fmt.Errorf("invalid fields: %v", fields)
}
