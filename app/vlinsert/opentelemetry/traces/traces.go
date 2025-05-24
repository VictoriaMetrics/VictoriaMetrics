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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/traceutil"
	"github.com/VictoriaMetrics/metrics"
)

var maxRequestSize = flagutil.NewBytes("opentelemetry.traces.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single OpenTelemetry trace export request.")

var (
	requestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)

	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
)

var (
	defaultStreamFields = []string{traceutil.ResourceAttrServiceName, traceutil.Name}
)

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
		lmp := cp.NewLogMessageProcessor("opentelelemtry_traces_protobuf", false)
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
		attributes := rs.Resource.Attributes
		commonFields = slicesutil.SetLength(commonFields, len(attributes))
		for i, attr := range attributes {
			commonFields[i].Name = traceutil.ResourceAttrPrefix + attr.Key
			commonFields[i].Value = attr.Value.FormatString(true)
		}
		commonFieldsLen := len(commonFields)
		for _, ss := range rs.ScopeSpans {
			commonFields = pushFieldsFromScopeSpans(ss, commonFields[:commonFieldsLen], lmp)
		}
	}
	return nil
}

func pushFieldsFromScopeSpans(ss *pb.ScopeSpans, commonFields []logstorage.Field, lmp insertutil.LogMessageProcessor) []logstorage.Field {
	commonFields = append(commonFields, logstorage.Field{
		Name:  traceutil.InstrumentationScopeName,
		Value: ss.Scope.Name,
	}, logstorage.Field{
		Name:  traceutil.InstrumentationScopeVersion,
		Value: ss.Scope.Version,
	})
	for _, attr := range ss.Scope.Attributes {
		commonFields = append(commonFields, logstorage.Field{
			Name:  traceutil.InstrumentationScopeAttrPrefix + attr.Key,
			Value: attr.Value.FormatString(true),
		})
	}
	commonFieldsLen := len(commonFields)
	for _, span := range ss.Spans {
		commonFields = pushFieldsFromSpan(span, commonFields[:commonFieldsLen], lmp)
	}
	return commonFields
}

func pushFieldsFromSpan(span *pb.Span, scopeCommonFields []logstorage.Field, lmp insertutil.LogMessageProcessor) []logstorage.Field {
	fields := scopeCommonFields
	fields = append(fields,
		logstorage.Field{Name: traceutil.TraceId, Value: span.TraceId},
		logstorage.Field{Name: traceutil.SpanId, Value: span.SpanId},
		logstorage.Field{Name: traceutil.TraceState, Value: span.TraceState},
		logstorage.Field{Name: traceutil.ParentSpanId, Value: span.ParentSpanId},
		logstorage.Field{Name: traceutil.Flags, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: traceutil.Name, Value: span.Name},
		logstorage.Field{Name: traceutil.Kind, Value: strconv.FormatInt(int64(span.Kind), 10)},
		logstorage.Field{Name: traceutil.StartTimeUnixNano, Value: strconv.FormatUint(span.StartTimeUnixNano, 10)},
		logstorage.Field{Name: traceutil.EndTimeUnixNano, Value: strconv.FormatUint(span.EndTimeUnixNano, 10)},
		logstorage.Field{Name: traceutil.Duration, Value: strconv.FormatUint(span.EndTimeUnixNano-span.StartTimeUnixNano, 10)},

		logstorage.Field{Name: traceutil.DroppedAttributesCount, Value: strconv.FormatUint(uint64(span.DroppedAttributesCount), 10)},
		logstorage.Field{Name: traceutil.DroppedEventsCount, Value: strconv.FormatUint(uint64(span.DroppedEventsCount), 10)},
		logstorage.Field{Name: traceutil.DroppedLinksCount, Value: strconv.FormatUint(uint64(span.DroppedLinksCount), 10)},

		logstorage.Field{Name: traceutil.StatusMessage, Value: span.Status.Message},
		logstorage.Field{Name: traceutil.StatusCode, Value: strconv.FormatInt(int64(span.Status.Code), 10)},
	)

	for _, attr := range span.Attributes {
		v := attr.Value.FormatString(true)
		if len(v) == 0 {
			// VictoriaLogs does not support empty string as field value. set it to "-" to preserve the field.
			v = "-"
		}
		fields = append(fields, logstorage.Field{
			Name:  traceutil.SpanAttrPrefix + attr.Key,
			Value: v,
		})
	}

	for idx, event := range span.Events {
		eventFieldPrefix := traceutil.EventPrefix + strconv.Itoa(idx) + ":"
		fields = append(fields,
			logstorage.Field{Name: eventFieldPrefix + traceutil.EventTimeUnixNano, Value: strconv.FormatUint(event.TimeUnixNano, 10)},
			logstorage.Field{Name: eventFieldPrefix + traceutil.EventName, Value: event.Name},
			logstorage.Field{Name: eventFieldPrefix + traceutil.EventDroppedAttributesCount, Value: strconv.FormatUint(uint64(event.DroppedAttributesCount), 10)},
		)
		for _, eventAttr := range event.Attributes {
			v := eventAttr.Value.FormatString(true)
			if len(v) == 0 {
				// VictoriaLogs does not support empty string as field value. set it to "-" to preserve the field.
				v = "-"
			}
			fields = append(fields, logstorage.Field{
				Name:  eventFieldPrefix + traceutil.EventAttrPrefix + eventAttr.Key,
				Value: v,
			})
		}
	}

	for idx, link := range span.Links {
		linkFieldPrefix := traceutil.LinkPrefix + strconv.Itoa(idx) + ":"

		fields = append(fields,
			logstorage.Field{Name: linkFieldPrefix + traceutil.LinkTraceId, Value: link.TraceId},
			logstorage.Field{Name: linkFieldPrefix + traceutil.LinkSpanId, Value: link.SpanId},
			logstorage.Field{Name: linkFieldPrefix + traceutil.LinkTraceState, Value: traceutil.LinkTraceState},
			logstorage.Field{Name: linkFieldPrefix + traceutil.LinkDroppedAttributesCount, Value: strconv.FormatUint(uint64(link.DroppedAttributesCount), 10)},
			logstorage.Field{Name: linkFieldPrefix + traceutil.LinkFlags, Value: strconv.FormatUint(uint64(link.Flags), 10)},
		)

		for _, linkAttr := range link.Attributes {
			v := linkAttr.Value.FormatString(true)
			if len(v) == 0 {
				// VictoriaLogs does not support empty string as field value. set it to "-" to preserve the field.
				v = "-"
			}
			fields = append(fields, logstorage.Field{
				Name:  linkFieldPrefix + traceutil.LinkAttrPrefix + linkAttr.Key,
				Value: v,
			})
		}
	}
	lmp.AddRow(int64(span.EndTimeUnixNano), fields, nil)
	return fields
}
