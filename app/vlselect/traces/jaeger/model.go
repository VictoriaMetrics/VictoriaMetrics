package jaeger

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/traceutil"
)

type trace struct {
	spans      []*span
	processMap []processMap
}

type processMap struct {
	processID string
	process   process
}

type process struct {
	serviceName string
	tags        []keyValue
}

type span struct {
	traceID       string
	spanID        string
	operationName string
	references    []spanRef
	//flags         uint32 // OTLP - jaeger conversion does not use this field, but it exists in jaeger definition.
	startTime int64
	duration  int64
	tags      []keyValue
	logs      []log
	process   *process
	processID string
	//warnings      []string // OTLP - jaeger conversion does not use this field, but it exists in jaeger definition.
}

type spanRef struct {
	traceID string
	spanID  string
	refType string
}

type keyValue struct {
	key  string
	vStr string
}

type log struct {
	timestamp int64
	fields    []keyValue
}

// fieldsToSpan convert OTLP spans in fields to Jaeger Spans.
func fieldsToSpan(fields []logstorage.Field) (*span, error) {
	sp := &span{
		process: &process{},
	}

	processTagList, spanTagList := make([]keyValue, 0, len(fields)), make([]keyValue, 0, len(fields))
	logsMap := make(map[string]*log)     // idx -> *Log
	refsMap := make(map[string]*spanRef) // idx -> *SpanRef

	parentSpanRef := spanRef{}
	for _, field := range fields {
		switch field.Name {
		case "_stream":
			//logstorage.GetStreamTags()
		case traceutil.TraceID:
			sp.traceID = field.Value
		case traceutil.SpanID:
			sp.spanID = field.Value
		case traceutil.Name:
			sp.operationName = field.Value
		case traceutil.ParentSpanID:
			parentSpanRef.spanID = field.Value
			parentSpanRef.refType = "CHILD_OF"
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
				spanTagList = append(spanTagList, keyValue{key: "span.kind", vStr: spanKind})
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
			sp.startTime = unixNano / 1000
		case traceutil.Duration:
			nano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			sp.duration = nano / 1000
		case traceutil.StatusCode:
			if field.Value == "2" {
				spanTagList = append(spanTagList, keyValue{key: "error", vStr: "true"})
			}
		case traceutil.StatusMessage:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "otel.status_description", vStr: field.Value})
			}
		case traceutil.TraceState:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "w3c.tracestate", vStr: field.Value})
			}
		// resource level fields
		case traceutil.ResourceAttrServiceName:
			sp.process.serviceName = field.Value
		// scope level fields
		case traceutil.InstrumentationScopeName:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "otel.scope.name", vStr: field.Value})
			}
		case traceutil.InstrumentationScopeVersion:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "otel.scope.version", vStr: field.Value})
			}
		default:
			if strings.HasPrefix(field.Name, traceutil.ResourceAttrPrefix) {
				processTagList = append(processTagList, keyValue{key: strings.TrimPrefix(field.Name, traceutil.ResourceAttrPrefix), vStr: field.Value})
			} else if strings.HasPrefix(field.Name, traceutil.SpanAttrPrefix) {
				spanTagList = append(spanTagList, keyValue{key: strings.TrimPrefix(field.Name, traceutil.SpanAttrPrefix), vStr: field.Value})
			} else if strings.HasPrefix(field.Name, traceutil.InstrumentationScopeAttrPrefix) {
				spanTagList = append(spanTagList, keyValue{key: strings.TrimPrefix(field.Name, traceutil.InstrumentationScopeAttrPrefix), vStr: field.Value})
			} else if strings.HasPrefix(field.Name, traceutil.EventPrefix) {
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, traceutil.EventPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid event field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := logsMap[idx]; !ok {
					logsMap[idx] = &log{}
				}
				log := logsMap[idx]
				switch fieldName {
				case traceutil.EventTimeUnixNano:
					unixNano, _ := strconv.ParseInt(field.Value, 10, 64)
					log.timestamp = unixNano / 1000
				case traceutil.EventName:
					log.fields = append(log.fields, keyValue{key: "event", vStr: field.Value})
				case traceutil.EventDroppedAttributesCount:
					//no need to display
					//log.Fields = append(log.Fields, KeyValue{Key: fieldName, VStr: field.Value})
				default:
					log.fields = append(log.fields, keyValue{key: strings.TrimPrefix(fieldName, traceutil.EventAttrPrefix), vStr: field.Value})
				}
			} else if strings.HasPrefix(field.Name, traceutil.LinkAttrPrefix) {
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, traceutil.LinkAttrPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid link field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := refsMap[idx]; !ok {
					refsMap[idx] = &spanRef{
						refType: "FOLLOW_FROM", // default FOLLOW_FROM
					}
				}
				ref := refsMap[idx]
				switch fieldName {
				case traceutil.LinkTraceID:
					ref.traceID = field.Value
				case traceutil.LinkSpanID:
					ref.spanID = field.Value
				//case LinkTraceState:
				//case LinkFlags:
				//case LinkDroppedAttributesCount:
				default:
					if strings.TrimPrefix(field.Name, traceutil.LinkPrefix) == "opentracing.ref_type" && field.Value == "child_of" {
						ref.refType = "CHILD_OF" // CHILD_OF
					}
				}
			}
		}
	}

	sp.tags = spanTagList
	sp.process.tags = processTagList

	if parentSpanRef.spanID != "" {
		parentSpanRef.traceID = sp.traceID
		sp.references = append(sp.references, parentSpanRef)
	}
	for i := 0; i < len(refsMap); i++ {
		idx := strconv.Itoa(i)
		if len(sp.references) > 0 && parentSpanRef.traceID == refsMap[idx].traceID && parentSpanRef.spanID == refsMap[idx].spanID {
			// We already added a reference to this span, but maybe with the wrong type, so override.
			sp.references[0].refType = refsMap[idx].refType
			continue
		}
		sp.references = append(sp.references, spanRef{
			refsMap[idx].traceID, refsMap[idx].spanID, refsMap[idx].refType,
		})
	}
	for i := 0; i < len(logsMap); i++ {
		idx := strconv.Itoa(i)
		sp.logs = append(sp.logs, log{
			logsMap[idx].timestamp, logsMap[idx].fields,
		})
	}

	if sp.spanID != "" {
		return sp, nil
	}
	return nil, fmt.Errorf("invalid fields: %v", fields)
}
