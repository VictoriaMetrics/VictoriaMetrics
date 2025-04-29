package traces

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/metrics"
	"net/http"
	"strconv"
	"time"
)

var maxRequestSize = flagutil.NewBytes("opentelemetry.traces.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single OpenTelemetry trace export request.")

var (
	requestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)

	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
)

func HandleProtobuf(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsProtobufTotal.Inc()

	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	err = protoparserutil.ReadUncompressedData(r.Body, encoding, maxRequestSize, func(data []byte) error {
		lmp := cp.NewLogMessageProcessor("opentelelemtry_protobuf", false)
		useDefaultStreamFields := len(cp.StreamFields) == 0
		err := pushProtobufRequest(data, lmp, useDefaultStreamFields)
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

func pushProtobufRequest(data []byte, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) error {
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
			commonFields[i].Name = ResourceAttrPrefix + attr.Key
			commonFields[i].Value = attr.Value.FormatString(true)
		}
		commonFieldsLen := len(commonFields)
		for _, ss := range rs.ScopeSpans {
			commonFields = pushFieldsFromScopeSpans(ss, commonFields[:commonFieldsLen], lmp, useDefaultStreamFields)
		}
	}
	return nil
}

func pushFieldsFromScopeSpans(ss *pb.ScopeSpans, commonFields []logstorage.Field, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) []logstorage.Field {
	commonFields = append(commonFields, logstorage.Field{
		Name:  InstrumentationScopeName,
		Value: ss.Scope.Name,
	}, logstorage.Field{
		Name:  InstrumentationScopeVersion,
		Value: ss.Scope.Version,
	})
	for _, attr := range ss.Scope.Attributes {
		commonFields = append(commonFields, logstorage.Field{
			Name:  instrumentationScopeAttrPrefix + attr.Key,
			Value: attr.Value.FormatString(true),
		})
	}
	commonFieldsLen := len(commonFields)
	for _, span := range ss.Spans {
		commonFields = pushFieldsFromSpan(span, commonFields[:commonFieldsLen], lmp, useDefaultStreamFields)
	}
	return commonFields
}

func pushFieldsFromSpan(span *pb.Span, scopeCommonFields []logstorage.Field, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) []logstorage.Field {
	fields := scopeCommonFields
	fields = append(fields,
		logstorage.Field{Name: TraceId, Value: span.TraceId},
		logstorage.Field{Name: SpanId, Value: span.SpanId},
		logstorage.Field{Name: TraceState, Value: span.TraceState},
		logstorage.Field{Name: ParentSpanId, Value: span.ParentSpanId},
		logstorage.Field{Name: Flags, Value: strconv.FormatUint(uint64(span.Flags), 10)},
		logstorage.Field{Name: Name, Value: span.Name},
		logstorage.Field{Name: Kind, Value: strconv.FormatInt(int64(span.Kind), 10)},
		logstorage.Field{Name: StartTimeUnixNano, Value: strconv.FormatUint(span.StartTimeUnixNano, 10)},
		logstorage.Field{Name: EndTimeUnixNano, Value: strconv.FormatUint(span.EndTimeUnixNano, 10)},

		logstorage.Field{Name: DroppedAttributesCount, Value: strconv.FormatUint(uint64(span.DroppedAttributesCount), 10)},
		logstorage.Field{Name: DroppedEventsCount, Value: strconv.FormatUint(uint64(span.DroppedEventsCount), 10)},
		logstorage.Field{Name: DroppedLinksCount, Value: strconv.FormatUint(uint64(span.DroppedLinksCount), 10)},
	)

	for _, attr := range span.Attributes {
		fields = append(fields, logstorage.Field{
			Name:  SpanAttrPrefix + attr.Key,
			Value: attr.Value.FormatString(true),
		})
	}

	for _, event := range span.Events {
		fields = append(fields,
			logstorage.Field{Name: EventTimeUnixNano, Value: strconv.FormatUint(event.TimeUnixNano, 10)},
			logstorage.Field{Name: EventName, Value: event.Name},
			logstorage.Field{Name: EventDroppedAttributesCount, Value: strconv.FormatUint(uint64(event.DroppedAttributesCount), 10)},
		)

		for _, eventAttr := range event.Attributes {
			fields = append(fields, logstorage.Field{
				Name:  EventAttrPrefix + eventAttr.Key,
				Value: eventAttr.Value.FormatString(true),
			})
		}
	}

	for _, link := range span.Links {
		fields = append(fields,
			logstorage.Field{Name: LinkTraceId, Value: link.TraceId},
			logstorage.Field{Name: LinkSpanId, Value: link.SpanId},
			logstorage.Field{Name: LinkTraceState, Value: LinkTraceState},
			logstorage.Field{Name: LinkDroppedAttributesCount, Value: strconv.FormatUint(uint64(link.DroppedAttributesCount), 10)},
			logstorage.Field{Name: LinkFlags, Value: strconv.FormatUint(uint64(link.Flags), 10)},
		)

		for _, linkAttr := range link.Attributes {
			fields = append(fields, logstorage.Field{
				Name:  LinkAttrPrefix + linkAttr.Key,
				Value: linkAttr.Value.FormatString(true),
			})
		}
	}
	var streamFields []logstorage.Field
	if useDefaultStreamFields {
		streamFields = scopeCommonFields
	}
	lmp.AddRow(int64(span.EndTimeUnixNano), fields, streamFields)
	return fields
}
