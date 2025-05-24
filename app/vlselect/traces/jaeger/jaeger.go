package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/traces/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/traces/query"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"
)

// Jaeger Query APIs metrics
var (
	jaegerServicesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/trace/jaeger/api/services"}`)
	jaegerServicesDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/trace/jaeger/api/services"}`)

	jaegerOperationsRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/trace/jaeger/api/services/*/operations"}`)
	jaegerOperationsDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/trace/jaeger/api/services/*/operations"}`)

	jaegerTracesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/trace/jaeger/api/traces"}`)
	jaegerTracesDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/trace/jaeger/api/traces"}`)

	jaegerTraceRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/trace/jaeger/api/traces/*"}`)
	jaegerTraceDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/trace/jaeger/api/traces/*"}`)

	jaegerDependenciesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/trace/jaeger/api/dependencies"}`)
	jaegerDependenciesDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/select/trace/jaeger/api/dependencies"}`)
)

// hash pool for ProcessID
var xxhashPool = &sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

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
		jaegerDependenciesDuration.UpdateDuration(startTime)
		return true
	}
	return false
}

func processGetServicesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := common.GetCommonParams(r)
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
	return
}

func processGetOperationsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := common.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	// extract the `service_name`.
	// the path must be like `/select/trace/api/services/<service_name>/operations`.
	paths := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(paths) < 6 {
		httpserver.Errorf(w, r, "incorrect query path [%s]", r.URL.Path)
		return
	}
	serviceName := paths[len(paths)-2]

	operationList, err := query.GetSpanNameList(ctx, cp, serviceName)
	if err != nil {
		httpserver.Errorf(w, r, "cannot get operation list: %s", err)
		return
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetOperationsResponse(w, operationList)
	return
}

func processGetTraceRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := common.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	// extract the `trace_id`.
	// the path must be like `/select/trace/api/traces/<trace_id>`.
	paths := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(paths) < 5 {
		httpserver.Errorf(w, r, "incorrect query path [%s]", r.URL.Path)
		return
	}
	traceID := paths[len(paths)-1]

	rows, err := query.GetTrace(ctx, cp, traceID)
	if err != nil {
		httpserver.Errorf(w, r, "cannot get traces: %s", err)
		return
	}

	trace := &Trace{}
	processIDMap := make(map[uint64]string) // process name -> id
	processMap := make(map[string]*Process) // trace_id -> map[processID]->Process
	for i := range rows {
		var sp *Span
		sp, err = FieldsToSpan(rows[i].Fields)
		if err != nil {
			logger.Errorf("cannot unmarshal log fields [%v] to span: %s", rows[i].Fields, err)
			continue
		}

		// Process ID
		ph := hashProcess(sp.Process)
		if _, ok := processIDMap[ph]; !ok {
			processID := "p" + strconv.Itoa(len(processIDMap)+1)
			processIDMap[ph] = processID
			processMap[processID] = sp.Process
		}

		sp.ProcessID = processIDMap[ph]
		trace.Spans = append(trace.Spans, sp)
	}

	// 6. attach process info to this trace
	trace.ProcessMap = make([]Trace_ProcessMapping, 0, len(processMap))
	for processID, process := range processMap {
		trace.ProcessMap = append(trace.ProcessMap, Trace_ProcessMapping{
			ProcessID: processID,
			Process:   *process,
		})
	}

	sort.Slice(trace.ProcessMap, func(i, j int) bool {
		return trace.ProcessMap[i].ProcessID < trace.ProcessMap[j].ProcessID
	})

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetTracesResponse(w, []*Trace{trace})
	return
}

func processGetTracesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cp, err := common.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect query params: %s", err)
		return
	}

	param, err := parseJaegerTraceQueryParam(ctx, r)
	if err != nil {
		httpserver.Errorf(w, r, "incorrect trace query params: %s", err)
	}

	traceIDList, rows, err := query.GetTraceList(ctx, cp, param)
	if len(rows) == 0 {
		// Write results
		w.Header().Set("Content-Type", "application/json")
		WriteGetTracesResponse(w, nil)
		return
	}

	// convert fields spans to jaeger spans, and group by trace_id.
	//
	// 1. prepare a trace_id -> *Trace map
	tracesMap := make(map[string]*Trace)
	traces := make([]*Trace, len(traceIDList), len(traceIDList))
	for i := range traceIDList {
		traces[i] = &Trace{}
		tracesMap[traceIDList[i]] = traces[i]
	}

	processHashMap := make(map[uint64]string)               // process_unique_hash -> pid
	traceProcessMap := make(map[string]map[string]*Process) // trace_id -> map[processID]->Process
	for i := range rows {
		// 2. convert fields to jaeger spans.
		var sp *Span
		sp, err = FieldsToSpan(rows[i].Fields)
		if err != nil {
			logger.Errorf("cannot unmarshal log fields [%v] to span: %s", rows[i].Fields, err)
			continue
		}

		// 3. calculate the process that this span belongs to
		procHash := hashProcess(sp.Process)
		if _, ok := processHashMap[procHash]; !ok {
			// format process id as Jaeger does: `p{idx}`, where {idx} starts from 1.
			processHashMap[procHash] = "p" + strconv.Itoa(len(processHashMap)+1)
		}
		// and attach the process info to the span.
		sp.ProcessID = processHashMap[procHash]

		// 4. add the process info to this trace (if process not exists).
		if _, ok := traceProcessMap[sp.TraceID]; !ok {
			traceProcessMap[sp.TraceID] = make(map[string]*Process)
		}
		if _, ok := traceProcessMap[sp.TraceID][sp.ProcessID]; !ok {
			traceProcessMap[sp.TraceID][sp.ProcessID] = sp.Process
		}

		// 5. append this span to the trace it belongs to.
		tracesMap[sp.TraceID].Spans = append(tracesMap[sp.TraceID].Spans, sp)
	}

	// 6. attach process info to each trace
	for traceID, trace := range tracesMap {
		trace.ProcessMap = make([]Trace_ProcessMapping, 0, len(traceProcessMap[traceID]))
		for processID, process := range traceProcessMap[traceID] {
			trace.ProcessMap = append(trace.ProcessMap, Trace_ProcessMapping{
				ProcessID: processID,
				Process:   *process,
			})
		}

		sort.Slice(trace.ProcessMap, func(i, j int) bool {
			return trace.ProcessMap[i].ProcessID < trace.ProcessMap[j].ProcessID
		})
	}

	// Write results
	w.Header().Set("Content-Type", "application/json")
	WriteGetTracesResponse(w, traces)
	return
}

// parseJaegerTraceQueryParam parse Jaeger request to unified query.TraceQueryParam.
func parseJaegerTraceQueryParam(ctx context.Context, r *http.Request) (*query.TraceQueryParam, error) {
	// default params
	p := &query.TraceQueryParam{
		StartTimeMin: time.Unix(0, 0),
		StartTimeMax: time.Now(),
		Limit:        20,
	}
	q := r.URL.Query()
	p.ServiceName = q.Get("service")
	p.SpanName = q.Get("operation")
	durationMin := q.Get("minDuration")
	if durationMin != "" {
		p.DurationMin, _ = time.ParseDuration(durationMin)
	}
	durationMax := q.Get("maxDuration")
	if durationMax != "" {
		p.DurationMax, _ = time.ParseDuration(durationMax)
	}
	limit := q.Get("limit")
	if limit != "" {
		p.Limit, _ = strconv.Atoi(limit)
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
	return p, nil
}

// hashProcess generate hash result for a process according to its tags.
func hashProcess(process *Process) uint64 {
	d := xxhashPool.Get().(*xxhash.Digest)
	sort.Slice(process.Tags, func(i, j int) bool {
		if process.Tags[i].Key < process.Tags[j].Key {
			return true
		}
		return false
	})
	_, _ = d.WriteString(process.ServiceName)
	for _, tag := range process.Tags {
		_, _ = d.WriteString(tag.Key)
		_, _ = d.WriteString(tag.VStr)
	}
	h := d.Sum64()
	d.Reset()
	xxhashPool.Put(d)
	return h
}
