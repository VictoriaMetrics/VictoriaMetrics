package elasticsearch

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	common "github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	pc "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"
)

// RequestHandler processes ElasticSearch insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	w.Header().Add("Content-Type", "application/json")
	// This header is needed for Logstash
	w.Header().Set("X-Elastic-Product", "Elasticsearch")

	if strings.HasPrefix(path, "/_ilm/policy") {
		// Return fake response for ElasticSearch ilm request.
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_index_template") {
		// Return fake response for ElasticSearch index template request.
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_ingest") {
		// Return fake response for ElasticSearch ingest pipeline request.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/put-pipeline-api.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_nodes") {
		// Return fake response for ElasticSearch nodes discovery request.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/cluster.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	switch path {
	case "/":
		switch r.Method {
		case http.MethodGet:
			// Return fake response for ElasticSearch ping request.
			// See the latest available version for ElasticSearch at https://github.com/elastic/elasticsearch/releases
			fmt.Fprintf(w, `{
			"version": {
				"number": "8.8.0"
			}
		}`)
		case http.MethodHead:
			// Return empty response for Logstash ping request.
		}

		return true
	case "/_license":
		// Return fake response for ElasticSearch license request.
		fmt.Fprintf(w, `{
			"license": {
				"uid": "cbff45e7-c553-41f7-ae4f-9205eabd80xx",
				"type": "oss",
				"status": "active",
				"expiry_date_in_millis" : 4000000000000
			}
		}`)
		return true
	case "/_bulk":
		startTime := time.Now()
		bulkRequestsTotal.Inc()

		// Extract tenantID
		tenantID, err := logstorage.GetTenantIDFromRequest(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}

		// Extract time field name from _time_field query arg
		var timeField = "_time"
		if tf := r.FormValue("_time_field"); tf != "" {
			timeField = tf
		}

		// Extract message field name from _msg_field query arg
		var msgField = ""
		if msgf := r.FormValue("_msg_field"); msgf != "" {
			msgField = msgf
		}

		// Extract stream field names from _stream_fields query arg
		var streamFields []string
		if sfs := r.FormValue("_stream_fields"); sfs != "" {
			streamFields = strings.Split(sfs, ",")
		}

		// Extract field names, which must be ignored
		var ignoreFields []string
		if ifs := r.FormValue("ignore_fields"); ifs != "" {
			ignoreFields = strings.Split(ifs, ",")
		}

		lr := logstorage.GetLogRows(streamFields, ignoreFields)
		processLogMessage := func(timestamp int64, fields []logstorage.Field) {
			lr.MustAdd(tenantID, timestamp, fields)
			if lr.NeedFlush() {
				vlstorage.MustAddRows(lr)
				lr.Reset()
			}
		}

		isGzip := r.Header.Get("Content-Encoding") == "gzip"
		n, err := readBulkRequest(r.Body, isGzip, timeField, msgField, processLogMessage)
		if err != nil {
			logger.Warnf("cannot decode log message #%d in /_bulk request: %s", n, err)
			return true
		}
		vlstorage.MustAddRows(lr)
		logstorage.PutLogRows(lr)

		tookMs := time.Since(startTime).Milliseconds()
		bw := bufferedwriter.Get(w)
		defer bufferedwriter.Put(bw)
		WriteBulkResponse(bw, n, tookMs)
		_ = bw.Flush()
		return true
	default:
		return false
	}
}

var bulkRequestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/elasticsearch/_bulk"}`)

func readBulkRequest(r io.Reader, isGzip bool, timeField, msgField string,
	processLogMessage func(timestamp int64, fields []logstorage.Field),
) (int, error) {
	// See https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html

	if isGzip {
		zr, err := pc.GetGzipReader(r)
		if err != nil {
			return 0, fmt.Errorf("cannot read gzipped _bulk request: %w", err)
		}
		defer pc.PutGzipReader(zr)
		r = zr
	}

	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lb := lineBufferPool.Get()
	defer lineBufferPool.Put(lb)

	lb.B = bytesutil.ResizeNoCopyNoOverallocate(lb.B, common.MaxLineSizeBytes.IntN())
	sc := bufio.NewScanner(wcr)
	sc.Buffer(lb.B, len(lb.B))

	n := 0
	nCheckpoint := 0
	for {
		ok, err := readBulkLine(sc, timeField, msgField, processLogMessage)
		wcr.DecConcurrency()
		if err != nil || !ok {
			rowsIngestedTotal.Add(n - nCheckpoint)
			return n, err
		}
		n++
		if batchSize := n - nCheckpoint; n >= 1000 {
			rowsIngestedTotal.Add(batchSize)
			nCheckpoint = n
		}
	}
}

var lineBufferPool bytesutil.ByteBufferPool

var rowsIngestedTotal = metrics.NewCounter(`vl_rows_ingested_total{type="elasticsearch_bulk"}`)

func readBulkLine(sc *bufio.Scanner, timeField, msgField string,
	processLogMessage func(timestamp int64, fields []logstorage.Field),
) (bool, error) {
	// Decode command, must be "create" or "index"
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return false, fmt.Errorf(`cannot read "create" or "index" command, since its size exceeds -insert.maxLineSizeBytes=%d`, common.MaxLineSizeBytes.IntN())
			}
			return false, err
		}
		return false, nil
	}
	line := sc.Bytes()
	p := parserPool.Get()
	v, err := p.ParseBytes(line)
	if err != nil {
		return false, fmt.Errorf(`cannot parse "create" or "index" command: %w`, err)
	}
	if v.GetObject("create") == nil && v.GetObject("index") == nil {
		return false, fmt.Errorf(`unexpected command %q; expected "create" or "index"`, v)
	}
	parserPool.Put(p)

	// Decode log message
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return false, fmt.Errorf("cannot read log message, since its size exceeds -insert.maxLineSizeBytes=%d", common.MaxLineSizeBytes.IntN())
			}
			return false, err
		}
		return false, fmt.Errorf(`missing log message after the "create" or "index" command`)
	}
	line = sc.Bytes()
	pctx := common.GetParserCtx()
	if err := pctx.ParseLogMessage(line); err != nil {
		invalidJSONLineLogger.Warnf("cannot parse json-encoded log entry: %s", err)
		return true, nil
	}

	timestamp, err := extractTimestampFromFields(timeField, pctx.Fields())
	if err != nil {
		invalidTimestampLogger.Warnf("skipping the log entry because cannot parse timestamp: %s", err)
		return true, nil
	}
	pctx.RenameField(msgField, "_msg")
	processLogMessage(timestamp, pctx.Fields())
	common.PutParserCtx(pctx)
	return true, nil
}

var parserPool fastjson.ParserPool

var (
	invalidTimestampLogger = logger.WithThrottler("invalidTimestampLogger", 5*time.Second)
	invalidJSONLineLogger  = logger.WithThrottler("invalidJSONLineLogger", 5*time.Second)
)

func extractTimestampFromFields(timeField string, fields []logstorage.Field) (int64, error) {
	for i := range fields {
		f := &fields[i]
		if f.Name != timeField {
			continue
		}
		timestamp, err := parseElasticsearchTimestamp(f.Value)
		if err != nil {
			return 0, err
		}
		f.Value = ""
		return timestamp, nil
	}
	return time.Now().UnixNano(), nil
}

func parseElasticsearchTimestamp(s string) (int64, error) {
	if len(s) < len("YYYY-MM-DD") || s[len("YYYY")] != '-' {
		// Try parsing timestamp in milliseconds
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse timestamp in milliseconds from %q: %w", s, err)
		}
		if n > int64(math.MaxInt64)/1e6 {
			return 0, fmt.Errorf("too big timestamp in milliseconds: %d; mustn't exceed %d", n, int64(math.MaxInt64)/1e6)
		}
		if n < int64(math.MinInt64)/1e6 {
			return 0, fmt.Errorf("too small timestamp in milliseconds: %d; must be bigger than %d", n, int64(math.MinInt64)/1e6)
		}
		n *= 1e6
		return n, nil
	}
	if len(s) == len("YYYY-MM-DD") {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return 0, fmt.Errorf("cannot parse date %q: %w", s, err)
		}
		return t.UnixNano(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("cannot parse timestamp %q: %w", s, err)
	}
	return t.UnixNano(), nil
}
