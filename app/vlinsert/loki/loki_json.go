package loki

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var parserPool fastjson.ParserPool

func handleJSON(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsJSONTotal.Inc()
	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			httpserver.Errorf(w, r, "cannot initialize gzip reader: %s", err)
			return
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	wcr := writeconcurrencylimiter.GetReader(reader)
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
	lmp := cp.NewLogMessageProcessor("loki_json")
	n, err := parseJSONRequest(data, lmp)
	lmp.MustClose()
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse Loki json request: %s; data=%s", err, data)
		return
	}

	rowsIngestedJSONTotal.Add(n)

	// update requestJSONDuration only for successfully parsed requests
	// There is no need in updating requestJSONDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestJSONDuration.UpdateDuration(startTime)
}

var (
	requestsJSONTotal     = metrics.NewCounter(`vl_http_requests_total{path="/insert/loki/api/v1/push",format="json"}`)
	rowsIngestedJSONTotal = metrics.NewCounter(`vl_rows_ingested_total{type="loki_json"}`)
	requestJSONDuration   = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/loki/api/v1/push",format="json"}`)
)

func parseJSONRequest(data []byte, lmp insertutils.LogMessageProcessor) (int, error) {
	p := parserPool.Get()
	defer parserPool.Put(p)
	v, err := p.ParseBytes(data)
	if err != nil {
		return 0, fmt.Errorf("cannot parse JSON request body: %w", err)
	}

	streamsV := v.Get("streams")
	if streamsV == nil {
		return 0, fmt.Errorf("missing `streams` item in the parsed JSON")
	}
	streams, err := streamsV.Array()
	if err != nil {
		return 0, fmt.Errorf("`streams` item in the parsed JSON must contain an array; got %q", streamsV)
	}

	currentTimestamp := time.Now().UnixNano()
	var commonFields []logstorage.Field
	rowsIngested := 0
	for _, stream := range streams {
		// populate common labels from `stream` dict
		commonFields = commonFields[:0]
		labelsV := stream.Get("stream")
		var labels *fastjson.Object
		if labelsV != nil {
			o, err := labelsV.Object()
			if err != nil {
				return rowsIngested, fmt.Errorf("`stream` item in the parsed JSON must contain an object; got %q", labelsV)
			}
			labels = o
		}
		labels.Visit(func(k []byte, v *fastjson.Value) {
			vStr, errLocal := v.StringBytes()
			if errLocal != nil {
				err = fmt.Errorf("unexpected label value type for %q:%q; want string", k, v)
				return
			}
			commonFields = append(commonFields, logstorage.Field{
				Name:  bytesutil.ToUnsafeString(k),
				Value: bytesutil.ToUnsafeString(vStr),
			})
		})
		if err != nil {
			return rowsIngested, fmt.Errorf("error when parsing `stream` object: %w", err)
		}

		// populate messages from `values` array
		linesV := stream.Get("values")
		if linesV == nil {
			return rowsIngested, fmt.Errorf("missing `values` item in the parsed `stream` object %q", stream)
		}
		lines, err := linesV.Array()
		if err != nil {
			return rowsIngested, fmt.Errorf("`values` item in the parsed JSON must contain an array; got %q", linesV)
		}

		fields := commonFields
		for _, line := range lines {
			lineA, err := line.Array()
			if err != nil {
				return rowsIngested, fmt.Errorf("unexpected contents of `values` item; want array; got %q", line)
			}
			if len(lineA) < 2 || len(lineA) > 3 {
				return rowsIngested, fmt.Errorf("unexpected number of values in `values` item array %q; got %d want 2 or 3", line, len(lineA))
			}

			// parse timestamp
			timestamp, err := lineA[0].StringBytes()
			if err != nil {
				return rowsIngested, fmt.Errorf("unexpected log timestamp type for %q; want string", lineA[0])
			}
			ts, err := parseLokiTimestamp(bytesutil.ToUnsafeString(timestamp))
			if err != nil {
				return rowsIngested, fmt.Errorf("cannot parse log timestamp %q: %w", timestamp, err)
			}
			if ts == 0 {
				ts = currentTimestamp
			}

			// parse log message
			msg, err := lineA[1].StringBytes()
			if err != nil {
				return rowsIngested, fmt.Errorf("unexpected log message type for %q; want string", lineA[1])
			}

			fields = append(fields[:len(commonFields)], logstorage.Field{
				Name:  "_msg",
				Value: bytesutil.ToUnsafeString(msg),
			})

			// parse structured metadata - see https://grafana.com/docs/loki/latest/reference/loki-http-api/#ingest-logs
			if len(lineA) > 2 {
				structuredMetadata, err := lineA[2].Object()
				if err != nil {
					return rowsIngested, fmt.Errorf("unexpected structured metadata type for %q; want JSON object", lineA[2])
				}

				structuredMetadata.Visit(func(k []byte, v *fastjson.Value) {
					vStr, errLocal := v.StringBytes()
					if errLocal != nil {
						err = fmt.Errorf("unexpected label value type for %q:%q; want string", k, v)
						return
					}

					fields = append(fields, logstorage.Field{
						Name:  bytesutil.ToUnsafeString(k),
						Value: bytesutil.ToUnsafeString(vStr),
					})
				})
				if err != nil {
					return rowsIngested, fmt.Errorf("error when parsing `structuredMetadata` object: %w", err)
				}
			}
			lmp.AddRow(ts, fields)
		}
		rowsIngested += len(lines)
	}

	return rowsIngested, nil
}

func parseLokiTimestamp(s string) (int64, error) {
	if s == "" {
		// Special case - an empty timestamp must be substituted with the current time by the caller.
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// Fall back to parsing floating-point value
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		if f > math.MaxInt64 {
			return 0, fmt.Errorf("too big timestamp in nanoseconds: %v; mustn't exceed %v", f, int64(math.MaxInt64))
		}
		if f < math.MinInt64 {
			return 0, fmt.Errorf("too small timestamp in nanoseconds: %v; must be bigger or equal to %v", f, int64(math.MinInt64))
		}
		n = int64(f)
	}
	if n < 0 {
		return 0, fmt.Errorf("too small timestamp in nanoseconds: %d; must be bigger than 0", n)
	}
	return n, nil
}
