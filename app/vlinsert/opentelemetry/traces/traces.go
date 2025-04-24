package traces

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/metrics"
	"net/http"
	"strings"
)

var (
	requestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)

	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/v1/traces",format="protobuf"}`)
)

func HandleProtobuf(r *http.Request, w http.ResponseWriter) {

}

func pushProtobufRequest(data []byte, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) error {
	var req pb.ExportTraceServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		errorsTotal.Inc()
		return fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	// crazy for-loop to attach trace span attributes to log fields.
	for _, resourceSpan := range req.ResourceSpans {
		// key-values of resource
		for _, attr := range resourceSpan.Resource.Attributes {
			fmt.Println(attr)
		}

		for _, scopeSpan := range resourceSpan.ScopeSpans {
			// key-values of scope
			for _, attr := range scopeSpan.Scope.Attributes {
				fmt.Println(attr)
			}

			for _, span := range scopeSpan.Spans {
				for _, event := range span.Events {
					// key-values of event
					for _, attr := range event.Attributes {
						fmt.Println(attr)
					}
				}
				for _, link := range span.Links {
					// key-values of link
					for _, attr := range link.Attributes {
						fmt.Println(attr)
					}
				}

				// Finally!!!!!! key-values of span
				for _, attr := range span.Attributes {
					fmt.Println(attr)
				}
			}
		}
	}

	var commonFields []logstorage.Field
	for _, rs := range req.ResourceSpans {
		attributes := rs.Resource.Attributes
		commonFields = slicesutil.SetLength(commonFields, len(attributes))
		for i, attr := range attributes {
			commonFields[i].Name = attr.Key
			commonFields[i].Value = attr.Value.FormatString(true)
		}
		commonFieldsLen := len(commonFields)
		for _, ss := range rs.ScopeSpans {
			commonFields = commonFields[:commonFieldsLen]
			commonFields = append(commonFields, logstorage.Field{
				Name:  "scope_name",
				Value: strings.Clone(ss.Scope.Name),
			}, logstorage.Field{
				Name:  "scope_version",
				Value: strings.Clone(ss.Scope.Version),
			})
			for _, attr := range ss.Scope.Attributes {
				commonFields = append(commonFields, logstorage.Field{
					Name:  "scope_attribute_" + attr.Key,
					Value: attr.Value.FormatString(true),
				})
			}
			commonFields = pushFieldsFromScopeSpans(ss, commonFields, lmp, useDefaultStreamFields)
		}
	}

	return nil
}

func pushFieldsFromScopeSpans(ss *pb.ScopeSpans, scopeFields []logstorage.Field, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) []logstorage.Field {
	spanFields := scopeFields
	for _, span := range ss.Spans {
		spanFields = spanFields[:len(scopeFields)]
		spanFields = append()
	}
}

func pushFieldsFromSpan(span *pb.Span, commonFields []logstorage.Field, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) []logstorage.Field {

}
