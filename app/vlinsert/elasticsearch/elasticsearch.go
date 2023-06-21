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
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"
)

var (
	maxLineSizeBytes = flagutil.NewBytes("insert.maxLineSizeBytes", 256*1024, "The maximum size of a single line, which can be read by /insert/* handlers")
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

		streamFields := httputils.GetArray(r, "_stream_fields")
		ignoreFields := httputils.GetArray(r, "ignore_fields")

		isDebug := httputils.GetBool(r, "debug")
		debugRequestURI := ""
		debugRemoteAddr := ""
		if isDebug {
			debugRequestURI = httpserver.GetRequestURI(r)
			debugRemoteAddr = httpserver.GetQuotedRemoteAddr(r)
		}

		lr := logstorage.GetLogRows(streamFields, ignoreFields)
		processLogMessage := func(timestamp int64, fields []logstorage.Field) {
			lr.MustAdd(tenantID, timestamp, fields)
			if isDebug {
				s := lr.GetRowString(0)
				lr.ResetKeepSettings()
				logger.Infof("remoteAddr=%s; requestURI=%s; ignoring log entry because of `debug` query arg: %s", debugRemoteAddr, debugRequestURI, s)
				rowsDroppedTotal.Inc()
				return
			}
			if lr.NeedFlush() {
				vlstorage.MustAddRows(lr)
				lr.ResetKeepSettings()
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

var (
	bulkRequestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/elasticsearch/_bulk"}`)
	rowsDroppedTotal  = metrics.NewCounter(`vl_rows_dropped_total{path="/insert/elasticsearch/_bulk",reason="debug"}`)
)

func readBulkRequest(r io.Reader, isGzip bool, timeField, msgField string,
	processLogMessage func(timestamp int64, fields []logstorage.Field),
) (int, error) {
	// See https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html

	if isGzip {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return 0, fmt.Errorf("cannot read gzipped _bulk request: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lb := lineBufferPool.Get()
	defer lineBufferPool.Put(lb)

	lb.B = bytesutil.ResizeNoCopyNoOverallocate(lb.B, maxLineSizeBytes.IntN())
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
				return false, fmt.Errorf(`cannot read "create" or "index" command, since its size exceeds -insert.maxLineSizeBytes=%d`, maxLineSizeBytes.IntN())
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
				return false, fmt.Errorf("cannot read log message, since its size exceeds -insert.maxLineSizeBytes=%d", maxLineSizeBytes.IntN())
			}
			return false, err
		}
		return false, fmt.Errorf(`missing log message after the "create" or "index" command`)
	}
	line = sc.Bytes()
	pctx := getParserCtx()
	if err := pctx.parseLogMessage(line); err != nil {
		invalidJSONLineLogger.Warnf("cannot parse json-encoded log entry: %s", err)
		return true, nil
	}

	timestamp, err := extractTimestampFromFields(timeField, pctx.fields)
	if err != nil {
		invalidTimestampLogger.Warnf("skipping the log entry because cannot parse timestamp: %s", err)
		return true, nil
	}
	updateMessageFieldName(msgField, pctx.fields)
	processLogMessage(timestamp, pctx.fields)
	putParserCtx(pctx)
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

func updateMessageFieldName(msgField string, fields []logstorage.Field) {
	if msgField == "" {
		return
	}
	for i := range fields {
		f := &fields[i]
		if f.Name == msgField {
			f.Name = "_msg"
			return
		}
	}
}

type parserCtx struct {
	p         fastjson.Parser
	buf       []byte
	prefixBuf []byte
	fields    []logstorage.Field
}

func (pctx *parserCtx) reset() {
	pctx.buf = pctx.buf[:0]
	pctx.prefixBuf = pctx.prefixBuf[:0]

	fields := pctx.fields
	for i := range fields {
		lf := &fields[i]
		lf.Name = ""
		lf.Value = ""
	}
	pctx.fields = fields[:0]
}

func getParserCtx() *parserCtx {
	v := parserCtxPool.Get()
	if v == nil {
		return &parserCtx{}
	}
	return v.(*parserCtx)
}

func putParserCtx(pctx *parserCtx) {
	pctx.reset()
	parserCtxPool.Put(pctx)
}

var parserCtxPool sync.Pool

func (pctx *parserCtx) parseLogMessage(msg []byte) error {
	s := bytesutil.ToUnsafeString(msg)
	v, err := pctx.p.Parse(s)
	if err != nil {
		return fmt.Errorf("cannot parse json: %w", err)
	}
	if t := v.Type(); t != fastjson.TypeObject {
		return fmt.Errorf("expecting json dictionary; got %s", t)
	}
	pctx.reset()
	pctx.fields, pctx.buf, pctx.prefixBuf = appendLogFields(pctx.fields, pctx.buf, pctx.prefixBuf, v)
	return nil
}

func appendLogFields(dst []logstorage.Field, dstBuf, prefixBuf []byte, v *fastjson.Value) ([]logstorage.Field, []byte, []byte) {
	o := v.GetObject()
	o.Visit(func(k []byte, v *fastjson.Value) {
		t := v.Type()
		switch t {
		case fastjson.TypeNull:
			// Skip nulls
		case fastjson.TypeObject:
			// Flatten nested JSON objects.
			// For example, {"foo":{"bar":"baz"}} is converted to {"foo.bar":"baz"}
			prefixLen := len(prefixBuf)
			prefixBuf = append(prefixBuf, k...)
			prefixBuf = append(prefixBuf, '.')
			dst, dstBuf, prefixBuf = appendLogFields(dst, dstBuf, prefixBuf, v)
			prefixBuf = prefixBuf[:prefixLen]
		case fastjson.TypeArray, fastjson.TypeNumber, fastjson.TypeTrue, fastjson.TypeFalse:
			// Convert JSON arrays, numbers, true and false values to their string representation
			dstBufLen := len(dstBuf)
			dstBuf = v.MarshalTo(dstBuf)
			value := dstBuf[dstBufLen:]
			dst, dstBuf = appendLogField(dst, dstBuf, prefixBuf, k, value)
		case fastjson.TypeString:
			// Decode JSON strings
			dstBufLen := len(dstBuf)
			dstBuf = append(dstBuf, v.GetStringBytes()...)
			value := dstBuf[dstBufLen:]
			dst, dstBuf = appendLogField(dst, dstBuf, prefixBuf, k, value)
		default:
			logger.Panicf("BUG: unexpected JSON type: %s", t)
		}
	})
	return dst, dstBuf, prefixBuf
}

func appendLogField(dst []logstorage.Field, dstBuf, prefixBuf, k, value []byte) ([]logstorage.Field, []byte) {
	dstBufLen := len(dstBuf)
	dstBuf = append(dstBuf, prefixBuf...)
	dstBuf = append(dstBuf, k...)
	name := dstBuf[dstBufLen:]

	dst = append(dst, logstorage.Field{
		Name:  bytesutil.ToUnsafeString(name),
		Value: bytesutil.ToUnsafeString(value),
	})
	return dst, dstBuf
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
