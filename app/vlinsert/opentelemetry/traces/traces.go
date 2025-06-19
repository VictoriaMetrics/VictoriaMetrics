package traces

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	otelpb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
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
	mandatoryStreamFields = []string{otelpb.ResourceAttrServiceName, otelpb.NameField}
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
	// stream fields must contain the service name and span name.
	// by using arguments and headers, users can also add other fields as stream fields
	// for potentially better efficiency.
	cp.StreamFields = append(mandatoryStreamFields, cp.StreamFields...)

	if err := insertutil.CanWriteData(); err != nil {
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
	var req otelpb.ExportTraceServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		errorsTotal.Inc()
		return fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	var commonFields []logstorage.Field
	for _, rs := range req.ResourceSpans {
		commonFields = commonFields[:0]
		attributes := rs.Resource.Attributes
		commonFields = appendKeyValuesWithPrefix(commonFields, attributes, "", otelpb.ResourceAttrPrefix)
		commonFieldsLen := len(commonFields)
		for _, ss := range rs.ScopeSpans {
			commonFields = pushFieldsFromScopeSpans(ss, commonFields[:commonFieldsLen], lmp)
		}
	}
	return nil
}

func pushFieldsFromScopeSpans(ss *otelpb.ScopeSpans, commonFields []logstorage.Field, lmp insertutil.LogMessageProcessor) []logstorage.Field {
	commonFields = append(commonFields, logstorage.Field{
		Name:  otelpb.InstrumentationScopeName,
		Value: ss.Scope.Name,
	}, logstorage.Field{
		Name:  otelpb.InstrumentationScopeVersion,
		Value: ss.Scope.Version,
	})
	commonFields = appendKeyValuesWithPrefix(commonFields, ss.Scope.Attributes, "", otelpb.InstrumentationScopeAttrPrefix)
	commonFieldsLen := len(commonFields)
	for _, span := range ss.Spans {
		commonFields = pushFieldsFromSpan(span, commonFields[:commonFieldsLen], lmp)
	}
	return commonFields
}

func pushFieldsFromSpan(span *otelpb.Span, scopeCommonFields []logstorage.Field, lmp insertutil.LogMessageProcessor) []logstorage.Field {
	fields := scopeCommonFields
	fields = append(fields,
		logstorage.Field{Name: otelpb.TraceIDField, Value: span.TraceID},
		logstorage.Field{Name: otelpb.SpanIDField, Value: span.SpanID},
		logstorage.Field{Name: otelpb.TraceStateField, Value: span.TraceState},
		logstorage.Field{Name: otelpb.ParentSpanIDField, Value: span.ParentSpanID},
		logstorage.Field{Name: otelpb.FlagsField, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: otelpb.NameField, Value: span.Name},
		logstorage.Field{Name: otelpb.KindField, Value: strconv.FormatInt(int64(span.Kind), 10)},
		logstorage.Field{Name: otelpb.StartTimeUnixNanoField, Value: strconv.FormatUint(span.StartTimeUnixNano, 10)},
		logstorage.Field{Name: otelpb.EndTimeUnixNanoField, Value: strconv.FormatUint(span.EndTimeUnixNano, 10)},
		logstorage.Field{Name: otelpb.DurationField, Value: strconv.FormatUint(span.EndTimeUnixNano-span.StartTimeUnixNano, 10)},

		logstorage.Field{Name: otelpb.DroppedAttributesCountField, Value: strconv.FormatUint(uint64(span.DroppedAttributesCount), 10)},
		logstorage.Field{Name: otelpb.DroppedEventsCountField, Value: strconv.FormatUint(uint64(span.DroppedEventsCount), 10)},
		logstorage.Field{Name: otelpb.DroppedLinksCountField, Value: strconv.FormatUint(uint64(span.DroppedLinksCount), 10)},

		logstorage.Field{Name: otelpb.StatusMessageField, Value: span.Status.Message},
		logstorage.Field{Name: otelpb.StatusCodeField, Value: strconv.FormatInt(int64(span.Status.Code), 10)},
	)

	// append span attributes
	fields = appendKeyValuesWithPrefix(fields, span.Attributes, "", otelpb.SpanAttrPrefixField)

	for idx, event := range span.Events {
		eventFieldPrefix := otelpb.EventPrefix + strconv.Itoa(idx) + ":"
		fields = append(fields,
			logstorage.Field{Name: eventFieldPrefix + otelpb.EventTimeUnixNanoField, Value: strconv.FormatUint(event.TimeUnixNano, 10)},
			logstorage.Field{Name: eventFieldPrefix + otelpb.EventNameField, Value: event.Name},
			logstorage.Field{Name: eventFieldPrefix + otelpb.EventDroppedAttributesCountField, Value: strconv.FormatUint(uint64(event.DroppedAttributesCount), 10)},
		)
		// append event attributes
		fields = appendKeyValuesWithPrefix(fields, event.Attributes, "", eventFieldPrefix+otelpb.EventAttrPrefix)
	}

	for idx, link := range span.Links {
		linkFieldPrefix := otelpb.LinkPrefix + strconv.Itoa(idx) + ":"

		fields = append(fields,
			logstorage.Field{Name: linkFieldPrefix + otelpb.LinkTraceIDField, Value: link.TraceID},
			logstorage.Field{Name: linkFieldPrefix + otelpb.LinkSpanIDField, Value: link.SpanID},
			logstorage.Field{Name: linkFieldPrefix + otelpb.LinkTraceStateField, Value: link.TraceState},
			logstorage.Field{Name: linkFieldPrefix + otelpb.LinkDroppedAttributesCountField, Value: strconv.FormatUint(uint64(link.DroppedAttributesCount), 10)},
			logstorage.Field{Name: linkFieldPrefix + otelpb.LinkFlagsField, Value: strconv.FormatUint(uint64(link.Flags), 10)},
		)

		// append link attributes
		fields = appendKeyValuesWithPrefix(fields, link.Attributes, "", linkFieldPrefix+otelpb.LinkAttrPrefix)
	}
	lmp.AddRow(int64(span.EndTimeUnixNano), fields, nil)
	return fields
}

func appendKeyValuesWithPrefix(fields []logstorage.Field, kvs []*otelpb.KeyValue, parentField, prefix string) []logstorage.Field {
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
