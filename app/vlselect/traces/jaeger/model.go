package jaeger

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
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
	process   process
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

// since Jaeger renamed some fields in OpenTelemetry
// into other span attributes during query, the following map
// is created to translate the span attributes filter into the
// original field names in OpenTelemetry (VictoriaTraces).
//
// format: <special span attributes in Jaeger>: <fields in OpenTelemetry>
var spanAttributeMap = map[string]string{
	// special cases that need to map string to int status code, see errorStatusCodeMap
	"error":     pb.StatusCodeField,
	"span.kind": pb.KindField,

	// only attributes/field name conversion.
	"otel.status_description": pb.StatusMessageField,
	"w3c.tracestate":          pb.TraceStateField,
	"otel.scope.name":         pb.InstrumentationScopeName,
	"otel.scope.version":      pb.InstrumentationScopeVersion,
	// scope attributes
}

var errorStatusCodeMap = map[string]string{
	"unset": "0",
	"true":  "2",
	"false": "1",
}

var spanKindMap = map[string]string{
	"internal": "1",
	"server":   "2",
	"client":   "3",
	"producer": "4",
	"consumer": "5",
}

// fieldsToSpan convert OTLP spans in fields to Jaeger Spans.
func fieldsToSpan(fields []logstorage.Field) (*span, error) {
	sp := &span{}

	processTagList, spanTagList := make([]keyValue, 0, len(fields)), make([]keyValue, 0, len(fields))
	logsMap := make(map[string]*log)     // idx -> *Log
	refsMap := make(map[string]*spanRef) // idx -> *SpanRef

	parentSpanRef := spanRef{}
	for _, field := range fields {
		switch field.Name {
		case "_stream":
			// no-op
		case pb.TraceIDField:
			sp.traceID = field.Value
		case pb.SpanIDField:
			sp.spanID = field.Value
		case pb.NameField:
			sp.operationName = field.Value
		case pb.ParentSpanIDField:
			parentSpanRef.spanID = field.Value
			parentSpanRef.refType = "CHILD_OF"
		case pb.KindField:
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
				default:
					// unexpected span kind.
					// this line does nothing should never be reached.
				}
				spanTagList = append(spanTagList, keyValue{key: "span.kind", vStr: spanKind})
			}
		case pb.FlagsField:
			// todo trace does not contain "flag" in result
			//flagU64, err := strconv.ParseUint(field.Value, 10, 32)
			//if err != nil {
			//	return nil, err
			//}
			//sp.Flags = uint32(flagU64)
		case pb.StartTimeUnixNanoField:
			unixNano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid start_time_unix_nano field: %s", err)
			}
			sp.startTime = unixNano / 1000
		case pb.DurationField:
			nano, err := strconv.ParseInt(field.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid duration field: %s", err)
			}
			sp.duration = nano / 1000
		case pb.StatusCodeField:
			v := "unset"
			switch field.Value {
			case "1":
				v = "false"
			case "2":
				v = "true"
			}
			spanTagList = append(spanTagList, keyValue{key: "error", vStr: v})
		case pb.StatusMessageField:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "otel.status_description", vStr: field.Value})
			}
		case pb.TraceStateField:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "w3c.tracestate", vStr: field.Value})
			}
		// resource level fields
		case pb.ResourceAttrServiceName:
			sp.process.serviceName = field.Value
		// scope level fields
		case pb.InstrumentationScopeName:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "otel.scope.name", vStr: field.Value})
			}
		case pb.InstrumentationScopeVersion:
			if field.Value != "" {
				spanTagList = append(spanTagList, keyValue{key: "otel.scope.version", vStr: field.Value})
			}
		default:
			if strings.HasPrefix(field.Name, pb.ResourceAttrPrefix) { // resource attributes
				processTagList = append(processTagList, keyValue{key: strings.TrimPrefix(field.Name, pb.ResourceAttrPrefix), vStr: field.Value})
			} else if strings.HasPrefix(field.Name, pb.SpanAttrPrefixField) { // span attributes
				spanTagList = append(spanTagList, keyValue{key: strings.TrimPrefix(field.Name, pb.SpanAttrPrefixField), vStr: field.Value})
			} else if strings.HasPrefix(field.Name, pb.InstrumentationScopeAttrPrefix) { // instrumentation scope attributes
				// we have to display `scope_attr:` prefix as there's no way to distinguish these from span attributes.
				spanTagList = append(spanTagList, keyValue{key: field.Name, vStr: field.Value})
			} else if strings.HasPrefix(field.Name, pb.EventPrefix) { // event list
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, pb.EventPrefix), ":", 2)
				if len(fieldSplit) != 2 {
					return nil, fmt.Errorf("invalid event field: %s", field.Name)
				}
				idx, fieldName := fieldSplit[0], fieldSplit[1]
				if _, ok := logsMap[idx]; !ok {
					logsMap[idx] = &log{}
				}
				lg := logsMap[idx]
				switch fieldName {
				case pb.EventTimeUnixNanoField:
					unixNano, _ := strconv.ParseInt(field.Value, 10, 64)
					lg.timestamp = unixNano / 1000
				case pb.EventNameField:
					lg.fields = append(lg.fields, keyValue{key: "event", vStr: field.Value})
				case pb.EventDroppedAttributesCountField:
					//no need to display
					//lg.Fields = append(lg.Fields, KeyValue{Key: fieldName, VStr: field.Value})
				default:
					lg.fields = append(lg.fields, keyValue{key: strings.TrimPrefix(fieldName, pb.EventAttrPrefix), vStr: field.Value})
				}
			} else if strings.HasPrefix(field.Name, pb.LinkPrefix) { // link list
				fieldSplit := strings.SplitN(strings.TrimPrefix(field.Name, pb.LinkPrefix), ":", 2)
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
				case pb.LinkTraceIDField:
					ref.traceID = field.Value
				case pb.LinkSpanIDField:
					ref.spanID = field.Value
				case pb.LinkTraceStateField, pb.LinkFlagsField, pb.LinkDroppedAttributesCountField:
				default:
					if strings.TrimPrefix(fieldName, pb.LinkAttrPrefix) == "opentracing.ref_type" && field.Value == "child_of" {
						ref.refType = "CHILD_OF" // CHILD_OF
					}
				}
			}
		}
	}

	if sp.spanID == "" || sp.traceID == "" {
		return nil, fmt.Errorf("invalid fields: %v", fields)
	}

	if len(spanTagList) > 0 {
		sp.tags = spanTagList
	}

	if len(processTagList) > 0 {
		sp.process.tags = processTagList
	}

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

	return sp, nil
}
