package opentelemetry

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/easyproto"
)

var (
	logSeverities = []string{
		"unspecified",
		"trace",
		"trace2",
		"trace3",
		"trace4",
		"debug",
		"debug2",
		"debug3",
		"debug4",
		"info",
		"info2",
		"info3",
		"info4",
		"error",
		"error2",
		"error3",
		"error4",
		"fatal",
		"fatal2",
		"fatal3",
		"fatal4",
	}
)

// ExportLogsServiceRequest represents the corresponding OTEL protobuf message
type ExportLogsServiceRequest struct {
	ResourceLogs []*ResourceLogs
}

func (r *ExportLogsServiceRequest) pushFields(cp *insertutils.CommonParams) int {
	var rowsIngested int
	var commonFields []logstorage.Field
	streamFieldsLen := len(cp.StreamFields)
	for _, rl := range r.ResourceLogs {
		if rl.Resource != nil {
			attributes := rl.Resource.Attributes
			commonFields = make([]logstorage.Field, len(attributes))
			for i, attr := range attributes {
				commonFields[i].Name = attr.Key
				commonFields[i].Value = attr.Value.FormatString()
				cp.StreamFields = append(cp.StreamFields, attr.Key)
			}
		}
		lmp := cp.NewLogMessageProcessor()
		for _, sc := range rl.ScopeLogs {
			rowsIngested += pushFieldsFromScopeLogs(sc, commonFields, lmp)
		}
		lmp.MustClose()
		cp.StreamFields = cp.StreamFields[:streamFieldsLen]
	}
	return rowsIngested
}

func pushFieldsFromScopeLogs(sc *ScopeLogs, commonFields []logstorage.Field, lmp insertutils.LogMessageProcessor) int {
	fields := commonFields
	for _, lr := range sc.LogRecords {
		fields = append(fields[:len(commonFields)], logstorage.Field{
			Name:  "_msg",
			Value: lr.Body.FormatString(),
		})
		for _, attr := range lr.Attributes {
			fields = append(fields, logstorage.Field{
				Name:  attr.Key,
				Value: attr.Value.FormatString(),
			})
		}
		if severity := lr.severity(); severity != "" {
			fields = append(fields, logstorage.Field{
				Name:  "severity",
				Value: severity,
			})
		}
		lmp.AddRow(lr.timestamp(), fields)
	}
	return len(sc.LogRecords)
}

// UnmarshalProtobuf unmarshals r from protobuf message at src.
func (r *ExportLogsServiceRequest) UnmarshalProtobuf(src []byte) error {
	r.ResourceLogs = nil
	return r.unmarshalProtobuf(src)
}

func (r *ExportLogsServiceRequest) unmarshalProtobuf(src []byte) (err error) {
	// message ExportLogsServiceRequest {
	//   repeated ResourceLogs resource_metrics = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ExportLogsServiceRequest: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ResourceLogs data")
			}
			r.ResourceLogs = append(r.ResourceLogs, &ResourceLogs{})
			rm := r.ResourceLogs[len(r.ResourceLogs)-1]
			if err := rm.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ResourceLogs: %w", err)
			}
		}
	}
	return nil
}

// ResourceLogs represents the corresponding OTEL protobuf message
type ResourceLogs struct {
	Resource  *pb.Resource
	ScopeLogs []*ScopeLogs
}

func (rl *ResourceLogs) unmarshalProtobuf(src []byte) (err error) {
	// message ResourceLogs {
	//   Resource resource = 1;
	//   repeated ScopeLogs scope_logs = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ResourceLogs: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Resource data")
			}
			rl.Resource = &pb.Resource{}
			if err := rl.Resource.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot umarshal Resource: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ScopeLogs data")
			}
			rl.ScopeLogs = append(rl.ScopeLogs, &ScopeLogs{})
			sl := rl.ScopeLogs[len(rl.ScopeLogs)-1]
			if err := sl.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ScopeLogs: %w", err)
			}
		}
	}
	return nil
}

// ScopeLogs represents the corresponding OTEL protobuf message
type ScopeLogs struct {
	LogRecords []*LogRecord
}

func (sl *ScopeLogs) unmarshalProtobuf(src []byte) (err error) {
	// message ScopeLogs {
	//   repeated LogRecord log_records = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ScopeLogs: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read LogRecord data")
			}
			sl.LogRecords = append(sl.LogRecords, &LogRecord{})
			l := sl.LogRecords[len(sl.LogRecords)-1]
			if err := l.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal LogRecord: %w", err)
			}
		}
	}
	return nil
}

// LogRecord represents the corresponding OTEL protobuf message
type LogRecord struct {
	TimeUnixNano         pb.Uint64
	ObservedTimeUnixNano pb.Uint64
	SeverityNumber       int32
	SeverityText         string
	Body                 *pb.AnyValue
	Attributes           []*pb.KeyValue
}

func (r *LogRecord) severity() string {
	if r.SeverityText != "" {
		return r.SeverityText
	}
	return logSeverities[r.SeverityNumber]
}

func (r *LogRecord) timestamp() int64 {
	if r.TimeUnixNano > 0 {
		return int64(r.TimeUnixNano)
	} else if r.ObservedTimeUnixNano > 0 {
		return int64(r.ObservedTimeUnixNano)
	}
	return 0
}

func (r *LogRecord) unmarshalProtobuf(src []byte) (err error) {
	// message LogRecord {
	//   fixed64 time_unix_nano = 1;
	//   fixed64 observed_time_unix_nano = 11;
	//   SeverityNumber severity_number = 2;
	//   string severity_text = 3;
	//   AnyValue body = 5;
	//   repeated KeyValue attributes = 6;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in LogRecord: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ts, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read log record timestamp")
			}
			r.TimeUnixNano = pb.Uint64(ts)
		case 11:
			ts, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read log record observed timestamp")
			}
			r.ObservedTimeUnixNano = pb.Uint64(ts)
		case 2:
			severityNumber, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read severity number")
			}
			r.SeverityNumber = severityNumber
		case 3:
			severityText, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read severity string")
			}
			r.SeverityText = strings.Clone(severityText)
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Body")
			}
			r.Body = &pb.AnyValue{}
			if err := r.Body.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Body: %w", err)
			}
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read attributes data")
			}
			r.Attributes = append(r.Attributes, &pb.KeyValue{})
			a := r.Attributes[len(r.Attributes)-1]
			if err := a.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Attribute: %w", err)
			}
		}
	}
	return nil
}
