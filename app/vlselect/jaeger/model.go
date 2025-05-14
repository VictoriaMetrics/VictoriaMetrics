package jaeger

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"strconv"
	"strings"
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

func FieldsToSpan(fields []logstorage.Field) (*Span, error) {
	sp := &Span{
		Process: &Process{},
	}

	processTagList, spanTagList := make([]KeyValue, 0, len(fields)), make([]KeyValue, 0, len(fields))
	logsMap := make(map[string]*Log)     // idx -> *Log
	refsMap := make(map[string]*SpanRef) // idx -> *SpanRef

	for _, field := range fields {
		switch field.Name {
		case "_stream":
			//logstorage.GetStreamTags()
		case TraceId:
			sp.TraceID = field.Value
		case SpanId:
			sp.SpanID = field.Value
		case Name:
			sp.OperationName = field.Value
		case Kind:
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
		case Flags: // todo map otlp flags to jaeger flags
			flagU64, err := strconv.ParseUint(field.Value, 10, 32)
			if err != nil {
				return nil, err
			}
			sp.Flags = uint32(flagU64)
		case StartTimeUnixNano:
			unixNano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.StartTime = unixNano / 1000
		case Duration:
			nano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.Duration = nano / 1000
		case StatusCode:
			if field.Value == "2" {
				spanTagList = append(spanTagList, KeyValue{Key: "error", VStr: "true"})
			}
		case StatusMessage:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "otel.status_description", VStr: field.Value})
			}
		case TraceState:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "w3c.tracestate", VStr: field.Value})
			}
		// resource level fields
		// case ProcessID: // todo map otlp flags to jaeger flags
		//	sp.ProcessID = field.Value
		case ResourceAttrPrefix + "service.name":
			sp.Process.ServiceName = field.Value
		// scope level fields
		case InstrumentationScopeName:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "otel.scope.name", VStr: field.Value})
			}
		case InstrumentationScopeVersion:
			if field.Value != "" {
				spanTagList = append(spanTagList, KeyValue{Key: "otel.scope.version", VStr: field.Value})
			}
		default:
			if strings.HasPrefix(field.Name, ResourceAttrPrefix) {
				processTagList = append(processTagList, KeyValue{Key: strings.TrimPrefix(field.Name, ResourceAttrPrefix), VStr: field.Value})
			} else if strings.HasPrefix(field.Name, SpanAttrPrefix) {
				spanTagList = append(spanTagList, KeyValue{Key: strings.TrimPrefix(field.Name, SpanAttrPrefix), VStr: field.Value})
			} else if strings.HasPrefix(field.Name, instrumentationScopeAttrPrefix) {
				spanTagList = append(spanTagList, KeyValue{Key: strings.TrimPrefix(field.Name, SpanAttrPrefix), VStr: field.Value})
			} else if strings.HasPrefix(field.Name, EventPrefix) {
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, EventPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid event field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := logsMap[idx]; !ok {
					logsMap[idx] = &Log{}
				}
				log := logsMap[idx]
				switch fieldName {
				case EventTimeUnixNano:
					unixNano, _ := strconv.ParseInt(field.Value, 10, 64)
					log.Timestamp = unixNano / 1000
				case EventName:
					log.Fields = append(log.Fields, KeyValue{Key: "event", VStr: field.Value})
				case EventDroppedAttributesCount:
					log.Fields = append(log.Fields, KeyValue{Key: fieldName, VStr: field.Value})
				default:
					log.Fields = append(log.Fields, KeyValue{Key: strings.TrimPrefix(field.Name, EventAttrPrefix), VStr: field.Value})
				}
			} else if strings.HasPrefix(field.Name, LinkAttrPrefix) {
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, LinkAttrPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid link field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := refsMap[idx]; !ok {
					refsMap[idx] = &SpanRef{
						RefType: "FOLLOW_FROM", // FOLLOW_FROM
					}
				}
				ref := refsMap[idx]
				switch fieldName {
				case LinkTraceId:
					ref.TraceID = field.Value
				case LinkSpanId:
					ref.SpanID = field.Value
				//case LinkTraceState:
				//case LinkFlags:
				//case LinkDroppedAttributesCount:
				default:
					if strings.TrimPrefix(field.Name, LinkPrefix) == "opentracing.ref_type" && field.Value == "child_of" {
						ref.RefType = "CHILD_OF" // CHILD_OF
					}
				}
			}
		}
	}

	sp.Tags = spanTagList
	sp.Process.Tags = processTagList

	if sp.SpanID != "" {
		return sp, nil
	}
	return nil, fmt.Errorf("invalid fields: %v", fields)
}
