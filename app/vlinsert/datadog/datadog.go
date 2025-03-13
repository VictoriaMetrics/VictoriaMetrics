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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var (
	datadogStreamFields = flagutil.NewArrayString("datadog.streamFields", "Datadog tags to be used as stream fields.")
	datadogIgnoreFields = flagutil.NewArrayString("datadog.ignoreFields", "Datadog tags to ignore.")
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

	encoding := r.Header.Get("Content-Encoding")
	reader, err := common.GetUncompressedReader(r.Body, encoding)
	if err != nil {
		httpserver.Errorf(w, r, "cannot read %s-compressed DataDog protocol data: %s", encoding, err)
		return true
	}
	defer common.PutUncompressedReader(reader, encoding)

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

	if len(cp.StreamFields) == 0 {
		cp.StreamFields = *datadogStreamFields
	}
	if len(cp.IgnoreFields) == 0 {
		cp.IgnoreFields = *datadogIgnoreFields
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

// datadog message field has two formats:
//   - regular log message with string text
//   - nested json format for serverless plugins
//     which has folowing format:
//     {"message": {"message": "text","lamdba": {"arn": "string","requestID": "string"}, "timestamp": int64} }
//
// See https://github.com/DataDog/datadog-lambda-extension/blob/28b90c7e4e985b72d60b5f5a5147c69c7ac693c4/bottlecap/src/logs/lambda/mod.rs#L24
func appendMsgFields(fields []logstorage.Field, v *fastjson.Value) ([]logstorage.Field, error) {
	switch v.Type() {
	case fastjson.TypeString:
		val := v.GetStringBytes()
		fields = append(fields, logstorage.Field{
			Name:  "_msg",
			Value: bytesutil.ToUnsafeString(val),
		})
	case fastjson.TypeObject:
		var firstErr error
		v.GetObject().Visit(func(k []byte, v *fastjson.Value) {
			if firstErr != nil {
				return
			}
			switch bytesutil.ToUnsafeString(k) {
			case "message":
				val := v.GetStringBytes()
				fields = append(fields, logstorage.Field{
					Name:  "_msg",
					Value: bytesutil.ToUnsafeString(val),
				})
			case "status":
				val := v.GetStringBytes()
				fields = append(fields, logstorage.Field{
					Name:  "status",
					Value: bytesutil.ToUnsafeString(val),
				})
			case "lamdba":
				obj, err := v.Object()
				if err != nil {
					firstErr = err
					firstErr = fmt.Errorf("unexpected lambda value type for %q:%q; want object", k, v)
					return
				}
				obj.Visit(func(k []byte, v *fastjson.Value) {
					if firstErr != nil {
						return
					}
					val, err := v.StringBytes()
					if err != nil {
						firstErr = fmt.Errorf("unexpected lambda label value type for %q:%q; want string", k, v)
						return
					}
					fields = append(fields, logstorage.Field{
						Name:  bytesutil.ToUnsafeString(k),
						Value: bytesutil.ToUnsafeString(val),
					})
				})

			}
		})
	default:
		return fields, fmt.Errorf("unsupported message type %q", v.Type().String())
	}
	return fields, nil
}

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
			switch bytesutil.ToUnsafeString(k) {
			case "message":
				fields, err = appendMsgFields(fields, v)
				if err != nil {
					return
				}
			case "timestamp":
				val, e := v.Int64()
				if e != nil {
					err = fmt.Errorf("failed to parse timestamp for %q:%q", k, v)
				}
				if val > 0 {
					ts = val * 1e6
				}
			case "ddtags":
				// https://docs.datadoghq.com/getting_started/tagging/
				val, e := v.StringBytes()
				if e != nil {
					err = fmt.Errorf("unexpected label value type for %q:%q; want string", k, v)
					return
				}
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
				val, e := v.StringBytes()
				if e != nil {
					err = fmt.Errorf("unexpected label value type for %q:%q; want string", k, v)
					return
				}
				fields = append(fields, logstorage.Field{
					Name:  bytesutil.ToUnsafeString(k),
					Value: bytesutil.ToUnsafeString(val),
				})
			}
		})
		if err != nil {
			return err
		}
		lmp.AddRow(ts, fields, nil)
		fields = fields[:0]
	}
	return nil
}
