package pb

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/easyproto"
)

// ExportLogsServiceRequest represents the corresponding OTEL protobuf message
type ExportLogsServiceRequest struct {
	ResourceLogs []ResourceLogs
}

// MarshalProtobuf marshals r to protobuf message, appends it to dst and returns the result.
func (r *ExportLogsServiceRequest) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	r.marshalProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

func (r *ExportLogsServiceRequest) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, rm := range r.ResourceLogs {
		rm.marshalProtobuf(mm.AppendMessage(1))
	}
}

// UnmarshalProtobuf unmarshals r from protobuf message at src.
func (r *ExportLogsServiceRequest) UnmarshalProtobuf(src []byte) (err error) {
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
			var rl ResourceLogs

			if err := rl.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ResourceLogs: %w", err)
			}
			r.ResourceLogs = append(r.ResourceLogs, rl)
		}
	}
	return nil
}

// ResourceLogs represents the corresponding OTEL protobuf message
type ResourceLogs struct {
	Resource  Resource    `json:"resource"`
	ScopeLogs []ScopeLogs `json:"scopeLogs"`
}

func (rl *ResourceLogs) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	rl.Resource.marshalProtobuf(mm.AppendMessage(1))
	for _, sm := range rl.ScopeLogs {
		sm.marshalProtobuf(mm.AppendMessage(2))
	}
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
			if err := rl.Resource.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot umarshal Resource: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ScopeLogs data")
			}
			var sl ScopeLogs
			if err := sl.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ScopeLogs: %w", err)
			}
			rl.ScopeLogs = append(rl.ScopeLogs, sl)
		}
	}
	return nil
}

// ScopeLogs represents the corresponding OTEL protobuf message
type ScopeLogs struct {
	LogRecords []LogRecord
}

func (sl *ScopeLogs) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, m := range sl.LogRecords {
		m.marshalProtobuf(mm.AppendMessage(2))
	}
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
			var lr LogRecord
			if err := lr.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal LogRecord: %w", err)
			}
			sl.LogRecords = append(sl.LogRecords, lr)
		}
	}
	return nil
}

// LogRecord represents the corresponding OTEL protobuf message
// https://github.com/open-telemetry/oteps/blob/main/text/logs/0097-log-data-model.md
type LogRecord struct {
	// time_unix_nano is the time when the event occurred.
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January 1970.
	// Value of 0 indicates unknown or missing timestamp.
	TimeUnixNano uint64
	// Time when the event was observed by the collection system.
	// For events that originate in OpenTelemetry (e.g. using OpenTelemetry Logging SDK)
	// this timestamp is typically set at the generation time and is equal to Timestamp.
	// For events originating externally and collected by OpenTelemetry (e.g. using
	// Collector) this is the time when OpenTelemetry's code observed the event measured
	// by the clock of the OpenTelemetry code. This field MUST be set once the event is
	// observed by OpenTelemetry.
	//
	// For converting OpenTelemetry log data to formats that support only one timestamp or
	// when receiving OpenTelemetry log data by recipients that support only one timestamp
	// internally the following logic is recommended:
	//   - Use time_unix_nano if it is present, otherwise use observed_time_unix_nano.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January 1970.
	// Value of 0 indicates unknown or missing timestamp.
	ObservedTimeUnixNano uint64
	// Numerical value of the severity, normalized to values described in Log Data Model.
	SeverityNumber int32
	SeverityText   string
	Body           AnyValue
	Attributes     []*KeyValue
	TraceID        string
	SpanID         string
}

func (lr *LogRecord) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendFixed64(1, lr.TimeUnixNano)
	mm.AppendInt32(2, lr.SeverityNumber)
	mm.AppendString(3, lr.SeverityText)
	lr.Body.marshalProtobuf(mm.AppendMessage(5))
	for _, a := range lr.Attributes {
		a.marshalProtobuf(mm.AppendMessage(6))
	}

	traceID, err := hex.DecodeString(lr.TraceID)
	if err != nil {
		traceID = []byte(lr.TraceID)
	}
	mm.AppendBytes(9, traceID)

	spanID, err := hex.DecodeString(lr.SpanID)
	if err != nil {
		spanID = []byte(lr.SpanID)
	}
	mm.AppendBytes(10, spanID)

	mm.AppendFixed64(11, lr.ObservedTimeUnixNano)
}

func (lr *LogRecord) unmarshalProtobuf(src []byte) (err error) {
	// message LogRecord {
	//   fixed64 time_unix_nano = 1;
	//   fixed64 observed_time_unix_nano = 11;
	//   SeverityNumber severity_number = 2;
	//   string severity_text = 3;
	//   AnyValue body = 5;
	//   repeated KeyValue attributes = 6;
	//   bytes trace_id = 9;
	//   bytes span_id = 10;
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
			lr.TimeUnixNano = ts
		case 11:
			ts, ok := fc.Fixed64()
			if !ok {
				return fmt.Errorf("cannot read log record observed timestamp")
			}
			lr.ObservedTimeUnixNano = ts
		case 2:
			severityNumber, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read severity number")
			}
			lr.SeverityNumber = severityNumber
		case 3:
			severityText, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read severity string")
			}
			lr.SeverityText = strings.Clone(severityText)
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Body")
			}
			if err := lr.Body.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Body: %w", err)
			}
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read attributes data")
			}
			lr.Attributes = append(lr.Attributes, &KeyValue{})
			a := lr.Attributes[len(lr.Attributes)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Attribute: %w", err)
			}
		case 9:
			traceID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read trace id")
			}
			lr.TraceID = hex.EncodeToString(traceID)
		case 10:
			spanID, ok := fc.Bytes()
			if !ok {
				return fmt.Errorf("cannot read span id")
			}
			lr.SpanID = hex.EncodeToString(spanID)
		}
	}
	return nil
}

// FormatSeverity returns normalized severity for log record
func (lr *LogRecord) FormatSeverity() string {
	if lr.SeverityText != "" {
		return lr.SeverityText
	}
	if lr.SeverityNumber < 0 || lr.SeverityNumber >= int32(len(logSeverities)) {
		return logSeverities[0]
	}
	return logSeverities[lr.SeverityNumber]
}

// ExtractTimestampNano returns timestamp for log record
func (lr *LogRecord) ExtractTimestampNano() int64 {
	switch {
	case lr.TimeUnixNano > 0:
		return int64(lr.TimeUnixNano)
	case lr.ObservedTimeUnixNano > 0:
		return int64(lr.ObservedTimeUnixNano)
	default:
		return time.Now().UnixNano()
	}
}

// https://github.com/open-telemetry/opentelemetry-collector/blob/cd1f7623fe67240e32e74735488c3db111fad47b/pdata/plog/severity_number.go#L41
var logSeverities = []string{
	"Unspecified",
	"Trace",
	"Trace2",
	"Trace3",
	"Trace4",
	"Debug",
	"Debug2",
	"Debug3",
	"Debug4",
	"Info",
	"Info2",
	"Info3",
	"Info4",
	"Warn",
	"Warn2",
	"Warn3",
	"Warn4",
	"Error",
	"Error2",
	"Error3",
	"Error4",
	"Fatal",
	"Fatal2",
	"Fatal3",
	"Fatal4",
}
