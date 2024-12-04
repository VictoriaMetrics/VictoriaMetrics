package loki

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var (
	bytesBufPool bytesutil.ByteBufferPool
	pushReqsPool sync.Pool
)

func handleProtobuf(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsProtobufTotal.Inc()
	wcr := writeconcurrencylimiter.GetReader(r.Body)
	data, err := io.ReadAll(wcr)
	writeconcurrencylimiter.PutReader(wcr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot read request body: %s", err)
		return
	}

	cp, err := getCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	lmp := cp.NewLogMessageProcessor("loki_protobuf")
	useDefaultStreamFields := len(cp.StreamFields) == 0
	err = parseProtobufRequest(data, lmp, useDefaultStreamFields)
	lmp.MustClose()
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse Loki protobuf request: %s", err)
		return
	}

	// update requestProtobufDuration only for successfully parsed requests
	// There is no need in updating requestProtobufDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestProtobufDuration.UpdateDuration(startTime)
}

var (
	requestsProtobufTotal   = metrics.NewCounter(`vl_http_requests_total{path="/insert/loki/api/v1/push",format="protobuf"}`)
	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/loki/api/v1/push",format="protobuf"}`)
)

func parseProtobufRequest(data []byte, lmp insertutils.LogMessageProcessor, useDefaultStreamFields bool) error {
	bb := bytesBufPool.Get()
	defer bytesBufPool.Put(bb)

	buf, err := snappy.Decode(bb.B[:cap(bb.B)], data)
	if err != nil {
		return fmt.Errorf("cannot decode snappy-encoded request body: %w", err)
	}
	bb.B = buf

	req := getPushRequest()
	defer putPushRequest(req)

	err = req.UnmarshalProtobuf(bb.B)
	if err != nil {
		return fmt.Errorf("cannot parse request body: %w", err)
	}

	fields := getFields()
	defer putFields(fields)

	streams := req.Streams
	currentTimestamp := time.Now().UnixNano()
	for i := range streams {
		stream := &streams[i]
		// st.Labels contains labels for the stream.
		// Labels are same for all entries in the stream.
		fields.fields, err = parsePromLabels(fields.fields[:0], stream.Labels)
		if err != nil {
			return fmt.Errorf("cannot parse stream labels %q: %w", stream.Labels, err)
		}
		commonFieldsLen := len(fields.fields)

		entries := stream.Entries
		for j := range entries {
			e := &entries[j]
			fields.fields = fields.fields[:commonFieldsLen]

			for _, lp := range e.StructuredMetadata {
				fields.fields = append(fields.fields, logstorage.Field{
					Name:  lp.Name,
					Value: lp.Value,
				})
			}

			fields.fields = append(fields.fields, logstorage.Field{
				Name:  "_msg",
				Value: e.Line,
			})

			ts := e.Timestamp.UnixNano()
			if ts == 0 {
				ts = currentTimestamp
			}

			var streamFields []logstorage.Field
			if useDefaultStreamFields {
				streamFields = fields.fields[:commonFieldsLen]
			}
			lmp.AddRow(ts, fields.fields, streamFields)
		}
	}
	return nil
}

func getFields() *fields {
	v := fieldsPool.Get()
	if v == nil {
		return &fields{}
	}
	return v.(*fields)
}

func putFields(f *fields) {
	f.fields = f.fields[:0]
	fieldsPool.Put(f)
}

var fieldsPool sync.Pool

type fields struct {
	fields []logstorage.Field
}

// parsePromLabels parses log fields in Prometheus text exposition format from s, appends them to dst and returns the result.
//
// See test data of promtail for examples: https://github.com/grafana/loki/blob/a24ef7b206e0ca63ee74ca6ecb0a09b745cd2258/pkg/push/types_test.go
func parsePromLabels(dst []logstorage.Field, s string) ([]logstorage.Field, error) {
	// Make sure s is wrapped into `{...}`
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return nil, fmt.Errorf("too short string to parse: %q", s)
	}
	if s[0] != '{' {
		return nil, fmt.Errorf("missing `{` at the beginning of %q", s)
	}
	if s[len(s)-1] != '}' {
		return nil, fmt.Errorf("missing `}` at the end of %q", s)
	}
	s = s[1 : len(s)-1]

	for len(s) > 0 {
		// Parse label name
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return nil, fmt.Errorf("cannot find `=` char for label value at %s", s)
		}
		name := s[:n]
		s = s[n+1:]

		// Parse label value
		qs, err := strconv.QuotedPrefix(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse value for label %q at %s: %w", name, s, err)
		}
		s = s[len(qs):]
		value, err := strconv.Unquote(qs)
		if err != nil {
			return nil, fmt.Errorf("cannot unquote value %q for label %q: %w", qs, name, err)
		}

		// Append the found field to dst.
		dst = append(dst, logstorage.Field{
			Name:  name,
			Value: value,
		})

		// Check whether there are other labels remaining
		if len(s) == 0 {
			break
		}
		if !strings.HasPrefix(s, ",") {
			return nil, fmt.Errorf("missing `,` char at %s", s)
		}
		s = s[1:]
		s = strings.TrimPrefix(s, " ")
	}
	return dst, nil
}

func getPushRequest() *PushRequest {
	v := pushReqsPool.Get()
	if v == nil {
		return &PushRequest{}
	}
	return v.(*PushRequest)
}

func putPushRequest(req *PushRequest) {
	req.reset()
	pushReqsPool.Put(req)
}
