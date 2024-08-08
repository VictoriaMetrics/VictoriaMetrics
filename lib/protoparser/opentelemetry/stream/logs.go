package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

// GetStreamFn is a function type, which returns logs processor function for a given slice of stream fields
type GetStreamFn func([]string) func(int64, []logstorage.Field)

// ParseLogsStream parses OpenTelemetry protobuf or json data from r and calls callback for the parsed rows.
func ParseLogsStream(r io.Reader, contentType string, isGzipped bool, getStream GetStreamFn) (int, error) {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return 0, fmt.Errorf("cannot read gzip-compressed OpenTelemetry protocol data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	wr := getWriteLogsContext()
	defer putWriteLogsContext(wr)
	req, err := wr.readAndUnpackLogsRequest(r, contentType)
	if err != nil {
		return 0, fmt.Errorf("cannot unpack OpenTelemetry logs: %w", err)
	}
	return wr.parseLogsRequest(req, getStream), nil
}

func (wr *writeLogsContext) readAndUnpackLogsRequest(r io.Reader, contentType string) (*pb.ExportLogsServiceRequest, error) {
	if _, err := wr.bb.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("cannot read request: %w", err)
	}
	var req pb.ExportLogsServiceRequest
	if contentType == "application/json" {
		if err := json.Unmarshal(wr.bb.B, &req); err != nil {
			return nil, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(wr.bb.B), err)
		}
	} else {
		if err := req.UnmarshalProtobuf(wr.bb.B); err != nil {
			return nil, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(wr.bb.B), err)
		}
	}
	return &req, nil
}

func (wr *writeLogsContext) parseLogsRequest(req *pb.ExportLogsServiceRequest, getStream GetStreamFn) int {
	var count int
	for _, rl := range req.ResourceLogs {
		var attributes []*pb.KeyValue
		if rl.Resource != nil {
			attributes = rl.Resource.Attributes
		}
		var streamFields []string
		wr.baseFields, streamFields = appendAttributesToFields(wr.baseFields[:0], attributes, true)
		processFn := getStream(streamFields)
		for _, sc := range rl.ScopeLogs {
			count += wr.pushFieldsFromScopeLogs(sc, processFn)
		}
	}
	return count
}

// appendAttributesToFields appends attributes to dst and returns the result.
func appendAttributesToFields(dst []logstorage.Field, attributes []*pb.KeyValue, useForStream bool) ([]logstorage.Field, []string) {
	var streamFields []string
	if useForStream {
		streamFields = make([]string, len(attributes))
	}
	for i, at := range attributes {
		dst = append(dst, logstorage.Field{
			Name:  at.Key,
			Value: at.Value.FormatString(),
		})
		if useForStream {
			streamFields[i] = at.Key
		}
	}
	return dst, streamFields
}

func (wr *writeLogsContext) pushFieldsFromScopeLogs(sc *pb.ScopeLogs, processFn func(int64, []logstorage.Field)) int {
	for _, lr := range sc.LogRecords {
		wr.fields, _ = appendAttributesToFields(wr.fields, lr.Attributes, false)
		if lr.Severity != "" {
			wr.fields = append(wr.fields, logstorage.Field{
				Name:  "severity",
				Value: lr.Severity,
			})
		}
		wr.fields = append(wr.fields, logstorage.Field{
			Name:  "_msg",
			Value: lr.Body.FormatString(),
		})
		processFn(int64(lr.Timestamp), append(wr.fields, wr.baseFields...))
	}
	return len(sc.LogRecords)
}

func (wr *writeLogsContext) reset() {
	wr.bb.Reset()

	clear(wr.fields)
	wr.fields = wr.fields[:0]
	clear(wr.baseFields)
	wr.baseFields = wr.baseFields[:0]
}

type writeLogsContext struct {
	// bb holds the original data (json or protobuf), which must be parsed.
	bb bytesutil.ByteBuffer

	// fields holds parsed logs fields
	fields []logstorage.Field

	// baseFields are fields, which must be added to all the ingested samples
	baseFields []logstorage.Field
}

var wrLogsPool sync.Pool

func getWriteLogsContext() *writeLogsContext {
	v := wrLogsPool.Get()
	if v == nil {
		return &writeLogsContext{}
	}
	return v.(*writeLogsContext)
}

func putWriteLogsContext(wr *writeLogsContext) {
	wr.reset()
	wrLogsPool.Put(wr)
}
