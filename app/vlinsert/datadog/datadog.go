package datadog

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var parserPool fastjson.ParserPool

// RequestHandler processes Datadog insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/api/v1/validate":
		fmt.Fprintf(w, `{}`)
		return true
	case "/api/v2/logs":
		return datadogLogsIngestion(w, r)
	default:
		return false
	}
}

func datadogLogsIngestion(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Add("Content-Type", "application/json")
	startTime := time.Now()
	v2LogsRequestsTotal.Inc()
	reader := r.Body

	var ts int64
	if tsValue := r.Header.Get("dd-message-timestamp"); tsValue != "" && tsValue != "0" {
		var err error
		ts, err = strconv.ParseInt(tsValue, 10, 64)
		if err != nil {
			httpserver.Errorf(w, r, "could not parse dd-message-timestamp header value: %s", err)
			return true
		}
		ts *= 1e6
	} else {
		ts = startTime.UnixNano()
	}

	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			httpserver.Errorf(w, r, "cannot read gzipped logs request: %s", err)
			return true
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	wcr := writeconcurrencylimiter.GetReader(reader)
	data, err := io.ReadAll(wcr)
	writeconcurrencylimiter.PutReader(wcr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot read request body: %s", err)
		return true
	}

	cp, err := insertutils.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return true
	}

	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return true
	}

	lmp := cp.NewLogMessageProcessor("datadog")
	err = readLogsRequest(ts, data, lmp)
	lmp.MustClose()
	if err != nil {
		logger.Warnf("cannot decode log message in /api/v2/logs request: %s, stream fields: %s", err, cp.StreamFields)
		return true
	}

	// update v2LogsRequestDuration only for successfully parsed requests
	// There is no need in updating v2LogsRequestDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	v2LogsRequestDuration.UpdateDuration(startTime)
	fmt.Fprintf(w, `{}`)
	return true
}

var (
	v2LogsRequestsTotal   = metrics.NewCounter(`vl_http_requests_total{path="/insert/datadog/api/v2/logs"}`)
	v2LogsRequestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/datadog/api/v2/logs"}`)
)

// readLogsRequest parses data according to DataDog logs format
// https://docs.datadoghq.com/api/latest/logs/#send-logs
func readLogsRequest(ts int64, data []byte, lmp insertutils.LogMessageProcessor) error {
	p := parserPool.Get()
	defer parserPool.Put(p)
	v, err := p.ParseBytes(data)
	if err != nil {
		return fmt.Errorf("cannot parse JSON request body: %w", err)
	}
	records, err := v.Array()
	if err != nil {
		return fmt.Errorf("cannot extract array from parsed JSON: %w", err)
	}

	var fields []logstorage.Field
	for _, r := range records {
		o, err := r.Object()
		if err != nil {
			return fmt.Errorf("could not extract log record: %w", err)
		}
		o.Visit(func(k []byte, v *fastjson.Value) {
			if err != nil {
				return
			}
			val, e := v.StringBytes()
			if e != nil {
				err = fmt.Errorf("unexpected label value type for %q:%q; want string", k, v)
				return
			}
			switch string(k) {
			case "message":
				fields = append(fields, logstorage.Field{
					Name:  "_msg",
					Value: bytesutil.ToUnsafeString(val),
				})
			case "ddtags":
				// https://docs.datadoghq.com/getting_started/tagging/
				var pair []byte
				idx := 0
				for idx >= 0 {
					idx = bytes.IndexByte(val, ',')
					if idx < 0 {
						pair = val
					} else {
						pair = val[:idx]
						val = val[idx+1:]
					}
					if len(pair) > 0 {
						n := bytes.IndexByte(pair, ':')
						if n < 0 {
							// No tag value.
							fields = append(fields, logstorage.Field{
								Name:  bytesutil.ToUnsafeString(pair),
								Value: "no_label_value",
							})
						}
						fields = append(fields, logstorage.Field{
							Name:  bytesutil.ToUnsafeString(pair[:n]),
							Value: bytesutil.ToUnsafeString(pair[n+1:]),
						})
					}
				}
			default:
				fields = append(fields, logstorage.Field{
					Name:  bytesutil.ToUnsafeString(k),
					Value: bytesutil.ToUnsafeString(val),
				})
			}
		})
		lmp.AddRow(ts, fields, nil)
		fields = fields[:0]
	}
	return nil
}
