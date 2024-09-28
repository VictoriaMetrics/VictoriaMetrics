package elasticsearch

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var (
	elasticsearchVersion = flag.String("elasticsearch.version", "8.9.0", "Elasticsearch version to report to client")
)

// RequestHandler processes Elasticsearch insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	w.Header().Add("Content-Type", "application/json")
	// This header is needed for Logstash
	w.Header().Set("X-Elastic-Product", "Elasticsearch")

	if strings.HasPrefix(path, "/_ilm/policy") {
		// Return fake response for Elasticsearch ilm request.
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_index_template") {
		// Return fake response for Elasticsearch index template request.
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_ingest") {
		// Return fake response for Elasticsearch ingest pipeline request.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/put-pipeline-api.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_nodes") {
		// Return fake response for Elasticsearch nodes discovery request.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/cluster.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/logstash") || strings.HasPrefix(path, "/_logstash") {
		// Return fake response for Logstash APIs requests.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/logstash-apis.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	switch path {
	case "/":
		switch r.Method {
		case http.MethodGet:
			// Return fake response for Elasticsearch ping request.
			// See the latest available version for Elasticsearch at https://github.com/elastic/elasticsearch/releases
			fmt.Fprintf(w, `{
			"version": {
				"number": %q
			}
		}`, *elasticsearchVersion)
		case http.MethodHead:
			// Return empty response for Logstash ping request.
		}

		return true
	case "/_license":
		// Return fake response for Elasticsearch license request.
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

		cp, err := insertutils.GetCommonParams(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		if err := vlstorage.CanWriteData(); err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		lmp := cp.NewLogMessageProcessor()
		isGzip := r.Header.Get("Content-Encoding") == "gzip"
		n, err := readBulkRequest(r.Body, isGzip, cp.TimeField, cp.MsgField, lmp)
		lmp.MustClose()
		if err != nil {
			logger.Warnf("cannot decode log message #%d in /_bulk request: %s, stream fields: %s", n, err, cp.StreamFields)
			return true
		}

		tookMs := time.Since(startTime).Milliseconds()
		bw := bufferedwriter.Get(w)
		defer bufferedwriter.Put(bw)
		WriteBulkResponse(bw, n, tookMs)
		_ = bw.Flush()

		// update bulkRequestDuration only for successfully parsed requests
		// There is no need in updating bulkRequestDuration for request errors,
		// since their timings are usually much smaller than the timing for successful request parsing.
		bulkRequestDuration.UpdateDuration(startTime)

		return true
	default:
		return false
	}
}

var (
	bulkRequestsTotal   = metrics.NewCounter(`vl_http_requests_total{path="/insert/elasticsearch/_bulk"}`)
	rowsIngestedTotal   = metrics.NewCounter(`vl_rows_ingested_total{type="elasticsearch_bulk"}`)
	bulkRequestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/elasticsearch/_bulk"}`)
)

func readBulkRequest(r io.Reader, isGzip bool, timeField, msgField string, lmp insertutils.LogMessageProcessor) (int, error) {
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

	lb.B = bytesutil.ResizeNoCopyNoOverallocate(lb.B, insertutils.MaxLineSizeBytes.IntN())
	sc := bufio.NewScanner(wcr)
	sc.Buffer(lb.B, len(lb.B))

	n := 0
	nCheckpoint := 0
	for {
		ok, err := readBulkLine(sc, timeField, msgField, lmp)
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

func readBulkLine(sc *bufio.Scanner, timeField, msgField string, lmp insertutils.LogMessageProcessor) (bool, error) {
	var line []byte

	// Read the command, must be "create" or "index"
	for len(line) == 0 {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				if errors.Is(err, bufio.ErrTooLong) {
					return false, fmt.Errorf(`cannot read "create" or "index" command, since its size exceeds -insert.maxLineSizeBytes=%d`,
						insertutils.MaxLineSizeBytes.IntN())
				}
				return false, err
			}
			return false, nil
		}
		line = sc.Bytes()
	}
	lineStr := bytesutil.ToUnsafeString(line)
	if !strings.Contains(lineStr, `"create"`) && !strings.Contains(lineStr, `"index"`) {
		return false, fmt.Errorf(`unexpected command %q; expecting "create" or "index"`, line)
	}

	// Decode log message
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return false, fmt.Errorf("cannot read log message, since its size exceeds -insert.maxLineSizeBytes=%d", insertutils.MaxLineSizeBytes.IntN())
			}
			return false, err
		}
		return false, fmt.Errorf(`missing log message after the "create" or "index" command`)
	}
	line = sc.Bytes()
	p := logstorage.GetJSONParser()
	if err := p.ParseLogMessage(line); err != nil {
		return false, fmt.Errorf("cannot parse json-encoded log entry: %w", err)
	}

	ts, err := extractTimestampFromFields(timeField, p.Fields)
	if err != nil {
		return false, fmt.Errorf("cannot parse timestamp: %w", err)
	}
	if ts == 0 {
		ts = time.Now().UnixNano()
	}
	logstorage.RenameField(p.Fields, msgField, "_msg")
	lmp.AddRow(ts, p.Fields)
	logstorage.PutJSONParser(p)

	return true, nil
}

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
	return 0, nil
}

func parseElasticsearchTimestamp(s string) (int64, error) {
	if s == "0" || s == "" {
		// Special case - zero or empty timestamp must be substituted
		// with the current time by the caller.
		return 0, nil
	}
	if len(s) < len("YYYY-MM-DD") || s[len("YYYY")] != '-' {
		// Try parsing timestamp in seconds or milliseconds
		return insertutils.ParseUnixTimestamp(s)
	}
	if len(s) == len("YYYY-MM-DD") {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return 0, fmt.Errorf("cannot parse date %q: %w", s, err)
		}
		return t.UnixNano(), nil
	}
	nsecs, ok := logstorage.TryParseTimestampRFC3339Nano(s)
	if !ok {
		return 0, fmt.Errorf("cannot parse timestamp %q", s)
	}
	return nsecs, nil
}
