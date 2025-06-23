package query

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	otelpb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

var (
	traceMaxDurationWindow = flag.Duration("search.traceMaxDurationWindow", 10*time.Minute, "The window of searching for the rest trace spans after finding one span."+
		"It allows extending the search start time and end time by `-search.traceMaxDurationWindow` to make sure all spans are included."+
		"It affects both Jaeger's `/api/traces` and `/api/traces/<trace_id>` APIs.")
	traceServiceAndSpanNameLookbehind = flag.Duration("search.traceServiceAndSpanNameLookbehind", 7*24*time.Hour, "The time range of searching for service name and span name. "+
		"It affects Jaeger's `/api/services` and `/api/services/*/operations` APIs.")
	traceSearchStep = flag.Duration("search.traceSearchStep", 24*time.Hour, "Splits the [0, now] time range into many small time ranges by -search.traceSearchStep "+
		"when searching for spans by trace_id. Once it finds spans in a time range, it performs an additional search according to -search.traceMaxDurationWindow and then stops. "+
		"It affects Jaeger's `/api/traces/<trace_id>` API.")
	traceMaxServiceNameList = flag.Uint64("search.traceMaxServiceNameList", 1000, "The maximum number of service name can return in a get service name request. "+
		"This limit affects Jaeger's `/api/services` API.")
	traceMaxSpanNameList = flag.Uint64("search.traceMaxSpanNameList", 1000, "The maximum number of span name can return in a get span name request. "+
		"This limit affects Jaeger's `/api/services/*/operations` API.")
)

var (
	traceIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-.:]*$`)
)

// CommonParams common query params that shared by all requests.
type CommonParams struct {
	TenantIDs []logstorage.TenantID
}

// GetCommonParams get common params from request for all traces query APIs.
func GetCommonParams(r *http.Request) (*CommonParams, error) {
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain tenanID: %w", err)
	}
	tenantIDs := []logstorage.TenantID{tenantID}
	cp := &CommonParams{
		TenantIDs: tenantIDs,
	}
	return cp, nil
}

// TraceQueryParam is the parameters for querying a batch of traces.
type TraceQueryParam struct {
	ServiceName  string
	SpanName     string
	Attributes   map[string]string
	StartTimeMin time.Time
	StartTimeMax time.Time
	DurationMin  time.Duration
	DurationMax  time.Duration
	Limit        int
}

// Row represent the query result of a trace span.
type Row struct {
	Timestamp int64
	Fields    []logstorage.Field
}

// GetServiceNameList returns all unique service names within *traceServiceAndSpanNameLookbehind window.
// todo: cache of recent result.
func GetServiceNameList(ctx context.Context, cp *CommonParams) ([]string, error) {
	currentTime := time.Now()

	// query: _time:[start, end] *
	qStr := "*"
	q, err := logstorage.ParseQueryAtTimestamp(qStr, currentTime.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(currentTime.Add(-*traceServiceAndSpanNameLookbehind).UnixNano(), currentTime.UnixNano())

	serviceHits, err := vlstorage.GetStreamFieldValues(ctx, cp.TenantIDs, q, otelpb.ResourceAttrServiceName, *traceMaxServiceNameList)
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}

	serviceList := make([]string, 0, len(serviceHits))
	for i := range serviceHits {
		serviceList = append(serviceList, serviceHits[i].Value)
	}
	return serviceList, nil
}

// GetSpanNameList returns all unique span names for a service within *traceServiceAndSpanNameLookbehind window.
// todo: cache of recent result.
func GetSpanNameList(ctx context.Context, cp *CommonParams, serviceName string) ([]string, error) {
	currentTime := time.Now()

	// query: _time:[start, end] {"resource_attr:service.name"=serviceName}
	qStr := fmt.Sprintf("_stream:{%s=%q}", otelpb.ResourceAttrServiceName, serviceName)
	q, err := logstorage.ParseQueryAtTimestamp(qStr, currentTime.Unix())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(currentTime.Add(-*traceServiceAndSpanNameLookbehind).UnixNano(), currentTime.UnixNano())

	spanNameHits, err := vlstorage.GetStreamFieldValues(ctx, cp.TenantIDs, q, otelpb.NameField, *traceMaxSpanNameList)
	if err != nil {
		return nil, fmt.Errorf("get span name hits error: %s", err)
	}

	spanNameList := make([]string, 0, len(spanNameHits))
	for i := range spanNameHits {
		spanNameList = append(spanNameList, spanNameHits[i].Value)
	}
	return spanNameList, nil
}

// GetTrace returns all spans of a trace in []*Row format.
// In order to avoid scanning all data blocks, search is performed on time range splitting by traceSearchStep.
// Once a trace is found, it assumes other spans will exist on the same time range, and only search this
// time range (with traceMaxDurationWindow).
//
// e.g.
//  1. find traces span on [now-traceSearchStep, now], no hit.
//  2. find traces span on [now-2 * traceSearchStep, now - traceSearchStep], hit.
//  3. make sure to include all the spans by an additional search on: [now-2 * traceSearchStep-traceMaxDurationWindow, now-2 * traceSearchStep].
//  4. skip [0,  now-2 * traceSearchStep-traceMaxDurationWindow] and return.
//
// todo in-memory cache of hot traces.
func GetTrace(ctx context.Context, cp *CommonParams, traceID string) ([]*Row, error) {
	currentTime := time.Now()

	// query: trace_id:traceID
	qStr := fmt.Sprintf(otelpb.TraceIDField+": %q", traceID)
	q, err := logstorage.ParseQueryAtTimestamp(qStr, currentTime.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)

	// search for trace spans and write to `rows []*Row`
	var rowsLock sync.Mutex
	var rows []*Row
	var missingTimeColumn atomic.Bool
	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		if missingTimeColumn.Load() {
			return
		}

		columns := db.Columns
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		timestamps, ok := db.GetTimestamps(nil)
		if !ok {
			missingTimeColumn.Store(true)
			cancel()
			return
		}

		for i, timestamp := range timestamps {
			fields := make([]logstorage.Field, 0, len(columns))
			for j := range columns {
				// column could be empty if this span does not contain such field.
				// only append non-empty columns.
				if columns[j].Values[i] != "" {
					fields = append(fields, logstorage.Field{
						Name:  clonedColumnNames[j],
						Value: strings.Clone(columns[j].Values[i]),
					})
				}
			}

			rowsLock.Lock()
			rows = append(rows, &Row{
				Timestamp: timestamp,
				Fields:    fields,
			})
			rowsLock.Unlock()
		}
	}

	startTime := currentTime.Add(-*traceSearchStep)
	endTime := currentTime
	for startTime.UnixNano() > 0 { // todo: no need to search time range before retention period.
		qq := q.CloneWithTimeFilter(currentTime.UnixNano(), startTime.UnixNano(), endTime.UnixNano())
		if err = vlstorage.RunQuery(ctxWithCancel, cp.TenantIDs, qq, writeBlock); err != nil {
			return nil, err
		}
		if missingTimeColumn.Load() {
			return nil, fmt.Errorf("missing _time column in the result for the query [%s]", qq)
		}

		// no hit in this time range, continue with step.
		if len(rows) == 0 {
			endTime = startTime
			startTime = startTime.Add(-*traceSearchStep)
			continue
		}

		// found result, perform extra search for traceMaxDurationWindow and then break.
		qq = q.CloneWithTimeFilter(currentTime.UnixNano(), startTime.Add(-*traceMaxDurationWindow).UnixNano(), startTime.UnixNano())
		if err = vlstorage.RunQuery(ctxWithCancel, cp.TenantIDs, qq, writeBlock); err != nil {
			return nil, err
		}
		if missingTimeColumn.Load() {
			return nil, fmt.Errorf("missing _time column in the result for the query [%s]", qq)
		}
		break
	}

	return rows, nil
}

// GetTraceList returns multiple traceIDs and spans of them in []*Row format.
// It search for traceIDs first, and then search for the spans of these traceIDs.
// To not miss any spans on the edge, it extends both the start time and end time
// by *traceMaxDurationWindow.
//
// e.g.:
// 1. input time range: [00:00, 09:00]
// 2. found 20 trace id, and adjust time range to: [08:00, 09:00]
// 3. find spans on time range: [08:00-traceMaxDurationWindow, 09:00+traceMaxDurationWindow]
func GetTraceList(ctx context.Context, cp *CommonParams, param *TraceQueryParam) ([]string, []*Row, error) {
	currentTime := time.Now()

	// query 1: * AND filter_conditions | last 1 by (_time) partition by (trace_id) | fields _time, trace_id | sort by (_time) desc
	traceIDs, startTime, err := getTraceIDList(ctx, cp, param)
	if err != nil {
		return nil, nil, fmt.Errorf("get trace id error: %w", err)
	}
	if len(traceIDs) == 0 {
		return nil, nil, nil
	}

	// query 2: trace_id:in(traceID, traceID, ...)
	qStr := fmt.Sprintf(otelpb.TraceIDField+":in(%s)", strings.Join(traceIDs, ","))
	q, err := logstorage.ParseQueryAtTimestamp(qStr, currentTime.UnixNano())
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}

	// adjust start time and end time with max duration window to make sure all spans are included.
	q.AddTimeFilter(startTime.Add(-*traceMaxDurationWindow).UnixNano(), param.StartTimeMax.Add(*traceMaxDurationWindow).UnixNano())

	ctxWithCancel, cancel := context.WithCancel(ctx)

	// search for trace spans and write to `rows []*Row`
	var rowsLock sync.Mutex
	var rows []*Row
	var missingTimeColumn atomic.Bool
	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		if missingTimeColumn.Load() {
			return
		}

		columns := db.Columns
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		timestamps, ok := db.GetTimestamps(nil)
		if !ok {
			missingTimeColumn.Store(true)
			cancel()
			return
		}

		for i, timestamp := range timestamps {
			fields := make([]logstorage.Field, 0, len(columns))
			for j := range columns {
				// column could be empty if this span does not contain such field.
				// only append non-empty columns.
				if columns[j].Values[i] != "" {
					fields = append(fields, logstorage.Field{Name: clonedColumnNames[j], Value: strings.Clone(columns[j].Values[i])})
				}
			}

			rowsLock.Lock()
			rows = append(rows, &Row{
				Timestamp: timestamp,
				Fields:    fields,
			})
			rowsLock.Unlock()
		}
	}

	if err = vlstorage.RunQuery(ctxWithCancel, cp.TenantIDs, q, writeBlock); err != nil {
		return nil, nil, err
	}
	if missingTimeColumn.Load() {
		return nil, nil, fmt.Errorf("missing _time column in the result for the query [%s]", q)
	}
	return traceIDs, rows, nil
}

// getTraceIDList returns traceIDs according to the search params.
// It also returns the earliest start time of these traces, to help reducing the time range for spans search.
func getTraceIDList(ctx context.Context, cp *CommonParams, param *TraceQueryParam) ([]string, time.Time, error) {
	currentTime := time.Now()
	// query: * AND <filter> | last 1 by (_time) partition by (trace_id) | fields _time, trace_id | sort by (_time) desc
	qStr := "*"
	if param.ServiceName != "" {
		qStr += fmt.Sprintf("AND _stream:{"+otelpb.ResourceAttrServiceName+"=%q} ", param.ServiceName)
	}
	if param.SpanName != "" {
		qStr += fmt.Sprintf("AND _stream:{"+otelpb.NameField+"=%q} ", param.SpanName)
	}
	if len(param.Attributes) > 0 {
		for k, v := range param.Attributes {
			qStr += fmt.Sprintf(`AND %q:=%q `, k, v)
		}
	}
	if param.DurationMin > 0 {
		qStr += fmt.Sprintf("AND "+otelpb.DurationField+":>%d ", param.DurationMin.Nanoseconds())
	}
	if param.DurationMax > 0 {
		qStr += fmt.Sprintf("AND duration:<%d ", param.DurationMax.Nanoseconds())
	}
	qStr += " | last 1 by (_time) partition by (" + otelpb.TraceIDField + ") | fields _time, " + otelpb.TraceIDField + " | sort by (_time) desc"

	q, err := logstorage.ParseQueryAtTimestamp(qStr, currentTime.UnixNano())
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddPipeLimit(uint64(param.Limit))

	traceIDs, maxStartTime, err := findTraceIDsSplitTimeRange(ctx, q, cp, param.StartTimeMin, param.StartTimeMax, param.Limit)
	if err != nil {
		return nil, time.Time{}, err
	}

	return traceIDs, maxStartTime, nil
}

// findTraceIDsSplitTimeRange try to search from the nearest time range of the end time.
// if the result already met requirement of `limit`, return.
// otherwise, amplify the time range to 5x and search again, until the start time exceed the input.
func findTraceIDsSplitTimeRange(ctx context.Context, q *logstorage.Query, cp *CommonParams, startTime, endTime time.Time, limit int) ([]string, time.Time, error) {
	currentTime := time.Now()

	step := time.Minute
	currentStartTime := endTime.Add(-step)

	var traceIDListLock sync.Mutex
	traceIDList := make([]string, 0, limit)
	maxStartTimeStr := endTime.Format(time.RFC3339)

	writeBlock := func(_ uint, db *logstorage.DataBlock) {
		columns := db.Columns
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}
		for i := range clonedColumnNames {
			if clonedColumnNames[i] == "trace_id" {
				traceIDListLock.Lock()
				for _, v := range columns[i].Values {
					traceIDList = append(traceIDList, strings.Clone(v))
				}
				traceIDListLock.Unlock()
			} else if clonedColumnNames[i] == "_time" {
				for _, v := range columns[i].Values {
					if v < maxStartTimeStr {
						maxStartTimeStr = strings.Clone(v)
					}
				}
			}
		}
	}

	for currentStartTime.After(startTime) {
		qClone := q.CloneWithTimeFilter(currentTime.UnixNano(), currentStartTime.UnixNano(), endTime.UnixNano())
		if err := vlstorage.RunQuery(ctx, cp.TenantIDs, qClone, writeBlock); err != nil {
			return nil, time.Time{}, err
		}

		// found enough trace_id, return directly
		if len(traceIDList) == limit {
			maxStartTime, err := time.Parse(time.RFC3339, maxStartTimeStr)
			if err != nil {
				return nil, maxStartTime, err
			}
			return traceIDList, maxStartTime, nil
		}

		// not enough trace_id, clear the result, extend the time range and try again.
		traceIDList = traceIDList[:0]
		step *= 5
		currentStartTime = currentStartTime.Add(-step)
	}

	// one last try with input time range
	if currentStartTime.Before(startTime) {
		currentStartTime = startTime
	}

	qClone := q.CloneWithTimeFilter(currentTime.UnixNano(), currentStartTime.UnixNano(), endTime.UnixNano())
	if err := vlstorage.RunQuery(ctx, cp.TenantIDs, qClone, writeBlock); err != nil {
		return nil, time.Time{}, err
	}

	maxStartTime, err := time.Parse(time.RFC3339, maxStartTimeStr)
	if err != nil {
		return nil, maxStartTime, err
	}

	return checkTraceIDList(traceIDList), maxStartTime, nil
}

// checkTraceIDList removes invalid `trace_id`. It helps prevent query injection.
func checkTraceIDList(traceIDList []string) []string {
	result := make([]string, 0, len(traceIDList))
	for i := range traceIDList {
		if traceIDRegex.MatchString(traceIDList[i]) {
			result = append(result, traceIDList[i])
		}
	}
	return result
}
