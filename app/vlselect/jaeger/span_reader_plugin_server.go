package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

// A SpanReaderPluginServer represents a Jaeger interface to read from gRPC storage backend
type SpanReaderPluginServer struct{}

type span struct {
	timestamp int64
	msg       string
}

func (s *SpanReaderPluginServer) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	qStr := fmt.Sprintf("trace_id:%s | fields _time, _msg", traceID.String())
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}

	var spanRows []span
	var spansLock sync.Mutex
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		for i, timestamp := range timestamps {
			for j := range columns {
				if columns[j].Name == "_msg" {
					spansLock.Lock()
					spanRows = append(spanRows, span{
						timestamp: timestamp,
						msg:       columns[j].Values[i],
					})
					spansLock.Unlock()
				}
			}

		}
	}
	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return nil, err
	}

	spans := make([]*model.Span, 0, len(spanRows))
	for i := range spanRows {
		var sp *model.Span
		msg := spanRows[i].msg
		err = json.Unmarshal([]byte(msg), &sp)
		if err != nil {
			logger.Errorf("cannot unmarshal [%s]: %s", spanRows[i].msg, err)
			//return nil, fmt.Errorf("cannot unmarshal [%s]: %s", spanRows[i].msg, err)
			continue
		}
		spans = append(spans, sp)
	}
	trace := &model.Trace{
		Spans: spans,
	}
	return trace, nil
}

func (s *SpanReaderPluginServer) GetServices(ctx context.Context) ([]string, error) {
	qStr := "*"
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(0, time.Now().UnixNano())
	serviceHits, err := vlstorage.GetStreamFieldValues(ctx, []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, "service_name", uint64(1000))
	if err != nil {
		return nil, err
	}

	serviceList := make([]string, 0)
	for i := range serviceHits {
		serviceList = append(serviceList, serviceHits[i].Value)
	}

	return serviceList, nil
}

func (s *SpanReaderPluginServer) GetOperations(ctx context.Context, req spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	qStr := fmt.Sprintf("_stream:{service_name=\"%s\"}", req.ServiceName) // todo spankind filter
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	operationHits, err := vlstorage.GetStreamFieldValues(ctx, []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, "operation_name", uint64(1000))
	if err != nil {
		return nil, err
	}

	operationList := make([]spanstore.Operation, 0)
	for i := range operationHits {
		operationList = append(operationList, spanstore.Operation{Name: operationHits[i].Value})
	}
	return operationList, nil
}

func (s *SpanReaderPluginServer) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	traceIDs, err := s.FindTraceIDs(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(traceIDs) == 0 {
		return nil, nil
	}

	traces := make([]*model.Trace, 0, len(traceIDs))
	for _, traceID := range traceIDs {
		t, err := s.GetTrace(ctx, traceID)
		if err != nil {
			return nil, err
		}
		traces = append(traces, t)
	}
	return traces, nil
}

func (s *SpanReaderPluginServer) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	qStr := ""
	if svcName := query.ServiceName; svcName != "" {
		qStr += fmt.Sprintf("AND _stream:{service_name=\"%s\"} ", svcName)
	}
	if operationName := query.OperationName; operationName != "" {
		qStr += fmt.Sprintf("AND _stream:{operation_name=\"%s\"} ", operationName)
	}

	if tags := query.Tags; len(tags) > 0 {
		for k, v := range tags {
			qStr += fmt.Sprintf("AND %s:%s ", k, v)
		}
	}
	if durationMin := query.DurationMin; durationMin > 0 {
		qStr += fmt.Sprintf("AND duration:>%d", durationMin.Nanoseconds())
	}
	if durationMax := query.DurationMax; durationMax > 0 {
		qStr += fmt.Sprintf("AND duration:<%d", durationMax.Nanoseconds())
	}
	qStr = strings.TrimLeft(qStr+" | fields _time, trace_id", "AND ")

	q, err := logstorage.ParseQueryAtTimestamp(qStr, query.StartTimeMax.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(query.StartTimeMin.UnixNano(), query.StartTimeMax.UnixNano())

	traceIDSet := make(map[string]struct{})
	traceIDLock := sync.Mutex{}
	writeBlock := func(_ uint, _ []int64, columns []logstorage.BlockColumn) {
		for i := range columns {
			if columns[i].Name == "trace_id" {
				traceIDLock.Lock()
				for _, v := range columns[i].Values {
					traceIDSet[fmt.Sprintf("%s", v)] = struct{}{}
				}
				traceIDLock.Unlock()

			}
		}
	}
	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return nil, err
	}
	traceIDList := make([]model.TraceID, 0, 10)
	for k := range traceIDSet {
		tid, err := model.TraceIDFromString(k)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal [%s]: %s", k, err)
		}
		traceIDList = append(traceIDList, tid)
	}
	return traceIDList, nil
}
