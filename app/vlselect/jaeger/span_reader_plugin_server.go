package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jaeger/proto"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// A SpanReaderPluginServer represents a Jaeger interface to read from gRPC storage backend
type SpanReaderPluginServer struct{}

type span struct {
	timestamp int64
	msg       string
}

func (s *SpanReaderPluginServer) GetTrace(req *proto.GetTraceRequest, svc proto.SpanReaderPlugin_GetTraceServer) error {
	req.TraceID = proto.TraceID{}
	qStr := fmt.Sprintf("trace_id:%s | fields _time, _msg", req.TraceID.String())
	q, err := logstorage.ParseQueryAtTimestamp(qStr, req.EndTime.UnixNano())
	if err != nil {
		return fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(req.StartTime.UnixNano(), req.EndTime.UnixNano())

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
				}
				spansLock.Unlock()
			}

		}
	}
	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return err
	}

	spans := make([]proto.Span, 0, len(spanRows))
	for i := range spanRows {
		var sp *proto.Span
		err = json.Unmarshal([]byte(spanRows[i].msg), &sp)
		if err != nil {
			return fmt.Errorf("cannot unmarshal [%s]: %s", spanRows[i].msg, err)
		}
		spans = append(spans, *sp)
	}
	return svc.Send(&proto.SpansResponseChunk{Spans: spans})
}

func (s *SpanReaderPluginServer) GetServices(ctx context.Context, _ *proto.GetServicesRequest) (*proto.GetServicesResponse, error) {
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

	return &proto.GetServicesResponse{
		Services: serviceList,
	}, nil
}

func (s *SpanReaderPluginServer) GetOperations(ctx context.Context, req *proto.GetOperationsRequest) (*proto.GetOperationsResponse, error) {
	qStr := fmt.Sprintf("_stream:{service_name=\"%s\"}", req.GetService()) // todo spankind filter
	q, err := logstorage.ParseQueryAtTimestamp(qStr, time.Now().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	operationHits, err := vlstorage.GetStreamFieldValues(ctx, []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, "operation_name", uint64(1000))
	if err != nil {
		return nil, err
	}

	operationList := make([]string, 0)
	for i := range operationHits {
		operationList = append(operationList, operationHits[i].Value)
	}

	return &proto.GetOperationsResponse{
		OperationNames: operationList,
		Operations:     make([]*proto.Operation, 0),
	}, nil
}

func (s *SpanReaderPluginServer) FindTraces(req *proto.FindTracesRequest, svc proto.SpanReaderPlugin_FindTracesServer) error {
	query := req.GetQuery()
	qStr := ""
	if svcName := query.GetServiceName(); svcName != "" {
		qStr += fmt.Sprintf("AND _stream:{service_name=\"%s\"} ", svcName)
	}
	if operationName := query.GetOperationName(); operationName != "" {
		qStr += fmt.Sprintf("AND _stream:{operation_name=\"%s\"} ", operationName)
	}

	if tags := query.GetTags(); len(tags) > 0 {
		for k, v := range tags {
			qStr += fmt.Sprintf("AND %s:%s ", k, v)
		}
	}
	if durationMin := query.GetDurationMin(); durationMin > 0 {
		qStr += fmt.Sprintf("AND duration:>%d", durationMin.Nanoseconds())
	}
	if durationMax := query.GetDurationMax(); durationMax > 0 {
		qStr += fmt.Sprintf("AND duration:<%d", durationMax.Nanoseconds())
	}
	qStr = strings.TrimLeft(qStr+" | fields _time, _msg", "AND ")

	q, err := logstorage.ParseQueryAtTimestamp(qStr, query.GetStartTimeMax().UnixNano())
	if err != nil {
		return fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(query.GetStartTimeMin().UnixNano(), query.GetStartTimeMax().UnixNano())

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
					s := span{
						timestamp: timestamp,
						msg:       strings.Clone(columns[j].Values[i]),
					}
					spansLock.Lock()
					spanRows = append(spanRows, s)
				}
			}

			spansLock.Unlock()
		}
	}

	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return err
	}
	spans := make([]proto.Span, 0, len(spanRows))
	for i := range spanRows {
		var sp *proto.Span
		err = json.Unmarshal([]byte(spanRows[i].msg), &sp)
		if err != nil {
			logger.Errorf("cannot unmarshal [%s]: %s", spanRows[i].msg, err)
			continue
		}
		spans = append(spans, *sp)
	}
	return svc.Send(&proto.SpansResponseChunk{Spans: spans})
}

func (s *SpanReaderPluginServer) FindTraceIDs(_ context.Context, req *proto.FindTraceIDsRequest) (*proto.FindTraceIDsResponse, error) {
	query := req.GetQuery()
	qStr := ""
	if svcName := query.GetServiceName(); svcName != "" {
		qStr += fmt.Sprintf("AND service_name:%s ", svcName)
	}
	if operationName := query.GetOperationName(); operationName != "" {
		qStr += fmt.Sprintf("AND operation_name:%s ", operationName)
	}

	if tags := query.GetTags(); len(tags) > 0 {
		for k, v := range tags {
			qStr += fmt.Sprintf("AND %s:%s ", k, v)
		}
	}
	if durationMin := query.GetDurationMin(); durationMin > 0 {
		qStr += fmt.Sprintf("AND duration:>%d", durationMin.Nanoseconds())
	}
	if durationMax := query.GetDurationMax(); durationMax > 0 {
		qStr += fmt.Sprintf("AND duration:<%d", durationMax.Nanoseconds())
	}
	qStr = strings.TrimLeft(qStr+" | fields trace_id", "AND ")

	q, err := logstorage.ParseQueryAtTimestamp(qStr, query.GetStartTimeMax().UnixNano())
	if err != nil {
		return nil, fmt.Errorf("cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(query.GetStartTimeMin().UnixNano(), query.GetStartTimeMax().UnixNano())

	traceIDSet := make(map[string]struct{})
	var traceIDLock sync.Mutex
	writeBlock := func(_ uint, _ []int64, columns []logstorage.BlockColumn) {
		for i := range columns {
			if columns[i].Name == "trace_id" {
				traceIDLock.Lock() //todo looks unnecessary
				for _, v := range columns[i].Values {
					traceIDSet[v] = struct{}{}
				}
				traceIDLock.Unlock()
			}
		}
	}
	if err = vlstorage.RunQuery(context.TODO(), []logstorage.TenantID{{AccountID: 0, ProjectID: 0}}, q, writeBlock); err != nil {
		return nil, err
	}

	traceIDList := make([]proto.TraceID, 0, len(traceIDSet))
	for k := range traceIDSet {
		tid := &proto.TraceID{}
		err = tid.Unmarshal([]byte(k))
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal [%s]: %s", k, err)
		}
		traceIDList = append(traceIDList, *tid)
	}
	return &proto.FindTraceIDsResponse{TraceIDs: traceIDList}, nil
}
