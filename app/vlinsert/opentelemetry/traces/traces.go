package traces

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

var maxRequestSize = flagutil.NewBytes("opentelemetry.traces.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single OpenTelemetry trace export request.")

var (
	requestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)

	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
)

var (
	defaultStreamFields = []string{pb.ResourceAttrServiceName, pb.NameField}
)

// HandleProtobuf handles the trace ingestion request.
func HandleProtobuf(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsProtobufTotal.Inc()

	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}
	if len(cp.StreamFields) == 0 {
		cp.StreamFields = defaultStreamFields
	}

	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	err = protoparserutil.ReadUncompressedData(r.Body, encoding, maxRequestSize, func(data []byte) error {
		lmp := cp.NewLogMessageProcessor("opentelemetry_traces_protobuf", false)
		err := pushProtobufRequest(data, lmp)
		lmp.MustClose()
		return err
	})
	if err != nil {
		httpserver.Errorf(w, r, "cannot read OpenTelemetry protocol data: %s", err)
		return
	}

	// update requestProtobufDuration only for successfully parsed requests
	// There is no need in updating requestProtobufDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestProtobufDuration.UpdateDuration(startTime)
}

func pushProtobufRequest(data []byte, lmp insertutil.LogMessageProcessor) error {
	var req pb.ExportTraceServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		errorsTotal.Inc()
		return fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	var commonFields []logstorage.Field
	for _, rs := range req.ResourceSpans {
		commonFields = commonFields[:0]
		attributes := rs.Resource.Attributes
		commonFields = appendKeyValuesWithPrefix(commonFields, attributes, "", pb.ResourceAttrPrefix)
		commonFieldsLen := len(commonFields)
		for _, ss := range rs.ScopeSpans {
			commonFields = pushFieldsFromScopeSpans(ss, commonFields[:commonFieldsLen], lmp)
		}
	}
	return nil
}

func pushFieldsFromScopeSpans(ss *pb.ScopeSpans, commonFields []logstorage.Field, lmp insertutil.LogMessageProcessor) []logstorage.Field {
	commonFields = append(commonFields, logstorage.Field{
		Name:  pb.InstrumentationScopeName,
		Value: ss.Scope.Name,
	}, logstorage.Field{
		Name:  pb.InstrumentationScopeVersion,
		Value: ss.Scope.Version,
	})
	commonFields = appendKeyValuesWithPrefix(commonFields, ss.Scope.Attributes, "", pb.InstrumentationScopeAttrPrefix)
	commonFieldsLen := len(commonFields)
	for _, span := range ss.Spans {
		commonFields = pushFieldsFromSpan(span, commonFields[:commonFieldsLen], lmp)
	}
	return commonFields
}

func pushFieldsFromSpan(span *pb.Span, scopeCommonFields []logstorage.Field, lmp insertutil.LogMessageProcessor) []logstorage.Field {
	fields := scopeCommonFields
	fields = append(fields,
		logstorage.Field{Name: pb.TraceIDField, Value: span.TraceID},
		logstorage.Field{Name: pb.SpanIDField, Value: span.SpanID},
		logstorage.Field{Name: pb.TraceStateField, Value: span.TraceState},
		logstorage.Field{Name: pb.ParentSpanIDField, Value: span.ParentSpanID},
		logstorage.Field{Name: pb.FlagsField, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: pb.NameField, Value: span.Name},
		logstorage.Field{Name: pb.KindField, Value: strconv.FormatInt(int64(span.Kind), 10)},
		logstorage.Field{Name: pb.StartTimeUnixNanoField, Value: strconv.FormatUint(span.StartTimeUnixNano, 10)},
		logstorage.Field{Name: pb.EndTimeUnixNanoField, Value: strconv.FormatUint(span.EndTimeUnixNano, 10)},
		logstorage.Field{Name: pb.DurationField, Value: strconv.FormatUint(span.EndTimeUnixNano-span.StartTimeUnixNano, 10)},

		logstorage.Field{Name: pb.DroppedAttributesCountField, Value: strconv.FormatUint(uint64(span.DroppedAttributesCount), 10)},
		logstorage.Field{Name: pb.DroppedEventsCountField, Value: strconv.FormatUint(uint64(span.DroppedEventsCount), 10)},
		logstorage.Field{Name: pb.DroppedLinksCountField, Value: strconv.FormatUint(uint64(span.DroppedLinksCount), 10)},

		logstorage.Field{Name: pb.StatusMessageField, Value: span.Status.Message},
		logstorage.Field{Name: pb.StatusCodeField, Value: strconv.FormatInt(int64(span.Status.Code), 10)},
	)

	// append span attributes
	fields = appendKeyValuesWithPrefix(fields, span.Attributes, "", pb.SpanAttrPrefixField)

	for idx, event := range span.Events {
		eventFieldPrefix := pb.EventPrefix + strconv.Itoa(idx) + ":"
		fields = append(fields,
			logstorage.Field{Name: eventFieldPrefix + pb.EventTimeUnixNanoField, Value: strconv.FormatUint(event.TimeUnixNano, 10)},
			logstorage.Field{Name: eventFieldPrefix + pb.EventNameField, Value: event.Name},
			logstorage.Field{Name: eventFieldPrefix + pb.EventDroppedAttributesCountField, Value: strconv.FormatUint(uint64(event.DroppedAttributesCount), 10)},
		)
		// append event attributes
		fields = appendKeyValuesWithPrefix(fields, event.Attributes, "", eventFieldPrefix+pb.EventAttrPrefix)
	}

	for idx, link := range span.Links {
		linkFieldPrefix := pb.LinkPrefix + strconv.Itoa(idx) + ":"

		fields = append(fields,
			logstorage.Field{Name: linkFieldPrefix + pb.LinkTraceIDField, Value: link.TraceID},
			logstorage.Field{Name: linkFieldPrefix + pb.LinkSpanIDField, Value: link.SpanID},
			logstorage.Field{Name: linkFieldPrefix + pb.LinkTraceStateField, Value: link.TraceState},
			logstorage.Field{Name: linkFieldPrefix + pb.LinkDroppedAttributesCountField, Value: strconv.FormatUint(uint64(link.DroppedAttributesCount), 10)},
			logstorage.Field{Name: linkFieldPrefix + pb.LinkFlagsField, Value: strconv.FormatUint(uint64(link.Flags), 10)},
		)

		// append link attributes
		fields = appendKeyValuesWithPrefix(fields, link.Attributes, "", linkFieldPrefix+pb.LinkAttrPrefix)
	}
	lmp.AddRow(int64(span.EndTimeUnixNano), fields, nil)
	return fields
}

func appendKeyValuesWithPrefix(fields []logstorage.Field, kvs []*pb.KeyValue, parentField, prefix string) []logstorage.Field {
	for _, attr := range kvs {
		fieldName := attr.Key
		if parentField != "" {
			fieldName = parentField + "." + fieldName
		}

		if attr.Value.KeyValueList != nil {
			fields = appendKeyValuesWithPrefix(fields, attr.Value.KeyValueList.Values, fieldName, prefix)
			continue
		}

		v := attr.Value.FormatString(true)
		if len(v) == 0 {
			// VictoriaLogs does not support empty string as field value. set it to "-" to preserve the field.
			v = "-"
		}
		fields = append(fields, logstorage.Field{
			Name:  prefix + fieldName,
			Value: v,
		})
	}
	return fields
}
