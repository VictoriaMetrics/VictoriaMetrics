package jaeger

import (
	"context"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jaeger"
	"strings"
	"sync"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// A SpanReaderPluginServer represents a Jaeger interface to read from gRPC storage backend
type SpanReaderPluginServer struct{}

type row struct {
	timestamp int64
	fields    []logstorage.Field
}

func (s *SpanReaderPluginServer) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	start := time.Now()
	defer func() {
		logger.Infof("GetTrace finished in %dms", time.Since(start).Milliseconds())
	}()
	qStr := fmt.Sprintf("%s:%s", jaeger.TraceID, traceID.String())
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}

	var rows []row
	var rowsLock sync.Mutex
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		for i, timestamp := range timestamps {
			fields := make([]logstorage.Field, len(columns))
			for j := range columns {
				f := &fields[j]
				f.Name = clonedColumnNames[j]
				f.Value = strings.Clone(columns[j].Values[i])
			}

			rowsLock.Lock()
			rows = append(rows, row{
				timestamp: timestamp,
				fields:    fields,
			})
			rowsLock.Unlock()
		}
	}
	logger.Infof("GetTrace query: %s", q.String())
	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return nil, err
	}

	spans := make([]*model.Span, 0, len(rows))
	for i := range rows {
		sp, err := jaeger.FieldsToSpan(rows[i].fields)
		if err != nil {
			logger.Errorf("cannot unmarshal log fields [%v] to span: %s", rows[i].fields, err)
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
	start := time.Now()
	defer func() {
		logger.Infof("GetServices finished in %dms", time.Since(start).Milliseconds())
	}()
	qStr := "*"
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(0, time.Now().UnixNano())
	logger.Infof("GetServices StreamFieldValues query: %s", q.String())
	serviceHits, err := vlstorage.GetStreamFieldValues(ctx, []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, jaeger.ProcessServiceName, uint64(1000))
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
	start := time.Now()
	defer func() {
		logger.Infof("GetOperations finished in %dms", time.Since(start).Milliseconds())
	}()
	qStr := fmt.Sprintf("_stream:{%s=\"%s\"}", jaeger.ProcessServiceName, req.ServiceName) // todo spankind filter
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	logger.Infof("GetOperations StreamFieldValues query: %s", q.String())
	operationHits, err := vlstorage.GetStreamFieldValues(ctx, []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, jaeger.OperationName, uint64(1000))
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
	start := time.Now()
	defer func() {
		logger.Infof("FindTraces finished in %dms", time.Since(start).Milliseconds())
	}()
	traceIDs, err := s.FindTraceIDs(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(traceIDs) == 0 {
		return nil, nil
	}
	traceIDStrList := make([]string, 0, len(traceIDs))
	for _, traceID := range traceIDs {
		traceIDStrList = append(traceIDStrList, traceID.String())
	}
	qStr := fmt.Sprintf(jaeger.TraceID+":in(%s)", strings.Join(traceIDStrList, ","))

	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(query.StartTimeMin.UnixNano(), query.StartTimeMax.UnixNano())

	var rows []row
	var rowsLock sync.Mutex
	writeBlock := func(_ uint, timestamps []int64, columns []logstorage.BlockColumn) {
		clonedColumnNames := make([]string, len(columns))
		for i, c := range columns {
			clonedColumnNames[i] = strings.Clone(c.Name)
		}

		for i, timestamp := range timestamps {
			fields := make([]logstorage.Field, len(columns))
			for j := range columns {
				f := &fields[j]
				f.Name = clonedColumnNames[j]
				f.Value = strings.Clone(columns[j].Values[i])
			}

			rowsLock.Lock()
			rows = append(rows, row{
				timestamp: timestamp,
				fields:    fields,
			})
			rowsLock.Unlock()
		}
	}
	logger.Infof("FindTraces query: %s", q.String())
	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return nil, err
	}
	tracesMap := make(map[string]*model.Trace)
	traces := make([]*model.Trace, len(traceIDs), len(traceIDs))
	for i := range traceIDs {
		traces[i] = &model.Trace{}
		tracesMap[traceIDs[i].String()] = traces[i]
	}

	for i := range rows {
		sp, err := jaeger.FieldsToSpan(rows[i].fields)
		if err != nil {
			logger.Errorf("cannot unmarshal log fields [%v] to span: %s", rows[i].fields, err)
			continue
		}

		tracesMap[sp.TraceID.String()].Spans = append(tracesMap[sp.TraceID.String()].Spans, sp)
	}
	return traces, nil
}

func (s *SpanReaderPluginServer) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	start := time.Now()
	defer func() {
		logger.Infof("FindTraceIDs finished in %dms", time.Since(start).Milliseconds())
	}()
	qStr := ""
	if svcName := query.ServiceName; svcName != "" {
		qStr += fmt.Sprintf("AND _stream:{"+jaeger.ProcessServiceName+"=\"%s\"} ", svcName)
	}
	if operationName := query.OperationName; operationName != "" {
		qStr += fmt.Sprintf("AND _stream:{"+jaeger.OperationName+"=\"%s\"} ", operationName)
	}

	if tags := query.Tags; len(tags) > 0 {
		for k, v := range tags {
			qStr += fmt.Sprintf("AND "+jaeger.TagKey+":%s ", k, v)
		}
	}
	if durationMin := query.DurationMin; durationMin > 0 {
		qStr += fmt.Sprintf("AND "+jaeger.Duration+":>%d ", durationMin.Nanoseconds())
	}
	if durationMax := query.DurationMax; durationMax > 0 {
		qStr += fmt.Sprintf("AND duration:<%d ", durationMax.Nanoseconds())
	}
	qStr = strings.TrimLeft(qStr+" | fields _time, "+jaeger.TraceID, "AND ")

	q, err := logstorage.ParseQueryAtTimestamp(qStr, query.StartTimeMax.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(query.StartTimeMin.UnixNano(), query.StartTimeMax.UnixNano())
	q.AddPipeLimit(uint64(query.NumTraces))

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
	logger.Infof("FindTraces query: %s", q.String())
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

func (s *SpanReaderPluginServer) GetDependencies(ctx context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	return []model.DependencyLink{}, nil
}
