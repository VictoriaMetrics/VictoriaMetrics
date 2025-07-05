package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/traces/query"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/hashpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	otelpb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/metrics"
)

const (
	maxLimit = 1000
)

// Jaeger Query APIs metrics
var (
	jaegerServicesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/jaeger/api/services"}`)
	jaegerServicesDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/jaeger/api/services"}`)

	jaegerOperationsRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/jaeger/api/services/*/operations"}`)
	jaegerOperationsDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/jaeger/api/services/*/operations"}`)

	jaegerTracesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/jaeger/api/traces"}`)
	jaegerTracesDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/jaeger/api/traces"}`)

	jaegerTraceRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/jaeger/api/traces/*"}`)
	jaegerTraceDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/jaeger/api/traces/*"}`)

	jaegerDependenciesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/jaeger/api/dependencies"}`)
	jaegerDependenciesDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/jaeger/api/dependencies"}`)
)

// RequestHandler is the entry point for all Jaeger query APIs.
// Jaeger JSON API is intentionally undocumented.
// So APIs are based on the implementation in Jaeger repository.
// See:
// 1. https://www.jaegertracing.io/docs/1.70/architecture/apis/#http-json-internal
// 2. https://github.com/jaegertracing/jaeger/blob/9a45f522422c548827b2f3897affc8170e4a3d8b/cmd/query/app/http_handler.go#L110
func RequestHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	httpserver.EnableCORS(w, r)
	startTime := time.Now()
	path := r.URL.Path
	if path == "/select/jaeger/api/services" {
		jaegerServicesRequests.Inc()
		processGetServicesRequest(ctx, w, r)
		jaegerServicesDuration.UpdateDuration(startTime)
		return true
	} else if strings.HasPrefix(path, "/select/jaeger/api/services/") && strings.HasSuffix(path, "/operations") {
		jaegerOperationsRequests.Inc()
		processGetOperationsRequest(ctx, w, r)
		jaegerOperationsDuration.UpdateDuration(startTime)
		return true
	} else if path == "/select/jaeger/api/traces" {
		jaegerTracesRequests.Inc()
		processGetTracesRequest(ctx, w, r)
		jaegerTracesDuration.UpdateDuration(startTime)
		return true
	} else if strings.HasPrefix(path, "/select/jaeger/api/traces/") && len(path) > len("/select/jaeger/api/traces/") {
		jaegerTraceRequests.Inc()
		processGetTraceRequest(ctx, w, r)
		jaegerTraceDuration.UpdateDuration(startTime)
		return true
	} else if path == "/select/jaeger/api/dependencies" {
		jaegerDependenciesRequests.Inc()
		// todo it require additional component to calculate the dependency graph. not implemented yet.
		httpserver.Errorf(w, r, "/api/dependencies API is not supported yet.")
		jaegerDependenciesDuration.UpdateDuration(startTime)
		return true
	}
	return false
}

// processGetServicesRequest handle the Jaeger /api/services API request.
// https://github.com/jaegertracing/jaeger/blob/9a45f522422c548827b2f3897affc8170e4a3d8b/cmd/query/app/http_handler.go#L146
func processGetServicesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := query.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	serviceList, err := query.GetServiceNameList(ctx, cp)
	if err != nil {
		httpserver.Errorf(w, r, "cannot get services list: %s", err)
		return
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetServicesResponse(w, serviceList)
}

// processGetOperationsRequest handle the Jaeger /api/services/<service_name>/operations API request.
// https://github.com/jaegertracing/jaeger/blob/9a45f522422c548827b2f3897affc8170e4a3d8b/cmd/query/app/http_handler.go#L158
func processGetOperationsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := query.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	// extract the `service_name`.
	// the path must be like `/select/jaeger/api/services/<service_name>/operations`.
	u := r.URL.Path[len("/select/jaeger/api/services/"):]

	// check for invalid path: /select/jaeger/api/services/operations
	if !strings.Contains(u, "/") {
		httpserver.Errorf(w, r, "incorrect query path [%s]", r.URL.Path)
		return
	}
	serviceName := u[:len(u)-len("/operations")]
	if len(serviceName) == 0 {
		httpserver.Errorf(w, r, "incorrect query path [%s]", r.URL.Path)
		return
	}

	operationList, err := query.GetSpanNameList(ctx, cp, serviceName)
	if err != nil {
		httpserver.Errorf(w, r, "cannot get operation list: %s", err)
		return
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetOperationsResponse(w, operationList)
}

// processGetTraceRequest handle the Jaeger /api/traces/<trace_id> API request.
// https://github.com/jaegertracing/jaeger/blob/9a45f522422c548827b2f3897affc8170e4a3d8b/cmd/query/app/http_handler.go#L465
func processGetTraceRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := query.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	// extract the `trace_id`.
	// the path must be like `/select/jaeger/api/traces/<trace_id>`.
	traceID := r.URL.Path[len("/select/jaeger/api/traces/"):]
	if len(traceID) == 0 {
		httpserver.Errorf(w, r, "incorrect query path [%s]", r.URL.Path)
		return
	}

	rows, err := query.GetTrace(ctx, cp, traceID)
	if err != nil {
		httpserver.Errorf(w, r, "cannot get traces: %s", err)
		return
	}

	t := &trace{}
	processHashIDMap := make(map[uint64]string)     // process name -> process id
	processIDProcessMap := make(map[string]process) // process id -> process
	for i := range rows {
		var sp *span
		sp, err = fieldsToSpan(rows[i].Fields)
		if err != nil {
			logger.Errorf("cannot unmarshal log fields [%v] to span: %s", rows[i].Fields, err)
			continue
		}

		// Process ID
		processHash := hashProcess(sp.process)
		if _, ok := processHashIDMap[processHash]; !ok {
			processID := "p" + strconv.Itoa(len(processHashIDMap)+1)
			processHashIDMap[processHash] = processID
			processIDProcessMap[processID] = sp.process
		}

		sp.processID = processHashIDMap[processHash]
		t.spans = append(t.spans, sp)
	}

	// 6. attach process info to this trace
	t.processMap = make([]processMap, 0, len(processIDProcessMap))
	for processID, p := range processIDProcessMap {
		t.processMap = append(t.processMap, processMap{
			processID: processID,
			process:   p,
		})
	}

	sort.Slice(t.processMap, func(i, j int) bool {
		return t.processMap[i].processID < t.processMap[j].processID
	})

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetTracesResponse(w, []*trace{t})
}

// processGetTracesRequest handle the Jaeger /api/traces API request.
// https://github.com/jaegertracing/jaeger/blob/9a45f522422c548827b2f3897affc8170e4a3d8b/cmd/query/app/http_handler.go#L227
func processGetTracesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := query.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	param, err := parseJaegerTraceQueryParam(ctx, r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect trace query params: %s", err)
		return
	}

	traceIDList, rows, err := query.GetTraceList(ctx, cp, param)
	if err != nil {
		httpserver.Errorf(w, r, "get trace list error: %s", err)
		return
	}
	if len(rows) == 0 {
		// Write empty results
		w.Header().Set("Content-Type", "application/json")
		WriteGetTracesResponse(w, nil)
		return
	}

	// convert fields spans to jaeger spans, and group by trace_id.
	//
	// 1. prepare a trace_id -> *trace map
	tracesMap := make(map[string]*trace)
	traces := make([]*trace, len(traceIDList))
	for i := range traceIDList {
		traces[i] = &trace{}
		tracesMap[traceIDList[i]] = traces[i]
	}

	processHashMap := make(map[uint64]string)              // process_unique_hash -> pid
	traceProcessMap := make(map[string]map[string]process) // trace_id -> map[processID]->process
	for i := range rows {
		// 2. convert fields to jaeger spans.
		var sp *span
		sp, err = fieldsToSpan(rows[i].Fields)
		if err != nil {
			logger.Errorf("cannot unmarshal log fields [%v] to span: %s", rows[i].Fields, err)
			continue
		}

		// 3. calculate the process that this span belongs to
		procHash := hashProcess(sp.process)
		if _, ok := processHashMap[procHash]; !ok {
			// format process id as Jaeger does: `p{idx}`, where {idx} starts from 1.
			processHashMap[procHash] = "p" + strconv.Itoa(len(processHashMap)+1)
		}
		// and attach the process info to the span.
		sp.processID = processHashMap[procHash]

		// 4. add the process info to this trace (if process not exists).
		if _, ok := traceProcessMap[sp.traceID]; !ok {
			traceProcessMap[sp.traceID] = make(map[string]process)
		}
		if _, ok := traceProcessMap[sp.traceID][sp.processID]; !ok {
			traceProcessMap[sp.traceID][sp.processID] = sp.process
		}

		// 5. append this span to the trace it belongs to.
		tracesMap[sp.traceID].spans = append(tracesMap[sp.traceID].spans, sp)
	}

	// 6. attach process info to each trace
	for traceID, trace := range tracesMap {
		trace.processMap = make([]processMap, 0, len(traceProcessMap[traceID]))
		for processID, process := range traceProcessMap[traceID] {
			trace.processMap = append(trace.processMap, processMap{
				processID: processID,
				process:   process,
			})
		}

		sort.Slice(trace.processMap, func(i, j int) bool {
			return trace.processMap[i].processID < trace.processMap[j].processID
		})
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetTracesResponse(w, traces)
}

// parseJaegerTraceQueryParam parse Jaeger request to unified query.TraceQueryParam.
func parseJaegerTraceQueryParam(_ context.Context, r *http.Request) (*query.TraceQueryParam, error) {
	var err error

	// default params
	p := &query.TraceQueryParam{
		StartTimeMin: time.Unix(0, 0),
		StartTimeMax: time.Now(),
		Limit:        20,
	}
	q := r.URL.Query()
	if p.ServiceName = q.Get("service"); p.ServiceName == "" {
		// service name is required.
		return nil, fmt.Errorf("service name is required")
	}
	p.SpanName = q.Get("operation")
	durationMin := q.Get("minDuration")
	if durationMin != "" {
		p.DurationMin, err = time.ParseDuration(durationMin)
		if err != nil {
			return nil, fmt.Errorf("cannot parse minDuration [%s]: %w", durationMin, err)
		}
	}

	durationMax := q.Get("maxDuration")
	if durationMax != "" {
		p.DurationMax, err = time.ParseDuration(durationMax)
		if err != nil {
			return nil, fmt.Errorf("cannot parse maxDuration [%s]: %w", durationMax, err)
		}
	}

	limit := q.Get("limit")
	if limit != "" {
		p.Limit, err = strconv.Atoi(limit)
		if err != nil {
			return nil, fmt.Errorf("cannot parse limit [%s]: %w", limit, err)
		}
		if p.Limit > maxLimit {
			return nil, fmt.Errorf("limit should be not higher than %d", maxLimit)
		}
	}

	startTimeMin := q.Get("start")
	if startTimeMin != "" {
		unixNano, err := strconv.ParseInt(startTimeMin, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse start [%s]: %w", startTimeMin, err)
		}
		p.StartTimeMin = time.UnixMicro(unixNano)
	}

	startTimeMax := q.Get("end")
	if startTimeMax != "" {
		unixNano, err := strconv.ParseInt(startTimeMax, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse end [%s]: %w", startTimeMax, err)
		}
		p.StartTimeMax = time.UnixMicro(unixNano)
	}

	tags := q.Get("tags")
	if tags != "" {
		if err := json.Unmarshal([]byte(tags), &p.Attributes); err != nil {
			return nil, fmt.Errorf("cannot parse tags [%s]: %w", tags, err)
		}
	}

	attributesFilter := make(map[string]string, len(p.Attributes))
	// some special fields in the OpenTelemetry span will be treated as span attributes/tags
	// in query result, so they should be converted to proper filters correspondingly.
	// e.g.: `otel.status_description` attribute in query result could be:
	// 1. retrieved from `span_attr:otel.status_description` field directly.
	// 2. converted from `status_message` field for Jaeger API.
	for k, v := range p.Attributes {
		// convert to OpenTelemetry field name in storage.
		if field, ok := spanAttributeMap[k]; ok {
			// 2 special cases that need to converted value as well.
			if k == "error" {
				v = errorStatusCodeMap[v]
			} else if k == "span.kind" {
				v = spanKindMap[v]
			}
			attributesFilter[field] = v
		} else if strings.HasPrefix(k, otelpb.InstrumentationScopeAttrPrefix) || strings.HasPrefix(k, otelpb.ResourceAttrPrefix) {
			attributesFilter[k] = v
		} else {
			attributesFilter[otelpb.SpanAttrPrefixField+k] = v
		}
	}
	p.Attributes = attributesFilter

	return p, nil
}

// hashProcess generate hash result for a process according to its tags.
func hashProcess(process process) uint64 {
	d := hashpool.Get()
	sort.Slice(process.tags, func(i, j int) bool {
		return process.tags[i].key < process.tags[j].key
	})
	_, _ = d.WriteString(process.serviceName)
	for _, tag := range process.tags {
		_, _ = d.WriteString(tag.key)
		_, _ = d.WriteString(tag.vStr)
	}
	h := d.Sum64()
	d.Reset()
	hashpool.Put(d)
	return h
}
